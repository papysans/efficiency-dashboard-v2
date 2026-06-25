package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3Backend S3 兼容对象存储实现（minio-go）。
// "目录"概念用 key 前缀模拟：Walk/Exists 对目录位置按 prefix 列举。
type s3Backend struct {
	client *minio.Client
}

func newS3Backend(cfg S3Config) (*s3Backend, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	}
	// 自签证书的内网 MinIO：用自定义 transport 跳过 TLS 校验。
	if cfg.UseSSL && cfg.SkipVerify {
		tr, err := minio.DefaultTransport(true)
		if err != nil {
			return nil, fmt.Errorf("构造 S3 transport 失败: %w", err)
		}
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.InsecureSkipVerify = true
		opts.Transport = tr
	}
	cli, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return nil, err
	}
	return &s3Backend{client: cli}, nil
}

// checkBucket 启动期连通性校验：验证 endpoint 可达、凭证有效且 bucket 存在
func (b *s3Backend) checkBucket(bucket string) error {
	ok, err := b.client.BucketExists(context.Background(), bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket 不存在")
	}
	return nil
}

// notExistErr 将 S3 的 NoSuchKey 类错误统一转换为满足 IsNotExist 的错误
func notExistErr(loc string, err error) error {
	resp := minio.ToErrorResponse(err)
	if resp.Code == "NoSuchKey" || resp.Code == "NoSuchBucket" {
		return fmt.Errorf("%s: %w", loc, fs.ErrNotExist)
	}
	return err
}

func (b *s3Backend) Walk(loc string, fn WalkFunc) error {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return err
	}
	prefix := key
	if prefix != "" {
		prefix += "/"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for obj := range b.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return fmt.Errorf("列举 %s 失败: %w", loc, obj.Err)
		}
		// 跳过"目录占位"对象（以 / 结尾、size 0）
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}
		full := s3Scheme + bucket + "/" + obj.Key
		if err := fn(full, FileInfo{Name: path.Base(obj.Key), Size: obj.Size, ModTime: obj.LastModified}); err != nil {
			return err
		}
	}
	return nil
}

func (b *s3Backend) ReadFile(loc string) ([]byte, error) {
	rc, err := b.Open(loc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, notExistErr(loc, err)
	}
	return data, nil
}

func (b *s3Backend) Open(loc string) (io.ReadCloser, error) {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return nil, err
	}
	obj, err := b.client.GetObject(context.Background(), bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, notExistErr(loc, err)
	}
	// GetObject 是惰性的，先 Stat 一次让"对象不存在"在 Open 阶段就暴露
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, notExistErr(loc, err)
	}
	return obj, nil
}

func (b *s3Backend) WriteFile(loc string, data []byte) error {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return err
	}
	_, err = b.client.PutObject(context.Background(), bucket, key,
		bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("写入 %s 失败: %w", loc, err)
	}
	return nil
}

func (b *s3Backend) Remove(loc string) error {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return err
	}
	// minio RemoveObject 对不存在的 key 不报错（幂等），符合接口语义。
	if err := b.client.RemoveObject(context.Background(), bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除 %s 失败: %w", loc, err)
	}
	return nil
}

func (b *s3Backend) Stat(loc string) (FileInfo, error) {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := b.client.StatObject(context.Background(), bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return FileInfo{}, notExistErr(loc, err)
	}
	return FileInfo{Name: path.Base(key), Size: info.Size, ModTime: info.LastModified}, nil
}

func (b *s3Backend) Exists(loc string) (bool, error) {
	// 先按精确对象查
	if _, err := b.Stat(loc); err == nil {
		return true, nil
	} else if !IsNotExist(err) {
		return false, err
	}
	// 再按目录前缀查：前缀下有任意对象即视为存在
	bucket, key, err := parseS3(loc)
	if err != nil {
		return false, err
	}
	prefix := key
	if prefix != "" {
		prefix += "/"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for obj := range b.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true, MaxKeys: 1}) {
		if obj.Err != nil {
			return false, obj.Err
		}
		return true, nil
	}
	return false, nil
}
