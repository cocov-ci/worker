package docker

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
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
	err := c.PullImage("cocov/dummy:v0.1")
	require.NoError(t, err)
}

func TestCreateContainer(t *testing.T) {
	work, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	defer os.RemoveAll(work)

	r := RunInformation{
		Image:     "cocov/dummy:v0.1",
		WorkDir:   work,
		RepoName:  "%#COCOV_WORKER_DOCKER_TEST",
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
	require.Equal(t, []byte("Success.\n"), output.Bytes())
	require.Zero(t, exitStatus)

	data, err := c.GetContainerOutput(result)
	require.NoError(t, err)

	// Content should be parseable JSON
	data = bytes.TrimSuffix(data, []byte{0x00})
	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &obj))
	assert.Equal(t, "foo", obj["file"].(string))

	err = c.AbortAndRemove(result)
	require.NoError(t, err)
}
