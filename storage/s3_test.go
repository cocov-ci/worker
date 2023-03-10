package storage

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/cocov-ci/worker/test_helpers"
)

var env = map[string]string{
	"AWS_ACCESS_KEY_ID":                  "minioadmin",
	"AWS_SECRET_ACCESS_KEY":              "minioadmin",
	"AWS_REGION":                         "us-east-1",
	"GIT_SERVICE_S3_STORAGE_BUCKET_NAME": "vito-cocov-storage",
	"COCOV_WORKER_S3_ENDPOINT":           "http://localhost:9000",
}

func prepareS3(t *testing.T, fn func(t *testing.T)) {
	t.Cleanup(func() {
		for k := range env {
			err := os.Unsetenv(k)
			assert.NoError(t, err)
		}
	})
	for k, v := range env {
		err := os.Setenv(k, v)
		require.NoError(t, err)
	}

	zap.ReplaceGlobals(zap.NewNop())

	fn(t)
}

func TestS3Download(t *testing.T) {
	tmp, err := os.CreateTemp("", "")
	require.NoError(t, err)

	prepareS3(t, func(t *testing.T) {
		s, err := NewS3("cocov-storage")
		require.NoError(t, err)

		err = s.(S3Storage).download("32a6fcbaa4543f0718079837a574f5835f3143fe/9cff62ad797c372277f6c6b71d10e643947b5340.tar.zst.shasum", tmp)
		assert.NoError(t, err)
		assert.NoError(t, tmp.Close())
		data, err := os.ReadFile(tmp.Name())
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(string(data), "sha256:"))
	})
}

func TestS3Storage_DownloadCommit(t *testing.T) {
	tmp := test_helpers.TmpPath(t)

	prepareS3(t, func(t *testing.T) {
		s, err := NewS3("cocov-storage")
		require.NoError(t, err)

		err = s.DownloadCommit("repo", "9cff62ad797c372277f6c6b71d10e643947b5340", tmp)
		assert.NoError(t, err)
	})
}
