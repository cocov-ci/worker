package redis

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cocov-ci/worker/test_helpers"
)

func makeClient(t *testing.T) *client {
	c, err := New("redis://localhost:6379/3")
	require.NoError(t, err)
	return c.(*client)
}

func TestPing(t *testing.T) {
	c := makeClient(t)
	err := c.Ping()
	require.NoError(t, err)
}

func TestClient(t *testing.T) {
	c := makeClient(t)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		c.Start()
		wg.Done()
	}()

	t.Run("push invalid data to dead queue", func(t *testing.T) {
		c.c.RPush(context.Background(), "cocov:checks", "heeeeeyoooooo")
		data, err := json.Marshal(Job{
			JobID:      "foo",
			Org:        "bar",
			Repo:       "baz",
			Commitish:  "buz",
			Checks:     nil,
			GitStorage: GitStorage{},
		})
		require.NoError(t, err)
		c.c.RPush(context.Background(), "cocov:checks", data)
		j := c.Next()
		require.NotNil(t, j)
		assert.Equal(t, "foo", j.JobID)
		s, err := c.c.LPop(context.Background(), "cocov:checks:dead").Result()
		require.NoError(t, err)
		assert.Equal(t, "heeeeeyoooooo", s)
	})

	t.Run("stop", func(t *testing.T) {
		test_helpers.Timeout(t, 3*time.Second, func() {
			c.Stop()
			c.Wait()
			wg.Wait()
		})
	})
}
