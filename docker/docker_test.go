package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/cocov-ci/worker/support"
	"github.com/cocov-ci/worker/test_helpers"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"path/filepath"
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
	c := makeClient(t)
	volume, err := c.PrepareVolume(filepath.Join(test_helpers.GitRoot(t), "storage", "fixtures", "test.tar.br"))
	require.NoError(t, err)

	r := RunInformation{
		Image:        "alpine",
		SourceVolume: volume,
		RepoName:     "%#COCOV_WORKER_DOCKER_TEST",
		Commitish:    "test",
		Command:      "/tmp/script",
	}

	result, err := c.CreateContainer(&r)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Needs dummy script
	dummy, err := support.Scripts.Open("dummy.tar")
	require.NoError(t, err)

	err = c.d.CopyToContainer(context.Background(), result.ContainerID, "/", dummy, types.CopyToContainerOptions{})
	require.NoError(t, err)

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

func TestClientImpl_PrepareVolume(t *testing.T) {
	root := filepath.Join(test_helpers.GitRoot(t), "storage", "fixtures", "test.tar.br")
	c := makeClient(t)
	vol, err := c.PrepareVolume(root)
	require.NoError(t, err)
	require.NoError(t, c.RemoveVolume(vol))
}
