package storage

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"aibuddy/backend/internal/config"
	"aibuddy/backend/internal/domain"
)

func TestMemoryRepositoryIsolationAndClickstream(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	if repo.Mode() != "memory" || !repo.Healthy(ctx) {
		t.Fatalf("unexpected memory repo metadata")
	}

	st1, err := repo.LoadState(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	// Mutate loaded clone and verify internal store is unaffected.
	st1.ActiveRole = domain.RoleManager
	st1.Onboarding.Checklist[0].Status = domain.StatusDone
	st1.Users[domain.RoleIntern].Permissions[0] = "mutated"

	st2, err := repo.LoadState(ctx)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if st2.ActiveRole != domain.RoleIntern {
		t.Fatalf("expected isolated active role, got %s", st2.ActiveRole)
	}
	if st2.Onboarding.Checklist[0].Status == domain.StatusDone {
		t.Fatalf("expected isolated checklist slice")
	}
	if st2.Users[domain.RoleIntern].Permissions[0] == "mutated" {
		t.Fatalf("expected isolated user permissions slice")
	}

	st2.ActiveRole = domain.RoleMentor
	st2.CorporateRequests = append(st2.CorporateRequests, domain.CorporateRequest{
		ID:      "req-1",
		Status:  domain.StatusPendingApproval,
		Details: map[string]interface{}{"tags": []string{"go"}, "nested": map[string]interface{}{"ok": true}},
	})
	st2.Messages = append(st2.Messages, domain.ChatMessage{
		ID:              "msg-1",
		Sender:          "assistant",
		ProcessingSteps: []string{"step1"},
		Citations:       []domain.KnowledgeCitation{{ID: "k1", Title: "T"}},
		ActionCard:      &domain.CorporateRequest{ID: "req-1", Details: map[string]interface{}{"arr": []interface{}{"x"}}},
	})
	if err := repo.SaveState(ctx, st2); err != nil {
		t.Fatalf("save state: %v", err)
	}

	st3, _ := repo.LoadState(ctx)
	if st3.ActiveRole != domain.RoleMentor || len(st3.Messages) != 1 || st3.Messages[0].ActionCard.ID != "req-1" {
		t.Fatalf("unexpected saved state: %+v", st3)
	}

	for i := 0; i < 510; i++ {
		_ = repo.AppendClickstream(ctx, ClickstreamRecord{
			ID:        fmt.Sprintf("evt-%d", i),
			EventType: "test",
			ActorRole: domain.RoleIntern,
			Metadata:  map[string]string{"i": strconv.Itoa(i)},
		})
	}
	if repo.ClickstreamCount() != 500 {
		t.Fatalf("expected clickstream bounded to 500, got %d", repo.ClickstreamCount())
	}
	_ = repo.Close()
}

func TestRecalculateOnboardingAndOrderedUsers(t *testing.T) {
	ob := DefaultOnboardingState()
	if ob.OverallProgress != 0 || ob.CurrentStageID != "stage-week-1" {
		t.Fatalf("unexpected initial onboarding progress=%d stage=%s", ob.OverallProgress, ob.CurrentStageID)
	}

	// Complete stage-week-1 items (first 2).
	ob.Checklist[0].Status = domain.StatusDone
	ob.Checklist[1].Status = domain.StatusDone
	RecalculateOnboarding(&ob)

	if ob.OverallProgress != 50 {
		t.Fatalf("expected overall 50%%, got %d", ob.OverallProgress)
	}
	if ob.Stages[0].Status != "completed" || ob.Stages[0].ProgressPercent != 100 {
		t.Fatalf("expected stage 1 completed 100%%, got %+v", ob.Stages[0])
	}
	if ob.Stages[1].Status != "current" || ob.CurrentStageID != "stage-week-2" {
		t.Fatalf("expected stage 2 current, got %+v / %s", ob.Stages[1], ob.CurrentStageID)
	}
	if ob.NextRecommendedStep == nil || ob.NextRecommendedStep.ID != "step-03-absence-policy" {
		t.Fatalf("expected next step step-03-absence-policy, got %+v", ob.NextRecommendedStep)
	}

	// Uncheck item -> stage 1 must roll back to current (50%), stage 2 back to upcoming.
	ob.Checklist[1].Status = domain.StatusTodo
	RecalculateOnboarding(&ob)
	if ob.OverallProgress != 25 || ob.Stages[0].Status != "current" || ob.Stages[0].ProgressPercent != 50 || ob.Stages[1].Status != "upcoming" {
		t.Fatalf("rollback failed: progress=%d stages=%+v", ob.OverallProgress, ob.Stages)
	}

	// Complete all items.
	for i := range ob.Checklist {
		ob.Checklist[i].Status = domain.StatusDone
	}
	RecalculateOnboarding(&ob)
	if ob.OverallProgress != 100 || ob.NextRecommendedStep != nil || ob.Stages[2].Status != "completed" {
		t.Fatalf("expected 100%% completion: %+v", ob)
	}

	users := DefaultUsers()
	users[domain.Role("guest")] = domain.UserProfile{ID: "guest", Role: domain.Role("guest")}
	ordered := OrderedUsers(users)
	if len(ordered) != 4 || ordered[0].Role != domain.RoleIntern || ordered[1].Role != domain.RoleMentor || ordered[2].Role != domain.RoleManager || ordered[3].Role != "guest" {
		t.Fatalf("unexpected user ordering: %+v", ordered)
	}
}

func startFakeRedisServer(t *testing.T, expectedPass string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake redis: %v", err)
	}

	data := make(map[string]string)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				authed := expectedPass == ""
				for {
					line, err := r.ReadString('\n')
					if err != nil || !strings.HasPrefix(line, "*") {
						return
					}
					argc, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
					args := make([]string, 0, argc)
					for i := 0; i < argc; i++ {
						hdr, err := r.ReadString('\n')
						if err != nil {
							return
						}
						sz, _ := strconv.Atoi(strings.TrimSpace(hdr[1:]))
						buf := make([]byte, sz+2)
						if _, err := io.ReadFull(r, buf); err != nil {
							return
						}
						args = append(args, string(buf[:sz]))
					}
					if len(args) == 0 {
						continue
					}
					cmd := strings.ToUpper(args[0])
					if cmd == "AUTH" && len(args) > 1 {
						if args[1] == expectedPass {
							authed = true
							_, _ = io.WriteString(c, "+OK\r\n")
						} else {
							_, _ = io.WriteString(c, "-ERR invalid password\r\n")
						}
						continue
					}
					if !authed {
						_, _ = io.WriteString(c, "-NOAUTH Authentication required\r\n")
						continue
					}
					switch cmd {
					case "PING":
						_, _ = io.WriteString(c, "+PONG\r\n")
					case "SET":
						if len(args) >= 3 {
							data[args[1]] = args[2]
						}
						_, _ = io.WriteString(c, "+OK\r\n")
					case "GET":
						val, ok := data[args[1]]
						if !ok {
							_, _ = io.WriteString(c, "$-1\r\n")
						} else {
							_, _ = io.WriteString(c, fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))
						}
					case "DEL":
						deleted := 0
						for _, k := range args[1:] {
							if _, ok := data[k]; ok {
								delete(data, k)
								deleted++
							}
						}
						_, _ = io.WriteString(c, fmt.Sprintf(":%d\r\n", deleted))
					default:
						_, _ = io.WriteString(c, "-ERR unknown command\r\n")
					}
				}
			}(conn)
		}
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func TestRedisClientAndCachedRepository(t *testing.T) {
	addr, stop := startFakeRedisServer(t, "secret-redis")
	defer stop()

	ctx := context.Background()
	rc := NewRedisClient(addr, "secret-redis", time.Second)
	if err := rc.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	if err := rc.SetEX(ctx, "k1", "v1", 60*time.Second); err != nil {
		t.Fatalf("setex failed: %v", err)
	}
	if val, ok, err := rc.Get(ctx, "k1"); err != nil || !ok || val != "v1" {
		t.Fatalf("get mismatch: val=%q ok=%v err=%v", val, ok, err)
	}
	if err := rc.Del(ctx, "k1"); err != nil {
		t.Fatalf("del failed: %v", err)
	}
	if _, ok, err := rc.Get(ctx, "k1"); err != nil || ok {
		t.Fatalf("expected cache miss after del")
	}

	badClient := NewRedisClient(addr, "wrong-pass", time.Second)
	if err := badClient.Ping(ctx); err == nil {
		t.Fatalf("expected auth error with wrong password")
	}

	mem := NewMemoryRepository()
	cached := NewCachedRepository(mem, rc)
	if cached.Mode() != "memory+redis" || !cached.Healthy(ctx) {
		t.Fatalf("unexpected cached repo metadata")
	}
	st, err := cached.LoadState(ctx)
	if err != nil {
		t.Fatalf("cached load state: %v", err)
	}
	st.ActiveRole = domain.RoleManager
	if err := cached.SaveState(ctx, st); err != nil {
		t.Fatalf("cached save state: %v", err)
	}
	st2, err := cached.LoadState(ctx)
	if err != nil || st2.ActiveRole != domain.RoleManager {
		t.Fatalf("expected cached hit with manager role, got %+v err=%v", st2, err)
	}
	_ = cached.AppendClickstream(ctx, ClickstreamRecord{ID: "c1", EventType: "ev", ActorRole: domain.RoleIntern})
	_ = cached.Close()
}

