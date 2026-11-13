package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRedisStateKey = "aibuddy:state:v1"
	defaultRedisTTL      = time.Hour
)

// RedisClient is a minimal, zero-dependency RESP2 TCP client for Redis 7.
type RedisClient struct {
	addr     string
	password string
	timeout  time.Duration
}

// NewRedisClient configures a RESP2 client for the given host:port and optional password.
func NewRedisClient(addr, password string, timeout time.Duration) *RedisClient {
	if timeout <= 0 {
		timeout = 800 * time.Millisecond
	}
	return &RedisClient{
		addr:     strings.TrimSpace(addr),
		password: password,
		timeout:  timeout,
	}
}

// Ping verifies connectivity and authentication against Redis.
func (r *RedisClient) Ping(ctx context.Context) error {
	resp, err := r.exec(ctx, "PING")
	if err != nil {
		return err
	}
	if !strings.EqualFold(resp.str, "PONG") {
		return fmt.Errorf("unexpected redis ping reply: %q", resp.str)
	}
	return nil
}

// Get reads a cached UTF-8/JSON payload by key. Returns found=false on cache miss.
func (r *RedisClient) Get(ctx context.Context, key string) (string, bool, error) {
	resp, err := r.exec(ctx, "GET", key)
	if err != nil {
		return "", false, err
	}
	if resp.isNil {
		return "", false, nil
	}
	return resp.str, true, nil
}

// SetEX stores key=value with an expiration duration.
func (r *RedisClient) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = int(defaultRedisTTL.Seconds())
	}
	_, err := r.exec(ctx, "SET", key, value, "EX", strconv.Itoa(seconds))
	return err
}

// Del deletes one or more keys from Redis.
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	args := make([]string, 0, len(keys)+1)
	args = append(args, "DEL")
	args = append(args, keys...)
	_, err := r.exec(ctx, args...)
	return err
}

type respValue struct {
	str   string
	num   int64
	isNil bool
}

func (r *RedisClient) exec(ctx context.Context, args ...string) (respValue, error) {
	if r == nil || r.addr == "" {
		return respValue{}, errors.New("redis address is empty")
	}

	dialer := net.Dialer{Timeout: r.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return respValue{}, err
	}
	defer conn.Close()

	deadline := time.Now().Add(r.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	reader := bufio.NewReader(conn)
	if r.password != "" {
		if err := writeRESPCommand(conn, "AUTH", r.password); err != nil {
			return respValue{}, err
		}
		if _, err := readRESPValue(reader); err != nil {
			return respValue{}, fmt.Errorf("redis auth failed: %w", err)
		}
	}

	if err := writeRESPCommand(conn, args...); err != nil {
		return respValue{}, err
	}
	return readRESPValue(reader)
}

func writeRESPCommand(w io.Writer, args ...string) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func readRESPValue(r *bufio.Reader) (respValue, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return respValue{}, err
	}
	if len(line) < 3 {
		return respValue{}, fmt.Errorf("malformed resp line: %q", line)
	}
	prefix := line[0]
	payload := strings.TrimSuffix(strings.TrimSuffix(line[1:], "\n"), "\r")

	switch prefix {
	case '+':
		return respValue{str: payload}, nil
	case '-':
		return respValue{}, fmt.Errorf("redis error: %s", payload)
	case ':':
		n, err := strconv.ParseInt(payload, 10, 64)
		if err != nil {
			return respValue{}, err
		}
		return respValue{num: n}, nil
	case '$':
		length, err := strconv.Atoi(payload)
		if err != nil {
			return respValue{}, err
		}
		if length < 0 {
			return respValue{isNil: true}, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return respValue{}, err
		}
		return respValue{str: string(buf[:length])}, nil
	default:
		return respValue{}, fmt.Errorf("unsupported resp prefix %q", prefix)
	}
}

// CachedRepository adds a non-blocking Redis read-through / write-through cache
// on top of a primary Repository (PostgreSQL or Memory).
type CachedRepository struct {
	primary Repository
	redis   *RedisClient
	key     string
	ttl     time.Duration
}

// NewCachedRepository wraps primary with a Redis 7 cache layer.
func NewCachedRepository(primary Repository, redis *RedisClient) *CachedRepository {
	return &CachedRepository{
		primary: primary,
		redis:   redis,
		key:     defaultRedisStateKey,
		ttl:     defaultRedisTTL,
	}
}

// LoadState checks Redis first for sub-millisecond reads, falling back to primary on miss or error.
func (c *CachedRepository) LoadState(ctx context.Context) (*AppState, error) {
	if c.redis != nil {
		if raw, ok, err := c.redis.Get(ctx, c.key); err == nil && ok {
			var state AppState
			if unmarshalErr := json.Unmarshal([]byte(raw), &state); unmarshalErr == nil {
				if state.Users == nil {
					state.Users = DefaultUsers()
				}
				return state.Clone(), nil
			}
		}
	}

	state, err := c.primary.LoadState(ctx)
	if err != nil {
		return nil, err
	}

	if c.redis != nil {
		if encoded, err := json.Marshal(state); err == nil {
			_ = c.redis.SetEX(ctx, c.key, string(encoded), c.ttl)
		}
	}
	return state, nil
}

// SaveState persists synchronously to primary and refreshes Redis cache cleanly.
func (c *CachedRepository) SaveState(ctx context.Context, state *AppState) error {
	if err := c.primary.SaveState(ctx, state); err != nil {
		return err
	}
	if c.redis != nil && state != nil {
		if encoded, err := json.Marshal(state); err == nil {
			_ = c.redis.SetEX(ctx, c.key, string(encoded), c.ttl)
		}
	}
	return nil
}

// AppendClickstream forwards telemetry records to primary storage.
func (c *CachedRepository) AppendClickstream(ctx context.Context, record ClickstreamRecord) error {
	return c.primary.AppendClickstream(ctx, record)
}

// Mode returns the composite backend label (e.g. "postgres+redis" or "memory+redis").
func (c *CachedRepository) Mode() string {
	return c.primary.Mode() + "+redis"
}

// Healthy returns true when primary storage is healthy.
func (c *CachedRepository) Healthy(ctx context.Context) bool {
	return c.primary.Healthy(ctx)
}

// Close closes the underlying primary repository.
func (c *CachedRepository) Close() error {
	return c.primary.Close()
}
