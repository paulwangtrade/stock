package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPriorityOrderAndYield(t *testing.T) {
	if P0ExecSafety >= P1PositionQuote || P3WatchlistScan >= P4Research {
		t.Fatal("priority numeric order broken")
	}
	if !ShouldYieldToHigher(P3WatchlistScan, P0ExecSafety) {
		t.Fatal("P3 should yield to P0")
	}
	if ShouldYieldToHigher(P0ExecSafety, P4Research) {
		t.Fatal("P0 must not yield to P4")
	}
	if P0ExecSafety.String() != "P0-exec-safety" {
		t.Fatalf("unexpected label: %s", P0ExecSafety.String())
	}
}

func TestTaskSchedulerRegister(t *testing.T) {
	s := &TaskScheduler{}
	s.Register(TaskMeta{Name: "MonitorStockPrices", Priority: P1PositionQuote})
	s.Register(TaskMeta{Name: "watchlistScan", Priority: P3WatchlistScan, Cancelable: true})
	got := s.Registered()
	if len(got) != 2 || got[0].Name != "MonitorStockPrices" {
		t.Fatalf("register failed: %+v", got)
	}
}

func waitRunning(t *testing.T, s *TaskScheduler, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.Stats().Running == name {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for running=%s stats=%+v", name, s.Stats())
}

func waitIdle(t *testing.T, s *TaskScheduler, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st := s.Stats()
		if st.Running == "" && st.Queued == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting idle stats=%+v", s.Stats())
}

func TestTaskSchedulerPriorityOrder(t *testing.T) {
	s := &TaskScheduler{}
	s.Start(context.Background())
	defer s.Stop()

	hold := make(chan struct{})
	var mu sync.Mutex
	order := make([]string, 0, 3)

	accepted, err := s.Submit(Task{
		Name:     "hold",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			select {
			case <-hold:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if !accepted || err != nil {
		t.Fatalf("hold submit: accepted=%v err=%v", accepted, err)
	}
	waitRunning(t, s, "hold", 2*time.Second)

	if _, err := s.Submit(Task{
		Name:       "late-p4",
		Priority:   P4Research,
		Cancelable: true,
		Fn: func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "p4")
			mu.Unlock()
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit(Task{
		Name:     "late-p0",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "p0")
			mu.Unlock()
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	close(hold)
	waitIdle(t, s, 2*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != "p0" || order[1] != "p4" {
		t.Fatalf("expected order [p0 p4], got %v", order)
	}
}

func TestTaskSchedulerSoftPreempt(t *testing.T) {
	s := &TaskScheduler{}
	s.Start(context.Background())
	defer s.Stop()

	canceled := make(chan struct{}, 1)
	p0Done := make(chan struct{}, 1)

	accepted, err := s.Submit(Task{
		Name:       "research",
		Priority:   P4Research,
		Cancelable: true,
		Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				canceled <- struct{}{}
				return ctx.Err()
			case <-time.After(3 * time.Second):
				t.Error("p4 should have been canceled")
				return nil
			}
		},
	})
	if !accepted || err != nil {
		t.Fatalf("p4 submit: accepted=%v err=%v", accepted, err)
	}
	waitRunning(t, s, "research", 2*time.Second)

	accepted, err = s.Submit(Task{
		Name:     "risk-p0",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			p0Done <- struct{}{}
			return nil
		},
	})
	if !accepted || err != nil {
		t.Fatalf("p0 submit: accepted=%v err=%v", accepted, err)
	}

	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected cancelable p4 to observe ctx cancel")
	}
	select {
	case <-p0Done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected p0 to run after preempt")
	}
	waitIdle(t, s, 2*time.Second)
}

func TestTaskSchedulerDedup(t *testing.T) {
	s := &TaskScheduler{}
	s.Start(context.Background())
	defer s.Stop()

	start := make(chan struct{})
	var runs int
	var mu sync.Mutex

	fn := func(ctx context.Context) error {
		<-start
		mu.Lock()
		runs++
		mu.Unlock()
		return nil
	}

	ok1, err := s.Submit(Task{Name: "scan", Priority: P3WatchlistScan, Cancelable: true, Fn: fn})
	if !ok1 || err != nil {
		t.Fatalf("first submit: %v %v", ok1, err)
	}
	waitRunning(t, s, "scan", 2*time.Second)

	ok2, err := s.Submit(Task{Name: "scan", Priority: P3WatchlistScan, Cancelable: true, Fn: fn})
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Fatal("duplicate name should be rejected while running")
	}

	close(start)
	waitIdle(t, s, 2*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if runs != 1 {
		t.Fatalf("expected 1 run, got %d", runs)
	}
}

func TestTaskSchedulerParentContextCancel(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	s := &TaskScheduler{}
	s.Start(parent)
	defer s.Stop()

	gotCancel := make(chan struct{}, 1)
	accepted, err := s.Submit(Task{
		Name:     "ctx-link",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				gotCancel <- struct{}{}
				return ctx.Err()
			case <-time.After(3 * time.Second):
				t.Error("task ctx should be canceled via parent")
				return nil
			}
		},
	})
	if !accepted || err != nil {
		t.Fatalf("submit: accepted=%v err=%v", accepted, err)
	}
	waitRunning(t, s, "ctx-link", 2*time.Second)

	cancelParent()

	select {
	case <-gotCancel:
	case <-time.After(2 * time.Second):
		t.Fatal("expected parent cancel to propagate into task ctx")
	}
	waitIdle(t, s, 2*time.Second)
}

func TestTaskSchedulerStopCancelsRunningCtx(t *testing.T) {
	s := &TaskScheduler{}
	s.Start(context.Background())

	gotCancel := make(chan struct{}, 1)
	accepted, err := s.Submit(Task{
		Name:     "stop-ctx",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				gotCancel <- struct{}{}
				return ctx.Err()
			case <-time.After(3 * time.Second):
				t.Error("task ctx should be canceled by Stop")
				return nil
			}
		},
	})
	if !accepted || err != nil {
		t.Fatalf("submit: accepted=%v err=%v", accepted, err)
	}
	waitRunning(t, s, "stop-ctx", 2*time.Second)

	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-gotCancel:
	case <-time.After(2 * time.Second):
		t.Fatal("expected Stop to cancel running task ctx")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop blocked after task observed cancel")
	}
}

