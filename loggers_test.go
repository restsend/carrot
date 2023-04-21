package carrot

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	mockData := bytes.NewBufferString("")
	log.Default().SetOutput(mockData)
	SetLogLevel(LevelDebug)
	Debug("debug")
	assert.Contains(t, mockData.String(), "[D] debug")
	Info("info")
	assert.Contains(t, mockData.String(), "[I] info")
	Warnning("warnning")
	assert.Contains(t, mockData.String(), "[W] warnning")
	Error("error")
	assert.Contains(t, mockData.String(), "[E] error")
}
