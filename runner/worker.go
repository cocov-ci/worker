package runner

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/storage"
)

// This is here just to make sure we can not really sleep during tests.
var sleepFunc = time.Sleep

func withBackoff(log *zap.Logger, operationName string, maxTries int, fn func() error) error {
	backoff := 1
	var err error
	for i := 0; i < maxTries; i++ {
		if err = fn(); err != nil {
			toWait := time.Duration(backoff) * time.Second
			log.Info("Backing off "+operationName, zap.Duration("delay", toWait), zap.String("attempt", fmt.Sprintf("%d/%d", i+1, maxTries)))
			sleepFunc(toWait)
			backoff *= 2
			continue
		}
		break
	}

	return err
}

const defaultAttemptCount = 10

type workerOpts struct {
	log           *zap.Logger
	id            int
	scheduler     IndirectScheduler
	dockerClient  docker.Client
	apiClient     api.Client
	storageClient storage.Base
	redisClient   redis.Client
	done          func()
	debugPlugins  bool
}

func newWorker(opts workerOpts) *worker {
	return &worker{
		id:   opts.id,
		log:  opts.log.With(zap.Int("worker_id", opts.id)),
		done: opts.done,

		scheduler:  opts.scheduler,
		docker:     opts.dockerClient,
		api:        opts.apiClient,
		storage:    opts.storageClient,
		redis:      opts.redisClient,
		debug:      opts.debugPlugins,
		jobStateMu: &sync.Mutex{},
		cancelMu:   &sync.Mutex{},

		containers: map[string]*docker.CreateContainerResult{},
	}
}

type worker struct {
	log  *zap.Logger
	done func()
	id   int

	scheduler IndirectScheduler
	docker    docker.Client
	api       api.Client
	storage   storage.Base
	redis     redis.Client

	jobStateMu *sync.Mutex
	job        *redis.Job
	workDir    string
	commitDir  string
	secretsDir string

	containers    map[string]*docker.CreateContainerResult
	secretsSource map[string]string
	emittedErrors map[string]bool
	sourceVolume  *docker.PrepareVolumeResult

	cancelMu *sync.Mutex
	canceled bool
	debug    bool
}

func (w *worker) SetCheckError(plugin string, message string) {
	err := withBackoff(w.log, "emitting status for check "+plugin, defaultAttemptCount, func() error {
		return w.api.SetCheckError(w.job, plugin, message)
	})

	if err != nil {
		w.log.Error("Failed to invoke SetCheckError",
			zap.String("plugin", plugin),
			zap.String("message", message),
			zap.Error(err))
		return
	}

	w.emittedErrors[plugin] = true
}

func (w *worker) CancelJob(jobID string) {
	w.jobStateMu.Lock()
	defer w.jobStateMu.Unlock()

	if w.job == nil || w.job.JobID != jobID {
		return
	}

	w.log.Info("Receive cancellation request for job", zap.String("id", jobID))

	w.cancelMu.Lock()
	w.canceled = true
	for id, v := range w.containers {
		w.log.Info("Requesting termination of associated container", zap.String("id", id))
		w.docker.TerminateContainer(v)
	}
	w.cancelMu.Unlock()
}

func (w *worker) Run() {
	defer w.done()

	w.log.Info("Worker started")

	for {
		job := w.redis.Next()
		if job == nil {
			w.log.Info("Stopping...")
			break
		}

		w.pickup(job)
		if err := w.perform(); err != nil {
			w.log.Error("Failed performing job", zap.Error(err))
			w.emitGeneralError()
		}
		w.releaseJob()
	}
}

func (w *worker) pickup(job *redis.Job) {
	w.jobStateMu.Lock()
	defer w.jobStateMu.Unlock()

	w.cleanup()
	err := withBackoff(w.log, "notifying api of set status", defaultAttemptCount, func() error {
		return w.api.SetSetRunning(job)
	})
	if err != nil {
		w.log.Error("Failed invoking SetCheckCanceled", zap.Error(err))
	}
	w.log.Info("Picked up", zap.String("job_id", job.JobID))
	w.scheduler.RegisterJob(job, w.id)
	w.job = job
}

func (w *worker) releaseJob() {
	w.jobStateMu.Lock()
	defer w.jobStateMu.Unlock()

	w.scheduler.DeregisterJob(w.job, w.id)
	w.cleanup()
}

func (w *worker) emitGeneralError() {
	for _, c := range w.job.Checks {
		if _, ok := w.emittedErrors[c.Plugin]; ok {
			continue
		}

		w.SetCheckError(c.Plugin, fmt.Sprintf("An internal error interrupted the initialization of job %s. Please refer to the scheduler's logs for further information.", w.job.JobID))
	}
}

