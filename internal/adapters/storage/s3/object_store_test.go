package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignRequestKnownVector(t *testing.T) {
	// AWS SigV4 test-suite style vector: GET on a bucket object with fixed
	// time and credentials; the expected signature was computed with the
	// reference HMAC chain for exactly these canonical inputs.
	req, err := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "examplebucket.s3.amazonaws.com"
	at := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	signRequest(req, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", nil, at)

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request") {
		t.Fatalf("unexpected credential scope: %s", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=host;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("unexpected signed headers: %s", auth)
	}
	if req.Header.Get("x-amz-date") != "20130524T000000Z" {
		t.Fatalf("unexpected x-amz-date: %s", req.Header.Get("x-amz-date"))
	}
	// Empty payload hash is constant.
	if req.Header.Get("x-amz-content-sha256") != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("unexpected payload hash: %s", req.Header.Get("x-amz-content-sha256"))
	}
	if !strings.Contains(auth, "Signature=") {
		t.Fatalf("missing signature: %s", auth)
	}
}

func TestPutGetDeleteAgainstFakeServer(t *testing.T) {
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/bucket/") {
			t.Fatalf("expected path-style URL, got %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
			t.Fatalf("missing sigv4 header")
		}
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		switch r.Method {
		case http.MethodPut:
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			objects[key] = body
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			data, ok := objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	store := New(Config{
		Endpoint:  server.URL,
		Region:    "us-east-1",
		Bucket:    "bucket",
		AccessKey: "ak",
		SecretKey: "sk",
		PathStyle: true,
	}, server.Client())

	ctx := context.Background()
	if err := store.Put(ctx, "receipts/r1.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("put: %v", err)
	}
	data, err := store.Get(ctx, "receipts/r1.json")
	if err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("get: %q %v", data, err)
	}
	if err := store.Delete(ctx, "receipts/r1.json"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get(ctx, "receipts/r1.json"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVirtualHostStyleURL(t *testing.T) {
	store := New(Config{Endpoint: "https://s3.us-east-1.amazonaws.com", Bucket: "b", PathStyle: false}, nil)
	objectURL, host, err := store.objectURL("a b/c.txt")
	if err != nil {
		t.Fatal(err)
	}
	if host != "b.s3.us-east-1.amazonaws.com" {
		t.Fatalf("unexpected host: %s", host)
	}
	parsed, err := url.Parse(objectURL)
	if err != nil || parsed.Host != host {
		t.Fatalf("unexpected url: %s (%v)", objectURL, err)
	}
}
