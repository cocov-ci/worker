package runner

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/storage"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// This is here just to make sure we can not really sleep during tests.
var sleepFunc = time.Sleep

func withBackoff(log *zap.Logger, operationName string, maxTries int, fn func() error) error {
	backoff := 1
	var err error
	for i := 0; i < maxTries; i++ {
		if err = fn(); err != nil {
			toWait := time.Duration(backoff) * time.Second
			log.Info("Backing off "+operationName, zap.Duration("delay", toWait))
			sleepFunc(toWait)
			backoff *= 2
			continue
		}
		break
	}

	return err
}

type containerResult struct {
	Error           error
	ExitStatus      int
	ContainerOutput string
	OutputFileData  []byte
	Image           string
}

func newWorker(log *zap.Logger, id int, scheduler IndirectScheduler, dockerClient docker.Client, apiClient api.Client, storageClient storage.Base, redisClient redis.Client, done func()) *worker {
	return &worker{
		id:   id,
		log:  log.With(zap.Int("worker_id", id)),
		done: done,

		scheduler: scheduler,
		docker:    dockerClient,
		api:       apiClient,
		storage:   storageClient,
		redis:     redisClient,

		containers:      map[string]*docker.CreateContainerResult{},
		containerStatus: NewConcurrentMap[string, *containerResult](),
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

	job        *redis.Job
	workDir    string
	commitDir  string
	secretsDir string

	containers      map[string]*docker.CreateContainerResult
	containerStatus *ConcurrentMap[string, *containerResult]
	secretsSource   map[string]string
	emittedErrors   map[string]bool
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
		w.cleanup()
		w.log.Info("Picked up", zap.String("job_id", job.JobID))
		w.scheduler.RegisterJob(job, w.id)
		w.job = job
		if err := w.perform(); err != nil {
			w.log.Error("Failed performing job", zap.Error(err))
			w.emitGeneralError()
		}
		w.scheduler.DeregisterJob(job, w.id)
	}
}

func (w *worker) emitGeneralError() {
	for _, c := range w.job.Checks {
		if _, ok := w.emittedErrors[c.Plugin]; ok {
			continue
		}

		if err := w.api.SetCheckError(w.job, c.Plugin, fmt.Sprintf("An internal error interrupted the initialization of job %s. Please refer to the scheduler's logs for further information.", w.job.JobID)); err != nil {
			w.log.Error("failed emitting general error", zap.Error(err))
		}
	}
}

func (w *worker) cleanup() {
	if w.workDir != "" {
		// TODO: Log?
		_ = os.RemoveAll(w.workDir)
	}
	w.workDir = ""
	w.job = nil
	for _, k := range w.containers {
		if err := w.docker.AbortAndRemove(k); err != nil {
			w.log.Error("Error removing container",
				zap.String("container_id", k.ContainerID),
				zap.Error(err))
		}
	}
	w.containers = map[string]*docker.CreateContainerResult{}
	w.emittedErrors = map[string]bool{}
	w.containerStatus.Reset()
}

func (w *worker) perform() error {
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

	if err = w.downloadImages(); err != nil {
		return err
	}

	if err = w.obtainSecrets(); err != nil {
		return err
	}

	if err = w.acquireCommit(); err != nil {
		return err
	}

	if err = w.prepareRuntime(); err != nil {
		return err
	}

	w.startContainers()

	finalStatus, checks := w.aggregateResults()

	err = w.api.PushIssues(w.job, checks, finalStatus)
	if err != nil {
		w.log.Error("Error pushing results", zap.Error(err))
	}
	return err
}

