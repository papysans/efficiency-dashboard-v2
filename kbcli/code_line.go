package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"kanban/core/utils"
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

	// 输入是合法 JSON-diff 数组（结构化格式）时一律按 JSON 解析返回——即使为空 / 全空字段
	// 也返回其结果（可能 0 行），绝不把 JSON 文本本身当裸代码兜底（否则空 diff 会产生幻影行）。
	// 真实裸代码不是合法的 []diffJSONEntry，Unmarshal 会失败并继续走下面的格式探测/裸代码兜底。
	var jsonDiff []diffJSONEntry
	if err := json.Unmarshal([]byte(diffText), &jsonDiff); err == nil {
		return extractFromJSONDiff(jsonDiff)
	}

	if strings.Contains(diffText, "<<< BEFORE") && strings.Contains(diffText, ">>> AFTER") {
		return extractFromBeforeAfterDiff(diffText)
	}

	unified := extractFromUnifiedDiff(diffText)
	if len(unified) > 0 {
		return unified
	}

	// 兜底：部分对话的 diff 字段是 AI 直接输出的裸代码（无 diff --git / @@ / + 标记，
	// 也没有文件名），unified-diff 解析会得到 0 行。此时把每个非空行当作新增代码行，
	// 以便统计 diff_lines（用于阶段分类 edit/exec）。注意：缺文件名时指纹无法与 commit
	// 侧匹配，故对 silica 帮助有限，主要用于恢复"执行"阶段。
	if !looksLikeUnifiedDiff(diffText) {
		return extractFromRawCode(diffText)
	}
	return unified
}

// looksLikeUnifiedDiff 判断文本是否为标准 unified diff（含 diff --git / @@ 块头 / +++ 文件头）。
func looksLikeUnifiedDiff(diffText string) bool {
	return strings.Contains(diffText, "diff --git") ||
		strings.Contains(diffText, "\n@@ ") || strings.HasPrefix(diffText, "@@ ") ||
		strings.Contains(diffText, "\n+++ ") || strings.HasPrefix(diffText, "+++ ")
}

// extractFromRawCode 把裸代码文本的每个非空行当作一条新增行（无文件路径）。
func extractFromRawCode(text string) []addedLine {
	var result []addedLine
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, addedLine{FilePath: "", Content: trimmed})
		}
	}
	return result
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

// minFingerprintLen 是参与指纹匹配的最小行长（去空白后字节数）。
// 短于此的行（如 "}"、"return nil"、"})"）视为样板噪声，不生成指纹——
// 因为 content-only 指纹丢掉了文件名维度，样板行会跨文件大量误匹配，撑高 silica。
// 取 16：能滤掉绝大多数单符号/单关键字样板，又保留有实际内容的代码行。偏覆盖，可调。
const minFingerprintLen = 16

// calcLineFingerprint 为单行新增代码生成「内容指纹」（content-only，不含文件名）。
//
// 设计变更（silica 裸代码适配）：原指纹 = sha256(basename(file)+content)，但对话侧 diff 多为
// 裸代码（无文件名 → basename="."），与 commit 侧真实文件名的指纹永不相等，导致 silica 覆盖仅 0.58%。
// 改为只 hash 去空白后的行内容，使裸代码对话也能与 commit 匹配。
//
// 护栏：去空白后长度 < minFingerprintLen 的行返回 ""，由 calcLineFingerprints 过滤，不进指纹集。
func calcLineFingerprint(al addedLine) string {
	c := utils.RemoveWhitespace(al.Content)
	if len(c) < minFingerprintLen {
		return ""
	}
	hash := sha256.Sum256([]byte(c))
	return hex.EncodeToString(hash[:])
}

// calcLineFingerprints 批量生成行内容指纹，并过滤掉护栏命中的空指纹。
// conv 侧（写 .silica.json 指纹）与 commit 侧（匹配）统一调用本函数，保证两侧护栏一致——
// 否则一侧滤、一侧不滤会让指纹集错位。
func calcLineFingerprints(als []addedLine) []string {
	fps := make([]string, 0, len(als))
	for _, al := range als {
		if fp := calcLineFingerprint(al); fp != "" {
			fps = append(fps, fp)
		}
	}
	return fps
}
