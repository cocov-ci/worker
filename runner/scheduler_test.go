package runner

import (
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/test_helpers"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
	"time"
)

func makeScheduler() *Scheduler {
	zap.ReplaceGlobals(zap.NewNop())
	return New(1, nil, nil, nil, nil)
}

func TestScheduler_RegisterJob(t *testing.T) {
	sch := makeScheduler()
	job := &redis.Job{
		JobID:      "a",
		Org:        "b",
		Repo:       "c",
		Commitish:  "d",
		Checks:     []string{},
		GitStorage: redis.GitStorage{},
	}

	sch.RegisterJob(job, 0)
	assert.NotZero(t, sch.currentJobs["a"])
	assert.Equal(t, "a", sch.workerJobs[0].JobID)
}

func TestScheduler_DeregisterJob(t *testing.T) {
	sch := makeScheduler()
	job := &redis.Job{
		JobID:      "a",
		Org:        "b",
		Repo:       "c",
		Commitish:  "d",
		Checks:     []string{},
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

func (d dummyRunner) Run() {
	if d.onStart != nil {
		d.onStart()
	}
	<-d.ok
	d.done()
}

func (d dummyRunner) prime() { close(d.ok) }

func TestScheduler_Run(t *testing.T) {
	sch := makeScheduler()
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
