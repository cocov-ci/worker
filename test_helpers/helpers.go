package test_helpers

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cocov-ci/worker/execute"
)

func Timeout(t *testing.T, d time.Duration, fn func()) {
	ok := make(chan bool)
	go func() {
		fn()
		close(ok)
	}()

	select {
	case <-ok:
		return
	case <-time.After(d):
		t.Error("Run time exceeded")
	}
}

func GitRoot(t *testing.T) string {
	out, err := execute.Exec([]string{"git", "rev-parse", "--show-toplevel"}, nil)
	require.NoError(t, err)
	return strings.TrimSpace(out.String())
}

func TmpPath(t *testing.T) string {
	n, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(n))
	t.Cleanup(func() { _ = os.RemoveAll(n) })
	return n
}
