package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/distribution/uuid"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"

	"github.com/cocov-ci/worker/execute"
	"github.com/cocov-ci/worker/storage"
	"github.com/cocov-ci/worker/support"
)

type ClientOpts struct {
	Socket         string
	CacheServerURL string
	TLSCAPath      string
	TLSKeyPath     string
	TLSCertPath    string
}

type RunInformation struct {
	JobID        string
	Image        string
	RepoName     string
	Commitish    string
	Mounts       map[string]string
	Envs         map[string]string
	SourceVolume *PrepareVolumeResult
	Command      string
}

type CreateContainerResult struct {
	ContainerID  string
	Image        string
	OutputFile   string
	sourceVolume string
}

type PrepareVolumeResult struct {
	VolumeID string
}

type Client interface {
	PullImage(name string) error
	CreateContainer(info *RunInformation) (*CreateContainerResult, error)
	AbortAndRemove(container *CreateContainerResult) error
	ContainerWait(id string) error
	GetContainerResult(id string) (containerOutput *bytes.Buffer, exitStatus int, err error)
	GetContainerOutput(result *CreateContainerResult) (output []byte, err error)
	ContainerStart(id string) error
	RemoveVolume(vol *PrepareVolumeResult) error
	PrepareVolume(zstPath string) (*PrepareVolumeResult, error)
	TerminateContainer(v *CreateContainerResult)
	RequestPrune()
}

func ensureAll(values ...string) bool {
	empty := false
	present := false
	for _, str := range values {
		if str == "" {
			empty = true
		}
		if str != "" {
			present = true
		}
	}

	return (!empty && present) || (empty && !present)
}

func New(opts ClientOpts) (Client, error) {
	clientOpts := []client.Opt{
		client.WithHost(opts.Socket),
		client.WithAPIVersionNegotiation(),
	}

	if !ensureAll(opts.TLSKeyPath, opts.TLSCAPath, opts.TLSCertPath) {
		return nil, fmt.Errorf("either all TLS properties must be provided, or none")
	}

	if opts.TLSCAPath != "" {
		clientOpts = append(clientOpts, client.WithTLSClientConfig(opts.TLSCAPath, opts.TLSCertPath, opts.TLSKeyPath))
	}

	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, err
	}

	c := &clientImpl{
		log:            zap.L().With(zap.String("facility", "docker")),
		d:              cli,
		cacheServerURL: opts.CacheServerURL,
		pruneLock:      &sync.Mutex{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	t, err := c.d.Ping(ctx)
	if err != nil {
		return nil, err
	}

	c.log.Info("Connected to Docker daemon",
		zap.String("api_version", t.APIVersion),
		zap.String("os_type", t.OSType),
		zap.String("builder_version", string(t.BuilderVersion)))

	if c.cacheServerURL != "" {
		c.log.Info("Support to Cache is enabled to Plugins supporting it.", zap.String("cache_server_url", opts.CacheServerURL))
	}

	return c, nil
}

type clientImpl struct {
	log            *zap.Logger
	d              *client.Client
	cacheServerURL string
	pruneLock      *sync.Mutex
}

func (c *clientImpl) PrepareVolume(zstPath string) (*PrepareVolumeResult, error) {
	tmpTar, err := os.CreateTemp("", "")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary file: %w", err)
	}
	_ = tmpTar.Close()
	defer func() {
		_ = os.Remove(tmpTar.Name())
	}()

	volumeName := uuid.Generate().String()
	vol, err := c.d.VolumeCreate(context.Background(), volume.VolumeCreateBody{
		Driver: "local",
		Labels: nil,
		Name:   volumeName,
	})

	if err != nil {
		return nil, fmt.Errorf("failed creating volume: %w", err)
	}

	removeVolume := func() {
		if err := c.d.VolumeRemove(context.Background(), vol.Name, true); err != nil {
			c.log.Warn("Failed removing volume", zap.String("name", vol.Name), zap.Error(err))
		}
	}

	cont, err := c.d.ContainerCreate(context.Background(), &container.Config{
		Image: "alpine",
		Cmd:   []string{"/root/script"},
		Env: []string{
			"EXTRACTOR_FILE_BASENAME=" + filepath.Base(tmpTar.Name()),
		},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: vol.Name,
				Target: "/volume",
			},
		},
	}, nil, nil, "")

	if err != nil {
		removeVolume()
		return nil, fmt.Errorf("failed creating container: %w", err)
	}

	removeContainer := func(volume bool) {
		if err := c.d.ContainerRemove(context.Background(), cont.ID, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			c.log.Warn("Failed removing container", zap.String("id", cont.ID), zap.Error(err))
		}
		if volume {
			removeVolume()
		}
	}

	extractorTar, err := support.Scripts.Open("extractor.tar")
	if err != nil {
		removeContainer(true)
		return nil, fmt.Errorf("error obtaining extractor.tar from embedded FS: %w", err)
	}

	if err = c.d.CopyToContainer(context.Background(), cont.ID, "/", extractorTar, types.CopyToContainerOptions{}); err != nil {
		removeContainer(true)
		return nil, fmt.Errorf("error copying extractor script to container: %w", err)
	}

	err = storage.InflateZstd(zstPath, func(s string) error {
		// tar the tar to make Docker happy
		_, err := execute.Exec([]string{"tar", "-cf", tmpTar.Name(), filepath.Base(s)}, &execute.Opts{
			Cwd: filepath.Dir(s),
		})
		if err != nil {
			return err
		}

		f, err := os.Open(tmpTar.Name())
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		return c.d.CopyToContainer(context.Background(), cont.ID, "/tmp", f, types.CopyToContainerOptions{})
	})

	if err != nil {
		removeContainer(true)
		return nil, fmt.Errorf("failed copying data into container: %w", err)
	}

	if err = c.ContainerStart(cont.ID); err != nil {
		removeContainer(true)
		return nil, fmt.Errorf("failed starting container: %w", err)
	}

	if err = c.ContainerWait(cont.ID); err != nil {
		removeContainer(true)
		return nil, fmt.Errorf("failed waiting for container: %w", err)
	}

	res, status, err := c.GetContainerResult(cont.ID)
	if err != nil {
		removeContainer(true)
		return nil, fmt.Errorf("failed reading container output: %w", err)
	}

	if status != 0 {
		removeContainer(true)
		return nil, fmt.Errorf("container exited with status %d: %s", status, res.String())
	}

	removeContainer(false)

	return &PrepareVolumeResult{VolumeID: volumeName}, nil
}

