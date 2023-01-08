package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"github.com/docker/distribution/uuid"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"time"
)

type RunInformation struct {
	Image     string
	WorkDir   string
	RepoName  string
	Commitish string
	Mounts    map[string]string
	Envs      map[string]string
}

type CreateContainerResult struct {
	ContainerID string
	Image       string
	OutputFile  string
}

type Client interface {
	PullImage(name string) error
	CreateContainer(info *RunInformation) (*CreateContainerResult, error)
	AbortAndRemove(container *CreateContainerResult) error
	ContainerWait(id string) error
	GetContainerResult(id string) (containerOutput *bytes.Buffer, exitStatus int, err error)
	GetContainerOutput(result *CreateContainerResult) (output []byte, err error)
	ContainerStart(id string) error
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
		Type:     mount.TypeBind,
		Source:   info.WorkDir,
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

	res, err := c.d.ContainerCreate(context.Background(), &container.Config{
		Env:   envs,
		Image: info.Image,
		User:  "1000",
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
