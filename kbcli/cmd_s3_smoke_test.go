package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"kanban/core/storage"
)

func TestRunS3SmokeUsesOnlyExactPutGetDeleteAndCleansUp(t *testing.T) {
	var mu sync.Mutex
	var stored []byte
	var methods []string
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)
		wantOperation := map[string]string{
			http.MethodPut:    "PutObject",
			http.MethodGet:    "GetObject",
			http.MethodDelete: "DeleteObject",
		}[r.Method]
		query := r.URL.Query()
		if got := query.Get("x-id"); got != wantOperation || len(query) != 1 {
			http.Error(w, "unexpected operation query", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			var err error
			stored, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case http.MethodGet:
			w.Header().Set("Content-Length", fmt.Sprint(len(stored)))
			_, _ = w.Write(stored)
		case http.MethodDelete:
			stored = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "operation not supported", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	err := storage.Configure(storage.Config{S3: storage.S3Config{
		Endpoint:  server.URL,
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		Region:    "us-east-1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Configure(storage.Config{}) }()

	var out bytes.Buffer
	if err := runS3Smoke("s3://kb-server/efficiency-dashboard/analysed", 70*1024, &out); err != nil {
		t.Fatalf("runS3Smoke: %v\noutput:\n%s", err, out.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := strings.Join(methods, ","), "PUT,GET,DELETE"; got != want {
		t.Fatalf("methods = %s, want %s", got, want)
	}
	if len(paths) != 3 || paths[0] != paths[1] || paths[1] != paths[2] {
		t.Fatalf("requests did not use one exact object: %v", paths)
	}
	if !strings.HasPrefix(paths[0], "/kb-server/efficiency-dashboard/analysed/_smoke/kbcli-") {
		t.Errorf("unexpected object path: %s", paths[0])
	}
	if stored != nil {
		t.Fatalf("temporary object was not deleted, bytes=%d sha256=%x", len(stored), sha256.Sum256(stored))
	}
	output := out.String()
	for _, text := range []string{"PUT ok", "GET verify ok", "DELETE ok"} {
		if !strings.Contains(output, text) {
			t.Errorf("output missing %q:\n%s", text, output)
		}
	}
}

func TestRunS3SmokeRejectsLocalAnalysedDir(t *testing.T) {
	if err := runS3Smoke("./analysed", 1024, io.Discard); err == nil {
		t.Fatal("expected local analysed_dir to be rejected")
	}
}

func TestRunS3SmokeCleansUpAfterGetFailure(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			http.Error(w, "gateway read failure", http.StatusInternalServerError)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	err := storage.Configure(storage.Config{S3: storage.S3Config{
		Endpoint:  server.URL,
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		Region:    "us-east-1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Configure(storage.Config{}) }()

	if err := runS3Smoke("s3://kb-server/analysed", 1024, io.Discard); err == nil {
		t.Fatal("expected GET failure")
	}
	if len(methods) < 3 || methods[0] != http.MethodPut || methods[len(methods)-1] != http.MethodDelete {
		t.Fatalf("methods = %v, want PUT, one or more GET retries, DELETE", methods)
	}
	for _, method := range methods[1 : len(methods)-1] {
		if method != http.MethodGet {
			t.Fatalf("methods = %v, unexpected operation %s", methods, method)
		}
	}
}
