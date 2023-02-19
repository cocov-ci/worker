package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/mocks"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/test_helpers"
)

type allMocks struct {
	docker    *mocks.DockerMock
	api       *mocks.APIMock
	storage   *mocks.StorageMock
	redis     *mocks.RedisMock
	scheduler *mocks.IndirectScheduler
	isDone    *atomic.Bool
}

func makeOutputFile(t *testing.T) string {
	return "/tmp/test"
}

func makeMocks(t *testing.T) (*allMocks, *worker) {
	ctrl := gomock.NewController(t)
	zap.ReplaceGlobals(zap.NewNop())
	dockerMock := mocks.NewDockerMock(ctrl)
	api := mocks.NewAPIMock(ctrl)
	storage := mocks.NewStorageMock(ctrl)
	redisMock := mocks.NewRedisMock(ctrl)
	scheduler := mocks.NewIndirectScheduler(ctrl)
	isDone := &atomic.Bool{}

	m := &allMocks{
		docker:    dockerMock,
		api:       api,
		storage:   storage,
		redis:     redisMock,
		scheduler: scheduler,
		isDone:    isDone,
	}

	w := newWorker(zap.L(), 0, scheduler, dockerMock, api, storage, redisMock, func() {
		isDone.Store(true)
	})

	sleepFunc = func(d time.Duration) {}

	t.Cleanup(func() {
		sleepFunc = time.Sleep
	})

	w.cleanup()
	return m, w
}

func TestBackoff(t *testing.T) {
	makeMocks(t)

	i := 0
	operation := func() error {
		i++
		if i == 2 {
			return nil
		}
		return fmt.Errorf("boom")
	}

	err := withBackoff(zap.L(), "test", 2, operation)
	assert.NoError(t, err)
	assert.Equal(t, 2, i)

	i = 0
	err = withBackoff(zap.L(), "test", 1, operation)
	assert.ErrorContains(t, err, "boom")
}

func makeJob() *redis.Job {
	return &redis.Job{JobID: "a", Org: "b", Repo: "c", Commitish: "d", Checks: []redis.Check{{Plugin: "foo"}}, GitStorage: redis.GitStorage{}}
}

// The intention here is to test the whole run loop by first returning a job
// and forcing it to fail downloading its image, and returning nil from the next
// "Next" call, making the worker stop and set isDone to true.
func TestWorker_Run(t *testing.T) {
	m, w := makeMocks(t)
	// Redis setup
	job := makeJob()
	jobs := []*redis.Job{job, nil}
	m.redis.EXPECT().Next().Times(2).DoAndReturn(func() *redis.Job {
		j := jobs[0]
		jobs = jobs[1:]
		return j
	})

	// Scheduler setup
	m.scheduler.EXPECT().RegisterJob(job, 0)
	m.scheduler.EXPECT().DeregisterJob(job, 0)

	// Docker setup
	m.docker.EXPECT().PullImage("foo").AnyTimes().Return(fmt.Errorf("boom"))
	m.api.EXPECT().SetCheckError(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)

	// Test
	test_helpers.Timeout(t, 5*time.Second, w.Run)

	assert.True(t, m.isDone.Load())
}

// This tests the case where acquireCommit is unable to obtain a commit past its
// maximum amount of attempts.
func TestWorker_RunAcquireCommitFailure(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()

	// Docker setup
	m.docker.EXPECT().PullImage("foo").Times(1).Return(nil)

	// Storage setup
	m.storage.EXPECT().DownloadCommit("c", "d", gomock.Any()).AnyTimes().Return(fmt.Errorf("boom"))

	assert.ErrorContains(t, w.perform(), "all attempts failed")
}

// This tests the case where creating a container fails
func TestWorker_RunPrepareRuntimeFailure(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()

	// Docker setup
	m.docker.EXPECT().PullImage("foo").Times(1).Return(nil)
	m.docker.EXPECT().CreateContainer(gomock.Any()).DoAndReturn(func(ri *docker.RunInformation) (*docker.CreateContainerResult, error) {
		assert.Equal(t, "foo", ri.Image)
		assert.Equal(t, "c", ri.RepoName)
		assert.Equal(t, "d", ri.Commitish)
		return nil, fmt.Errorf("boom")
	})
	volume := &docker.PrepareVolumeResult{VolumeID: "yay"}
	m.docker.EXPECT().PrepareVolume(gomock.Any()).DoAndReturn(func(string) (*docker.PrepareVolumeResult, error) {
		return volume, nil
	})

	// Storage Setup
	m.storage.EXPECT().DownloadCommit("c", "d", gomock.Any()).Times(1).Return(nil)

	assert.ErrorContains(t, w.perform(), "boom")
}

