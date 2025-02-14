package carrot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type AvgUsage struct {
	Threshold time.Duration
	StartAt   time.Time
	vals      []time.Duration
}

type Worker[T any] struct {
	queue     chan T
	usage     AvgUsage
	Name      string
	Num       int
	QueueSize int
	Handler   func(T) error
	DumpStats func(queueSize, doneCount int, avgUsage time.Duration)
}

func (at *AvgUsage) GetCount() int {
	if time.Since(at.StartAt) >= at.Threshold {
		at.StartAt = time.Now()
		at.vals = nil
	}
	return len(at.vals)
}

func (at *AvgUsage) Add(v time.Duration) {
	if time.Since(at.StartAt) >= at.Threshold {
		at.StartAt = time.Now()
		at.vals = nil
	}
	at.vals = append(at.vals, v)
}

func (at *AvgUsage) CountPerMinute() float64 {
	us := time.Since(at.StartAt)
	if us <= 1*time.Minute {
		return float64(len(at.vals))
	}
	// get counts per minute
	return float64(len(at.vals)) / float64(us.Minutes())
}

func (at *AvgUsage) Get() time.Duration {
	var x time.Duration = 0
	if len(at.vals) <= 0 {
		return 0
	}

	for _, v := range at.vals {
		x += v
	}
	return time.Duration(float64(x) / float64(len(at.vals)))
}

func (w *Worker[T]) Start(ctx context.Context) {
	if w.QueueSize <= 0 {
		return
	}

	if int64(w.usage.Threshold) <= 0 {
		w.usage.Threshold = 1 * time.Minute
	}

	w.queue = make(chan T, w.QueueSize)
	wg := sync.WaitGroup{}
	wg.Add(w.Num + 1)

	go func() {
		t := time.NewTicker(w.usage.Threshold)
		defer t.Stop()
		defer close(w.queue)
		wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if w.DumpStats != nil {
					w.DumpStats(len(w.queue), w.usage.GetCount(), w.usage.Get())
				} else {
					logrus.WithFields(logrus.Fields{
						"worker": w.Name,
						"num":    w.Num,
						"queue":  len(w.queue),
						"size":   w.QueueSize,
						"avg":    w.usage.Get(),
						"count":  w.usage.GetCount(),
					}).Info("worker: stats")
				}
			}
		}
	}()

	for i := 0; i < w.Num; i++ {
		go func() {
			wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case req := <-w.queue:
					st := time.Now()
					w.Handler(req)
					w.usage.Add(time.Since(st))
				}
			}
		}()
	}
	wg.Wait()

	//Info("worker started name:", w.Name, "num:", w.Num, "queue size:", w.QueueSize)
	logrus.WithFields(logrus.Fields{
		"worker": w.Name,
		"num":    w.Num,
		"queue":  w.QueueSize,
	}).Info("worker: started")
}

func (w *Worker[T]) Push(req T) error {
	if w.Handler == nil {
		return fmt.Errorf("worker: %s handler is nil", w.Name)
	}

	if w.queue == nil {
		st := time.Now()
		w.Handler(req)
		w.usage.Add(time.Since(st))
		return nil
	}

	select {
	case w.queue <- req:
		return nil
	default:
		return fmt.Errorf("worker: %s queue is full", w.Name)
	}
}