type fakePGRunner struct {
	statePayload string
	lastSQL      []string
	failQuery    bool
}

func (f *fakePGRunner) Query(_ context.Context, sql string) ([][]string, error) {
	f.lastSQL = append(f.lastSQL, sql)
	if f.failQuery {
		return nil, errors.New("pg error")
	}
	if strings.HasPrefix(sql, "SELECT payload::text") {
		if f.statePayload == "" {
			return nil, nil
		}
		return [][]string{{f.statePayload}}, nil
	}
	if sql == "SELECT 1;" {
		return [][]string{{"1"}}, nil
	}
	if strings.HasPrefix(sql, "INSERT INTO aibuddy_state") {
		// Extract JSON payload inside the second dollar-quoted value (<key>, <payload>::jsonb, ...).
		afterComma := strings.Index(sql, ", $ab_")
		if afterComma >= 0 {
			rest := sql[afterComma+2:]
			tagEnd := strings.Index(rest[4:], "$")
			if tagEnd >= 0 {
				tag := rest[:4+tagEnd+1]
				bodyStart := len(tag)
				bodyEnd := strings.Index(rest[bodyStart:], tag)
				if bodyEnd >= 0 {
					f.statePayload = rest[bodyStart : bodyStart+bodyEnd]
				}
			}
		}
	}
	return nil, nil
}

