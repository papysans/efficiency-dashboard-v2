// Package storage 提供 disk / s3 双后端的统一文件读写抽象。
// 路径以 "s3://bucket/key" 开头时走 S3 兼容对象存储（MinIO 等），否则走本地磁盘；
// 同一进程内允许不同目录混搭两种后端。使用 S3 前必须先 Configure。
package storage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// S3Config S3 兼容对象存储连接配置（MinIO / Ceph 等）
type S3Config struct {
	Endpoint        string `yaml:"endpoint"` // 如 minio.intranet:9000（不含 scheme）
	AccessKey       string `yaml:"access_key"`
	SecretKey       string `yaml:"secret_key"`
	UseSSL          bool   `yaml:"use_ssl"`
	SkipBucketCheck bool   `yaml:"skip_bucket_check"`
	// SkipVerify 跳过 TLS 证书校验（仅 use_ssl=true 时生效）。
	// 内网自建 MinIO 多用自签证书且 endpoint 走 IP/转发端口，证书域名对不上，
	// 必须跳过校验才能连上（上游 user-indicator 即 useSSL+skipVerify）。
	SkipVerify bool   `yaml:"skip_verify"`
	Region     string `yaml:"region"` // 可选，MinIO 通常留空
}

// Config 存储后端配置。目录路径以 s3:// 开头时使用 S3，否则使用本地磁盘。
type Config struct {
	S3 S3Config `yaml:"s3"`
}

// Redacted 返回凭证脱敏后的副本，供启动日志打印（避免 AK/SK 明文进日志）。
func (c Config) Redacted() Config {
	if c.S3.AccessKey != "" {
		c.S3.AccessKey = "***"
	}
	if c.S3.SecretKey != "" {
		c.S3.SecretKey = "***"
	}
	return c
}

// FileInfo Walk / Stat 返回的文件元信息
type FileInfo struct {
	Name    string // 文件名（不含目录）
	Size    int64
	ModTime time.Time
}

// WalkFunc 遍历回调。只回调文件，不回调目录；path 为含 scheme 的完整位置。
type WalkFunc func(path string, info FileInfo) error

// backend 单一存储后端需要实现的操作集合
type backend interface {
	Walk(loc string, fn WalkFunc) error
	ReadFile(loc string) ([]byte, error)
	Open(loc string) (io.ReadCloser, error)
	WriteFile(loc string, data []byte) error
	Stat(loc string) (FileInfo, error)
	Exists(loc string) (bool, error) // 文件存在，或目录/前缀下存在任意对象
	Remove(loc string) error         // 删除单个文件/对象；不存在视为成功（幂等）
}

const s3Scheme = "s3://"

var (
	mu         sync.RWMutex
	s3back     backend
	activeConf Config
	disk       backend = diskBackend{}
)

// Configure 根据配置初始化 S3 客户端。endpoint 为空表示未启用 S3（仅本地磁盘），
// 重复调用以最后一次为准。仅做配置形状校验，不发起网络请求。
func Configure(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	activeConf = cfg
	if cfg.S3.Endpoint == "" {
		s3back = nil
		return nil
	}
	if cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" {
		return fmt.Errorf("storage.s3 配置不完整: endpoint 已设置但 access_key/secret_key 为空")
	}
	b, err := newS3Backend(cfg.S3)
	if err != nil {
		return fmt.Errorf("初始化 S3 客户端失败(endpoint=%s): %w", cfg.S3.Endpoint, err)
	}
	s3back = b
	return nil
}

// ValidateLocations 启动期 fail-fast 校验：
// 任一位置为 s3:// 时要求已 Configure，且对应 bucket 可访问（验证 endpoint/凭证）。
func ValidateLocations(locs ...string) error {
	checked := map[string]bool{}
	for _, loc := range locs {
		if !IsS3(loc) {
			continue
		}
		b, err := backendFor(loc)
		if err != nil {
			return err
		}
		bucket, _, err := parseS3(loc)
		if err != nil {
			return err
		}
		if checked[bucket] {
			continue
		}
		if !activeConf.S3.SkipBucketCheck {
			if err := b.(*s3Backend).checkBucket(bucket); err != nil {
				return fmt.Errorf("S3 bucket %q 不可访问(检查 endpoint/凭证/bucket): %w", bucket, err)
			}
		}
		checked[bucket] = true
	}
	return nil
}

// IsS3 判断位置是否为 S3 路径
func IsS3(loc string) bool {
	return strings.HasPrefix(loc, s3Scheme)
}

