package main

type TaskIdSet struct {
	tasks map[string]bool //任务ID的集合
}

func NewTaskIdSet() *TaskIdSet {
	return &TaskIdSet{
		tasks: make(map[string]bool),
	}
}

// 获取tasks中所有ID的列表
func (ts *TaskIdSet) GetTaskIds() []string {
	tasks := []string{}
	for tid := range ts.tasks {
		tasks = append(tasks, tid)
	}
	return tasks
}
