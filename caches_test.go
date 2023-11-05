package carrot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCaches(t *testing.T) {
	c := NewExpiredLRUCache[string, string](10, 100*time.Millisecond)
	c.Add("a", "b")
	_, ok := c.Get("a")
	assert.True(t, ok)
	time.Sleep(150 * time.Millisecond)
	_, ok = c.Get("a")
	assert.False(t, ok)
}
