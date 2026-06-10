// Package httpretry provides the shared JSON-over-HTTP call pattern for
// remote provider adapters: POST a JSON body, retry transient failures
// (429 and 5xx, honoring Retry-After) with exponential backoff, and decode
// the JSON response.
package httpretry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const maxAttempts = 3

var backoff = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}

func PostJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body any, out any) error {
	if client == nil {
		client = http.DefaultClient
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff[attempt-1]):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil {
				return nil
			}
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			return nil
		}
		lastErr = fmt.Errorf("%s: status %d: %s", url, resp.StatusCode, truncate(data, 4096))
		if !retryable(resp.StatusCode) {
			return lastErr
		}
		if wait := retryAfter(resp); wait > 0 && attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	return lastErr
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func retryAfter(resp *http.Response) time.Duration {
	raw := resp.Header.Get("Retry-After")
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 && seconds <= 30 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func truncate(data []byte, limit int) string {
	if len(data) > limit {
		data = data[:limit]
	}
	return string(data)
}