func (c *clientImpl) PullImage(name string) error {
	reader, err := c.d.ImagePull(context.Background(), name, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	_, err = io.Copy(io.Discard, reader)
	return err
}

func (c *clientImpl) resolveCacheServerURL(rawUrl string) ([]string, error) {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	hostname := parsed.Hostname()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", hostname)

	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipStr := ip.String()
		// IP.String indicates that it returns "<nil>" on zero-length ips, but
		// it seems there's no way to check an IP's length beforehand.
		if ipStr[0] == '<' {
			continue
		}
		result = append(result, hostname+":"+ip.String())
	}

	return result, nil
}

func (c *clientImpl) CreateContainer(info *RunInformation) (*CreateContainerResult, error) {
	workDirTarget := uuid.Generate().String()
	outputTarget := uuid.Generate().String()
	rwVolumeName := uuid.Generate().String()

	var envs []string
	var extraHosts []string
	{
		env := map[string]string{}
		systemEnvs := map[string]string{
			"COCOV_WORKDIR":        "/work/" + workDirTarget,
			"COCOV_REPO_NAME":      info.RepoName,
			"COCOV_COMMIT_SHA":     info.Commitish,
			"COCOV_OUTPUT_FILE":    "/tmp/" + outputTarget,
			"COCOV_JOB_IDENTIFIER": info.JobID,
		}

		if rawUrl := c.cacheServerURL; rawUrl != "" {
			extra, err := c.resolveCacheServerURL(rawUrl)
			if err != nil {
				c.log.Error("Could not resolve cache server URL. Caching will be disabled for this plugin",
					zap.String("job_id", info.JobID),
					zap.String("plugin", info.Image),
					zap.Error(err))
			} else {
				systemEnvs["COCOV_CACHE_SERVICE_URL"] = rawUrl
				extraHosts = extra
			}
		}

		for k, v := range info.Envs {
			env[k] = v
		}

		for k, v := range systemEnvs {
			env[k] = v
		}

		envs = make([]string, 0, len(env))
		for k, v := range env {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
	}

	sourceVolume, err := c.d.VolumeCreate(context.Background(), volume.VolumeCreateBody{
		Driver: "local",
		Name:   rwVolumeName,
	})

	if err != nil {
		c.log.Error("Failed creating new volume for RW source", zap.Error(err))
		return nil, err
	}

	mounts := []mount.Mount{
		{
			Type:     mount.TypeVolume,
			Source:   sourceVolume.Name,
			Target:   "/work/" + workDirTarget,
			ReadOnly: false,
		},
	}

	if err = c.copySourceToLocalVolume(info.SourceVolume, sourceVolume.Name); err != nil {
		c.log.Error("Failed copying source into rw volume", zap.Error(err))
		if err := c.d.VolumeRemove(context.Background(), sourceVolume.Name, true); err != nil {
			c.log.Error("Failed removing rw volume", zap.String("name", sourceVolume.Name), zap.Error(err))
		}
		return nil, err
	}

	var cmd []string
	if info.Command != "" {
		cmd = []string{info.Command}
	}

	res, err := c.d.ContainerCreate(context.Background(), &container.Config{
		Env:   envs,
		Image: info.Image,
		User:  "1000",
		Cmd:   cmd,
	}, &container.HostConfig{
		AutoRemove: false,
		Mounts:     mounts,
		CapDrop:    strslice.StrSlice{"dac_override", "setgid", "setuid", "setpcap", "net_raw", "sys_chroot", "mknod", "audit_write"},
		ExtraHosts: extraHosts,
	}, nil, nil, "")

	if err != nil {
		if err := c.d.VolumeRemove(context.Background(), sourceVolume.Name, true); err != nil {
			c.log.Error("Failed removing rw volume", zap.String("name", sourceVolume.Name), zap.Error(err))
		}
		return nil, err
	}

	c.log.Info("Created container", zap.String("id", res.ID))
	result := &CreateContainerResult{
		ContainerID:  res.ID,
		OutputFile:   "/tmp/" + outputTarget,
		Image:        info.Image,
		sourceVolume: sourceVolume.Name,
	}

	if err = c.copyMountsToContainer(result, info.Mounts); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *clientImpl) copyMountsToContainer(container *CreateContainerResult, mounts map[string]string) error {
	removeContainer := func() {
		if err := c.AbortAndRemove(container); err != nil {
			c.log.Error("Failed removing container after previous error", zap.Error(err))
		}
	}
	buf := bytes.Buffer{}
	for from, to := range mounts {
		log := c.log.With(zap.String("source", from), zap.String("destination", to))
		f, err := os.Open(from)
		if err != nil {
			log.Error("Failed opening mount source", zap.Error(err))
			removeContainer()
			return fmt.Errorf("failed opening mount source '%s': %w", from, err)
		}

		stat, err := f.Stat()
		if err != nil {
			log.Error("Failed stating mount source", zap.Error(err))
			_ = f.Close()
			removeContainer()
			return fmt.Errorf("failed stating mount source '%s': %w", from, err)
		}

		buf.Reset()
		writer := tar.NewWriter(&buf)
		hdr := &tar.Header{
			Name: filepath.Base(to),
			Mode: 0655,
			Size: stat.Size(),
		}
		if err = writer.WriteHeader(hdr); err != nil {
			log.Error("Failed writing tar header for mount", zap.Error(err))
			_ = f.Close()
			removeContainer()
			return fmt.Errorf("failed writing tar header for '%s': %w", from, err)
		}

		if _, err = io.Copy(writer, f); err != nil {
			log.Error("Failed copying mount data into tar writer", zap.Error(err))
			_ = f.Close()
			removeContainer()
			return fmt.Errorf("failed copying mount source '%s' into tar writer: %w", from, err)
		}

		if err = writer.Close(); err != nil {
			log.Error("Failed closing tar stream", zap.Error(err))
			_ = f.Close()
			removeContainer()
			return fmt.Errorf("failed closing tar stream for mount source '%s': %w", from, err)
		}

		_ = f.Close()

		err = c.d.CopyToContainer(context.Background(), container.ContainerID, filepath.Dir(to), &buf, types.CopyToContainerOptions{})
		if err != nil {
			log.Error("Failed copying mount source into container", zap.String("container_id", container.ContainerID), zap.Error(err))
			return fmt.Errorf("failed copying mount source '%s' into container '%s': %w", from, container.ContainerID, err)
		}
	}

	return nil
}

func (c *clientImpl) AbortAndRemove(container *CreateContainerResult) error {
	if err := os.Remove(container.OutputFile); err != nil && !os.IsNotExist(err) {
		c.log.Error("Failed removing outputFile", zap.Error(err), zap.String("container_id", container.ContainerID))
	}

	removeErr := c.d.ContainerRemove(context.Background(), container.ContainerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})

	if err := c.d.VolumeRemove(context.Background(), container.sourceVolume, true); err != nil {
		c.log.Error(
			"Failed removing rw volume for container",
			zap.String("container_id", container.ContainerID),
			zap.String("volume_id", container.sourceVolume),
			zap.Error(err),
		)
	}

	return removeErr
}

