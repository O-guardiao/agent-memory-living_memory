package s3

// Placeholder for S3-compatible object store adapter.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound is returned by Get for missing keys.
var ErrNotFound = errors.New("s3: object not found")

type Config struct {
	// Endpoint like http://minio:9000; empty targets AWS for the region.
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PathStyle bool
}

// ObjectStore implements ports.ObjectStore against any S3-compatible
// service using stdlib SigV4 signing (whole-object PUT/GET/DELETE).
type ObjectStore struct {
	cfg    Config
	client *http.Client
	now    func() time.Time
}

func New(cfg Config, client *http.Client) *ObjectStore {
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", cfg.Region)
	}
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	return &ObjectStore{cfg: cfg, client: client, now: time.Now}
}

func (s *ObjectStore) Put(ctx context.Context, key string, data []byte) error {
	resp, err := s.do(ctx, http.MethodPut, key, data)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 {
		return s.statusError("put", key, resp)
	}
	return nil
}

func (s *ObjectStore) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := s.do(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	defer drain(resp)
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode >= 300 {
		return nil, s.statusError("get", key, resp)
	}
	return io.ReadAll(resp.Body)
}

func (s *ObjectStore) Delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return s.statusError("delete", key, resp)
	}
	return nil
}

func (s *ObjectStore) do(ctx context.Context, method, key string, body []byte) (*http.Response, error) {
	objectURL, host, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, objectURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Host = host
	signRequest(req, s.cfg.AccessKey, s.cfg.SecretKey, s.cfg.Region, body, s.now().UTC())
	return s.client.Do(req)
}

func (s *ObjectStore) objectURL(key string) (string, string, error) {
	base, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return "", "", err
	}
	escaped := escapeKey(key)
	if s.cfg.PathStyle {
		return fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, escaped), base.Host, nil
	}
	host := s.cfg.Bucket + "." + base.Host
	return fmt.Sprintf("%s://%s/%s", base.Scheme, host, escaped), host, nil
}

func (s *ObjectStore) statusError(op, key string, resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("s3 %s %s: status %d: %s", op, key, resp.StatusCode, string(data))
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	resp.Body.Close()
}
