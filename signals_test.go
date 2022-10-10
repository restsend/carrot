package carrot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignal(t *testing.T) {
	var val string
	eid := Sig().Connect("mock_test", func(sender interface{}, params ...interface{}) {
		val = sender.(string)
	})
	Sig().Emit("mock_test", "unittest")
	assert.Equal(t, val, "unittest")
	Sig().Disconnect("mock_test", eid)
}