func TestPostgresRepositoryLogicAndDSNParsing(t *testing.T) {
	if _, err := parsePostgresDSN(""); err == nil {
		t.Fatalf("expected error for empty DSN")
	}
	cfgURL, err := parsePostgresDSN("postgres://aibuddy:secret@pg-host:5433/aibuddy_db?sslmode=disable")
	if err != nil || cfgURL.Host != "pg-host" || cfgURL.Port != "5433" || cfgURL.User != "aibuddy" || cfgURL.Database != "aibuddy_db" {
		t.Fatalf("unexpected parsed URL DSN: %+v err=%v", cfgURL, err)
	}
	cfgKV, err := parsePostgresDSN("host=127.0.0.1 port=5432 user=demo password=pw dbname=demo sslmode=disable")
	if err != nil || cfgKV.Host != "127.0.0.1" || cfgKV.Password != "pw" {
		t.Fatalf("unexpected parsed KV DSN: %+v err=%v", cfgKV, err)
	}

	runner := &fakePGRunner{}
	pg := NewPostgresRepositoryWithRunner(runner)
	ctx := context.Background()

	if pg.Mode() != "postgres" || !pg.Healthy(ctx) {
		t.Fatalf("unexpected postgres healthy/mode")
	}
	if err := pg.InitSchema(ctx); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if _, err := pg.LoadState(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound before save, got %v", err)
	}

	st := DefaultAppState()
	st.ActiveRole = domain.RoleMentor
	if err := pg.SaveState(ctx, st); err != nil {
		t.Fatalf("save state: %v", err)
	}
	loaded, err := pg.LoadState(ctx)
	if err != nil || loaded.ActiveRole != domain.RoleMentor {
		t.Fatalf("expected loaded mentor role, got %+v err=%v", loaded, err)
	}
	if err := pg.AppendClickstream(ctx, ClickstreamRecord{ID: "evt-pg-1", EventType: "chat", ActorRole: domain.RoleIntern, LatencyMs: 4.2}); err != nil {
		t.Fatalf("append clickstream: %v", err)
	}

	runner.failQuery = true
	if pg.Healthy(ctx) {
		t.Fatalf("expected unhealthy when query fails")
	}
	_ = pg.Close()
}

