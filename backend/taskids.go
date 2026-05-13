package main

import "encoding/json"

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

// 把一组StatCommit中的TaskIds解析出来，合并到tasks中
func (ts *TaskIdSet) MergeCommitsTasks(commits []StatCommit) {
	for _, cm := range commits {
		var ids []string
		if len(cm.TaskIds) > 0 && string(cm.TaskIds) != "null" && string(cm.TaskIds) != "[]" {
			json.Unmarshal(cm.TaskIds, &ids)
		}
		for _, id := range ids {
			ts.tasks[id] = true
		}
	}
}

func (ts *TaskIdSet) MergeCommitMapTasks(commits map[string]*StatCommit) {
	for _, cm := range commits {
		var ids []string
		if len(cm.TaskIds) > 0 && string(cm.TaskIds) != "null" && string(cm.TaskIds) != "[]" {
			json.Unmarshal(cm.TaskIds, &ids)
		}
		for _, id := range ids {
			ts.tasks[id] = true
		}
	}
}

func (ts *TaskIdSet) MergeTaskIds(idstr []byte) {
	if len(idstr) > 0 && string(idstr) != "null" && string(idstr) != "[]" {
		var ids []string
		if err := json.Unmarshal(idstr, &ids); err == nil {
			for _, id := range ids {
				ts.tasks[id] = true
			}
		}
	}
}
