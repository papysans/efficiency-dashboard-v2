package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const defaultS3Region = "us-east-1"

// s3Backend uses path-style requests so restricted S3-compatible gateways only
// need exact object operations. Production import paths enumerate objects from
// PostgreSQL indexes and do not depend on ListObjects.
type s3Backend struct {
	client *s3.Client
}

func newS3Backend(cfg S3Config) (*s3Backend, error) {
	endpoint, secure, err := s3Endpoint(cfg)
	if err != nil {
		return nil, err
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = defaultS3Region
	}
	httpClient, err := s3HTTPClient(secure && cfg.SkipVerify)
	if err != nil {
		return nil, err
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
		awsconfig.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 S3 配置失败: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	return &s3Backend{client: client}, nil
}

func s3Endpoint(cfg S3Config) (endpoint string, secure bool, err error) {
	endpoint = strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return "", false, fmt.Errorf("S3 endpoint 不能为空")
	}
	if !strings.Contains(endpoint, "://") {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "", false, fmt.Errorf("无效的 S3 endpoint %q", cfg.Endpoint)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false, fmt.Errorf("S3 endpoint 仅支持 http/https: %q", cfg.Endpoint)
	}
	return strings.TrimRight(endpoint, "/"), u.Scheme == "https", nil
}

func s3HTTPClient(skipVerify bool) (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("默认 HTTP transport 类型异常")
	}
	customTransport := transport.Clone()
	customTransport.ResponseHeaderTimeout = 30 * time.Second
	if skipVerify {
		tlsConfig := &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		if customTransport.TLSClientConfig != nil {
			tlsConfig = customTransport.TLSClientConfig.Clone()
			tlsConfig.InsecureSkipVerify = true //nolint:gosec
		}
		customTransport.TLSClientConfig = tlsConfig
	}
	return &http.Client{Transport: customTransport}, nil
}

// checkBucket is optional because restricted deployments may grant object
// Put/Get permission without HeadBucket.
func (b *s3Backend) checkBucket(bucket string) error {
	_, err := b.client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

// notExistErr converts S3 NoSuchKey/404 errors to an fs.ErrNotExist wrapper.
func notExistErr(loc string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NotFound", "404":
			return fmt.Errorf("%s: %w", loc, fs.ErrNotExist)
		}
	}
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == http.StatusNotFound {
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
	paginator := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	ctx := context.Background()
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("列举 %s 失败: %w", loc, err)
		}
		for _, obj := range page.Contents {
			objectKey := aws.ToString(obj.Key)
			if strings.HasSuffix(objectKey, "/") {
				continue
			}
			full := s3Scheme + bucket + "/" + objectKey
			if err := fn(full, FileInfo{
				Name:    path.Base(objectKey),
				Size:    aws.ToInt64(obj.Size),
				ModTime: aws.ToTime(obj.LastModified),
			}); err != nil {
				return err
			}
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
	output, err := b.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, notExistErr(loc, err)
	}
	return output.Body, nil
}

func (b *s3Backend) WriteFile(loc string, data []byte) error {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return err
	}
	_, err = b.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
	})
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
	if _, err := b.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("删除 %s 失败: %w", loc, err)
	}
	return nil
}

func (b *s3Backend) Stat(loc string) (FileInfo, error) {
	bucket, key, err := parseS3(loc)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := b.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return FileInfo{}, notExistErr(loc, err)
	}
	return FileInfo{
		Name:    path.Base(key),
		Size:    aws.ToInt64(info.ContentLength),
		ModTime: aws.ToTime(info.LastModified),
	}, nil
}

func (b *s3Backend) Exists(loc string) (bool, error) {
	if _, err := b.Stat(loc); err == nil {
		return true, nil
	} else if !IsNotExist(err) {
		return false, err
	}
	bucket, key, err := parseS3(loc)
	if err != nil {
		return false, err
	}
	prefix := key
	if prefix != "" {
		prefix += "/"
	}
	output, err := b.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}
	return len(output.Contents) > 0, nil
}
