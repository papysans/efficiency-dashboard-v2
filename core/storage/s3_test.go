package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type observedS3Put struct {
	method           string
	path             string
	query            url.Values
	contentLength    int64
	header           http.Header
	trailer          http.Header
	transferEncoding []string
	body             []byte
}

func TestS3WriteUsesPlainSeekableBodyWithoutOptionalChecksums(t *testing.T) {
	var observed observedS3Put
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		observed = observedS3Put{
			method:           r.Method,
			path:             r.URL.Path,
			query:            r.URL.Query(),
			contentLength:    r.ContentLength,
			header:           r.Header.Clone(),
			trailer:          r.Trailer.Clone(),
			transferEncoding: append([]string(nil), r.TransferEncoding...),
			body:             body,
		}
		w.Header().Set("ETag", `"test-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend, err := newS3Backend(S3Config{
		Endpoint:  server.URL,
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		Region:    "us-east-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"session_id":"session-1","fingerprints":["abc"]}`)
	loc := "s3://kb-server/efficiency-dashboard/analysed/task/conversation/session-1.silica.json"
	if err := backend.WriteFile(loc, payload); err != nil {
		t.Fatal(err)
	}

	if observed.method != http.MethodPut {
		t.Fatalf("method = %q, want PUT", observed.method)
	}
	wantPath := "/kb-server/efficiency-dashboard/analysed/task/conversation/session-1.silica.json"
	if observed.path != wantPath {
		t.Errorf("path = %q, want %q", observed.path, wantPath)
	}
	if got := observed.query.Get("x-id"); got != "PutObject" {
		t.Errorf("x-id = %q, want PutObject", got)
	}
	if len(observed.query) != 1 {
		t.Errorf("unexpected query: %v", observed.query)
	}
	if observed.contentLength != int64(len(payload)) {
		t.Errorf("Content-Length = %d, want %d", observed.contentLength, len(payload))
	}
	if !bytes.Equal(observed.body, payload) {
		t.Errorf("body = %q, want %q", observed.body, payload)
	}
	if len(observed.transferEncoding) != 0 {
		t.Errorf("unexpected transfer encoding: %v", observed.transferEncoding)
	}
	if len(observed.trailer) != 0 {
		t.Errorf("unexpected trailer: %v", observed.trailer)
	}
	for name, values := range observed.header {
		lowerName := strings.ToLower(name)
		lowerValues := strings.ToLower(strings.Join(values, ","))
		if strings.Contains(lowerName, "crc32") ||
			strings.Contains(lowerName, "checksum") ||
			lowerName == "x-amz-trailer" ||
			strings.Contains(lowerValues, "aws-chunked") {
			t.Errorf("unsupported S3 header %s=%q", name, values)
		}
	}
	sum := sha256.Sum256(payload)
	if got, want := observed.header.Get("X-Amz-Content-Sha256"), hex.EncodeToString(sum[:]); got != want {
		t.Errorf("X-Amz-Content-Sha256 = %q, want payload hash %q", got, want)
	}
}

func TestS3OpenUsesOneExactGetWithoutHeadOrChecksumMode(t *testing.T) {
	var methods []string
	var observedPath string
	var observedQuery url.Values
	var observedHeader http.Header
	payload := []byte("stored object")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		observedPath = r.URL.Path
		observedQuery = r.URL.Query()
		observedHeader = r.Header.Clone()
		w.Header().Set("Content-Length", "13")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	backend, err := newS3Backend(S3Config{
		Endpoint:  server.URL,
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	rc, err := backend.Open("s3://kb-server/exact/key.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("body = %q, want %q", got, payload)
	}
	if len(methods) != 1 || methods[0] != http.MethodGet {
		t.Fatalf("methods = %v, want one GET", methods)
	}
	if observedPath != "/kb-server/exact/key.json" {
		t.Errorf("path = %q", observedPath)
	}
	if got := observedQuery.Get("x-id"); got != "GetObject" || len(observedQuery) != 1 {
		t.Errorf("query = %v, want only x-id=GetObject", observedQuery)
	}
	if value := observedHeader.Get("X-Amz-Checksum-Mode"); value != "" {
		t.Errorf("unexpected X-Amz-Checksum-Mode: %q", value)
	}
}

func TestS3OpenMapsNoSuchKeyToNotExist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`)
	}))
	defer server.Close()

	backend, err := newS3Backend(S3Config{
		Endpoint:  server.URL,
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Open("s3://kb-server/missing.json"); !IsNotExist(err) {
		t.Fatalf("Open missing err = %v, want IsNotExist", err)
	}
}

func TestS3EndpointSupportsLegacyAndExplicitSchemes(t *testing.T) {
	tests := []struct {
		name       string
		cfg        S3Config
		want       string
		wantSecure bool
		wantErr    bool
	}{
		{
			name: "legacy http",
			cfg:  S3Config{Endpoint: "s3.internal:80"},
			want: "http://s3.internal:80",
		},
		{
			name:       "legacy https",
			cfg:        S3Config{Endpoint: "s3.internal:443", UseSSL: true},
			want:       "https://s3.internal:443",
			wantSecure: true,
		},
		{
			name:       "explicit scheme",
			cfg:        S3Config{Endpoint: "https://s3.internal/base/"},
			want:       "https://s3.internal/base",
			wantSecure: true,
		},
		{
			name:    "unsupported scheme",
			cfg:     S3Config{Endpoint: "ftp://s3.internal"},
			wantErr: true,
		},
		{
			name:    "missing host",
			cfg:     S3Config{Endpoint: "https://"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, secure, err := s3Endpoint(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("s3Endpoint() err=%v, wantErr=%v", err, tt.wantErr)
			}
			if err == nil && (got != tt.want || secure != tt.wantSecure) {
				t.Errorf("s3Endpoint() = (%q, %v), want (%q, %v)", got, secure, tt.want, tt.wantSecure)
			}
		})
	}
}
