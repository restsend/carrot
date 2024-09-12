package carrot

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandText(t *testing.T) {
	v := RandText(10)
	assert.Equal(t, len(v), 10)

	v2 := RandNumberText(5)
	assert.Equal(t, len(v2), 5)
	_, err := strconv.ParseInt(v2, 10, 64)
	assert.Nil(t, err)
}

func TestFormatSizeHuman(t *testing.T) {
	v := FormatSizeHuman(1024)
	assert.Equal(t, v, "1.0 KB")

	v = FormatSizeHuman(1024 * 1024)
	assert.Equal(t, v, "1.0 MB")

	v = FormatSizeHuman(1024*100 + 100)
	assert.Equal(t, v, "100.1 KB")

	v = FormatSizeHuman(1024 * 1024 * 1024)
	assert.Equal(t, v, "1.0 GB")
}
