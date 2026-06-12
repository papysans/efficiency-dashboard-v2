package rawdump

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"kanban/core/storage"
)

// TestMinIOSpike 真实 MinIO 往返验证(默认跳过)。
// 验证上游 raw-dump 分片布局: 用 Resolve 取回种子 task 的全部分片 → 重组 → 每行可解析,
// 同时实锤「完整 key = <prefix>/task/conversation/YYYY/MM/DD/<id>/00000N.jsonl」的映射。
//
// 先开隧道(本机直连内网 MinIO):
//
//	ssh -L 9000:10.20.19.101:9000 root@10.20.19.2
//
// 再跑(种子值已内置, 通常只需给 endpoint 即可):
//
//	MINIO_TEST_ENDPOINT=127.0.0.1:9000 \
//	go -C core test ./rawdump/ -run TestMinIOSpike -v
//
// 可覆盖的 env: MINIO_TEST_AK/SK(默认 minioadmin)、MINIO_TEST_BUCKET(默认 user-indicator)、
// RAWDUMP_PREFIX(默认 raw-dump)、RAWDUMP_TASK(默认种子 task)、RAWDUMP_DATE(默认 2026/05/13)。
func TestMinIOSpike(t *testing.T) {
	endpoint := os.Getenv("MINIO_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("未设置 MINIO_TEST_ENDPOINT，跳过真实 MinIO spike")
	}
	ak := envOr("MINIO_TEST_AK", "minioadmin")
	sk := envOr("MINIO_TEST_SK", "minioadmin")
	bucket := envOr("MINIO_TEST_BUCKET", "user-indicator")
	prefix := envOr("RAWDUMP_PREFIX", "raw-dump")
	task := envOr("RAWDUMP_TASK", "019e1f12-8963-77cd-b3e8-29aae6278dcc")
	date := envOr("RAWDUMP_DATE", "2026/05/13")

	// 通过隧道连本机转发端口, MinIO 为 https + 自签证书, 故 UseSSL+SkipVerify。
	if err := storage.Configure(storage.Config{S3: storage.S3Config{
		Endpoint:   endpoint,
		AccessKey:  ak,
		SecretKey:  sk,
		UseSSL:     true,
		SkipVerify: true,
	}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	defer storage.Configure(storage.Config{})

	root := fmt.Sprintf("s3://%s/%s/task/conversation", bucket, prefix)
	dateDir := storage.Join(root, strings.Split(date, "/")...)

	ref, found, err := Resolve(dateDir, task)
	if err != nil {
		t.Fatalf("Resolve(%s, %s) 存储故障: %v", dateDir, task, err)
	}
	if !found {
		t.Fatalf("种子 task 未找到: %s/%s —— 检查 prefix/date/bucket 是否对", dateDir, task)
	}
	t.Logf("✓ key 映射成立, 取到 %d 个分片, 首片=%s", ref.ChunkCount(), ref.Paths[0])

	data, err := ref.Read()
	if err != nil {
		t.Fatalf("重组失败: %v", err)
	}

	// 逐行校验是合法 JSON, 并统计带 request_id 的对话条数。
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	convCount := 0
	for i, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var obj map[string]any
		if e := json.Unmarshal([]byte(ln), &obj); e != nil {
			t.Fatalf("第 %d 行非法 JSON(分片可能被字节切断, 需确认): %v\n内容: %.120s", i+1, e, ln)
		}
		if _, ok := obj["request_id"]; ok {
			convCount++
		}
	}
	t.Logf("✓ 重组往返成功: %d 行, 其中 %d 条带 request_id 的对话", len(lines), convCount)
	if convCount == 0 {
		t.Errorf("重组后无任何带 request_id 的对话, 与预期(每片一条记录)不符")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
