package runner

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConcurrentMap(t *testing.T) {
	m := NewConcurrentMap[string, string]()

	t.Run("set", func(t *testing.T) {
		m.Set("foo", "bar")
	})

	t.Run("get", func(t *testing.T) {
		x := m.Get("foo")
		assert.Equal(t, "bar", x)
	})

	t.Run("maybe get", func(t *testing.T) {
		x, ok := m.MaybeGet("foo")
		assert.True(t, ok)
		assert.Equal(t, "bar", x)

		x, ok = m.MaybeGet("fuz")
		assert.False(t, ok)
		assert.Zero(t, x)
	})

	t.Run("contains", func(t *testing.T) {
		assert.True(t, m.Contains("foo"))
		assert.False(t, m.Contains("fuz"))
	})

	t.Run("keys", func(t *testing.T) {
		k := m.Keys()
		assert.Len(t, k, 1)
		assert.Equal(t, "foo", k[0])
	})

	t.Run("map", func(t *testing.T) {
		m := m.Map()
		assert.Equal(t, "bar", m["foo"])
	})

	t.Run("reset", func(t *testing.T) {
		m.Reset()
		assert.False(t, m.Contains("foo"))
	})

}
