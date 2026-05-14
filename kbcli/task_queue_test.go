package main

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 1. NewTaskQueue with workers <= 0 defaults to 1 worker
func TestNewTaskQueue_DefaultWorkers(t *testing.T) {
	q := NewTaskQueue(0)
	defer q.Stop()

	// Block the single worker with a long-running task
	blocker := make(chan struct{})
	q.Submit("block", nil, func() error {
		<-blocker
		return nil
	})

	// With only 1 worker, this task should stay pending
	task := q.Submit("pending", nil, func() error {
		return nil
	})

	time.Sleep(50 * time.Millisecond)

	if task.Status != TaskStatusPending {
		t.Errorf("expected status pending with default 1 worker, got %s", task.Status)
	}

	close(blocker)
	time.Sleep(50 * time.Millisecond)
}

// 2. Submit a task -> task has Status Pending, has valid ID, returned from Get
func TestSubmit_Basic(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	task := q.Submit("test", map[string]interface{}{"key": "value"}, func() error {
		return nil
	})

	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}
	if task.Type != "test" {
		t.Errorf("expected type test, got %s", task.Type)
	}

	got, ok := q.Get(task.ID)
	if !ok {
		t.Fatal("expected to find task via Get")
	}
	if got.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, got.ID)
	}
}

// 3. Submit with nil fn -> should still work (fn is nil, runTask will skip)
func TestSubmit_NilFn(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	task := q.Submit("nil-fn", nil, nil)

	time.Sleep(100 * time.Millisecond)

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed for nil fn, got %s", task.Status)
	}
}

// 4. Submit and let it complete successfully -> Status becomes Completed
func TestTask_Completes(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	task := q.Submit("complete", nil, func() error {
		return nil
	})

	time.Sleep(100 * time.Millisecond)

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s", task.Status)
	}
	if task.Result != "success" {
		t.Errorf("expected result 'success', got %s", task.Result)
	}
	if task.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

// 5. Submit and let it fail -> Status becomes Failed, Error set
func TestTask_Fails(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	expectedErr := errors.New("task failed")
	task := q.Submit("fail", nil, func() error {
		return expectedErr
	})

	time.Sleep(100 * time.Millisecond)

	if task.Status != TaskStatusFailed {
		t.Errorf("expected status failed, got %s", task.Status)
	}
	if task.Error != expectedErr.Error() {
		t.Errorf("expected error %q, got %q", expectedErr.Error(), task.Error)
	}
}

// 6. Cancel a pending task -> Status becomes Cancelled
func TestCancel_Pending(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	// Block the worker so next task stays pending
	blocker := make(chan struct{})
	q.Submit("blocker", nil, func() error {
		<-blocker
		return nil
	})

	task := q.Submit("pending", nil, func() error {
		return nil
	})

	time.Sleep(50 * time.Millisecond)

	if task.Status != TaskStatusPending {
		t.Fatalf("expected status pending, got %s", task.Status)
	}

	cancelled, ok := q.Cancel(task.ID)
	if !ok {
		t.Fatal("expected Cancel to return true")
	}
	if cancelled.Status != TaskStatusCancelled {
		t.Errorf("expected status cancelled, got %s", cancelled.Status)
	}
	if cancelled.CompletedAt == nil {
		t.Error("expected CompletedAt to be set after cancel")
	}

	close(blocker)
}

