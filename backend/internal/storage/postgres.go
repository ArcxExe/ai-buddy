package storage

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const pgDefaultStateKey = "default"

type pgConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

type pgRunner interface {
	Query(ctx context.Context, sql string) ([][]string, error)
}

// PostgresRepository stores canonical AppState snapshots and Clickstream events in PostgreSQL 16.
type PostgresRepository struct {
	runner pgRunner
}

// NewPostgresRepository creates a PostgreSQL-backed Repository using the built-in Postgres v3 wire client.
func NewPostgresRepository(dsn string, timeout time.Duration) (*PostgresRepository, error) {
	cfg, err := parsePostgresDSN(dsn)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &PostgresRepository{
		runner: &wirePGClient{cfg: cfg, timeout: timeout},
	}, nil
}

// NewPostgresRepositoryWithRunner injects a custom pgRunner (for deterministic unit testing).
func NewPostgresRepositoryWithRunner(runner pgRunner) *PostgresRepository {
	return &PostgresRepository{runner: runner}
}

// InitSchema ensures the PostgreSQL 16 persistence tables exist.
func (p *PostgresRepository) InitSchema(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS aibuddy_state (
	state_key TEXT PRIMARY KEY,
	payload JSONB NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS aibuddy_clickstream (
	id TEXT PRIMARY KEY,
	event_type TEXT NOT NULL,
	actor_role TEXT NOT NULL,
	route TEXT,
	latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
	payload JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`
	_, err := p.runner.Query(ctx, ddl)
	return err
}

// LoadState fetches and decodes the persisted AppState JSONB document.
func (p *PostgresRepository) LoadState(ctx context.Context) (*AppState, error) {
	sql := fmt.Sprintf("SELECT payload::text FROM aibuddy_state WHERE state_key = %s LIMIT 1;", dollarQuote(pgDefaultStateKey))
	rows, err := p.runner.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || len(rows[0]) == 0 || strings.TrimSpace(rows[0][0]) == "" {
		return nil, ErrNotFound
	}
	var state AppState
	if err := json.Unmarshal([]byte(rows[0][0]), &state); err != nil {
		return nil, fmt.Errorf("postgres decode state: %w", err)
	}
	if state.Users == nil {
		state.Users = DefaultUsers()
	}
	return state.Clone(), nil
}

// SaveState upserts AppState atomically as JSONB.
func (p *PostgresRepository) SaveState(ctx context.Context, state *AppState) error {
	if state == nil {
		return nil
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf(
		"INSERT INTO aibuddy_state (state_key, payload, updated_at) VALUES (%s, %s::jsonb, NOW()) ON CONFLICT (state_key) DO UPDATE SET payload = EXCLUDED.payload, updated_at = NOW();",
		dollarQuote(pgDefaultStateKey),
		dollarQuote(string(payload)),
	)
	_, err = p.runner.Query(ctx, sql)
	return err
}

// AppendClickstream persists a telemetry record into PostgreSQL.
func (p *PostgresRepository) AppendClickstream(ctx context.Context, record ClickstreamRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf(
		"INSERT INTO aibuddy_clickstream (id, event_type, actor_role, route, latency_ms, payload) VALUES (%s, %s, %s, %s, %.3f, %s::jsonb) ON CONFLICT (id) DO NOTHING;",
		dollarQuote(record.ID),
		dollarQuote(record.EventType),
		dollarQuote(string(record.ActorRole)),
		dollarQuote(record.Route),
		record.LatencyMs,
		dollarQuote(string(payload)),
	)
	_, err = p.runner.Query(ctx, sql)
	return err
}

// Mode identifies the PostgreSQL storage driver.
func (p *PostgresRepository) Mode() string {
	return "postgres"
}

// Healthy runs a lightweight SELECT 1 probe against PostgreSQL.
func (p *PostgresRepository) Healthy(ctx context.Context) bool {
	rows, err := p.runner.Query(ctx, "SELECT 1;")
	return err == nil && len(rows) > 0
}

// Close releases PostgresRepository resources (connections are short-lived per query).
func (p *PostgresRepository) Close() error {
	return nil
}

func dollarQuote(val string) string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	tag := "$ab_" + hex.EncodeToString(buf[:]) + "$"
	for strings.Contains(val, tag) {
		_, _ = rand.Read(buf[:])
		tag = "$ab_" + hex.EncodeToString(buf[:]) + "$"
	}
	return tag + val + tag
}

func parsePostgresDSN(dsn string) (pgConfig, error) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return pgConfig{}, errors.New("postgres dsn is empty")
	}

	cfg := pgConfig{
		Host:    "localhost",
		Port:    "5432",
		SSLMode: "disable",
	}

	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return pgConfig{}, fmt.Errorf("invalid postgres dsn: %w", err)
		}
		if host := u.Hostname(); host != "" {
			cfg.Host = host
		}
		if port := u.Port(); port != "" {
			cfg.Port = port
		}
		if u.User != nil {
			cfg.User = u.User.Username()
			cfg.Password, _ = u.User.Password()
		}
		cfg.Database = strings.TrimPrefix(u.Path, "/")
		if ssl := u.Query().Get("sslmode"); ssl != "" {
			cfg.SSLMode = ssl
		}
	} else {
		for _, field := range strings.Fields(trimmed) {
			kv := strings.SplitN(field, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			v := strings.Trim(strings.TrimSpace(kv[1]), "'\"")
			switch k {
			case "host":
				cfg.Host = v
			case "port":
				cfg.Port = v
			case "user":
				cfg.User = v
			case "password":
				cfg.Password = v
			case "dbname", "database":
				cfg.Database = v
			case "sslmode":
				cfg.SSLMode = v
			}
		}
	}

	if cfg.User == "" {
		cfg.User = "postgres"
	}
	if cfg.Database == "" {
		cfg.Database = cfg.User
	}
	return cfg, nil
}

type wirePGClient struct {
	cfg     pgConfig
	timeout time.Duration
}

func (c *wirePGClient) Query(ctx context.Context, sql string) ([][]string, error) {
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(c.cfg.Host, c.cfg.Port))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	// Negotiate SSL when sslmode != disable.
	if c.cfg.SSLMode != "disable" && c.cfg.SSLMode != "" {
		var sslReq [8]byte
		binary.BigEndian.PutUint32(sslReq[0:4], 8)
		binary.BigEndian.PutUint32(sslReq[4:8], 80877103)
		if _, err := conn.Write(sslReq[:]); err != nil {
			return nil, err
		}
		var reply [1]byte
		if _, err := io.ReadFull(conn, reply[:]); err != nil {
			return nil, err
		}
		if reply[0] == 'S' {
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         c.cfg.Host,
				InsecureSkipVerify: c.cfg.SSLMode == "require", //nolint:gosec
			})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				return nil, err
			}
			conn = tlsConn
		}
	}

	reader := bufio.NewReader(conn)
	if err := pgStartupAndAuth(conn, reader, c.cfg); err != nil {
		return nil, err
	}

	// Send Terminate cleanly when exiting.
	defer func() {
		_, _ = conn.Write([]byte{'X', 0, 0, 0, 4})
	}()

	if err := pgWriteMsg(conn, 'Q', append([]byte(sql), 0)); err != nil {
		return nil, err
	}

	var rows [][]string
	var queryErr error
	for {
		msgType, body, err := pgReadMsg(reader)
		if err != nil {
			return nil, err
		}
		switch msgType {
		case 'D':
			rows = append(rows, pgParseDataRow(body))
		case 'E':
			queryErr = fmt.Errorf("postgres error: %s", pgParseErrorMessage(body))
		case 'Z':
			if queryErr != nil {
				return nil, queryErr
			}
			return rows, nil
		}
	}
}

func pgStartupAndAuth(conn net.Conn, reader *bufio.Reader, cfg pgConfig) error {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, int32(196608)) // protocol 3.0
	body.WriteString("user\x00" + cfg.User + "\x00")
	body.WriteString("database\x00" + cfg.Database + "\x00")
	body.WriteByte(0)

	var pkt bytes.Buffer
	_ = binary.Write(&pkt, binary.BigEndian, int32(body.Len()+4))
	pkt.Write(body.Bytes())
	if _, err := conn.Write(pkt.Bytes()); err != nil {
		return err
	}

	var scram *scramConversation
	for {
		msgType, msgBody, err := pgReadMsg(reader)
		if err != nil {
			return err
		}
		switch msgType {
		case 'R':
			if len(msgBody) < 4 {
				return errors.New("postgres truncated auth message")
			}
			authType := int32(binary.BigEndian.Uint32(msgBody[0:4]))
			switch authType {
			case 0:
				// AuthenticationOk
			case 3:
				pw := append([]byte(cfg.Password), 0)
				if err := pgWriteMsg(conn, 'p', pw); err != nil {
					return err
				}
			case 5:
				if len(msgBody) < 8 {
					return errors.New("postgres md5 auth missing salt")
				}
				digest := pgMD5Digest(cfg.User, cfg.Password, msgBody[4:8])
				if err := pgWriteMsg(conn, 'p', append([]byte(digest), 0)); err != nil {
					return err
				}
			case 10:
				mechs := strings.Split(string(msgBody[4:]), "\x00")
				supportsSCRAM := false
				for _, m := range mechs {
					if m == "SCRAM-SHA-256" {
						supportsSCRAM = true
						break
					}
				}
				if !supportsSCRAM {
					return errors.New("postgres server does not offer SCRAM-SHA-256")
				}
				scram, err = newScramConversation(cfg.Password)
				if err != nil {
					return err
				}
				initMsg := scram.initialPayload()
				if err := pgWriteMsg(conn, 'p', initMsg); err != nil {
					return err
				}
			case 11:
				if scram == nil {
					return errors.New("unexpected SASLContinue without conversation")
				}
				finalResp, err := scram.step(string(msgBody[4:]))
				if err != nil {
					return err
				}
				if err := pgWriteMsg(conn, 'p', []byte(finalResp)); err != nil {
					return err
				}
			case 12:
				if scram == nil {
					return errors.New("unexpected SASLFinal without conversation")
				}
				if err := scram.verifyFinal(string(msgBody[4:])); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported postgres auth type %d", authType)
			}
		case 'E':
			return fmt.Errorf("postgres auth failed: %s", pgParseErrorMessage(msgBody))
		case 'Z':
			return nil
		}
	}
}

func pgMD5Digest(user, password string, salt []byte) string {
	h1 := md5.Sum([]byte(password + user))
	h1Hex := hex.EncodeToString(h1[:])
	h2Input := make([]byte, len(h1Hex)+len(salt))
	copy(h2Input, h1Hex)
	copy(h2Input[len(h1Hex):], salt)
	h2 := md5.Sum(h2Input)
	return "md5" + hex.EncodeToString(h2[:])
}

type scramConversation struct {
	password    string
	clientNonce string
	clientBare  string
	serverSig   []byte
}

func newScramConversation(password string) (*scramConversation, error) {
	var raw [18]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	nonce := base64.StdEncoding.EncodeToString(raw[:])
	return &scramConversation{
		password:    password,
		clientNonce: nonce,
		clientBare:  "n=,r=" + nonce,
	}, nil
}

func (s *scramConversation) initialPayload() []byte {
	clientFirst := "n,," + s.clientBare
	var buf bytes.Buffer
	buf.WriteString("SCRAM-SHA-256\x00")
	_ = binary.Write(&buf, binary.BigEndian, int32(len(clientFirst)))
	buf.WriteString(clientFirst)
	return buf.Bytes()
}

func (s *scramConversation) step(serverFirst string) (string, error) {
	parts := parseSCRAMFields(serverFirst)
	nonce := parts["r"]
	saltB64 := parts["s"]
	iterStr := parts["i"]

	if !strings.HasPrefix(nonce, s.clientNonce) {
		return "", errors.New("scram server nonce mismatch")
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return "", fmt.Errorf("scram decode salt: %w", err)
	}
	iter, err := strconv.Atoi(iterStr)
	if err != nil || iter <= 0 {
		return "", fmt.Errorf("scram invalid iteration count %q", iterStr)
	}

	saltedPassword := pbkdf2HMACSHA256([]byte(s.password), salt, iter, 32)
	clientKey := hmacSHA256(saltedPassword, []byte("Client Key"))
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(saltedPassword, []byte("Server Key"))

	clientFinalWithoutProof := "c=biws,r=" + nonce
	authMessage := s.clientBare + "," + serverFirst + "," + clientFinalWithoutProof

	clientSignature := hmacSHA256(storedKey[:], []byte(authMessage))
	proof := make([]byte, len(clientKey))
	for i := range clientKey {
		proof[i] = clientKey[i] ^ clientSignature[i]
	}

	s.serverSig = hmacSHA256(serverKey, []byte(authMessage))
	return clientFinalWithoutProof + ",p=" + base64.StdEncoding.EncodeToString(proof), nil
}

func (s *scramConversation) verifyFinal(serverFinal string) error {
	parts := parseSCRAMFields(serverFinal)
	sigB64, ok := parts["v"]
	if !ok {
		return fmt.Errorf("scram server final missing signature: %s", serverFinal)
	}
	gotSig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return err
	}
	if !hmac.Equal(gotSig, s.serverSig) {
		return errors.New("scram server signature verification failed")
	}
	return nil
}

func parseSCRAMFields(msg string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(msg, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func pbkdf2HMACSHA256(password, salt []byte, iterations, keyLen int) []byte {
	mac := hmac.New(sha256.New, password)
	hashLen := mac.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var dk []byte

	for block := 1; block <= numBlocks; block++ {
		mac.Reset()
		_, _ = mac.Write(salt)
		var be [4]byte
		binary.BigEndian.PutUint32(be[:], uint32(block))
		_, _ = mac.Write(be[:])
		u := mac.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)

		for i := 2; i <= iterations; i++ {
			mac.Reset()
			_, _ = mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

func pgWriteMsg(w io.Writer, msgType byte, body []byte) error {
	var hdr [5]byte
	hdr[0] = msgType
	binary.BigEndian.PutUint32(hdr[1:5], uint32(len(body)+4))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			return err
		}
	}
	return nil
}

func pgReadMsg(r *bufio.Reader) (byte, []byte, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	length := int(binary.BigEndian.Uint32(hdr[1:5]))
	if length < 4 || length > 32*1024*1024 {
		return 0, nil, fmt.Errorf("invalid postgres message length %d", length)
	}
	body := make([]byte, length-4)
	if len(body) > 0 {
		if _, err := io.ReadFull(r, body); err != nil {
			return 0, nil, err
		}
	}
	return hdr[0], body, nil
}

func pgParseDataRow(body []byte) []string {
	if len(body) < 2 {
		return nil
	}
	colCount := int(binary.BigEndian.Uint16(body[0:2]))
	offset := 2
	row := make([]string, 0, colCount)
	for i := 0; i < colCount && offset+4 <= len(body); i++ {
		colLen := int(int32(binary.BigEndian.Uint32(body[offset : offset+4])))
		offset += 4
		if colLen < 0 {
			row = append(row, "")
			continue
		}
		if offset+colLen > len(body) {
			break
		}
		row = append(row, string(body[offset:offset+colLen]))
		offset += colLen
	}
	return row
}

func pgParseErrorMessage(body []byte) string {
	idx := 0
	var msg, code string
	for idx < len(body) {
		fieldType := body[idx]
		idx++
		if fieldType == 0 {
			break
		}
		end := bytes.IndexByte(body[idx:], 0)
		if end < 0 {
			break
		}
		val := string(body[idx : idx+end])
		idx += end + 1
		switch fieldType {
		case 'M':
			msg = val
		case 'C':
			code = val
		}
	}
	if msg == "" {
		return "unknown error"
	}
	if code != "" {
		return msg + " (SQLSTATE " + code + ")"
	}
	return msg
}
