package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"kanban/core/utils"
	"path/filepath"
	"strings"
)

type addedLine struct {
	FilePath string
	Content  string
}

type diffJSONEntry struct {
	File      string `json:"file"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"`
}

func extractAddedLinesFromDiff(diffText string) []addedLine {
	if strings.TrimSpace(diffText) == "" {
		return nil
	}

	var jsonDiff []diffJSONEntry
	if err := json.Unmarshal([]byte(diffText), &jsonDiff); err == nil && len(jsonDiff) > 0 {
		hasExpected := false
		for _, d := range jsonDiff {
			if d.File != "" || d.After != "" || d.Before != "" || d.Additions > 0 || d.Deletions > 0 {
				hasExpected = true
				break
			}
		}
		if hasExpected {
			return extractFromJSONDiff(jsonDiff)
		}
	}

	if strings.Contains(diffText, "<<< BEFORE") && strings.Contains(diffText, ">>> AFTER") {
		return extractFromBeforeAfterDiff(diffText)
	}

	return extractFromUnifiedDiff(diffText)
}

func extractFromUnifiedDiff(diffText string) []addedLine {
	var result []addedLine
	var currentFile string

	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = line[6:]
			continue
		}
		if strings.HasPrefix(line, "+++ /dev/null") {
			currentFile = ""
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			content := line[1:]
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				result = append(result, addedLine{FilePath: currentFile, Content: trimmed})
			}
		}
	}
	return result
}

func extractFromJSONDiff(jsonDiff []diffJSONEntry) []addedLine {
	var result []addedLine
	for _, d := range jsonDiff {
		if d.After == "" {
			continue
		}
		beforeLines := make(map[string]bool)
		if d.Before != "" {
			for _, line := range strings.Split(d.Before, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					beforeLines[trimmed] = true
				}
			}
		}
		for _, line := range strings.Split(d.After, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !beforeLines[trimmed] {
				result = append(result, addedLine{FilePath: d.File, Content: trimmed})
			}
		}
	}
	return result
}

func extractFromBeforeAfterDiff(diffText string) []addedLine {
	var result []addedLine
	var currentFile string
	var beforeContent, afterContent strings.Builder
	var inBefore, inAfter bool

	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "--- ") {
			if afterContent.Len() > 0 || beforeContent.Len() > 0 {
				result = append(result, computeDiffAddedLines(currentFile, beforeContent.String(), afterContent.String())...)
				beforeContent.Reset()
				afterContent.Reset()
			}
			currentFile = strings.TrimSpace(line[4:])
			inBefore = false
			inAfter = false
			continue
		}
		if strings.TrimSpace(line) == "<<< BEFORE" {
			inBefore = true
			inAfter = false
			continue
		}
		if strings.TrimSpace(line) == ">>> AFTER" {
			inBefore = false
			inAfter = true
			continue
		}
		if inBefore {
			beforeContent.WriteString(line)
			beforeContent.WriteByte('\n')
		} else if inAfter {
			afterContent.WriteString(line)
			afterContent.WriteByte('\n')
		}
	}
	if afterContent.Len() > 0 || beforeContent.Len() > 0 {
		result = append(result, computeDiffAddedLines(currentFile, beforeContent.String(), afterContent.String())...)
	}
	return result
}

func computeDiffAddedLines(filePath, before, after string) []addedLine {
	beforeLines := make(map[string]bool)
	for _, line := range strings.Split(before, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			beforeLines[trimmed] = true
		}
	}
	var result []addedLine
	for _, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !beforeLines[trimmed] {
			result = append(result, addedLine{FilePath: filePath, Content: trimmed})
		}
	}
	return result
}

func calcLineFingerprint(al addedLine) string {
	path := filepath.Base(al.FilePath)
	hash := sha256.Sum256([]byte(utils.RemoveWhitespace(path + al.Content)))
	return hex.EncodeToString(hash[:])
}
