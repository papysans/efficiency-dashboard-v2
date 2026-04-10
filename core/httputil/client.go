// Package httputil provides shared HTTP client instances with standard timeouts.
package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultClient is a shared HTTP client with a 30-second timeout.
// Use for general API calls where responses may be slow.
var DefaultClient = &http.Client{
	Timeout: 30 * time.Second,
}

// FastClient is a shared HTTP client with a 10-second timeout.
// Use for K-line and real-time data fetches where speed matters.
var FastClient = &http.Client{
	Timeout: 10 * time.Second,
}

// FetchURL performs a GET request with optional headers and 3-retry exponential backoff.
func FetchURL(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	delays := []time.Duration{0, time.Second, 2 * time.Second}
	var lastErr error
	for i, delay := range delays {
		if delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			if i < len(delays)-1 {
				continue
			}
			return nil, lastErr
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
			if i < len(delays)-1 {
				continue
			}
			return nil, lastErr
		}
		return io.ReadAll(resp.Body)
	}
	return nil, lastErr
}