// parseS3 拆解 "s3://bucket/key/..." 为 bucket 与 key（key 可为空）
func parseS3(loc string) (bucket, key string, err error) {
	rest := strings.TrimPrefix(loc, s3Scheme)
	if rest == loc {
		return "", "", fmt.Errorf("非 s3 路径: %s", loc)
	}
	bucket, key, _ = strings.Cut(rest, "/")
	if bucket == "" {
		return "", "", fmt.Errorf("s3 路径缺少 bucket: %s", loc)
	}
	return bucket, strings.Trim(key, "/"), nil
}

func backendFor(loc string) (backend, error) {
	if !IsS3(loc) {
		return disk, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	if s3back == nil {
		return nil, fmt.Errorf("路径 %s 为 s3:// 但未配置 storage.s3（检查配置文件 storage 块）", loc)
	}
	return s3back, nil
}

// Join 按位置 scheme 拼接路径：s3 走 path.Join（始终斜杠），本地走 filepath.Join
func Join(loc string, elem ...string) string {
	if IsS3(loc) {
		rest := strings.TrimPrefix(loc, s3Scheme)
		parts := append([]string{rest}, elem...)
		return s3Scheme + path.Join(parts...)
	}
	return filepath.Join(append([]string{loc}, elem...)...)
}

// Dir 返回位置的父目录。注意不能对 s3 路径用 filepath.Dir（Clean 会把 "s3://" 折叠成 "s3:/"）。
func Dir(loc string) string {
	if !IsS3(loc) {
		return filepath.Dir(loc)
	}
	bucket, key, err := parseS3(loc)
	if err != nil {
		return loc
	}
	d := path.Dir(key)
	if key == "" || d == "." || d == "/" {
		return s3Scheme + bucket
	}
	return s3Scheme + bucket + "/" + d
}

// Rel 计算 target 相对 base 的路径。s3 下两者必须同 bucket 且 target 在 base 前缀下。
func Rel(base, target string) (string, error) {
	if IsS3(base) != IsS3(target) {
		return "", fmt.Errorf("Rel: base 与 target 存储类型不一致: %s vs %s", base, target)
	}
	if !IsS3(base) {
		return filepath.Rel(base, target)
	}
	bb, bk, err := parseS3(base)
	if err != nil {
		return "", err
	}
	tb, tk, err := parseS3(target)
	if err != nil {
		return "", err
	}
	if bb != tb {
		return "", fmt.Errorf("Rel: bucket 不一致: %s vs %s", bb, tb)
	}
	if bk == "" {
		return tk, nil
	}
	rel := strings.TrimPrefix(tk, bk+"/")
	if rel == tk && tk != bk {
		return "", fmt.Errorf("Rel: %s 不在 %s 之下", target, base)
	}
	if tk == bk {
		return ".", nil
	}
	return rel, nil
}

// Walk 递归遍历 root 下所有文件（不含目录）。
// 本地 root 不存在时返回 fs.ErrNotExist；s3 前缀为空时回调零次、不报错。
func Walk(root string, fn WalkFunc) error {
	b, err := backendFor(root)
	if err != nil {
		return err
	}
	return b.Walk(root, fn)
}

// ReadFile 读取整个文件内容
func ReadFile(loc string) ([]byte, error) {
	b, err := backendFor(loc)
	if err != nil {
		return nil, err
	}
	return b.ReadFile(loc)
}

// Open 打开文件做流式读取，调用方负责 Close
func Open(loc string) (io.ReadCloser, error) {
	b, err := backendFor(loc)
	if err != nil {
		return nil, err
	}
	return b.Open(loc)
}

// WriteFile 写入整个文件（本地自动建父目录，权限 0644）
func WriteFile(loc string, data []byte) error {
	b, err := backendFor(loc)
	if err != nil {
		return err
	}
	return b.WriteFile(loc, data)
}

// Stat 获取文件元信息；不存在时返回的错误满足 IsNotExist
func Stat(loc string) (FileInfo, error) {
	b, err := backendFor(loc)
	if err != nil {
		return FileInfo{}, err
	}
	return b.Stat(loc)
}

// Exists 判断文件存在，或目录（s3 下为前缀）下存在任意对象。
// 后端不可用（如 s3 未配置）时返回 false 并附错误。
func Exists(loc string) (bool, error) {
	b, err := backendFor(loc)
	if err != nil {
		return false, err
	}
	return b.Exists(loc)
}

// Remove 删除单个文件/对象。不存在视为成功（幂等），供卸载对象的孤儿清理/裁旧用。
func Remove(loc string) error {
	b, err := backendFor(loc)
	if err != nil {
		return err
	}
	return b.Remove(loc)
}

// IsNotExist 判断错误是否表示文件/对象不存在
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