func (w *worker) cleanup() {
	if w.workDir != "" {
		// TODO: Log?
		_ = os.RemoveAll(w.workDir)
	}
	w.workDir = ""
	w.job = nil
	w.canceled = false
	for _, k := range w.containers {
		if err := w.docker.AbortAndRemove(k); err != nil {
			w.log.Error("Error removing container",
				zap.String("container_id", k.ContainerID),
				zap.Error(err))
		}
	}
	w.containers = map[string]*docker.CreateContainerResult{}
	w.emittedErrors = map[string]bool{}
	if w.sourceVolume != nil {
		if err := w.docker.RemoveVolume(w.sourceVolume); err != nil {
			w.log.Error("Error removing volume",
				zap.String("volume_name", w.sourceVolume.VolumeID),
				zap.Error(err))
		}
		w.sourceVolume = nil
	}
}

var cancelledErr = fmt.Errorf("received cancelation request for this job")

func (w *worker) earlyCancelCheck() {
	w.cancelMu.Lock()
	defer w.cancelMu.Unlock()
	if w.canceled {
		w.log.Info("Caught early cancellation signal")
		panic(cancelledErr)
	}
}

func (w *worker) operationCancelCheck() {
	w.cancelMu.Lock()
	defer w.cancelMu.Unlock()
	if w.canceled {
		w.log.Info("Caught operation cancellation signal")
		panic(cancelledErr)
	}
}

func (w *worker) perform() error {
	defer func() {
		if err := recover(); err != nil {
			if err == cancelledErr {
				return
			} else {
				panic(err)
			}
		}
	}()

	tempPath, err := os.MkdirTemp("", "")
	if err != nil {
		w.log.Error("Failed generating temporary directory", zap.Error(err))
		return err
	}

	w.workDir = tempPath
	w.commitDir = filepath.Join(w.workDir, "src")
	w.secretsDir = filepath.Join(w.workDir, "private")

	if err = os.MkdirAll(w.commitDir, 0750); err != nil {
		return err
	}

	if err = os.MkdirAll(w.secretsDir, 0750); err != nil {
		return err
	}

	operations := []func() error{
		w.downloadImages,
		w.obtainSecrets,
		w.acquireCommit,
		w.prepareRuntime,
	}

	for _, op := range operations {
		w.earlyCancelCheck()
		if err = op(); err != nil {
			w.log.Error("Failed initializing runtime", zap.Error(err))

			w.log.Info("Voiding all checks due to previous error")
			msg := fmt.Sprintf("Failed initializing runtime: %s", err)
			for _, ch := range w.job.Checks {
				w.SetCheckError(ch.Plugin, msg)
			}
			if err := w.wrapUp(); err != nil {
				w.log.Error("Error emitting wrap up signal", zap.Error(err))
			}

			return err
		}
		w.earlyCancelCheck()
	}

	w.startContainers()

	if err = w.wrapUp(); err != nil {
		w.log.Error("Error emitting wrap up signal", zap.Error(err))
	}
	return err
}

func (w *worker) wrapUp() error {
	return withBackoff(w.log, "emit wrap up signal", defaultAttemptCount, func() error {
		return w.api.WrapUp(w.job)
	})
}