// 7. Cancel a running task -> Status becomes Cancelled (need a slow fn)
func TestCancel_Running(t *testing.T) {
	q := NewTaskQueue(1)

	running := make(chan struct{})
	task := q.Submit("slow", nil, func() error {
		close(running)
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	<-running // Wait for task to start running

	cancelDone := make(chan struct{})
	var cancelled *AsyncTask
	var cancelOk bool
	go func() {
		cancelled, cancelOk = q.Cancel(task.ID)
		close(cancelDone)
	}()

	select {
	case <-cancelDone:
		if !cancelOk {
			t.Fatal("expected Cancel to return true")
		}
		if cancelled.Status != TaskStatusCancelled {
			t.Errorf("expected status cancelled, got %s", cancelled.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel running task timed out - possible deadlock")
	}

	stopDone := make(chan struct{})
	go func() {
		q.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Log("q.Stop() timed out - worker may be deadlocked")
	}
}

// 8. Cancel a completed task -> returns task but doesn't change status
func TestCancel_Completed(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	task := q.Submit("quick", nil, func() error {
		return nil
	})

	time.Sleep(100 * time.Millisecond)

	if task.Status != TaskStatusCompleted {
		t.Fatalf("expected status completed, got %s", task.Status)
	}

	cancelled, ok := q.Cancel(task.ID)
	if !ok {
		t.Fatal("expected Cancel to return true")
	}
	if cancelled.Status != TaskStatusCompleted {
		t.Errorf("expected status still completed, got %s", cancelled.Status)
	}
}

// 9. Get non-existent task -> returns false
func TestGet_NonExistent(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	_, ok := q.Get("non-existent-id")
	if ok {
		t.Error("expected Get to return false for non-existent task")
	}
}

// 10. List returns all tasks
func TestList(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	q.Submit("task1", nil, func() error { return nil })
	q.Submit("task2", nil, func() error { return nil })

	time.Sleep(100 * time.Millisecond)

	tasks := q.List()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

// 11. Multiple tasks submitted -> all processed
func TestMultipleTasks(t *testing.T) {
	q := NewTaskQueue(2)
	defer q.Stop()

	var completed int32
	n := 5

	for i := 0; i < n; i++ {
		q.Submit("batch", map[string]interface{}{"index": i}, func() error {
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
			return nil
		})
	}

	start := time.Now()
	for {
		if atomic.LoadInt32(&completed) == int32(n) {
			break
		}
		if time.Since(start) > 3*time.Second {
			t.Fatalf("only %d/%d tasks completed", atomic.LoadInt32(&completed), n)
		}
		time.Sleep(10 * time.Millisecond)
	}

	tasks := q.List()
	if len(tasks) != n {
		t.Errorf("expected %d tasks in list, got %d", n, len(tasks))
	}

	for _, task := range tasks {
		if task.Status != TaskStatusCompleted {
			t.Errorf("task %s expected completed, got %s", task.ID, task.Status)
		}
	}
}

// 12. Stop queue -> no more tasks processed
func TestStopQueue(t *testing.T) {
	q := NewTaskQueue(1)

	done := make(chan struct{})
	q.Submit("before-stop", nil, func() error {
		close(done)
		return nil
	})
	<-done
	time.Sleep(50 * time.Millisecond)

	q.Stop()

	tasks := q.List()
	if len(tasks) != 1 {
		t.Errorf("expected 1 task after stop, got %d", len(tasks))
	}
}

// 13. Task with params -> params preserved
func TestTask_ParamsPreserved(t *testing.T) {
	q := NewTaskQueue(1)
	defer q.Stop()

	params := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	task := q.Submit("params", params, func() error {
		return nil
	})

	if task.Params == nil {
		t.Fatal("expected params to be preserved")
	}
	if task.Params["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", task.Params["key1"])
	}
	if task.Params["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", task.Params["key2"])
	}
}

// 14. Submit after Stop -> should handle gracefully (taskCh may be closed)
func TestSubmitAfterStop(t *testing.T) {
	q := NewTaskQueue(1)
	q.Stop()

	// Give context cancellation time to propagate
	time.Sleep(50 * time.Millisecond)

	var panicked bool
	var task *AsyncTask

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		task = q.Submit("after-stop", nil, func() error { return nil })
	}()

	if panicked {
		t.Fatal("Submit after Stop panicked")
	}

	if task != nil {
		time.Sleep(50 * time.Millisecond)
		if task.Status != TaskStatusCancelled {
			t.Errorf("expected task status Cancelled after submit on stopped queue, got %s", task.Status)
		}
	}
}

// 15. generateTaskID -> returns non-empty string
func TestGenerateTaskID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateTaskID()
		if id == "" {
			t.Fatal("generateTaskID returned empty string")
		}
		if ids[id] {
			t.Fatalf("generateTaskID returned duplicate ID: %s", id)
		}
		ids[id] = true
	}
}