// This tests the cleanup mechanism in case a container is created but the
// second one fails
func TestWorker_RunPrepareRuntimeFailureCleanup(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.job.Checks = append(w.job.Checks, redis.Check{Plugin: "bar"})

	fooContainerInfo := &docker.CreateContainerResult{
		ContainerID: "1",
		Image:       "a",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	gomock.InOrder(
		m.docker.EXPECT().PullImage("foo").Times(1).Return(nil),
		m.docker.EXPECT().PullImage("bar").Times(1).Return(nil),
	)

	volume := &docker.PrepareVolumeResult{VolumeID: "yay"}
	m.docker.EXPECT().PrepareVolume(gomock.Any()).DoAndReturn(func(string) (*docker.PrepareVolumeResult, error) {
		return volume, nil
	})

	m.docker.EXPECT().CreateContainer(gomock.Any()).Times(2).DoAndReturn(func(ri *docker.RunInformation) (*docker.CreateContainerResult, error) {
		if ri.Image == "foo" {
			assert.Equal(t, "c", ri.RepoName)
			assert.Equal(t, "d", ri.Commitish)
			return fooContainerInfo, nil
		}

		return nil, fmt.Errorf("boom")
	})

	// container 1 should also be removed right after
	m.docker.EXPECT().AbortAndRemove(fooContainerInfo).Return(nil)

	// Storage Setup
	m.storage.EXPECT().DownloadCommit("c", "d", gomock.Any()).Times(1).Return(nil)

	assert.ErrorContains(t, w.perform(), "boom")
	assert.Empty(t, w.containers)
}

// This tests a case in which trying to remove a container during prepareRuntime
// fails. When that happens, the container should still be registered in the
// containers map.
func TestWorker_RunPrepareRuntimeFailureCleanupFailure(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.job.Checks = append(w.job.Checks, redis.Check{Plugin: "bar"})

	fooContainerInfo := &docker.CreateContainerResult{
		ContainerID: "1",
		Image:       "a",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().CreateContainer(gomock.Any()).Times(2).DoAndReturn(func(ri *docker.RunInformation) (*docker.CreateContainerResult, error) {
		if ri.Image == "foo" {
			assert.Equal(t, "c", ri.RepoName)
			assert.Equal(t, "d", ri.Commitish)
			return fooContainerInfo, nil
		}

		return nil, fmt.Errorf("boom")
	})

	// container 1 should also be removed right after
	m.docker.EXPECT().AbortAndRemove(fooContainerInfo).Return(fmt.Errorf("nope"))

	assert.ErrorContains(t, w.prepareRuntime(), "boom")
	assert.NotEmpty(t, w.containers)
}

// startContainers cannot really fail, it just starts N goroutines to get output
// and wait for them to return. This will cause a small side effect since we
// will briefly hit #serviceContainer. Look the last spec for a full run with
// success and such.
func TestWorker_StartContainers(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(fmt.Errorf("boom"))
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	m.api.EXPECT().SetCheckError(w.job, "b", "Failed initializing container for b: boom")

	test_helpers.Timeout(t, 3*time.Second, w.startContainers)
}

// docker#ContainerStart succeeds, SetCheckRunning fails without aborting the
// operation; then, ContainerWait should be called. We will stop there, since
// we just want to make sure SetCheckingRunning failing does not stop it.
func TestWorkerServiceContainerSetRunningFailure(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	// This should break it.
	m.docker.EXPECT().ContainerWait("a").Return(fmt.Errorf("boom"))
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	// SetCheckRunning may be called several times due to the backoff mechanism
	m.api.EXPECT().SetCheckRunning(w.job, "b").AnyTimes().Return(fmt.Errorf("boom"))
	m.api.EXPECT().SetCheckError(w.job, "b", "Failed waiting container result for b: boom").Return(nil)

	w.serviceContainer("a", func() {})
}

// Tests a docker#GetContainerResult failing
func TestWorkServiceGetContainerResultFailure(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	// This should break it.
	m.docker.EXPECT().GetContainerResult("a").Return(nil, -1, fmt.Errorf("boom"))
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	m.api.EXPECT().SetCheckRunning(w.job, "b").Return(nil)
	m.api.EXPECT().SetCheckError(w.job, "b", "Failed obtaining result data for b: boom").Return(nil)

	w.serviceContainer("a", func() {})
}

// Tests when trying to read the output file for a container fails
func TestWorkServiceReadOutputFails(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	m.docker.EXPECT().GetContainerResult("a").Return(&bytes.Buffer{}, 0, nil)
	m.docker.EXPECT().GetContainerOutput(w.containers["a"]).Return(nil, fmt.Errorf("boom"))
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	m.api.EXPECT().SetCheckRunning(w.job, "b").Return(nil)
	m.api.EXPECT().SetCheckError(w.job, "b", gomock.Any()).DoAndReturn(func(_ *redis.Job, _, reason string) error {
		assert.Contains(t, reason, "Failed reading output data for b: ")
		return nil
	})

	w.serviceContainer("a", func() {})
}

// Tests the last part of ServiceContainer in which we store its status,
// and attempt to remove it from Docker. Here we will pretend it went well
// and the container should have been gone from the Worker's containers map.
func TestWorkServiceRemoveContainerOK(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	m.docker.EXPECT().GetContainerResult("a").Return(&bytes.Buffer{}, 0, nil)
	m.docker.EXPECT().GetContainerOutput(w.containers["a"]).Return([]byte{}, nil)
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	m.api.EXPECT().SetCheckRunning(w.job, "b").Return(nil)
	m.api.EXPECT().SetCheckSucceeded(w.job, "b").Return(nil)

	w.serviceContainer("a", func() {})
	assert.NotContains(t, w.containers, "a")
}

// Tests the last part of ServiceContainer in which we store its status,
// and attempt to remove it from Docker. Here we will pretend it failed
// and the container should not have been removed from the Worker's containers
// map.
func TestWorkServiceRemoveContainerFailure(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	m.docker.EXPECT().GetContainerResult("a").Return(&bytes.Buffer{}, 0, nil)
	m.docker.EXPECT().GetContainerOutput(w.containers["a"]).Return([]byte{}, nil)
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(fmt.Errorf("nope"))

	// API setup
	m.api.EXPECT().SetCheckRunning(w.job, "b").Return(nil)
	m.api.EXPECT().SetCheckSucceeded(w.job, "b").Return(nil)

	w.serviceContainer("a", func() {})
	assert.Contains(t, w.containers, "a")
}

// Asserts that ServiceContainer emits the correct check status as failed when a
// container exits with a non-zero exit status.
func TestWorkServiceContainerNonZero(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	m.docker.EXPECT().GetContainerResult("a").Return(bytes.NewBuffer([]byte("Boom!")), 1, nil)
	m.docker.EXPECT().GetContainerOutput(w.containers["a"]).Return([]byte{}, nil)
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	m.api.EXPECT().SetCheckRunning(w.job, "b").Return(nil)
	m.api.EXPECT().SetCheckError(w.job, "b", "Boom!").Return(nil)

	w.serviceContainer("a", func() {})
}

// Asserts that AggregateResults emits the correct check status as failed when a
// container emits corrupt report items
func TestWorkServiceCorruptReportItems(t *testing.T) {
	m, w := makeMocks(t)
	w.job = makeJob()
	w.containers["a"] = &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "b",
		OutputFile:  makeOutputFile(t),
	}

	// Docker setup
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	m.docker.EXPECT().GetContainerResult("a").Return(&bytes.Buffer{}, 0, nil)
	m.docker.EXPECT().GetContainerOutput(w.containers["a"]).Return([]byte("corrupted"), nil)
	m.docker.EXPECT().AbortAndRemove(w.containers["a"]).Return(nil)

	// API setup
	m.api.EXPECT().SetCheckRunning(w.job, "b").Return(nil)
	m.api.EXPECT().SetCheckError(w.job, "b", "Failed parsing output for b: invalid character 'c' looking for beginning of value").Return(nil)

	w.serviceContainer("a", func() {})
}

func TestWorker_RunFull(t *testing.T) {
	m, w := makeMocks(t)
	// Redis setup
	w.job = makeJob()
	jobs := []*redis.Job{w.job, nil}
	m.redis.EXPECT().Next().Times(2).DoAndReturn(func() *redis.Job {
		j := jobs[0]
		jobs = jobs[1:]
		return j
	})

	// Scheduler setup
	m.scheduler.EXPECT().RegisterJob(w.job, 0)
	m.scheduler.EXPECT().DeregisterJob(w.job, 0)

	// --- perform begins here

	// downloadImages
	m.docker.EXPECT().PullImage("foo")

	// acquireCommit
	m.storage.EXPECT().DownloadCommit("c", "d", gomock.Any()).AnyTimes().Return(nil)

	// prepareRuntime
	volume := &docker.PrepareVolumeResult{VolumeID: "yay"}
	m.docker.EXPECT().PrepareVolume(gomock.Any()).DoAndReturn(func(string) (*docker.PrepareVolumeResult, error) {
		return volume, nil
	})
	createResult := &docker.CreateContainerResult{
		ContainerID: "a",
		Image:       "foo",
		OutputFile:  makeOutputFile(t),
	}
	m.docker.EXPECT().CreateContainer(gomock.Any()).Return(createResult, nil)
	w.containers = map[string]*docker.CreateContainerResult{"a": createResult}

	issues, err := json.Marshal(ReportItem{
		Kind:      "bug",
		File:      "foo.rb",
		LineStart: 1,
		LineEnd:   1,
		Message:   "boom",
		UID:       "foobar",
	})
	require.NoError(t, err)

	// serviceContainer
	m.docker.EXPECT().ContainerStart("a").Return(nil)
	m.api.EXPECT().SetCheckRunning(w.job, "foo").Return(nil)
	m.api.EXPECT().SetCheckSucceeded(w.job, "foo").Return(nil)
	m.api.EXPECT().PushIssues(w.job, "foo", gomock.Any()).Return(nil)
	m.docker.EXPECT().ContainerWait("a").Return(nil)
	m.docker.EXPECT().GetContainerResult("a").Return(&bytes.Buffer{}, 0, nil)
	m.docker.EXPECT().GetContainerOutput(w.containers["a"]).Return(issues, nil)
	m.docker.EXPECT().AbortAndRemove(createResult).AnyTimes().Return(nil)

	// Back to perform
	m.api.EXPECT().WrapUp(w.job).Return(nil)

	// Test
	test_helpers.Timeout(t, 5*time.Second, w.Run)

	assert.True(t, m.isDone.Load())
}
