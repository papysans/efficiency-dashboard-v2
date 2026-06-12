package rawdump

import (
	"os"
	"testing"

	"kanban/core/storage"
)

// write 用 storage 写一个 disk 文件(自动建父目录), 失败即 fatal。
func write(t *testing.T, loc, content string) {
	t.Helper()
	if err := storage.WriteFile(loc, []byte(content)); err != nil {
		t.Fatalf("写 fixture %s 失败: %v", loc, err)
	}
}

func TestClassifyRelPath(t *testing.T) {
	cases := []struct {
		name      string
		rel       string
		wantSid   string
		wantDt    string
		wantChunk bool
		wantOk    bool
	}{
		{"旧单文件", "2026/05/13/019e1f12-abc.jsonl", "019e1f12-abc", "2026/05/13", false, true},
		{"新分片首片", "2026/05/13/019e1f12-abc/000001.jsonl", "019e1f12-abc", "2026/05/13", true, true},
		{"新分片第五片", "2026/05/13/019e1f12-abc/000005.jsonl", "019e1f12-abc", "2026/05/13", true, true},
		{"反斜杠分隔(Windows)", "2026\\05\\13\\sid.jsonl", "sid", "2026/05/13", false, true},
		{"前导斜杠", "/2026/05/13/sid.jsonl", "sid", "2026/05/13", false, true},
		{"非分片名的五段", "2026/05/13/sid/notes.txt", "", "", false, false},
		{"分片名任意位数也认", "2026/05/13/sid/00001.jsonl", "sid", "2026/05/13", true, true},
		{"分片名非数字不认", "2026/05/13/sid/abc.jsonl", "", "", false, false},
		{"段数太少", "sid.jsonl", "", "", false, false},
		{"段数太多", "2026/05/13/sid/sub/000001.jsonl", "", "", false, false},
		{"四段但非jsonl", "2026/05/13/sid.json", "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sid, dt, isChunk, ok := ClassifyRelPath(c.rel)
			if ok != c.wantOk || sid != c.wantSid || dt != c.wantDt || isChunk != c.wantChunk {
				t.Fatalf("ClassifyRelPath(%q) = (%q,%q,chunk=%v,%v), 期望 (%q,%q,chunk=%v,%v)",
					c.rel, sid, dt, isChunk, ok, c.wantSid, c.wantDt, c.wantChunk, c.wantOk)
			}
		})
	}
}

func TestMissingChunkNumbers(t *testing.T) {
	mk := func(nums ...string) ConversationRef {
		paths := make([]string, len(nums))
		for i, n := range nums {
			paths[i] = "s3://b/c/2026/05/13/sid/" + n + ".jsonl"
		}
		return ConversationRef{SessionID: "sid", Paths: paths}
	}
	// 连续 → 无缺
	if got := mk("000001", "000002", "000003").MissingChunkNumbers(); got != nil {
		t.Fatalf("连续应无缺号, 得到 %v", got)
	}
	// 缺中间
	if got := mk("000001", "000003").MissingChunkNumbers(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("应报缺 [2], 得到 %v", got)
	}
	// 单文件布局(非数字名) → 不判缺号
	if got := (ConversationRef{Paths: []string{"x/2026/05/13/sid.jsonl"}}).MissingChunkNumbers(); got != nil {
		t.Fatalf("单文件不应判缺号, 得到 %v", got)
	}
}

func TestResolveSingleFile(t *testing.T) {
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	single := storage.Join(dateDir, "sid.jsonl")
	write(t, single, `{"request_id":"r1"}`+"\n")

	ref, found, err := Resolve(dateDir, "sid")
	if err != nil || !found {
		t.Fatalf("Resolve = found=%v err=%v, 期望 found=true err=nil", found, err)
	}
	if ref.ChunkCount() != 1 || ref.Paths[0] != single {
		t.Fatalf("单文件 ref 不符: %+v", ref)
	}
	data, err := ref.Read()
	if err != nil || string(data) != `{"request_id":"r1"}`+"\n" {
		t.Fatalf("Read = %q err=%v", data, err)
	}
}

func TestResolveChunkedOrdered(t *testing.T) {
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	sidDir := storage.Join(dateDir, "sid")
	// 故意乱序创建(先 3 再 1 再 2), 验证 Resolve 内部按数字序排好。
	write(t, storage.Join(sidDir, "000003.jsonl"), `{"r":3}`+"\n")
	write(t, storage.Join(sidDir, "000001.jsonl"), `{"r":1}`+"\n")
	write(t, storage.Join(sidDir, "000002.jsonl"), `{"r":2}`+"\n")

	ref, found, err := Resolve(dateDir, "sid")
	if err != nil || !found {
		t.Fatalf("Resolve = found=%v err=%v", found, err)
	}
	if ref.ChunkCount() != 3 {
		t.Fatalf("片数 = %d, 期望 3", ref.ChunkCount())
	}
	data, err := ref.Read()
	if err != nil {
		t.Fatalf("Read err=%v", err)
	}
	want := `{"r":1}` + "\n" + `{"r":2}` + "\n" + `{"r":3}` + "\n"
	if string(data) != want {
		t.Fatalf("拼接顺序错误:\n得到 %q\n期望 %q", data, want)
	}
}