func (c *clientImpl) ContainerWait(id string) error {
	statusCh, errCh := c.d.ContainerWait(context.Background(), id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return err
	case <-statusCh:
		return nil
	}
}

func (c *clientImpl) GetContainerResult(id string) (output *bytes.Buffer, exitStatus int, err error) {
	output = &bytes.Buffer{}

	var out io.ReadCloser
	out, err = c.d.ContainerLogs(context.Background(), id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return
	}
	var inspect types.ContainerJSON
	inspect, err = c.d.ContainerInspect(context.Background(), id)
	if err != nil {
		return
	}
	defer func() { _ = out.Close() }()

	exitStatus = inspect.State.ExitCode
	if inspect.State.OOMKilled {
		c.log.Warn("Detected OOM-killed container", zap.String("container_id", id))
	}

	if _, err = stdcopy.StdCopy(output, output, out); err != nil {
		return
	}

	return
}

func (c *clientImpl) GetContainerOutput(result *CreateContainerResult) (output []byte, err error) {
	reader, stat, err := c.d.CopyFromContainer(context.Background(), result.ContainerID, result.OutputFile)

	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()

	if stat.Mode.IsDir() {
		return nil, fmt.Errorf("failed obtaining output: Process generated a directory at the output path, expected a file")
	}

	tr := tar.NewReader(reader)
	ok := false
	buffer := &bytes.Buffer{}
	expectedFile := filepath.Base(result.OutputFile)
	for {
		hdr, err := tr.Next()

		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed iterating tar archive: %w", err)
		}

		if hdr.Name != expectedFile {
			continue
		}

		if _, err := io.Copy(buffer, tr); err != nil {
			return nil, err
		}
		ok = true
		break
	}

	if !ok {
		return nil, fmt.Errorf("could not obtain output file from container: item was not found on tar data")
	}

	return buffer.Bytes(), err
}

