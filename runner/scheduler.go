package runner

import (
	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/storage"
	"go.uber.org/zap"
	"sync"
	"time"
)

type IndirectScheduler interface {
	RegisterJob(j *redis.Job, workerID int)
	DeregisterJob(j *redis.Job, workerID int)
}

type runnable interface {
	Run()
}

func New(maxJobs int, api api.Client, docker docker.Client, redisClient redis.Client, storage storage.Base) *Scheduler {
	return &Scheduler{
		maxJobs:     maxJobs,
		currentJobs: map[string]time.Time{},
		stopped:     false,
		mu:          &sync.Mutex{},
		api:         api,
		docker:      docker,
		redis:       redisClient,
		storage:     storage,
		log:         zap.L().With(zap.String("facility", "runner")),
		workerJobs:  map[int]*redis.Job{},
	}
}

type Scheduler struct {
	log         *zap.Logger
	maxJobs     int
	currentJobs map[string]time.Time
	workerJobs  map[int]*redis.Job
	stopped     bool
	mu          *sync.Mutex

	api     api.Client
	docker  docker.Client
	redis   redis.Client
	storage storage.Base

	workerFactory func(done func()) runnable
}

func (r *Scheduler) RegisterJob(job *redis.Job, workerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentJobs[job.JobID] = time.Now().UTC()
	r.workerJobs[workerID] = job
	r.log.Info("Registered job", zap.String("job_id", job.JobID))
}

func (r *Scheduler) DeregisterJob(job *redis.Job, workerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	duration := time.Since(r.currentJobs[job.JobID])
	delete(r.currentJobs, job.JobID)
	delete(r.workerJobs, workerID)
	r.log.Info("Deregistered job", zap.String("job_id", job.JobID), zap.Duration("duration", duration))
}

func (r *Scheduler) statsLoop() {
}

func (r *Scheduler) Run() {
	go r.statsLoop()
	wg := sync.WaitGroup{}
	for i := 0; i < r.maxJobs; i++ {
		wg.Add(1)
		var w runnable
		if r.workerFactory != nil {
			w = r.workerFactory(wg.Done)
		} else {
			w = newWorker(r.log, i, r, r.docker, r.api, r.storage, r.redis, wg.Done)
		}
		go w.Run()
	}
	wg.Wait()
}
