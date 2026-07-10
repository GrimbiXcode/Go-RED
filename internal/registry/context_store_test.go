package registry

import (
    "sync"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestContextStore(t *testing.T) {
    t.Run("Get on an empty store returns false", func(t *testing.T) {
        s := NewContextStore()
        _, ok := s.Get("missing")
        assert.False(t, ok)
    })

    t.Run("Set then Get returns the stored value", func(t *testing.T) {
        s := NewContextStore()
        s.Set("counter", 42)

        v, ok := s.Get("counter")
        assert.True(t, ok)
        assert.Equal(t, 42, v)
    })

    t.Run("Set overwrites an existing value", func(t *testing.T) {
        s := NewContextStore()
        s.Set("key", "first")
        s.Set("key", "second")

        v, _ := s.Get("key")
        assert.Equal(t, "second", v)
    })

    t.Run("Delete removes a key", func(t *testing.T) {
        s := NewContextStore()
        s.Set("key", "value")
        s.Delete("key")

        _, ok := s.Get("key")
        assert.False(t, ok)
    })

    t.Run("Delete on a missing key is a no-op", func(t *testing.T) {
        s := NewContextStore()
        s.Delete("missing")
    })

    t.Run("Keys returns every stored key", func(t *testing.T) {
        s := NewContextStore()
        s.Set("a", 1)
        s.Set("b", 2)

        assert.ElementsMatch(t, []string{"a", "b"}, s.Keys())
    })

    t.Run("is safe for concurrent use", func(t *testing.T) {
        s := NewContextStore()
        var wg sync.WaitGroup
        for i := 0; i < 50; i++ {
            wg.Add(1)
            go func(i int) {
                defer wg.Done()
                s.Set("key", i)
                s.Get("key")
                s.Keys()
            }(i)
        }
        wg.Wait()
    })
}
