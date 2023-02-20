package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type RunInformation struct {
	Image        string
	RepoName     string
	Commitish    string
	Mounts       map[string]string
	Envs         map[string]string
	SourceVolume *PrepareVolumeResult
	Command      string
}

type CreateContainerResult struct {
	ContainerID string
	Image       string
	OutputFile  string
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
	PrepareVolume(brotliPath string) (*PrepareVolumeResult, error)
	TerminateContainer(v *CreateContainerResult)
}

func New(host string) (Client, error) {
	cli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	c := &clientImpl{
		log: zap.L().With(zap.String("facility", "docker")),
		d:   cli,
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
	return c, nil
}

type clientImpl struct {
	log *zap.Logger
	d   *client.Client
}

func (c *clientImpl) PrepareVolume(brotliPath string) (*PrepareVolumeResult, error) {
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

	err = storage.InflateBrotli(brotliPath, func(s string) error {
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

func (c *clientImpl) CreateContainer(info *RunInformation) (*CreateContainerResult, error) {
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	if err = tempFile.Close(); err != nil {
		return nil, err
	}

	workDirTarget := uuid.Generate().String()
	outputTarget := uuid.Generate().String()

	var envs []string
	{
		env := map[string]string{}
		systemEnvs := map[string]string{
			"COCOV_WORKDIR":     "/work/" + workDirTarget,
			"COCOV_REPO_NAME":   info.RepoName,
			"COCOV_COMMIT_SHA":  info.Commitish,
			"COCOV_OUTPUT_FILE": "/tmp/" + outputTarget,
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

	mounts := make([]mount.Mount, 0, len(info.Mounts)+1)
	mounts = append(mounts, mount.Mount{
		Type:     mount.TypeVolume,
		Source:   info.SourceVolume.VolumeID,
		Target:   "/work/" + workDirTarget,
		ReadOnly: true,
	})

	for from, to := range info.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   from,
			Target:   to,
			ReadOnly: true,
		})
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
	}, nil, nil, "")

	if err != nil {
		return nil, err
	}

	c.log.Info("Created container", zap.String("id", res.ID))
	return &CreateContainerResult{
		ContainerID: res.ID,
		OutputFile:  "/tmp/" + outputTarget,
		Image:       info.Image,
	}, nil
}

func (c *clientImpl) AbortAndRemove(container *CreateContainerResult) error {
	if err := os.Remove(container.OutputFile); err != nil && !os.IsNotExist(err) {
		c.log.Error("Failed removing outputFile", zap.Error(err), zap.String("container_id", container.ContainerID))
	}

	return c.d.ContainerRemove(context.Background(), container.ContainerID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
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
