package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/test_helpers"
)

func makeScheduler(t *testing.T) (*allMocks, *Scheduler) {
	zap.ReplaceGlobals(zap.NewNop())
	mocks, _ := makeMocks(t)

	return mocks, New(SchedulerOpts{
		MaxJobs:      1,
		API:          nil,
		Docker:       nil,
		RedisClient:  mocks.redis,
		Storage:      nil,
		DebugPlugins: false,
	})
}

func TestScheduler_RegisterJob(t *testing.T) {
	_, sch := makeScheduler(t)
	job := &redis.Job{
		JobID:      "a",
		Org:        "b",
		Repo:       "c",
		Commitish:  "d",
		Checks:     []redis.Check{},
		GitStorage: redis.GitStorage{},
	}

	sch.RegisterJob(job, 0)
	assert.NotZero(t, sch.currentJobs["a"])
	assert.Equal(t, "a", sch.workerJobs[0].JobID)
}

func TestScheduler_DeregisterJob(t *testing.T) {
	_, sch := makeScheduler(t)
	job := &redis.Job{
		JobID:      "a",
		Org:        "b",
		Repo:       "c",
		Commitish:  "d",
		Checks:     []redis.Check{},
		GitStorage: redis.GitStorage{},
	}

	sch.RegisterJob(job, 0)
	sch.DeregisterJob(job, 0)

	assert.Empty(t, sch.currentJobs)
	assert.Empty(t, sch.workerJobs)
}

func newDummyRunner(done func()) dummyRunner {
	return dummyRunner{
		done: done,
		ok:   make(chan bool),
	}
}

type dummyRunner struct {
	done    func()
	ok      chan bool
	onStart func()
}

func (d dummyRunner) CancelJob(jobID string) {}

func (d dummyRunner) Run() {
	if d.onStart != nil {
		d.onStart()
	}
	<-d.ok
	d.done()
}

func (d dummyRunner) prime() { close(d.ok) }

func TestScheduler_Run(t *testing.T) {
	mocks, sch := makeScheduler(t)
	mocks.redis.EXPECT().NextControlRequest().AnyTimes().Return(nil)
	mocks.redis.EXPECT().ShutdownControlChecks()

	var prime func()
	wait := make(chan bool)
	done := make(chan bool)

	sch.workerFactory = func(done func()) runnable {
		w := newDummyRunner(done)
		w.onStart = func() { close(wait) }
		prime = w.prime
		return w
	}

	go func() {
		sch.Run()
		close(done)
	}()

	<-wait

	prime()
	test_helpers.Timeout(t, 3*time.Second, func() { <-done })
}
