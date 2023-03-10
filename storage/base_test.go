package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cocov-ci/worker/test_helpers"
)

func Test_shasum(t *testing.T) {
	assert.Equal(t, "8843d7f92416211de9ebb963ff4ce28125932878", shasum("foobar"))
}

// testing repositoryPath indirectly here
func Test_commitPath(t *testing.T) {
	assert.Equal(t, "/foo/8843d7f92416211de9ebb963ff4ce28125932878/shasha",
		commitPath("/foo", "foobar", "shasha"))
}

func Test_validateSha(t *testing.T) {
	root := filepath.Join(test_helpers.GitRoot(t), "storage", "fixtures", "shasum_test")
	itemMissing := filepath.Join(root, "item.nope")
	shasumMissing := filepath.Join(root, "item.nope")
	itemDir := filepath.Join(root, "item.dir")
	shasumDir := filepath.Join(root, "shasum.dir")
	itemBin := filepath.Join(root, "item.bin")
	shasumInvalid := filepath.Join(root, "shasum.invalid")
	shasumValid := filepath.Join(root, "shasum.valid")

	t.Run("when item does not exist", func(t *testing.T) {
		err := validateSha(shasumValid, itemMissing)
		assert.True(t, os.IsNotExist(err))
	})
	t.Run("when item is a directory", func(t *testing.T) {
		err := validateSha(shasumValid, itemDir)
		assert.ErrorContains(t, err, "is a directory")
	})

	t.Run("when shasum does not exist", func(t *testing.T) {
		err := validateSha(shasumMissing, itemBin)
		assert.True(t, os.IsNotExist(err))
	})
	t.Run("when shasum is a directory", func(t *testing.T) {
		err := validateSha(shasumDir, itemBin)
		assert.ErrorContains(t, err, "is a directory")
	})

	t.Run("when sha does not match", func(t *testing.T) {
		err := validateSha(shasumInvalid, itemBin)
		shaErr := err.(SHASumError)
		assert.Equal(t, "Local copy with digest 13e94f4ee9f8e911ac33beb70e30637a069ff465aeea73b8cc0726db7a7f9735 does not match expected digest 13e94f4ee9f8e911ac33beb70e30637a069ff465aeea73b8cc0726db7a7f9734", shaErr.Output)
		assert.Equal(t, 1, shaErr.Status)
	})
	t.Run("when sha does match", func(t *testing.T) {
		err := validateSha(shasumValid, itemBin)
		assert.NoError(t, err)
	})
}

func TestInflateBrotli(t *testing.T) {
	root := test_helpers.GitRoot(t)
	tmpdir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	err = InflateZstd(filepath.Join(root, "storage", "fixtures", "test.tar.zst"), func(s string) error {
		return untar(s, tmpdir)
	})
	require.NoError(t, err)

	f, err := os.ReadFile(filepath.Join(tmpdir, "test.txt"))
	require.NoError(t, err)

	assert.Equal(t, "This is a test", string(f))
}
