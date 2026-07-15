package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxQueueDefault = 64
	maxQueueP0      = 128

	// DefaultStopTimeout 是 Stop 等待 worker 退出的最长时长；超时后放弃并返回，避免永久阻塞。
	DefaultStopTimeout = 5 * time.Second
)

// stopFnGrace：Stop 取消任务后，等待 Fn 自行返回的宽限时间；超时则遗弃该 Fn goroutine。
// 测试可临时调小。软抢占（非 stopping）在宽限后仍会等到 Fn 结束。
var stopFnGrace = 500 * time.Millisecond

// TaskFunc 是可调度任务体；应响应 ctx 取消（尤其 Cancelable 任务）。
type TaskFunc func(ctx context.Context) error

// Task 表示一次可入队执行的工作单元。
type Task struct {
	ID         string
	Name       string
	Priority   Priority
	Cancelable bool
	Fn         TaskFunc
}

// SchedulerStats 供观测：队列深度与当前运行任务。
type SchedulerStats struct {
	Started    bool
	Running    string
	Queued     int
	ByPriority [5]int
}

type queuedTask struct {
	task Task
}

type runningState struct {
	name       string
	priority   Priority
	cancelable bool
	cancel     context.CancelFunc
}

// TaskScheduler 单 worker 优先级队列：P0 先于 P4；可对 Cancelable 任务软抢占。
// Stop 在 DefaultStopTimeout 内返回；忽略 cancel 的 Fn 在 stopFnGrace 后被遗弃，不再卡住 Stop。
type TaskScheduler struct {
	mu sync.Mutex

	registered []TaskMeta
	queues     [5][]*queuedTask
	pending    map[string]struct{}

	running *runningState

	started  bool
	stopping bool
	gen      uint64

	wake   chan struct{}
	stopCh chan struct{}
	doneCh chan struct{}

	baseCtx    context.Context
	baseCancel context.CancelFunc
	seq        atomic.Uint64
}

// Default 全局实例（App 启动时 Start，退出时 Stop）。
var Default = &TaskScheduler{}

func queueLimit(p Priority) int {
	if p == P0ExecSafety {
		return maxQueueP0
	}
	return maxQueueDefault
}

// Register 记录任务元数据；不启动执行。
func (s *TaskScheduler) Register(meta TaskMeta) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registered = append(s.registered, meta)
}

// Registered 返回已登记任务的副本。
func (s *TaskScheduler) Registered() []TaskMeta {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.registered) == 0 {
		return nil
	}
	out := make([]TaskMeta, len(s.registered))
	copy(out, s.registered)
	return out
}

// Start 启动单 worker；幂等。
func (s *TaskScheduler) Start(parent context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	s.baseCtx = ctx
	s.baseCancel = cancel
	s.wake = make(chan struct{}, 1)
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.pending = make(map[string]struct{})
	s.started = true
	s.stopping = false
	s.gen++
	gen := s.gen
	stopCh := s.stopCh
	wake := s.wake
	doneCh := s.doneCh
	go s.loop(gen, stopCh, wake, doneCh)
}

// Stop 停止 worker，等价于 StopWithTimeout(DefaultStopTimeout)。
func (s *TaskScheduler) Stop() {
	s.StopWithTimeout(DefaultStopTimeout)
}

// StopWithTimeout 取消运行中任务并等待 worker 退出；超时后仍清理状态并返回，避免永久阻塞。
// 若 Fn 忽略 cancel，宽限 stopFnGrace 后遗弃该 Fn goroutine；晚到的旧 worker 通过 gen 失效，不会污染新生命周期。
func (s *TaskScheduler) StopWithTimeout(timeout time.Duration) {
	if s == nil {
		return
	}
	if timeout <= 0 {
		timeout = DefaultStopTimeout
	}

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	if s.stopping {
		done := s.doneCh
		s.mu.Unlock()
		s.waitDone(done, timeout)
		return
	}
	s.stopping = true
	done := s.doneCh
	if s.baseCancel != nil {
		s.baseCancel()
	}
	if s.running != nil && s.running.cancel != nil {
		s.running.cancel()
	}
	close(s.stopCh)
	s.mu.Unlock()

	s.waitDone(done, timeout)

	s.mu.Lock()
	// 无论 worker 是否已退出，推进 gen，使迟到的旧循环失效。
	s.gen++
	s.clearQueuesLocked()
	s.started = false
	s.stopping = false
	s.running = nil
	s.baseCtx = nil
	s.baseCancel = nil
	s.wake = nil
	s.stopCh = nil
	s.doneCh = nil
	s.mu.Unlock()
}

