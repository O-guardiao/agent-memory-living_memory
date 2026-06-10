// Package redis implements the small RESP2 subset the project needs
// (GET/SET/DEL/INCR/PEXPIRE) over plain TCP, keeping the no-dependency
// ethos of the other remote adapters.
package redis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Value is a decoded RESP reply.
type Value struct {
	Kind  byte // '+', '-', ':', '$', '*'
	Str   string
	Int   int64
	Null  bool
	Array []Value
}

type Client struct {
	addr     string
	password string

	mu   sync.Mutex
	conn net.Conn
	r    *bufio.Reader
}

func NewClient(addr, password string) *Client {
	return &Client{addr: addr, password: password}
}

// Do sends a command and returns the decoded reply. On transport errors it
// reconnects once and retries.
func (c *Client) Do(ctx context.Context, args ...string) (Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, err := c.doLocked(ctx, args)
	if err != nil && !isRedisError(err) {
		c.closeLocked()
		value, err = c.doLocked(ctx, args)
	}
	return value, err
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
	return nil
}

func (c *Client) doLocked(ctx context.Context, args []string) (Value, error) {
	if err := c.connectLocked(ctx); err != nil {
		return Value{}, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(deadline)
	} else {
		_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	if _, err := c.conn.Write(encodeCommand(args)); err != nil {
		return Value{}, err
	}
	return readReply(c.r)
}

func (c *Client) connectLocked(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}
	c.conn = conn
	c.r = bufio.NewReader(conn)
	if c.password != "" {
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.Write(encodeCommand([]string{"AUTH", c.password})); err != nil {
			c.closeLocked()
			return err
		}
		reply, err := readReply(c.r)
		if err != nil {
			c.closeLocked()
			return err
		}
		if reply.Kind == '-' {
			c.closeLocked()
			return fmt.Errorf("redis auth: %s", reply.Str)
		}
	}
	return nil
}

func (c *Client) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.r = nil
	}
}

func encodeCommand(args []string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return []byte(b.String())
}

type redisError struct{ msg string }

func (e redisError) Error() string { return "redis: " + e.msg }

func isRedisError(err error) bool {
	var re redisError
	return errors.As(err, &re)
}

func readReply(r *bufio.Reader) (Value, error) {
	line, err := readLine(r)
	if err != nil {
		return Value{}, err
	}
	if len(line) == 0 {
		return Value{}, errors.New("redis: empty reply line")
	}
	kind, rest := line[0], line[1:]
	switch kind {
	case '+':
		return Value{Kind: kind, Str: rest}, nil
	case '-':
		return Value{Kind: kind, Str: rest}, redisError{msg: rest}
	case ':':
		n, err := strconv.ParseInt(rest, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("redis: bad integer reply %q", rest)
		}
		return Value{Kind: kind, Int: n}, nil
	case '$':
		size, err := strconv.Atoi(rest)
		if err != nil {
			return Value{}, fmt.Errorf("redis: bad bulk length %q", rest)
		}
		if size < 0 {
			return Value{Kind: kind, Null: true}, nil
		}
		buf := make([]byte, size+2)
		if _, err := ioReadFull(r, buf); err != nil {
			return Value{}, err
		}
		return Value{Kind: kind, Str: string(buf[:size])}, nil
	case '*':
		count, err := strconv.Atoi(rest)
		if err != nil {
			return Value{}, fmt.Errorf("redis: bad array length %q", rest)
		}
		if count < 0 {
			return Value{Kind: kind, Null: true}, nil
		}
		items := make([]Value, 0, count)
		for i := 0; i < count; i++ {
			item, err := readReply(r)
			if err != nil && !isRedisError(err) {
				return Value{}, err
			}
			items = append(items, item)
		}
		return Value{Kind: kind, Array: items}, nil
	default:
		return Value{}, fmt.Errorf("redis: unknown reply type %q", kind)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
