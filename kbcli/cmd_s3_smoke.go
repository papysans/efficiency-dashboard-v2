package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"

	"github.com/spf13/cobra"
)

const defaultS3SmokePayloadBytes = 256 * 1024

type s3SmokePayload struct {
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
	Nonce     string `json:"nonce"`
	Data      string `json:"data"`
}

func newS3SmokePayload(dataBytes int) ([]byte, string, error) {
	if dataBytes <= 0 {
		return nil, "", fmt.Errorf("payload-bytes 必须大于 0")
	}
	data := make([]byte, dataBytes)
	if _, err := rand.Read(data); err != nil {
		return nil, "", fmt.Errorf("生成随机 payload 失败: %w", err)
	}
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, "", fmt.Errorf("生成随机 nonce 失败: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	payload, err := json.Marshal(s3SmokePayload{
		Kind:      "kbcli-restricted-s3-smoke",
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Nonce:     nonce,
		Data:      base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return nil, "", fmt.Errorf("编码 smoke payload 失败: %w", err)
	}
	return payload, nonce, nil
}

func runS3Smoke(analysedDir string, dataBytes int, out io.Writer) (retErr error) {
	if !storage.IsS3(analysedDir) {
		return fmt.Errorf("s3-smoke 要求 analysed_dir 为 s3:// 路径，当前为 %q", analysedDir)
	}
	payload, nonce, err := newS3SmokePayload(dataBytes)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("kbcli-%s-%s.json",
		time.Now().UTC().Format("20060102T150405.000000000Z"), nonce)
	loc := storage.Join(strings.TrimRight(analysedDir, "/"), "_smoke", key)
	wantHash := sha256.Sum256(payload)

	fmt.Fprintf(out, "S3 smoke target: %s\n", loc)
	fmt.Fprintf(out, "PUT bytes=%d sha256=%x\n", len(payload), wantHash)

	putAttempted := false
	defer func() {
		if !putAttempted {
			return
		}
		if err := storage.Remove(loc); err != nil {
			cleanupErr := fmt.Errorf("DELETE 清理失败，临时对象仍可能存在 [%s]: %w", loc, err)
			if retErr == nil {
				retErr = cleanupErr
			} else {
				retErr = fmt.Errorf("%w; %v", retErr, cleanupErr)
			}
			return
		}
		fmt.Fprintf(out, "DELETE ok: %s\n", loc)
	}()

	putAttempted = true
	if err := storage.WriteFile(loc, payload); err != nil {
		return fmt.Errorf("PUT E2E 失败: %w", err)
	}
	fmt.Fprintln(out, "PUT ok")

	got, err := storage.ReadFile(loc)
	if err != nil {
		return fmt.Errorf("GET E2E 失败: %w", err)
	}
	gotHash := sha256.Sum256(got)
	fmt.Fprintf(out, "GET bytes=%d sha256=%x\n", len(got), gotHash)
	if len(got) != len(payload) {
		return fmt.Errorf("回读长度不一致: got=%d want=%d", len(got), len(payload))
	}
	if gotHash != wantHash {
		return fmt.Errorf("回读 SHA-256 不一致: got=%x want=%x", gotHash, wantHash)
	}
	fmt.Fprintln(out, "GET verify ok: length and SHA-256 match")
	return nil
}

var s3SmokeCmd = &cobra.Command{
	Use:         "s3-smoke",
	Short:       "验证受限 S3 的精确对象 PUT/GET/DELETE",
	Annotations: map[string]string{"skipStorageLocationValidation": "true"},
	Long: `对 analysed_dir 下的唯一临时对象执行一次精确 PutObject、GetObject 和 DeleteObject。
不调用 HeadBucket、HeadObject、ListBucket、ListObjects 或 multipart API。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataBytes, _ := cmd.Flags().GetInt("payload-bytes")
		return runS3Smoke(appconfig.Cfg.AnalysedDir, dataBytes, os.Stdout)
	},
}

func init() {
	s3SmokeCmd.Flags().Int("payload-bytes", defaultS3SmokePayloadBytes, "随机数据字节数（编码后的 JSON 会更大）")
	rootCmd.AddCommand(s3SmokeCmd)
}
