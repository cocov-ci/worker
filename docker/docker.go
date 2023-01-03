package docker

import (
	"bytes"
	"context"
	"github.com/docker/distribution/uuid"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
	"io"
	"os"
	"time"
)

type RunInformation struct {
	Image     string
	WorkDir   string
	RepoName  string
	Commitish string
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
	outputFilePath := tempFile.Name()

	res, err := c.d.ContainerCreate(context.Background(), &container.Config{
		Env: []string{
			"COCOV_WORKDIR=/work/" + workDirTarget,
			"COCOV_REPO_NAME=" + info.RepoName,
			"COCOV_COMMIT_SHA=" + info.Commitish,
			"COCOV_OUTPUT_FILE=/tmp/" + outputTarget,
		},
		Image: info.Image,
	}, &container.HostConfig{
		AutoRemove: false,
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   info.WorkDir,
				Target:   "/work/" + workDirTarget,
				ReadOnly: true,
			},
			{
				Type:     mount.TypeBind,
				Source:   outputFilePath,
				Target:   "/tmp/" + outputTarget,
				ReadOnly: false,
			},
		},
	}, nil, nil, "")

	if err != nil {
		_ = os.Remove(outputFilePath)
		return nil, err
	}

	c.log.Info("Created container", zap.String("id", res.ID), zap.String("output_file", outputFilePath))
	return &CreateContainerResult{
		ContainerID: res.ID,
		OutputFile:  outputFilePath,
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

func (c *clientImpl) ContainerStart(id string) error {
	return c.d.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
}
