package models

import (
	"os"
	"path/filepath"
	"strings"
)

type PathConfig struct {
	AnalysedDir string
	TaskDir     string
}

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

func GetSummaryFilePath(taskDir string, task *Task) (string, bool) {
	yyyy, mm, dd, ok := GetDatePartsFromTask(task, "summary")
	if !ok {
		return "", false
	}
	filePath := filepath.Join(taskDir, "summary", yyyy, mm, dd, task.SessionId+".json")
	if _, err := os.Stat(filePath); err == nil {
		return filePath, true
	}
	return "", false
}

func GetConversationFilePath(taskDir string, task *Task) (string, bool) {
	yyyy, mm, dd, ok := GetDatePartsFromTask(task, "conversation")
	if !ok {
		return "", false
	}
	filePath := filepath.Join(taskDir, "conversation", yyyy, mm, dd, task.SessionId+".jsonl")
	if _, err := os.Stat(filePath); err == nil {
		return filePath, true
	}
	return "", false
}
