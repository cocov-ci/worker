package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/cocov-ci/worker/test_helpers"
)

func TestNewLocal(t *testing.T) {
	t.Run("When base path does not exist", func(t *testing.T) {
		tmp, err := os.CreateTemp("", "")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())
		require.NoError(t, os.RemoveAll(tmp.Name()))

		b, err := NewLocal(tmp.Name())
		assert.Nil(t, b)
		require.ErrorContains(t, err, "no such file or directory")
	})

	t.Run("When base path is not a directory", func(t *testing.T) {
		tmp, err := os.CreateTemp("", "")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())
		t.Cleanup(func() { _ = os.RemoveAll(tmp.Name()) })

		b, err := NewLocal(tmp.Name())
		assert.Nil(t, b)
		require.ErrorContains(t, err, "not a directory")
	})

	t.Run("When base path is a directory", func(t *testing.T) {
		tmp, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(tmp) })
		zap.ReplaceGlobals(zap.NewNop())

		b, err := NewLocal(tmp)
		assert.NotNil(t, b)
		require.NoError(t, err)
	})
}

func makeLocalStorage(t *testing.T) LocalStorage {
	root := test_helpers.GitRoot(t)
	b, err := NewLocal(filepath.Join(root, "storage", "fixtures", "fake-s3"))
	assert.NotNil(t, b)
	require.NoError(t, err)

	return b.(LocalStorage)
}

func TestLocalStorage_RepositoryPath(t *testing.T) {
	local := makeLocalStorage(t)
	base := local.basePath
	current := local.RepositoryPath("repo")
	expected := filepath.Join(base, "32a6fcbaa4543f0718079837a574f5835f3143fe")
	assert.Equal(t, expected, current)
}

func TestLocalStorage_CommitPath(t *testing.T) {
	local := makeLocalStorage(t)
	base := local.basePath
	current := local.CommitPath("repo", "sha")
	expected := filepath.Join(base, "32a6fcbaa4543f0718079837a574f5835f3143fe", "sha")
	assert.Equal(t, expected, current)
}

func TestDownloadCommit(t *testing.T) {
	local := makeLocalStorage(t)
	target := test_helpers.TmpPath(t)

	err := local.DownloadCommit("repo", "9cff62ad797c372277f6c6b71d10e643947b5340", target)
	assert.NoError(t, err)
}
