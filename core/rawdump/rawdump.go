// Package rawdump 适配上游 raw-dump 的 conversation 数据布局。
//
// 上游把每个会话的 conversation 从「单文件 <sessionId>.jsonl」改成了「目录分片
// <sessionId>/000001.jsonl … 00000N.jsonl」(每次请求写一个独立分片, DB 原子递增序号保证有序)。
// 本包把两种布局统一成「按数字序拼接全部分片字节」再交给上层解析, 让 kbcli(import/silica)
// 与 backend(原文接口) 复用同一套重组逻辑, 避免三处各写一份「单文件假设」。
//
// 设计要点:
//   - 单文件布局: ConversationRef.Paths 仅含 1 个 <id>.jsonl。
//   - 分片布局:   ConversationRef.Paths 为按数字序排好的 <id>/00000N.jsonl 列表。
//   - 两种布局对 Read/Aggregate 行为一致, 上层无需感知差异。
//   - 任一分片读取失败 → 整会话失败(绝不返回半截), 与 import 既有「坏 session 不污染批量」一致。
//   - S3 路径拼接全走 storage.Join(禁用 filepath, 否则折叠 s3:// → s3:/)。
package rawdump

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"kanban/core/storage"
)

// chunkFileRe 匹配分片文件名: 纯数字 + .jsonl, 如 000001.jsonl。
// 上游当前用六位零填充, 但这里不锁死位宽——排序统一按数字解析(见 sortChunkPaths),
// 即便上游某天改宽度(如 7 位)也不会静默丢片或乱序。
var chunkFileRe = regexp.MustCompile(`^\d+\.jsonl$`)

// ConversationRef 指向一个会话的 conversation 数据(单文件或一组有序分片)。
type ConversationRef struct {
	SessionID string
	// Paths 为已按读取顺序排好的路径(含 scheme 的完整位置)。
	// 单文件布局长度为 1; 分片布局为 N 个 00000N.jsonl。
	Paths []string
}

// ChunkCount 返回分片数(单文件布局为 1)。
func (r ConversationRef) ChunkCount() int { return len(r.Paths) }

// MissingChunkNumbers 检测分片序号相对 1..max 连续序列的缺号(如有 000001/000003 缺 000002 → [2])。
// 仅对分片布局有意义；单文件布局(或非纯数字命名)返回 nil。core 不持有日志器,
// 由调用方据此告警(缺片=不完整会话,多为上游写入/列举问题)。
func (r ConversationRef) MissingChunkNumbers() []int {
	if len(r.Paths) == 0 {
		return nil
	}
	present := make(map[int]bool, len(r.Paths))
	max := 0
	for _, p := range r.Paths {
		n := chunkNum(p)
		if n <= 0 {
			return nil // 含非分片(单文件等)路径,不判缺号
		}
		present[n] = true
		if n > max {
			max = n
		}
	}
	var missing []int
	for i := 1; i <= max; i++ {
		if !present[i] {
			missing = append(missing, i)
		}
	}
	return missing
}

// Aggregate 返回所有分片字节之和与片数, 作为增量检测信号(替代旧的单文件 size 比对:
// 上游每追加一片, 总字节与片数都会变, 据此触发重导)。
// 任一分片 Stat 失败即返回错误(调用方可用 storage.IsNotExist 区分存储故障与不存在)。
func (r ConversationRef) Aggregate() (totalSize int64, chunks int, err error) {
	for _, p := range r.Paths {
		info, e := storage.Stat(p)
		if e != nil {
			return 0, 0, fmt.Errorf("stat 分片 %s 失败: %w", p, e)
		}
		totalSize += info.Size
	}
	return totalSize, len(r.Paths), nil
}