func TestResolveChunkedNumericOrderAcrossWidths(t *testing.T) {
	// 不同位宽的分片(1/2/10)必须按数字序而非字典序拼接(字典序会把 10 排到 2 前面)。
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	sidDir := storage.Join(dateDir, "sid")
	write(t, storage.Join(sidDir, "10.jsonl"), `{"r":10}`+"\n")
	write(t, storage.Join(sidDir, "2.jsonl"), `{"r":2}`+"\n")
	write(t, storage.Join(sidDir, "1.jsonl"), `{"r":1}`+"\n")

	ref, found, err := Resolve(dateDir, "sid")
	if err != nil || !found || ref.ChunkCount() != 3 {
		t.Fatalf("Resolve = found=%v chunks=%d err=%v", found, ref.ChunkCount(), err)
	}
	data, _ := ref.Read()
	want := `{"r":1}` + "\n" + `{"r":2}` + "\n" + `{"r":10}` + "\n"
	if string(data) != want {
		t.Fatalf("数字序拼接错误:\n得到 %q\n期望 %q", data, want)
	}
}

func TestResolveSingleFilePreferredOverChunkDir(t *testing.T) {
	// 同一 session 同时存在单文件与分片目录时, 优先单文件(向后兼容: 旧数据为准)。
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	write(t, storage.Join(dateDir, "sid.jsonl"), `{"single":true}`+"\n")
	write(t, storage.Join(dateDir, "sid", "000001.jsonl"), `{"chunk":true}`+"\n")

	ref, found, err := Resolve(dateDir, "sid")
	if err != nil || !found {
		t.Fatalf("Resolve = found=%v err=%v", found, err)
	}
	data, _ := ref.Read()
	if string(data) != `{"single":true}`+"\n" {
		t.Fatalf("应优先单文件, 得到 %q", data)
	}
}

func TestResolveMissing(t *testing.T) {
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	ref, found, err := Resolve(dateDir, "nope")
	if err != nil {
		t.Fatalf("不存在时不应报错, err=%v", err)
	}
	if found || ref.ChunkCount() != 0 {
		t.Fatalf("期望 found=false 空 ref, 得到 found=%v ref=%+v", found, ref)
	}
}

func TestResolveZeroChunks(t *testing.T) {
	// 分片目录存在但无规范分片文件(只有杂项) → 视为不存在。
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	write(t, storage.Join(dateDir, "sid", "README.txt"), "noise")

	_, found, err := Resolve(dateDir, "sid")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if found {
		t.Fatalf("无规范分片应 found=false")
	}
}

func TestReadInsertsNewlineBetweenChunks(t *testing.T) {
	// 上游某片末尾缺换行时, 拼接处补换行, 防止两条记录粘连成一行。
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	sidDir := storage.Join(dateDir, "sid")
	write(t, storage.Join(sidDir, "000001.jsonl"), `{"r":1}`) // 无结尾换行
	write(t, storage.Join(sidDir, "000002.jsonl"), `{"r":2}`+"\n")

	ref, _, _ := Resolve(dateDir, "sid")
	data, err := ref.Read()
	if err != nil {
		t.Fatalf("Read err=%v", err)
	}
	want := `{"r":1}` + "\n" + `{"r":2}` + "\n"
	if string(data) != want {
		t.Fatalf("补换行错误:\n得到 %q\n期望 %q", data, want)
	}
}

func TestAggregate(t *testing.T) {
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	sidDir := storage.Join(dateDir, "sid")
	write(t, storage.Join(sidDir, "000001.jsonl"), "aaaa")  // 4
	write(t, storage.Join(sidDir, "000002.jsonl"), "bbbbb") // 5

	ref, _, _ := Resolve(dateDir, "sid")
	size, chunks, err := ref.Aggregate()
	if err != nil {
		t.Fatalf("Aggregate err=%v", err)
	}
	if size != 9 || chunks != 2 {
		t.Fatalf("Aggregate = (size=%d, chunks=%d), 期望 (9, 2)", size, chunks)
	}
}

func TestReadAbortsOnChunkFailure(t *testing.T) {
	// 任一分片读取失败 → 整会话失败, 不返回半截。
	root := t.TempDir()
	dateDir := storage.Join(root, "2026", "05", "13")
	sidDir := storage.Join(dateDir, "sid")
	write(t, storage.Join(sidDir, "000001.jsonl"), `{"r":1}`+"\n")
	p2 := storage.Join(sidDir, "000002.jsonl")
	write(t, p2, `{"r":2}`+"\n")

	ref, _, _ := Resolve(dateDir, "sid")
	// 解析出 ref 后删掉第二片, 模拟读取中途失败。
	if err := os.Remove(p2); err != nil {
		t.Fatalf("删片失败: %v", err)
	}
	if _, err := ref.Read(); err == nil {
		t.Fatal("缺片时 Read 应报错, 实际成功")
	}
}
