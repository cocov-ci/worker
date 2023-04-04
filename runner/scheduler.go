package runner

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/storage"
)

type IndirectScheduler interface {
	RegisterJob(j *redis.Job, workerID int)
	DeregisterJob(j *redis.Job, workerID int)
}

type runnable interface {
	Run()
	CancelJob(jobID string)
}

type SchedulerOpts struct {
	MaxJobs      int
	API          api.Client
	Docker       docker.Client
	RedisClient  redis.Client
	Storage      storage.Base
	DebugPlugins bool
}

func New(opts SchedulerOpts) *Scheduler {
	return &Scheduler{
		maxJobs:       opts.MaxJobs,
		currentJobs:   map[string]time.Time{},
		stopped:       false,
		mu:            &sync.Mutex{},
		api:           opts.API,
		docker:        opts.Docker,
		redis:         opts.RedisClient,
		storage:       opts.Storage,
		debugPlugins:  opts.DebugPlugins,
		log:           zap.L().With(zap.String("facility", "runner")),
		workerJobs:    map[int]*redis.Job{},
		workers:       map[int]runnable{},
		workerByJobID: map[string]int{},
	}
}

type Scheduler struct {
	log           *zap.Logger
	maxJobs       int
	currentJobs   map[string]time.Time
	workerJobs    map[int]*redis.Job
	workerByJobID map[string]int
	stopped       bool
	mu            *sync.Mutex
	debugPlugins  bool

	api            api.Client
	docker         docker.Client
	redis          redis.Client
	storage        storage.Base
	cacheServerURL string

	workerFactory func(done func()) runnable
	workers       map[int]runnable
}

func (r *Scheduler) RegisterJob(job *redis.Job, workerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentJobs[job.JobID] = time.Now().UTC()
	r.workerJobs[workerID] = job
	r.workerByJobID[job.JobID] = workerID
	r.log.Info("Registered job", zap.String("job_id", job.JobID))
}

func (r *Scheduler) DeregisterJob(job *redis.Job, workerID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	duration := time.Since(r.currentJobs[job.JobID])
	delete(r.currentJobs, job.JobID)
	delete(r.workerJobs, workerID)
	delete(r.workerByJobID, job.JobID)
	r.log.Info("Deregistered job", zap.String("job_id", job.JobID), zap.Duration("duration", duration))
}

func (r *Scheduler) CancelJob(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	workerID, ok := r.workerByJobID[jobID]
	if !ok {
		return
	}
	r.workers[workerID].CancelJob(jobID)
}

func (r *Scheduler) statsLoop() {
}

func (r *Scheduler) controlListener() {
	r.log.Info("ControlListener is running")
	for {
		ctrl := r.redis.NextControlRequest()
		if ctrl == nil {
			break
		}

		switch req := ctrl.(type) {
		case *redis.CancellationRequest:
			r.CancelJob(req.JobId)
		default:
			continue
		}
	}

	r.log.Info("ControlListener stopped")
}

func (r *Scheduler) Run() {
	go r.statsLoop()
	go r.controlListener()
	wg := sync.WaitGroup{}
	for i := 0; i < r.maxJobs; i++ {
		wg.Add(1)
		var w runnable
		if r.workerFactory != nil {
			w = r.workerFactory(wg.Done)
		} else {
			w = newWorker(workerOpts{
				log:           r.log,
				id:            i,
				scheduler:     r,
				dockerClient:  r.docker,
				apiClient:     r.api,
				storageClient: r.storage,
				redisClient:   r.redis,
				done:          wg.Done,
				debugPlugins:  r.debugPlugins,
			})
		}
		r.workers[i] = w
		go w.Run()
	}
	wg.Wait()
	r.redis.ShutdownControlChecks()
}

func (r *Scheduler) Shutdown() {
	r.log.Info("Received shutdown signal")
	r.redis.Shutdown()
}
