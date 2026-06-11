package service

import (
	"fmt"
	"sync"
)

// JudgeTask represents a submission to be judged.
type JudgeTask struct {
	OperatorID  string
	ZipPath     string
	ProblemID   uint
	TimeLimitSec int
}

// TaskQueue manages background judging tasks with concurrent worker goroutines.
type TaskQueue struct {
	tasks   chan JudgeTask
	workers int
	mu      sync.Mutex
}

// NewTaskQueue creates a new task queue with the specified number of workers.
func NewTaskQueue(numWorkers int) *TaskQueue {
	tq := &TaskQueue{
		tasks:   make(chan JudgeTask, 100), // Buffer up to 100 tasks
		workers: numWorkers,
	}
	tq.start()
	return tq
}

// start begins the worker goroutines.
func (tq *TaskQueue) start() {
	for i := 0; i < tq.workers; i++ {
		go tq.worker(i)
	}
	fmt.Printf("[Queue] Started %d judge workers\n", tq.workers)
}

// worker processes tasks from the queue.
func (tq *TaskQueue) worker(id int) {
	for task := range tq.tasks {
		fmt.Printf("[Queue] Worker %d: processing %s (problem %d)\n", id, task.OperatorID, task.ProblemID)
		StartJudge(task.OperatorID, task.ZipPath, task.ProblemID, task.TimeLimitSec)
		fmt.Printf("[Queue] Worker %d: finished %s\n", id, task.OperatorID)
	}
}

// Enqueue adds a task to the queue (non-blocking, returns immediately).
func (tq *TaskQueue) Enqueue(task JudgeTask) error {
	select {
	case tq.tasks <- task:
		fmt.Printf("[Queue] Enqueued task %s\n", task.OperatorID)
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// Close gracefully shuts down the queue and waits for pending tasks.
func (tq *TaskQueue) Close() {
	close(tq.tasks)
	fmt.Printf("[Queue] Closed task queue\n")
}

// Global task queue instance (initialized on startup).
var GlobalQueue *TaskQueue

// InitQueue initializes the global task queue with numWorkers.
func InitQueue(numWorkers int) {
	GlobalQueue = NewTaskQueue(numWorkers)
}