func (w *worker) acquireCommit() error {
	w.log.Info("Acquiring commit",
		zap.String("repository", w.job.Repo),
		zap.String("sha", w.job.Commitish))
	err := withBackoff(w.log, "downloading commit", 10, func() error {
		err := w.storage.DownloadCommit(w.job.Repo, w.job.Commitish, w.commitDir)
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
	return nil
}

func (w *worker) downloadImages() error {
	w.log.Info("Downloading images for checks")
	for _, v := range w.job.Checks {
		log := w.log.With(zap.String("image", v.Plugin))
		log.Info("Downloading image", zap.String("image", v.Plugin))
		err := withBackoff(log, "downloading image", 10, func() error {
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
			err = withBackoff(w.log, "obtaining secret", 10, func() error {
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
			bindings := map[string]string{}
			for _, m := range check.Mounts {
				local, ok := w.secretsSource[m.Authorization]
				if !ok {
					w.log.Error("Failed mounting secret for path, as the source could not be located",
						zap.String("path", m.Target),
						zap.String("check", check.Plugin))
					abortAndCleanup()
					return fmt.Errorf("failed mounting secret for '%s': unable to locate secret path %s", check.Plugin, m.Target)
				}
				name := hash(m.Target)
				mounts[local] = "/secrets/" + name
				bindings[name] = m.Target
			}
			{
				bindingPath := filepath.Join(w.secretsDir, "bindings")
				bindingFile, err := os.Create(bindingPath)
				if err != nil {
					w.log.Error("Failed creating secret binding file", zap.Error(err))
					abortAndCleanup()
					return err
				}
				data := bytes.Buffer{}
				for k, v := range bindings {
					data.Write([]byte(fmt.Sprintf("%s=%s", k, v)))
					data.WriteByte(0x00)
				}

				dataBytes := data.Bytes()
				if err = writeAll(dataBytes[:len(dataBytes)-1], bindingFile, true); err != nil {
					w.log.Error("Failed writing to secret binding file", zap.Error(err))
					abortAndCleanup()
					return err
				}
				mounts[bindingPath] = "/secrets/bindings"
			}
		}

		res, err := w.docker.CreateContainer(&docker.RunInformation{
			Image:     check.Plugin,
			WorkDir:   w.commitDir,
			RepoName:  w.job.Repo,
			Commitish: w.job.Commitish,
			Mounts:    mounts,
			Envs:      check.Envs,
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

func (w *worker) aggregateResults() (string, map[string]any) {
	checks := map[string]any{}
	failed := false

	for _, v := range w.containerStatus.Map() {
		if v.Error != nil {
			w.emitCheckError(v.Image, fmt.Sprintf("Execution failed due to internal error: %s", v.Error))
			failed = true
			continue
		}

		if v.ExitStatus != 0 {
			w.emitCheckError(v.Image, fmt.Sprintf("Execution failed. Plugin exited with status %d:\n%s", v.ExitStatus, v.ContainerOutput))
			failed = true
			continue
		}

		if records, err := ParseReportItems(v.OutputFileData); err != nil {
			w.emitCheckError(v.Image, fmt.Sprintf("Execution failed. Error parsing plugin output: %s", err.Error()))
			failed = true
			continue
		} else {
			checks[strings.SplitN(v.Image, ":", 2)[0]] = records
		}

		if err := w.api.SetCheckSucceeded(w.job, v.Image); err != nil {
			w.log.Error("Failed invoking SetCheckSucceeded", zap.Error(err))
		}
	}

	finalStatus := "processed"
	if failed {
		finalStatus = "errored"
	}

	return finalStatus, checks
}

func (w *worker) emitCheckError(check string, msg string) {
	w.emittedErrors[check] = true
	if err := w.api.SetCheckError(w.job, check, msg); err != nil {
		w.log.Error("Failed invoking SetCheckError", zap.Error(err))
	}
}

func (w *worker) serviceContainer(cID string, done func()) {
	defer done()
	cInfo := w.containers[cID]
	log := w.log.With(zap.String("check", cInfo.Image))

	if err := w.docker.ContainerStart(cID); err != nil {
		log.Error("Failed starting container", zap.Error(err))
		w.emitCheckError(cInfo.Image, fmt.Sprintf("Failed initializing container for %s: %s", cInfo.Image, err.Error()))
		w.containerStatus.Set(cID, &containerResult{Error: fmt.Errorf("failed executing ContainerStart: %w", err)})
		return
	}

	if err := w.api.SetCheckRunning(w.job, cInfo.Image); err != nil {
		log.Error("Failed invoking SetCheckRunning", zap.Error(err))
	}

	log.Info("Container started. Waiting for completion...")
	if err := w.docker.ContainerWait(cID); err != nil {
		log.Error("Error waiting container", zap.Error(err))
		w.emitCheckError(cInfo.Image, fmt.Sprintf("Failed waiting container result for %s: %s", cInfo.Image, err.Error()))
		w.containerStatus.Set(cID, &containerResult{Error: fmt.Errorf("failed executing ContainerWait: %w", err)})
		return
	}

	log.Info("Container finished. Copying results...")
	containerOutput, exitStatus, err := w.docker.GetContainerResult(cID)
	if err != nil {
		log.Error("GetContainerResult failed", zap.Error(err))
		w.emitCheckError(cInfo.Image, fmt.Sprintf("Failed obtaining result data for %s: %s", cInfo.Image, err.Error()))
		w.containerStatus.Set(cID, &containerResult{Error: fmt.Errorf("failed executing GetContainerResult: %w", err)})
		return
	}

	log.Info("Reading output data...")
	var output []byte
	output, err = os.ReadFile(cInfo.OutputFile)
	if err != nil {
		log.Error("Error reading output file", zap.Error(err), zap.String("output_file", cInfo.OutputFile))
		w.emitCheckError(cInfo.Image, fmt.Sprintf("Failed reading output data for %s: %s", cInfo.Image, err.Error()))
		w.containerStatus.Set(cID, &containerResult{Error: fmt.Errorf("failed reading output file: %w", err)})
		return
	}

	log.Info("Operation completed. Removing container...")
	w.containerStatus.Set(cID, &containerResult{
		Image:           cInfo.Image,
		Error:           nil,
		ExitStatus:      exitStatus,
		ContainerOutput: containerOutput.String(),
		OutputFileData:  output,
	})

	// At this point, all required information has been read. Delete the container.
	if err := w.docker.AbortAndRemove(cInfo); err != nil {
		log.Error("Failed invoking AbortAndRemove", zap.Error(err))
	} else {
		delete(w.containers, cID)
	}
}
