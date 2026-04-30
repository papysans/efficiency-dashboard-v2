package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TaskStatus 异步任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// AsyncTask 异步任务
type AsyncTask struct {
	ID          string                 `json:"task_id"`
	Type        string                 `json:"type"`
	Status      TaskStatus             `json:"status"`
	Params      map[string]interface{} `json:"params"`
	Result      string                 `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ctx         context.Context
	cancelFunc  context.CancelFunc
	mu          sync.RWMutex
}

// TaskQueue 异步任务队列
type TaskQueue struct {
	tasks   map[string]*AsyncTask
	taskCh  chan string
	mu      sync.RWMutex
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewTaskQueue 创建新的任务队列
func NewTaskQueue(workers int) *TaskQueue {
	ctx, cancel := context.WithCancel(context.Background())
	if workers <= 0 {
		workers = 1
	}
	q := &TaskQueue{
		tasks:   make(map[string]*AsyncTask),
		taskCh:  make(chan string, 100),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}

// Stop 停止任务队列
func (q *TaskQueue) Stop() {
	q.cancel()
	close(q.taskCh)
	q.wg.Wait()
}

func generateTaskID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// Submit 提交新任务
func (q *TaskQueue) Submit(taskType string, params map[string]interface{}) *AsyncTask {
	taskID := generateTaskID()
	taskCtx, cancel := context.WithCancel(q.ctx)
	task := &AsyncTask{
		ID:         taskID,
		Type:       taskType,
		Status:     TaskStatusPending,
		Params:     params,
		CreatedAt:  time.Now(),
		ctx:        taskCtx,
		cancelFunc: cancel,
	}

	q.mu.Lock()
	q.tasks[taskID] = task
	q.mu.Unlock()

	select {
	case q.taskCh <- taskID:
	case <-taskCtx.Done():
		q.mu.Lock()
		task.Status = TaskStatusCancelled
		now := time.Now()
		task.CompletedAt = &now
		q.mu.Unlock()
	}

	return task
}

// Get 通过ID查询任务
func (q *TaskQueue) Get(taskID string) (*AsyncTask, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	task, ok := q.tasks[taskID]
	return task, ok
}

// List 列举所有任务
func (q *TaskQueue) List() []*AsyncTask {
	q.mu.RLock()
	defer q.mu.RUnlock()
	tasks := make([]*AsyncTask, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// Cancel 取消任务
func (q *TaskQueue) Cancel(taskID string) (*AsyncTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	task, ok := q.tasks[taskID]
	if !ok {
		return nil, false
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusCancelled {
		return task, true
	}

	if task.cancelFunc != nil {
		task.cancelFunc()
	}
	task.Status = TaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now

	return task, true
}

func (q *TaskQueue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.ctx.Done():
			return
		case taskID, ok := <-q.taskCh:
			if !ok {
				return
			}
			q.runTask(taskID)
		}
	}
}

func (q *TaskQueue) runTask(taskID string) {
	q.mu.RLock()
	task, ok := q.tasks[taskID]
	q.mu.RUnlock()
	if !ok {
		return
	}

	task.mu.Lock()
	if task.Status != TaskStatusPending {
		task.mu.Unlock()
		return
	}
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	taskCtx := task.ctx
	task.mu.Unlock()

	// 监听取消信号
	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		runErr = executeTaskByType(task.Type, task.Params)
	}()

	select {
	case <-done:
		task.mu.Lock()
		if task.Status == TaskStatusRunning {
			if runErr != nil {
				task.Status = TaskStatusFailed
				task.Error = runErr.Error()
			} else {
				task.Status = TaskStatusCompleted
				task.Result = "success"
			}
			completedAt := time.Now()
			task.CompletedAt = &completedAt
		}
		task.mu.Unlock()
	case <-taskCtx.Done():
		// 任务被取消
		task.mu.Lock()
		task.Status = TaskStatusCancelled
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		task.mu.Unlock()
		<-done // 等待goroutine完成
	}
}

// executeTaskByType 根据任务类型执行对应的命令逻辑
func executeTaskByType(taskType string, params map[string]interface{}) error {
	switch taskType {
	case "import-task":
		return executeImportTask(params)
	case "import-repo":
		return executeImportRepo(params)
	case "import-org":
		return executeImportOrg(params)
	case "silica":
		return executeSilica(params)
	case "efficiency":
		return executeEfficiency(params)
	default:
		return fmt.Errorf("未知任务类型: %s", taskType)
	}
}

func getStringParam(params map[string]interface{}, key string, defaultVal string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1"
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return defaultVal
}

func executeImportTask(params map[string]interface{}) error {
	taskDir := getStringParam(params, "task_dir", cfg.TaskDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	return runImportTask(taskDir, analysedDir, force)
}

func executeImportRepo(params map[string]interface{}) error {
	repoDir := getStringParam(params, "repo_dir", cfg.RepoDir)
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	return runImportRepo(repoDir, analysedDir, force)
}

func executeImportOrg(params map[string]interface{}) error {
	fromDB := getStringParam(params, "from_db", cfg.OrgDSN)
	fromCSV := getStringParam(params, "from_csv", "")
	toCSV := getStringParam(params, "to_csv", "")
	return runImportOrg(fromDB, fromCSV, toCSV)
}

func executeSilica(params map[string]interface{}) error {
	analysedDir := getStringParam(params, "analysed_dir", cfg.AnalysedDir)
	force := getBoolParam(params, "force", false)
	return runSilica(analysedDir, force)
}

func executeEfficiency(params map[string]interface{}) error {
	date := getStringParam(params, "date", "")
	return runEfficiency(date)
}

// MarshalJSON 自定义序列化，避免死锁
func (t *AsyncTask) MarshalJSON() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	type Alias AsyncTask
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	})
}
