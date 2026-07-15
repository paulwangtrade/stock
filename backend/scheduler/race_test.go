package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// 这些用例主要配合 `go test -race` 验证 Start/Stop/Submit 并发安全性。

func TestTaskSchedulerConcurrentSubmitStopRace(t *testing.T) {
	oldGrace := stopFnGrace
	stopFnGrace = 5 * time.Millisecond
	defer func() { stopFnGrace = oldGrace }()

	s := &TaskScheduler{}
	s.Start(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				name := fmt.Sprintf("job-%d-%d", id, j%5)
				_, _ = s.Submit(Task{
					Name:       name,
					Priority:   Priority(j % 5),
					Cancelable: j%2 == 0,
					Fn: func(ctx context.Context) error {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(time.Millisecond):
							return nil
						}
					},
				})
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		s.StopWithTimeout(200 * time.Millisecond)
	}()

	wg.Wait()
	_ = s.Stats()
}

func TestTaskSchedulerStopStartRace(t *testing.T) {
	oldGrace := stopFnGrace
	stopFnGrace = 5 * time.Millisecond
	defer func() { stopFnGrace = oldGrace }()

	s := &TaskScheduler{}
	for round := 0; round < 30; round++ {
		s.Start(context.Background())

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, _ = s.Submit(Task{
					Name:       fmt.Sprintf("r%d-%d", round, i%3),
					Priority:   P3WatchlistScan,
					Cancelable: true,
					Fn: func(ctx context.Context) error {
						time.Sleep(time.Millisecond)
						return nil
					},
				})
			}
		}()
		go func() {
			defer wg.Done()
			time.Sleep(2 * time.Millisecond)
			s.StopWithTimeout(100 * time.Millisecond)
		}()
		wg.Wait()

		// 立即重启，覆盖旧 worker 与新生命周期交错。
		s.Start(context.Background())
		_, _ = s.Submit(Task{
			Name:     "after-restart",
			Priority: P0ExecSafety,
			Fn: func(ctx context.Context) error {
				return nil
			},
		})
		waitIdle(t, s, time.Second)
		s.StopWithTimeout(100 * time.Millisecond)
	}
}

func TestTaskSchedulerStubbornFnStopStartRace(t *testing.T) {
	oldGrace := stopFnGrace
	stopFnGrace = 10 * time.Millisecond
	defer func() { stopFnGrace = oldGrace }()

	s := &TaskScheduler{}
	s.Start(context.Background())

	started := make(chan struct{})
	_, err := s.Submit(Task{
		Name:     "ignore-cancel",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			close(started)
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	waitRunning(t, s, "ignore-cancel", time.Second)

	s.StopWithTimeout(50 * time.Millisecond)
	s.Start(context.Background())
	accepted, err := s.Submit(Task{
		Name:     "next-gen",
		Priority: P1PositionQuote,
		Fn:       func(ctx context.Context) error { return nil },
	})
	if err != nil || !accepted {
		t.Fatalf("next-gen submit: accepted=%v err=%v", accepted, err)
	}
	waitIdle(t, s, time.Second)
	s.StopWithTimeout(100 * time.Millisecond)
}