func TestTaskSchedulerStopUnblocksWhenFnIgnoresCancel(t *testing.T) {
	oldGrace := stopFnGrace
	stopFnGrace = 30 * time.Millisecond
	defer func() { stopFnGrace = oldGrace }()

	s := &TaskScheduler{}
	s.Start(context.Background())

	started := make(chan struct{})
	accepted, err := s.Submit(Task{
		Name:     "stubborn",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			close(started)
			// 故意忽略 ctx，模拟永久阻塞任务。
			time.Sleep(3 * time.Second)
			return nil
		},
	})
	if !accepted || err != nil {
		t.Fatalf("submit: accepted=%v err=%v", accepted, err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("stubborn task did not start")
	}
	waitRunning(t, s, "stubborn", 2*time.Second)

	begin := time.Now()
	s.StopWithTimeout(500 * time.Millisecond)
	elapsed := time.Since(begin)
	if elapsed > 400*time.Millisecond {
		t.Fatalf("Stop blocked too long: %v", elapsed)
	}
	if s.Stats().Started {
		t.Fatal("scheduler should be stopped")
	}
}

func TestTaskSchedulerStopDropsQueued(t *testing.T) {
	s := &TaskScheduler{}
	s.Start(context.Background())

	hold := make(chan struct{})
	ran := make(chan struct{}, 1)

	if _, err := s.Submit(Task{
		Name:     "blocker",
		Priority: P0ExecSafety,
		Fn: func(ctx context.Context) error {
			select {
			case <-hold:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}); err != nil {
		t.Fatal(err)
	}
	waitRunning(t, s, "blocker", 2*time.Second)

	if _, err := s.Submit(Task{
		Name:       "queued-p4",
		Priority:   P4Research,
		Cancelable: true,
		Fn: func(ctx context.Context) error {
			ran <- struct{}{}
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	s.Stop()
	close(hold)

	select {
	case <-ran:
		t.Fatal("queued task must not run after Stop")
	case <-time.After(200 * time.Millisecond):
	}

	if s.Stats().Started {
		t.Fatal("scheduler should report not started after Stop")
	}
}