func startFakePostgresWireServer(t *testing.T, authMode string, password string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake pg: %v", err)
	}
	done := make(chan struct{})

	writeServerMsg := func(w io.Writer, msgType byte, payload []byte) {
		var hdr [5]byte
		hdr[0] = msgType
		binary.BigEndian.PutUint32(hdr[1:5], uint32(len(payload)+4))
		_, _ = w.Write(hdr[:])
		if len(payload) > 0 {
			_, _ = w.Write(payload)
		}
	}

	readFrontendPacket := func(r *bufio.Reader) (byte, []byte, error) {
		var hdr [5]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return 0, nil, err
		}
		l := int(binary.BigEndian.Uint32(hdr[1:5]))
		body := make([]byte, l-4)
		if len(body) > 0 {
			if _, err := io.ReadFull(r, body); err != nil {
				return 0, nil, err
			}
		}
		return hdr[0], body, nil
	}

	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				r := bufio.NewReader(conn)

				// Read StartupMessage length
				var lenBuf [4]byte
				if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
					return
				}
				startupLen := int(binary.BigEndian.Uint32(lenBuf[:]))
				startupBody := make([]byte, startupLen-4)
				if _, err := io.ReadFull(r, startupBody); err != nil {
					return
				}

				switch authMode {
				case "ok":
					var okPayload [4]byte
					writeServerMsg(conn, 'R', okPayload[:])
					writeServerMsg(conn, 'Z', []byte{'I'})
				case "cleartext":
					var b [4]byte
					binary.BigEndian.PutUint32(b[:], 3)
					writeServerMsg(conn, 'R', b[:])
					_, pwMsg, err := readFrontendPacket(r)
					if err != nil || strings.TrimRight(string(pwMsg), "\x00") != password {
						writeServerMsg(conn, 'E', []byte("Mbad password\x00C28P01\x00\x00"))
						return
					}
					writeServerMsg(conn, 'R', make([]byte, 4))
					writeServerMsg(conn, 'Z', []byte{'I'})
				case "md5":
					salt := []byte{1, 2, 3, 4}
					var p [8]byte
					binary.BigEndian.PutUint32(p[0:4], 5)
					copy(p[4:8], salt)
					writeServerMsg(conn, 'R', p[:])
					_, pwMsg, err := readFrontendPacket(r)
					expected := pgMD5Digest("aibuddy", password, salt)
					if err != nil || strings.TrimRight(string(pwMsg), "\x00") != expected {
						writeServerMsg(conn, 'E', []byte("Mbad md5\x00C28P01\x00\x00"))
						return
					}
					writeServerMsg(conn, 'R', make([]byte, 4))
					writeServerMsg(conn, 'Z', []byte{'I'})
				case "scram":
					var saslList bytes.Buffer
					_ = binary.Write(&saslList, binary.BigEndian, int32(10))
					saslList.WriteString("SCRAM-SHA-256\x00\x00")
					writeServerMsg(conn, 'R', saslList.Bytes())

					_, initPkt, err := readFrontendPacket(r)
					if err != nil {
						return
					}
					nullIdx := bytes.IndexByte(initPkt, 0)
					clientFirst := string(initPkt[nullIdx+5:])
					clientBare := strings.TrimPrefix(clientFirst, "n,,")
					cParts := parseSCRAMFields(clientBare)

					fullNonce := cParts["r"] + "ServerNonce123"
					saltRaw := []byte("randomsalt123456")
					saltB64 := base64.StdEncoding.EncodeToString(saltRaw)
					serverFirst := "r=" + fullNonce + ",s=" + saltB64 + ",i=4096"

					var saslCont bytes.Buffer
					_ = binary.Write(&saslCont, binary.BigEndian, int32(11))
					saslCont.WriteString(serverFirst)
					writeServerMsg(conn, 'R', saslCont.Bytes())

					_, finalPkt, err := readFrontendPacket(r)
					if err != nil {
						return
					}
					finalStr := string(finalPkt)
					fParts := parseSCRAMFields(finalStr)
					clientProofGot, _ := base64.StdEncoding.DecodeString(fParts["p"])

					saltedPw := pbkdf2HMACSHA256([]byte(password), saltRaw, 4096, 32)
					clientKey := hmacSHA256(saltedPw, []byte("Client Key"))
					storedKey := sha256.Sum256(clientKey)
					serverKey := hmacSHA256(saltedPw, []byte("Server Key"))
					authMsg := clientBare + "," + serverFirst + ",c=biws,r=" + fullNonce
					clientSig := hmacSHA256(storedKey[:], []byte(authMsg))
					recoveredKey := make([]byte, len(clientKey))
					for i := range clientKey {
						recoveredKey[i] = clientProofGot[i] ^ clientSig[i]
					}
					recoveredStored := sha256.Sum256(recoveredKey)
					if !bytes.Equal(recoveredStored[:], storedKey[:]) {
						writeServerMsg(conn, 'E', []byte("Mscram auth failed\x00C28P01\x00\x00"))
						return
					}

					serverSig := hmacSHA256(serverKey, []byte(authMsg))
					var saslFinal bytes.Buffer
					_ = binary.Write(&saslFinal, binary.BigEndian, int32(12))
					saslFinal.WriteString("v=" + base64.StdEncoding.EncodeToString(serverSig))
					writeServerMsg(conn, 'R', saslFinal.Bytes())
					writeServerMsg(conn, 'R', make([]byte, 4))
					writeServerMsg(conn, 'Z', []byte{'I'})
				}

				// Handle query loop.
				for {
					msgType, body, err := readFrontendPacket(r)
					if err != nil || msgType == 'X' {
						return
					}
					if msgType == 'Q' {
						q := strings.TrimRight(string(body), "\x00")
						if strings.Contains(q, "FAIL_QUERY") {
							writeServerMsg(conn, 'E', []byte("Mforced syntax error\x00C42601\x00\x00"))
							writeServerMsg(conn, 'Z', []byte{'I'})
							continue
						}
						// Return 1 DataRow with value "ok" or "1".
						var row bytes.Buffer
						_ = binary.Write(&row, binary.BigEndian, int16(1))
						_ = binary.Write(&row, binary.BigEndian, int32(1))
						row.WriteByte('1')
						writeServerMsg(conn, 'D', row.Bytes())
						writeServerMsg(conn, 'C', []byte("SELECT 1\x00"))
						writeServerMsg(conn, 'Z', []byte{'I'})
					}
				}
			}(c)
		}
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func TestPostgresWireAuthModesAndErrors(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"ok", "cleartext", "md5", "scram"} {
		t.Run(mode, func(t *testing.T) {
			addr, stop := startFakePostgresWireServer(t, mode, "pg-secret")
			defer stop()

			dsn := fmt.Sprintf("postgres://aibuddy:pg-secret@%s/aibuddy?sslmode=disable", addr)
			pg, err := NewPostgresRepository(dsn, time.Second)
			if err != nil {
				t.Fatalf("new pg repo: %v", err)
			}
			if !pg.Healthy(ctx) {
				t.Fatalf("expected healthy pg connection for auth mode %s", mode)
			}
			if _, err := pg.runner.Query(ctx, "FAIL_QUERY"); err == nil {
				t.Fatalf("expected error on FAIL_QUERY")
			}
		})
	}
}

func TestNewRepositoryFactoryFallback(t *testing.T) {
	ctx := context.Background()
	// Unavailable postgres & redis must gracefully fall back to memory store.
	repo := NewRepository(ctx, config.Config{
		StorageMode: "postgres",
		PostgresDSN: "postgres://aibuddy:pw@127.0.0.1:1/aibuddy?sslmode=disable",
		RedisAddr:   "127.0.0.1:1",
	})
	if repo.Mode() != "memory" {
		t.Fatalf("expected graceful fallback to memory, got %s", repo.Mode())
	}

	// Working redis + memory primary.
	addr, stop := startFakeRedisServer(t, "")
	defer stop()
	repoWithRedis := NewRepository(ctx, config.Config{
		StorageMode: "memory",
		RedisAddr:   addr,
	})
	if repoWithRedis.Mode() != "memory+redis" {
		t.Fatalf("expected memory+redis mode, got %s", repoWithRedis.Mode())
	}

	// Unused imports guard sanity check on hex/md5 helpers.
	_ = hex.EncodeToString(md5.New().Sum(nil))
}