// Read 按序读取并拼接全部分片字节。任一分片读取失败 → 整体失败, 绝不返回半截会话。
// 分片之间若上游某片末尾缺换行, 补一个 '\n', 防止相邻两条记录在拼接处粘连成一行。
func (r ConversationRef) Read() ([]byte, error) {
	var buf bytes.Buffer
	for _, p := range r.Paths {
		data, err := storage.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("读取分片 %s 失败: %w", p, err)
		}
		buf.Write(data)
		// 仅在分片确有内容且末尾非换行时补换行, 保证行边界清晰。
		if len(data) > 0 && data[len(data)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

// Open 返回拼接后的只读流, 供逐行解析(parseConversationFile 等)。
// 内部一次性重组(单会话体量为 KB 级, 全缓冲无压力), 调用方负责 Close。
func (r ConversationRef) Open() (io.ReadCloser, error) {
	data, err := r.Read()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// ClassifyRelPath 判定一个「相对 conversation 根目录」的 .jsonl 路径属于哪种布局,
// 并提取 sessionID 与日期(YYYY/MM/DD)。用于 import 遍历时按 session 分组。
// ok=false 表示路径不符合任一已知布局(调用方应跳过该文件)。
//
//	旧单文件: YYYY/MM/DD/<sessionId>.jsonl          (4 段) → isChunk=false
//	新分片:   YYYY/MM/DD/<sessionId>/00000N.jsonl   (5 段, 末段为规范分片名) → isChunk=true
func ClassifyRelPath(relPath string) (sessionID, date string, isChunk, ok bool) {
	parts := strings.Split(strings.Trim(filepathToSlash(relPath), "/"), "/")
	switch len(parts) {
	case 4: // 旧单文件: Y/M/D/<id>.jsonl
		name := parts[3]
		if !strings.HasSuffix(name, ".jsonl") {
			return "", "", false, false
		}
		return strings.TrimSuffix(name, ".jsonl"), strings.Join(parts[0:3], "/"), false, true
	case 5: // 新分片: Y/M/D/<id>/00000N.jsonl
		if !chunkFileRe.MatchString(parts[4]) {
			return "", "", false, false
		}
		return parts[3], strings.Join(parts[0:3], "/"), true, true
	default:
		return "", "", false, false
	}
}

// Resolve 在已知 dateDir(= <conversationDir>/YYYY/MM/DD) 下定位某 session 的 conversation,
// 自动识别单文件或分片目录。用于 backend 已知 session+date 取原文。
//
// 返回: found=false 表示不存在; err 仅在存储故障(非「不存在」)时返回, 让上层把 S3 中断
// 与 404 区分对待(backend 错误处理约定)。优先单文件, 再退分片目录。
func Resolve(dateDir, sessionID string) (ref ConversationRef, found bool, err error) {
	// 1) 旧单文件 <id>.jsonl
	single := storage.Join(dateDir, sessionID+".jsonl")
	if _, e := storage.Stat(single); e == nil {
		return ConversationRef{SessionID: sessionID, Paths: []string{single}}, true, nil
	} else if !storage.IsNotExist(e) {
		return ConversationRef{}, false, e // 存储故障上抛
	}
	// 2) 新分片目录 <id>/00000N.jsonl
	dir := storage.Join(dateDir, sessionID)
	paths, e := listChunks(dir)
	if e != nil {
		return ConversationRef{}, false, e
	}
	if len(paths) == 0 {
		return ConversationRef{}, false, nil // 两种布局都不存在
	}
	return ConversationRef{SessionID: sessionID, Paths: paths}, true, nil
}

// listChunks 列出 dir 下的规范分片文件并按数字序(字典序)排序。
// dir 不存在时返回空切片不报错(交由调用方判 found)。
func listChunks(dir string) ([]string, error) {
	ok, err := storage.Exists(dir)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	var paths []string
	err = storage.Walk(dir, func(p string, info storage.FileInfo) error {
		if chunkFileRe.MatchString(info.Name) {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortChunkPaths(paths)
	return paths, nil
}

// SortChunkPaths 把一组分片路径按文件名数字序排好(供 import 分组后排序)。
// 单文件布局(长度 1)原样返回。
func SortChunkPaths(paths []string) {
	sortChunkPaths(paths)
}

// sortChunkPaths 按分片文件名的数字值升序排序(不依赖位宽对齐的字典序),
// 数字相同时用完整路径兜底, 保证排序稳定。
func sortChunkPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		ni, nj := chunkNum(paths[i]), chunkNum(paths[j])
		if ni != nj {
			return ni < nj
		}
		return paths[i] < paths[j]
	})
}

// chunkNum 解析分片文件名的数字序号(如 .../000123.jsonl → 123)。非数字名返回 0。
func chunkNum(p string) int {
	base := path.Base(filepathToSlash(p))
	n, _ := strconv.Atoi(strings.TrimSuffix(base, ".jsonl"))
	return n
}

// filepathToSlash 把本地分隔符统一成 '/', 以便对 disk 与 s3 路径用同一套字符串逻辑。
// 不用 filepath.ToSlash 直接处理是为了对 s3:// 路径也安全(s3 始终用 '/')。
func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