func (w *worker) acquireCommit() error {
	w.log.Info("Acquiring commit",
		zap.String("repository", w.job.Repo),
		zap.String("sha", w.job.Commitish))
	zstPath := filepath.Join(w.commitDir, w.job.Commitish+".tar.zst")
	err := withBackoff(w.log, "downloading commit", defaultAttemptCount, func() error {
		err := w.storage.DownloadCommit(w.job.Repo, w.job.Commitish, zstPath)
		if err != nil {
			w.log.Error("Error acquiring commit",
				zap.String("repo", w.job.Repo),
				zap.String("commitish", w.job.Commitish),
				zap.Error(err))
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("could not download %s@%s; all attempts failed. Last error: %w",
			w.job.Repo, w.job.Commitish, err)
	}

	w.log.Info("Preparing source volume",
		zap.String("repository", w.job.Repo),
		zap.String("sha", w.job.Commitish))
	vol, err := w.docker.PrepareVolume(zstPath)
	if err != nil {
		w.log.Info("Failed preparing source volume",
			zap.String("repository", w.job.Repo),
			zap.String("sha", w.job.Commitish),
			zap.Error(err))
		return err
	}
	w.sourceVolume = vol
	return nil
}

func (w *worker) downloadImages() error {
	w.log.Info("Downloading images for checks")
	for _, v := range w.job.Checks {
		log := w.log.With(zap.String("image", v.Plugin))
		log.Info("Downloading image", zap.String("image", v.Plugin))
		err := withBackoff(log, "downloading image", defaultAttemptCount, func() error {
			return w.docker.PullImage(v.Plugin)
		})

		if err != nil {
			log.Error("Failed downloading image", zap.Error(err))
			return err
		}
		log.Info("Downloaded image")
	}
	return nil
}

func writeAll(data []byte, into io.WriteCloser, close bool) error {
	toWrite := len(data)
	written := 0
	for written < toWrite {
		n, err := into.Write(data[written:])
		if err != nil {
			return fmt.Errorf("failed copying stream: %w", err)
		}
		written += n
	}

	if close {
		err := into.Close()
		if err != nil {
			return fmt.Errorf("failed closing stream: %w", err)
		}
	}
	return nil
}

func hash(str string) string {
	sha := sha1.New()
	sha.Write([]byte(str))
	return hex.EncodeToString(sha.Sum(nil))
}

func (w *worker) obtainSecrets() error {
	allSecrets := map[string]string{}
	purge := func() {
		for _, p := range allSecrets {
			if err := os.RemoveAll(p); err != nil {
				w.log.Error("Failed removing secret piece", zap.Error(err))
			}
		}
	}

	for _, c := range w.job.Checks {
		for _, m := range c.Mounts {
			secretPath := filepath.Join(w.secretsDir, hash(m.Authorization))
			tmp, err := os.Create(secretPath)
			if err != nil {
				w.log.Error("Creating secretPath failed", zap.String("path", secretPath), zap.Error(err))
				purge()
				return err
			}

			var secretData []byte
			err = withBackoff(w.log, "obtaining secret", defaultAttemptCount, func() error {
				m := m
				data, err := w.api.GetSecret(&m)
				if err != nil {
					return err
				}
				secretData = data
				return nil
			})

			if err != nil {
				purge()
				w.log.Error("Failed obtaining secret. All attempts failed.", zap.Error(err))
				return err
			}

			if err = writeAll(secretData, tmp, true); err != nil {
				purge()
				w.log.Error("Failed flushing secret.", zap.Error(err))
				return err
			}

			allSecrets[m.Authorization] = secretPath
		}
	}

	w.secretsSource = allSecrets
	return nil
}

func (w *worker) prepareRuntime() error {
	w.log.Info("Preparing runtime environment")
	abortAndCleanup := func() {
		toRemove := make([]string, 0, len(w.containers))
		for k, v := range w.containers {
			if err := w.docker.AbortAndRemove(v); err != nil {
				w.log.Error("Error removing container",
					zap.Error(err),
					zap.String("container_id", v.ContainerID))
				continue
			}

			toRemove = append(toRemove, k)
		}
		for _, k := range toRemove {
			delete(w.containers, k)
		}
	}

	for _, check := range w.job.Checks {
		var mounts map[string]string
		if len(check.Mounts) > 0 {
			mounts = map[string]string{}
			for _, m := range check.Mounts {
				local, ok := w.secretsSource[m.Authorization]
				if !ok {
					w.log.Error("Failed mounting secret for path, as the source could not be located",
						zap.String("path", m.Target),
						zap.String("check", check.Plugin))
					abortAndCleanup()
					return fmt.Errorf("failed mounting secret for '%s': unable to locate secret path %s", check.Plugin, m.Target)
				}
				if strings.HasPrefix(m.Target, "~") {
					m.Target = "/home/cocov" + m.Target[1:]
				}
				mounts[local] = m.Target
			}
		}

		res, err := w.docker.CreateContainer(&docker.RunInformation{
			Image:        check.Plugin,
			RepoName:     w.job.Repo,
			Commitish:    w.job.Commitish,
			Mounts:       mounts,
			Envs:         check.Envs,
			SourceVolume: w.sourceVolume,
			JobID:        w.job.JobID,
		})
		if err != nil {
			w.log.Error("Error creating container. Aborting operation.",
				zap.Error(err))
			abortAndCleanup()
			return err
		}
		w.containers[res.ContainerID] = res
		w.log.Info("Created container",
			zap.String("container_id", res.ContainerID),
			zap.String("for_check", check.Plugin))
	}

	return nil
}

func (w *worker) startContainers() {
	w.log.Info("Starting containers")
	wg := sync.WaitGroup{}
	wg.Add(len(w.containers))

	for containerID := range w.containers {
		go w.serviceContainer(containerID, wg.Done)
	}

	w.log.Info("Waiting for containers...")
	thence := time.Now()
	wg.Wait()
	duration := time.Since(thence)
	w.log.Info("Containers finished", zap.Duration("total_duration", duration))
}

func (w *worker) serviceContainer(cID string, done func()) {
	defer done()
	cInfo := w.containers[cID]
	log := w.log.With(zap.String("check", cInfo.Image))

	defer func() {
		// At this point, all required information has been read. Delete the container.
		if err := w.docker.AbortAndRemove(cInfo); err != nil {
			log.Error("Failed invoking AbortAndRemove", zap.Error(err))
		} else {
			delete(w.containers, cID)
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			if err == cancelledErr {
				err := withBackoff(log, "setting canceled status", defaultAttemptCount, func() error {
					return w.api.SetCheckCanceled(w.job, cInfo.Image)
				})
				if err != nil {
					log.Error("Failed invoking SetCheckCanceled", zap.Error(err))
				}
			} else {
				if notError, ok := err.(error); !ok {
					err = fmt.Errorf("recovered from non-error value passed to panic: %s", notError)
				}

				log.Error("Recovered from panic", zap.Error(err.(error)))
				err := withBackoff(log, "setting failure status", defaultAttemptCount, func() error {
					return w.api.SetCheckError(w.job, cInfo.Image, err.(error).Error())
				})
				if err != nil {
					log.Error("Failed invoking SetCheckError", zap.Error(err))
				}
			}
		}
	}()

	if err := w.docker.ContainerStart(cID); err != nil {
		log.Error("Failed starting container", zap.Error(err))
		w.SetCheckError(cInfo.Image, fmt.Sprintf("Failed initializing container for %s: %s", cInfo.Image, err.Error()))
		return
	}

	w.operationCancelCheck()

	err := withBackoff(log, "setting running status", defaultAttemptCount, func() error {
		return w.api.SetCheckRunning(w.job, cInfo.Image)
	})
	if err != nil {
		log.Error("Failed invoking SetCheckRunning", zap.Error(err))
	}

	w.operationCancelCheck()

	log.Info("Container started. Waiting for completion...")
	if err := w.docker.ContainerWait(cID); err != nil {
		w.operationCancelCheck()

		log.Error("Error waiting container", zap.Error(err))
		w.SetCheckError(cInfo.Image, fmt.Sprintf("Failed waiting container result for %s: %s", cInfo.Image, err.Error()))
		return
	}

	w.operationCancelCheck()

	log.Info("Container finished. Copying results...")
	containerOutput, exitStatus, err := w.docker.GetContainerResult(cID)
	if err != nil {
		w.operationCancelCheck()
		log.Error("GetContainerResult failed", zap.Error(err))
		w.SetCheckError(cInfo.Image, fmt.Sprintf("Failed obtaining result data for %s: %s", cInfo.Image, err.Error()))
		return
	}

	if w.debug {
		log.Info("Container output", zap.String("output", containerOutput.String()))
	}

	w.operationCancelCheck()

	log.Info("Reading output data...")
	output, err := w.docker.GetContainerOutput(cInfo)
	if err != nil {
		log.Error("Error reading output file from container", zap.Error(err))
		w.SetCheckError(cInfo.Image, fmt.Sprintf("Failed reading output data for %s: %s", cInfo.Image, err.Error()))
		return
	}

	if exitStatus == 0 {
		var reportData []ReportItem
		reportData, err = ParseReportItems(output)
		if err != nil {
			log.Error("Failed parsing container output", zap.Error(err))
			w.SetCheckError(cInfo.Image, fmt.Sprintf("Failed parsing output for %s: %s", cInfo.Image, err.Error()))
			return
		}

		if len(reportData) > 0 {
			err = withBackoff(log, "emit check issues", defaultAttemptCount, func() error {
				return w.api.PushIssues(w.job, cInfo.Image, reportData)
			})
		}

		if err == nil {
			err = withBackoff(log, "emit success check status", defaultAttemptCount, func() error {
				return w.api.SetCheckSucceeded(w.job, cInfo.Image)
			})
		}
	} else {
		err = withBackoff(log, "emit failed check status", defaultAttemptCount, func() error {
			return w.api.SetCheckError(w.job, cInfo.Image, containerOutput.String())
		})
	}

	if err != nil {
		log.Error("Failed emitting final check status", zap.Error(err))
	}

	log.Info("Operation completed. Removing container...")
}
