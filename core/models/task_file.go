package models

import (
	"strings"

	"kanban/core/storage"
)

func GetDatePartsFromTask(task *Task, typ string) (yyyy, mm, dd string, ok bool) {
	var dateStr string
	if typ == "summary" {
		dateStr = task.SessionDate
	} else {
		dateStr = task.ConversationDate
	}
	if dateStr == "" {
		return "", "", "", false
	}
	parts := strings.Split(dateStr, "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// 返回值：路径、是否存在、存储故障。存储故障（S3 端点不可达/凭证失效/限流等）
// 必须区别于"文件不存在"上抛，否则 S3 中断期间会被伪装成 404 且无日志。
func GetSummaryFilePath(taskDir string, task *Task) (string, bool, error) {
	yyyy, mm, dd, ok := GetDatePartsFromTask(task, "summary")
	if !ok {
		return "", false, nil
	}
	filePath := storage.Join(taskDir, "summary", yyyy, mm, dd, task.SessionId+".json")
	if _, err := storage.Stat(filePath); err != nil {
		if storage.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return filePath, true, nil
}

func GetConversationFilePath(taskDir string, task *Task) (string, bool, error) {
	yyyy, mm, dd, ok := GetDatePartsFromTask(task, "conversation")
	if !ok {
		return "", false, nil
	}
	filePath := storage.Join(taskDir, "conversation", yyyy, mm, dd, task.SessionId+".jsonl")
	if _, err := storage.Stat(filePath); err != nil {
		if storage.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return filePath, true, nil
}