func (s *TaskScheduler) waitDone(done chan struct{}, timeout time.Duration) {
	if done == nil {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

// Submit 入队任务。同名已在队列或运行中则拒绝（accepted=false）。
func (s *TaskScheduler) Submit(task Task) (accepted bool, err error) {
	if s == nil {
		return false, errors.New("nil scheduler")
	}
	if task.Fn == nil {
		return false, errors.New("nil task Fn")
	}
	if task.Name == "" {
		return false, errors.New("empty task name")
	}
	if task.Priority < P0ExecSafety || task.Priority > P4Research {
		return false, fmt.Errorf("invalid priority: %d", task.Priority)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.stopping {
		return false, errors.New("scheduler not started")
	}
	if _, ok := s.pending[task.Name]; ok {
		return false, nil
	}
	if s.running != nil && s.running.name == task.Name {
		return false, nil
	}

	limit := queueLimit(task.Priority)
	idx := int(task.Priority)
	if len(s.queues[idx]) >= limit {
		return false, fmt.Errorf("queue full for %s", task.Priority.String())
	}

	if task.ID == "" {
		task.ID = fmt.Sprintf("%s-%d", task.Name, s.seq.Add(1))
	}

	s.queues[idx] = append(s.queues[idx], &queuedTask{task: task})
	s.pending[task.Name] = struct{}{}
	s.ensureRegisteredLocked(TaskMeta{
		Name:       task.Name,
		Priority:   task.Priority,
		Cancelable: task.Cancelable,
	})

	if s.running != nil &&
		s.running.cancelable &&
		ShouldYieldToHigher(s.running.priority, task.Priority) &&
		s.running.cancel != nil {
		s.running.cancel()
	}

	s.signalWakeLocked()
	return true, nil
}

// Stats 返回当前队列与运行快照。
func (s *TaskScheduler) Stats() SchedulerStats {
	var st SchedulerStats
	if s == nil {
		return st
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st.Started = s.started && !s.stopping
	if s.running != nil {
		st.Running = s.running.name
	}
	for i := 0; i < 5; i++ {
		n := len(s.queues[i])
		st.ByPriority[i] = n
		st.Queued += n
	}
	return st
}

func (s *TaskScheduler) ensureRegisteredLocked(meta TaskMeta) {
	for _, r := range s.registered {
		if r.Name == meta.Name {
			return
		}
	}
	s.registered = append(s.registered, meta)
}

func (s *TaskScheduler) signalWakeLocked() {
	if s.wake == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *TaskScheduler) clearQueuesLocked() {
	for i := range s.queues {
		s.queues[i] = nil
	}
	s.pending = make(map[string]struct{})
}

func (s *TaskScheduler) popHighestLocked() *queuedTask {
	for i := 0; i < 5; i++ {
		if len(s.queues[i]) == 0 {
			continue
		}
		qt := s.queues[i][0]
		s.queues[i] = s.queues[i][1:]
		return qt
	}
	return nil
}

func (s *TaskScheduler) loop(gen uint64, stopCh, wake, doneCh chan struct{}) {
	defer close(doneCh)
	for {
		select {
		case <-stopCh:
			return
		case <-wake:
		}

		for {
			s.mu.Lock()
			if s.gen != gen || s.stopping {
				s.mu.Unlock()
				return
			}
			qt := s.popHighestLocked()
			if qt == nil {
				s.mu.Unlock()
				break
			}
			task := qt.task
			parent := s.baseCtx
			if parent == nil {
				parent = context.Background()
			}
			runCtx, cancel := context.WithCancel(parent)
			s.running = &runningState{
				name:       task.Name,
				priority:   task.Priority,
				cancelable: task.Cancelable,
				cancel:     cancel,
			}
			s.mu.Unlock()

			s.execTask(runCtx, task, gen)
			cancel()

			s.mu.Lock()
			if s.gen != gen {
				s.mu.Unlock()
				return
			}
			delete(s.pending, task.Name)
			s.running = nil
			stopping := s.stopping
			s.mu.Unlock()
			if stopping {
				return
			}
		}
	}
}

func (s *TaskScheduler) execTask(ctx context.Context, task Task, gen uint64) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("scheduler task panic name=%s: %v", task.Name, r)
			}
		}()
		if err := task.Fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("scheduler task error name=%s: %v", task.Name, err)
		}
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
	}

	grace := stopFnGrace
	if grace < 0 {
		grace = 0
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-timer.C:
	}

	s.mu.Lock()
	abandon := s.stopping || s.gen != gen
	s.mu.Unlock()
	if abandon {
		// 遗弃仍在运行的 Fn，保证 Stop 可继续推进 worker 退出。
		return
	}
	// 软抢占：继续等待 Fn，避免同一代 worker 并发执行下一任务。
	<-done
}
