package models

import (
	"strings"

	"kanban/core/rawdump"
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

// ResolveConversation 定位某任务的 conversation 数据，自动兼容「旧单文件 <id>.jsonl」与
// 「新目录分片 <id>/00000N.jsonl」两种布局，返回可重组的 ConversationRef。
// 返回值：ref、是否存在、存储故障。存储故障（S3 端点不可达/凭证失效/限流等）必须区别于
// "文件不存在"上抛，否则 S3 中断期间会被伪装成 404 且无日志。
func ResolveConversation(taskDir string, task *Task) (rawdump.ConversationRef, bool, error) {
	yyyy, mm, dd, ok := GetDatePartsFromTask(task, "conversation")
	if !ok {
		return rawdump.ConversationRef{}, false, nil
	}
	dateDir := storage.Join(taskDir, "conversation", yyyy, mm, dd)
	return rawdump.Resolve(dateDir, task.SessionId)
}
