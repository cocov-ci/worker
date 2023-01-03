package docker

import (
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"testing"
)

func makeClient(t *testing.T) *clientImpl {
	zap.ReplaceGlobals(zap.NewNop())
	c, err := New("unix:///var/run/docker.sock")
	require.NoError(t, err)
	return c.(*clientImpl)
}

func TestDockerPullImage(t *testing.T) {
	c := makeClient(t)
	err := c.PullImage("alpine")
	require.NoError(t, err)
}

func TestCreateContainer(t *testing.T) {
	work, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	defer os.RemoveAll(work)

	r := RunInformation{
		Image:     "alpine",
		WorkDir:   work,
		RepoName:  "hello",
		Commitish: "test",
	}

	c := makeClient(t)
	result, err := c.CreateContainer(&r)
	require.NoError(t, err)
	require.NotNil(t, result)

	err = c.ContainerStart(result.ContainerID)
	require.NoError(t, err)

	err = c.ContainerWait(result.ContainerID)
	require.NoError(t, err)

	output, exitStatus, err := c.GetContainerResult(result.ContainerID)
	require.Nil(t, err)
	require.Empty(t, output.Bytes())
	require.Zero(t, exitStatus)

	err = c.AbortAndRemove(result)
	require.NoError(t, err)
}