func (c *clientImpl) ContainerStart(id string) error {
	return c.d.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
}

func (c *clientImpl) RemoveVolume(vol *PrepareVolumeResult) error {
	return c.d.VolumeRemove(context.Background(), vol.VolumeID, true)
}

func (c *clientImpl) TerminateContainer(v *CreateContainerResult) {
	_ = c.d.ContainerKill(context.Background(), v.ContainerID, "SIGKILL")
}

func (c *clientImpl) copySourceToLocalVolume(sourceVolume *PrepareVolumeResult, rwVolumeName string) error {
	cont, err := c.d.ContainerCreate(context.Background(), &container.Config{
		Image: "alpine",
		Cmd:   []string{"ash", "-c", "cp -ra /src/from/. /src/to && chown -R 1000:1000 /src/to"},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeVolume,
				Source:   sourceVolume.VolumeID,
				Target:   "/src/from",
				ReadOnly: true,
			},
			{
				Type:     mount.TypeVolume,
				Source:   rwVolumeName,
				Target:   "/src/to",
				ReadOnly: false,
			},
		},
	}, nil, nil, "")

	if err != nil {
		return fmt.Errorf("failed creating helper container: %w", err)
	}

	if err = c.d.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed starting helper container: %w", err)
	}

	failed := false
	if err = c.ContainerWait(cont.ID); err != nil {
		c.log.Error("Failed waiting for helper container", zap.Error(err))
		failed = true
	}

	if failed {
		output, status, err := c.GetContainerResult(cont.ID)
		if err != nil {
			c.log.Error("Failed obtaining container result", zap.Error(err))
			return fmt.Errorf("failed running helper container, no output is available")
		}

		return fmt.Errorf("failed running helper container, status %d, output: %s", status, output.String())
	}

	err = c.d.ContainerRemove(context.Background(), cont.ID, types.ContainerRemoveOptions{
		Force: true,
	})

	if err != nil {
		c.log.Error("Failed removing helper container", zap.String("container_id", cont.ID), zap.Error(err))
	}

	return nil
}

func (c *clientImpl) performPrune() {
	defer c.pruneLock.Unlock()

	listFilters := filters.NewArgs()
	listFilters.Add("dangling", "true")

	list, err := c.d.ImageList(context.Background(), types.ImageListOptions{
		All:     true,
		Filters: listFilters,
	})
	if err != nil {
		c.log.Error("Failed listing dangling images", zap.Error(err))
	} else {
		for _, i := range list {
			_, err = c.d.ImageRemove(context.Background(), i.ID, types.ImageRemoveOptions{
				Force:         false,
				PruneChildren: true,
			})
			if err != nil {
				c.log.Warn("Failed removing dangling image", zap.String("image_id", i.ID), zap.Error(err))
			}
		}
	}

	listFilters = filters.NewArgs()
	listFilters.Add("until", "7 days")
	listFilters.Add("dangling", "false")
	list, err = c.d.ImageList(context.Background(), types.ImageListOptions{
		All:     true,
		Filters: listFilters,
	})
	if err != nil {
		c.log.Error("Failed listing stale images", zap.Error(err))
	} else {
		for _, i := range list {
			_, err = c.d.ImageRemove(context.Background(), i.ID, types.ImageRemoveOptions{
				Force:         false,
				PruneChildren: true,
			})
			if err != nil {
				c.log.Warn("Failed removing stale image", zap.String("image_id", i.ID), zap.Error(err))
			}
		}
	}

	c.log.Info("Finished image prune")
}

func (c *clientImpl) RequestPrune() {
	if !c.pruneLock.TryLock() {
		return
	}

	// we got the lock, prune away
	go c.performPrune()
}
