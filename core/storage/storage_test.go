package storage

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestParseS3(t *testing.T) {
	cases := []struct {
		loc, bucket, key string
		wantErr          bool
	}{
		{"s3://b/raw/task", "b", "raw/task", false},
		{"s3://b", "b", "", false},
		{"s3://b/", "b", "", false},
		{"s3://b/k/", "b", "k", false},
		{"/local/path", "", "", true},
		{"s3://", "", "", true},
	}
	for _, c := range cases {
		bucket, key, err := parseS3(c.loc)
		if (err != nil) != c.wantErr {
			t.Errorf("parseS3(%q) err=%v wantErr=%v", c.loc, err, c.wantErr)
			continue
		}
		if bucket != c.bucket || key != c.key {
			t.Errorf("parseS3(%q) = (%q,%q), want (%q,%q)", c.loc, bucket, key, c.bucket, c.key)
		}
	}
}

func TestJoin(t *testing.T) {
	if got := Join("s3://b/raw", "conversation", "2026/06/01"); got != "s3://b/raw/conversation/2026/06/01" {
		t.Errorf("s3 Join = %q", got)
	}
	if got := Join("/tmp/raw", "x.json"); got != filepath.Join("/tmp/raw", "x.json") {
		t.Errorf("disk Join = %q", got)
	}
}

func TestDir(t *testing.T) {
	cases := []struct{ loc, want string }{
		{"s3://b/x/y/z.json", "s3://b/x/y"},
		{"s3://b/x", "s3://b"},
		{"s3://b", "s3://b"},
		{"/a/b/c.json", filepath.Dir("/a/b/c.json")},
	}
	for _, c := range cases {
		if got := Dir(c.loc); got != c.want {
			t.Errorf("Dir(%q) = %q, want %q", c.loc, got, c.want)
		}
	}
}

func TestRel(t *testing.T) {
	got, err := Rel("s3://b/raw/task", "s3://b/raw/task/2026/06/01/x.jsonl")
	if err != nil || got != "2026/06/01/x.jsonl" {
		t.Errorf("s3 Rel = %q, err=%v", got, err)
	}
	if _, err := Rel("s3://b/raw", "s3://other/raw/x"); err == nil {
		t.Error("跨 bucket Rel 应报错")
	}
	if _, err := Rel("/local", "s3://b/k"); err == nil {
		t.Error("混合 scheme Rel 应报错")
	}
	got, err = Rel("/a/b", "/a/b/c/d.json")
	if err != nil || filepath.ToSlash(got) != "c/d.json" {
		t.Errorf("disk Rel = %q, err=%v", got, err)
	}
}

func TestDiskRoundtrip(t *testing.T) {
	dir := t.TempDir()
	loc := filepath.Join(dir, "sub", "a.json")

	// WriteFile 自动建父目录
	if err := WriteFile(loc, []byte("hello")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := ReadFile(loc)
	if err != nil || string(data) != "hello" {
		t.Fatalf("ReadFile = %q, err=%v", data, err)
	}
	info, err := Stat(loc)
	if err != nil || info.Size != 5 || info.Name != "a.json" {
		t.Fatalf("Stat = %+v, err=%v", info, err)
	}
	rc, err := Open(loc)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "hello" {
		t.Fatalf("Open read = %q", got)
	}

	// Exists：文件、目录、不存在
	for _, c := range []struct {
		loc  string
		want bool
	}{{loc, true}, {filepath.Join(dir, "sub"), true}, {filepath.Join(dir, "nope"), false}} {
		ok, err := Exists(c.loc)
		if err != nil || ok != c.want {
			t.Errorf("Exists(%q) = %v, err=%v, want %v", c.loc, ok, err, c.want)
		}
	}

	// Walk 只回调文件
	if err := WriteFile(filepath.Join(dir, "sub", "b.jsonl"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	var names []string
	err = Walk(dir, func(p string, info FileInfo) error {
		names = append(names, info.Name)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "a.json" || names[1] != "b.jsonl" {
		t.Errorf("Walk names = %v", names)
	}

	// Walk 不存在的根目录报 IsNotExist
	if err := Walk(filepath.Join(dir, "nope"), func(string, FileInfo) error { return nil }); err == nil || !os.IsNotExist(err) {
		t.Errorf("Walk(不存在) err = %v", err)
	}

	// Stat 不存在 → IsNotExist
	if _, err := Stat(filepath.Join(dir, "nope.json")); !IsNotExist(err) {
		t.Errorf("Stat(不存在) err = %v", err)
	}
}

func TestS3NotConfigured(t *testing.T) {
	if err := Configure(Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile("s3://b/k"); err == nil {
		t.Error("未配置 storage.s3 时 s3 读取应报错")
	}
	if err := Configure(Config{S3: S3Config{Endpoint: "x:9000"}}); err == nil {
		t.Error("缺 access_key/secret_key 应报错")
	}
	if err := Configure(Config{S3: S3Config{Endpoint: "x:9000", AccessKey: "a", SecretKey: "s"}}); err != nil {
		t.Errorf("完整配置应通过: %v", err)
	}
	Configure(Config{}) // 还原
}

func TestConfigRedacted(t *testing.T) {
	c := Config{S3: S3Config{Endpoint: "e:9000", AccessKey: "AK", SecretKey: "SK"}}
	r := c.Redacted()
	if r.S3.AccessKey != "***" || r.S3.SecretKey != "***" {
		t.Errorf("凭证未脱敏: %+v", r.S3)
	}
	if r.S3.Endpoint != "e:9000" {
		t.Errorf("非凭证字段不应改动: %s", r.S3.Endpoint)
	}
	if c.S3.AccessKey != "AK" || c.S3.SecretKey != "SK" {
		t.Errorf("原配置被改动: %+v", c.S3)
	}
	if empty := (Config{}).Redacted(); empty.S3.AccessKey != "" || empty.S3.SecretKey != "" {
		t.Errorf("空凭证不应标星: %+v", empty.S3)
	}
}
