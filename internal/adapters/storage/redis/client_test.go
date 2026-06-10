package redis

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestEncodeCommand(t *testing.T) {
	got := string(encodeCommand([]string{"SET", "k", "v"}))
	want := "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"
	if got != want {
		t.Fatalf("unexpected encoding: %q", got)
	}
}

func TestReadReplyKinds(t *testing.T) {
	cases := []struct {
		raw   string
		check func(t *testing.T, v Value, err error)
	}{
		{"+OK\r\n", func(t *testing.T, v Value, err error) {
			if err != nil || v.Str != "OK" {
				t.Fatalf("simple string: %v %v", v, err)
			}
		}},
		{":42\r\n", func(t *testing.T, v Value, err error) {
			if err != nil || v.Int != 42 {
				t.Fatalf("integer: %v %v", v, err)
			}
		}},
		{"$5\r\nhello\r\n", func(t *testing.T, v Value, err error) {
			if err != nil || v.Str != "hello" {
				t.Fatalf("bulk: %v %v", v, err)
			}
		}},
		{"$-1\r\n", func(t *testing.T, v Value, err error) {
			if err != nil || !v.Null {
				t.Fatalf("null bulk: %v %v", v, err)
			}
		}},
		{"-ERR boom\r\n", func(t *testing.T, v Value, err error) {
			if err == nil || !isRedisError(err) {
				t.Fatalf("expected redis error, got %v %v", v, err)
			}
		}},
		{"*2\r\n:1\r\n$1\r\na\r\n", func(t *testing.T, v Value, err error) {
			if err != nil || len(v.Array) != 2 || v.Array[1].Str != "a" {
				t.Fatalf("array: %v %v", v, err)
			}
		}},
	}
	for _, tc := range cases {
		v, err := readReply(bufio.NewReader(strings.NewReader(tc.raw)))
		tc.check(t, v, err)
	}
}

// fakeServer answers each received command with the next canned reply.
func fakeServer(t *testing.T, replies []string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for _, reply := range replies {
			// Consume one full command (array of bulk strings).
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			count := 0
			if _, err := fmtSscanf(line, &count); err != nil {
				return
			}
			for i := 0; i < count*2; i++ {
				if _, err := reader.ReadString('\n'); err != nil {
					return
				}
			}
			if _, err := conn.Write([]byte(reply)); err != nil {
				return
			}
		}
	}()
	return listener.Addr().String()
}

func fmtSscanf(line string, count *int) (int, error) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
	n, err := parseInt(line)
	*count = n
	return n, err
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &net.ParseError{Type: "int", Text: s}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func TestCacheAgainstFakeServer(t *testing.T) {
	addr := fakeServer(t, []string{"+OK\r\n", "$5\r\nhello\r\n"})
	cache := NewCache(NewClient(addr, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := cache.Set(ctx, "k", []byte("hello"), time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, ok, err := cache.Get(ctx, "k")
	if err != nil || !ok || string(value) != "hello" {
		t.Fatalf("get: %q ok=%v err=%v", value, ok, err)
	}
}

func TestRateLimiterCountsWindow(t *testing.T) {
	addr := fakeServer(t, []string{":1\r\n", ":1\r\n", ":3\r\n"})
	limiter := NewRateLimiter(NewClient(addr, ""), 1, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	allowed, err := limiter.Allow(ctx, "tenant")
	if err != nil || !allowed {
		t.Fatalf("first allow: %v %v", allowed, err)
	}
	allowed, err = limiter.Allow(ctx, "tenant")
	if err != nil || allowed {
		t.Fatalf("expected limit at count 3 (rps+burst=2): %v %v", allowed, err)
	}
}
