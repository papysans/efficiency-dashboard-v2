package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// TestS3Integration 针对真实 S3 兼容存储（MinIO）的集成测试。
// 默认跳过；设置环境变量后运行：
//
//	MINIO_TEST_ENDPOINT=127.0.0.1:9000 MINIO_TEST_AK=minioadmin MINIO_TEST_SK=minioadmin \
//	MINIO_TEST_BUCKET=kanban-test go test ./storage/ -run TestS3Integration -v
//
// bucket 需预先创建。
func TestS3Integration(t *testing.T) {
	endpoint := os.Getenv("MINIO_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("未设置 MINIO_TEST_ENDPOINT，跳过 S3 集成测试")
	}
	bucket := os.Getenv("MINIO_TEST_BUCKET")
	if bucket == "" {
		bucket = "kanban-test"
	}
	// 测试前置：bucket 不存在则创建
	cli, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(os.Getenv("MINIO_TEST_AK"), os.Getenv("MINIO_TEST_SK"), ""),
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	if ok, err := cli.BucketExists(context.Background(), bucket); err != nil {
		t.Fatalf("BucketExists: %v", err)
	} else if !ok {
		if err := cli.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
			t.Fatalf("MakeBucket: %v", err)
		}
	}

	err = Configure(Config{S3: S3Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("MINIO_TEST_AK"),
		SecretKey: os.Getenv("MINIO_TEST_SK"),
		UseSSL:    false,
	}})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	defer Configure(Config{}) // 还原，避免影响其他测试

	root := fmt.Sprintf("s3://%s/it-root", bucket)

	// 启动期校验
	if err := ValidateLocations(root); err != nil {
		t.Fatalf("ValidateLocations: %v", err)
	}
	if err := ValidateLocations("s3://no-such-bucket-xyz/k"); err == nil {
		t.Error("不存在的 bucket 校验应失败")
	}

	// 写 → 读 → Stat → Open
	loc := Join(root, "conversation", "2026/06/01", "sess-1.jsonl")
	content := []byte(`{"a":1}` + "\n" + `{"a":2}` + "\n")
	if err := WriteFile(loc, content); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := ReadFile(loc)
	if err != nil || string(data) != string(content) {
		t.Fatalf("ReadFile = %q, err=%v", data, err)
	}
	info, err := Stat(loc)
	if err != nil || info.Size != int64(len(content)) || info.Name != "sess-1.jsonl" {
		t.Fatalf("Stat = %+v, err=%v", info, err)
	}
	rc, err := Open(loc)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != string(content) {
		t.Fatalf("Open read = %q", got)
	}

	// 第二个文件 + Walk
	loc2 := Join(root, "conversation", "2026/06/02", "sess-2.jsonl")
	if err := WriteFile(loc2, []byte("x")); err != nil {
		t.Fatal(err)
	}
	var walked []string
	err = Walk(Join(root, "conversation"), func(p string, fi FileInfo) error {
		walked = append(walked, fi.Name)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(walked)
	if strings.Join(walked, ",") != "sess-1.jsonl,sess-2.jsonl" {
		t.Errorf("Walk = %v", walked)
	}

	// Rel 与日期提取路径形态
	rel, err := Rel(Join(root, "conversation"), loc)
	if err != nil || rel != "2026/06/01/sess-1.jsonl" {
		t.Errorf("Rel = %q, err=%v", rel, err)
	}

	// Exists：精确对象 / 前缀 / 不存在
	for _, c := range []struct {
		loc  string
		want bool
	}{
		{loc, true},
		{Join(root, "conversation"), true},
		{Join(root, "nope"), false},
	} {
		ok, err := Exists(c.loc)
		if err != nil || ok != c.want {
			t.Errorf("Exists(%q) = %v, err=%v, want %v", c.loc, ok, err, c.want)
		}
	}

	// Stat / ReadFile 不存在 → IsNotExist
	if _, err := Stat(Join(root, "nope.json")); !IsNotExist(err) {
		t.Errorf("Stat(不存在) err = %v", err)
	}
	if _, err := ReadFile(Join(root, "nope.json")); !IsNotExist(err) {
		t.Errorf("ReadFile(不存在) err = %v", err)
	}
	if _, err := Open(Join(root, "nope.json")); !IsNotExist(err) {
		t.Errorf("Open(不存在) err = %v", err)
	}

	// 空前缀 Walk 不报错、零回调
	calls := 0
	if err := Walk(Join(root, "empty-prefix"), func(string, FileInfo) error { calls++; return nil }); err != nil || calls != 0 {
		t.Errorf("空前缀 Walk calls=%d err=%v", calls, err)
	}
}
