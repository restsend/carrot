package carrot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAvgUsage(t *testing.T) {
	avg := AvgUsage{
		Threshold: 100 * time.Millisecond,
		StartAt:   time.Now(),
	}
	avg.Add(100 * time.Millisecond)
	avg.Add(2 * time.Second)
	assert.Equal(t, 2, avg.GetCount())
	assert.Equal(t, time.Duration(1050000000), avg.Get())

	time.Sleep(150 * time.Millisecond)
	avg.Add(100 * time.Millisecond)

	assert.Equal(t, 1, avg.GetCount())
	assert.Equal(t, 100*time.Millisecond, avg.Get())
	assert.GreaterOrEqual(t, 1.0, avg.CountPerMinute())

	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, 0, avg.GetCount())
}

func TestWorker(t *testing.T) {
	w := Worker[int]{
		Name:      "test",
		QueueSize: 1,
		Num:       1,
		usage: AvgUsage{
			Threshold: 50 * time.Millisecond,
		},
	}
	w.Handler = func(i int) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	err := w.Push(1)
	assert.Nil(t, err)
	err = w.Push(2)
	assert.Nil(t, err)

	err = w.Push(3)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "queue is full")

	time.Sleep(100 * time.Millisecond) // test avg usage
	cancel()
	time.Sleep(100 * time.Millisecond) // wait for worker to exit
}

func TestWorkerBadInit(t *testing.T) {
	w := Worker[int]{
		Name:      "test",
		QueueSize: 1,
		Num:       1,
	}
	err := w.Push(1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "test handler is nil")
	w.Handler = func(i int) error {
		return nil
	}
	err = w.Push(1)
	assert.Nil(t, err)
}
