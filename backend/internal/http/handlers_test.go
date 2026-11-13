package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/config"
	"aibuddy/backend/internal/domain"
	httpapi "aibuddy/backend/internal/http"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/telemetry"
)

func setupTestServer() *httpapi.Server {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)
	kb := rag.NewKnowledgeBase()
	kb.SetArticles([]domain.KnowledgeArticle{
		{
			ID:          "kb-test-1",
			Title:       "Доступы",
			Category:    "onboarding",
			Sensitivity: "intern",
			Tags:        []string{"доступы"},
		},
		{
			ID:          "kb-test-2",
			Title:       "Зарплаты",
			Category:    "management",
			Sensitivity: "manager_only",
			Tags:        []string{"зарплаты"},
		},
	})
	cfg := ai.Config{
		Provider: "openrouter",
		Model:    "test-model",
	}
	agent := ai.NewAgentOrchestrator(cfg, kb, exec)
	return httpapi.NewServer(store, exec, kb, agent)
}

func TestHealthEndpoint(t *testing.T) {
	s := setupTestServer()
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "ok" || body["storage_mode"] != "memory" {
		t.Errorf("unexpected health payload: %+v", body)
	}
}

func TestContextEndpointAndDeterministicRolesOrder(t *testing.T) {
	s := setupTestServer()
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/v1/context", nil)
		w := httptest.NewRecorder()
		s.Routes().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var ctxResp struct {
			AvailableRoles []domain.UserProfile `json:"available_roles"`
		}
		if err := json.NewDecoder(w.Body).Decode(&ctxResp); err != nil {
			t.Fatalf("decode context: %v", err)
		}
		if len(ctxResp.AvailableRoles) != 3 ||
			ctxResp.AvailableRoles[0].Role != domain.RoleIntern ||
			ctxResp.AvailableRoles[1].Role != domain.RoleMentor ||
			ctxResp.AvailableRoles[2].Role != domain.RoleManager {
			t.Fatalf("expected deterministic intern->mentor->manager order, got %+v", ctxResp.AvailableRoles)
		}
	}
}

func TestSwitchRole(t *testing.T) {
	s := setupTestServer()

	body := bytes.NewBufferString(`{"role":"mentor"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/switch-role", body)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	badBody := bytes.NewBufferString(`{"role":"superadmin"}`)
	badReq := httptest.NewRequest("POST", "/api/v1/auth/switch-role", badBody)
	badW := httptest.NewRecorder()
	s.Routes().ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad role, got %d", badW.Code)
	}

	// Invalid JSON check
	invalidReq := httptest.NewRequest("POST", "/api/v1/auth/switch-role", bytes.NewBufferString("{"))
	invalidW := httptest.NewRecorder()
	s.Routes().ServeHTTP(invalidW, invalidReq)
	if invalidW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid json, got %d", invalidW.Code)
	}
}

func TestUpdateChecklist(t *testing.T) {
	s := setupTestServer()
	body := bytes.NewBufferString(`{"status":"done"}`)
	req := httptest.NewRequest("PATCH", "/api/v1/onboarding/checklist/step-01-access", body)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Invalid status -> 400
	badReq := httptest.NewRequest("PATCH", "/api/v1/onboarding/checklist/step-01-access", bytes.NewBufferString(`{"status":"bogus"}`))
	badW := httptest.NewRecorder()
	s.Routes().ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid status, got %d", badW.Code)
	}

	// Not found item -> 404
	reqNotFound := httptest.NewRequest("PATCH", "/api/v1/onboarding/checklist/non-existent", bytes.NewBufferString(`{"status":"done"}`))
	wNotFound := httptest.NewRecorder()
	s.Routes().ServeHTTP(wNotFound, reqNotFound)
	if wNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", wNotFound.Code)
	}
}

func TestChatEndpointsAndActionCardSyncAfterConfirmReview(t *testing.T) {
	s := setupTestServer()
	router := s.Routes()

	// Empty message -> 400
	emptyW := httptest.NewRecorder()
	router.ServeHTTP(emptyW, httptest.NewRequest("POST", "/api/v1/chat/message", bytes.NewBufferString(`{"content":"   "}`)))
	if emptyW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty chat message, got %d", emptyW.Code)
	}

	// 1. Send absence message (creates draft_confirmation_required ActionCard in message)
	msgBody := bytes.NewBufferString(`{"content":"Как оформить отгул на экзамен?","scenario_hint":"scenario_02_absence_day_off"}`)
	req := httptest.NewRequest("POST", "/api/v1/chat/message", msgBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send message failed: %d, body: %s", w.Code, w.Body.String())
	}

	var sendResp struct {
		AssistantMessage domain.ChatMessage `json:"assistant_message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&sendResp); err != nil {
		t.Fatalf("decode send response: %v", err)
	}
	if sendResp.AssistantMessage.ActionCard == nil {
		t.Fatalf("expected ActionCard in assistant response")
	}
	reqID := sendResp.AssistantMessage.ActionCard.ID

	// 2. Confirm action via REST -> chat GET messages MUST now show pending_approval on card!
	confirmW := httptest.NewRecorder()
	router.ServeHTTP(confirmW, httptest.NewRequest("POST", "/api/v1/corporate/actions/confirm", bytes.NewBufferString(`{"request_id":"`+reqID+`"}`)))
	if confirmW.Code != http.StatusOK {
		t.Fatalf("confirm failed: %d", confirmW.Code)
	}

	// 3. Review as mentor -> chat GET messages MUST now show approved + reviewer comment!
	revReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/"+reqID+"/review", bytes.NewBufferString(`{"decision":"approved","comment":"Согласовано"}`))
	revReq.Header.Set("X-User-Role", "mentor")
	revW := httptest.NewRecorder()
	router.ServeHTTP(revW, revReq)
	if revW.Code != http.StatusOK {
		t.Fatalf("review failed: %d", revW.Code)
	}

	// 4. Get messages & check synced ActionCard
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, httptest.NewRequest("GET", "/api/v1/chat/messages", nil))
	if getW.Code != http.StatusOK {
		t.Fatalf("get messages failed: %d", getW.Code)
	}
	var msgs []domain.ChatMessage
	if err := json.NewDecoder(getW.Body).Decode(&msgs); err != nil {
		t.Fatalf("failed decoding msgs: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[1].ActionCard == nil || msgs[1].ActionCard.Status != domain.StatusApproved || msgs[1].ActionCard.ReviewerComment != "Согласовано" {
		t.Fatalf("expected synced ActionCard after confirm+review, got %+v", msgs[1].ActionCard)
	}

	// 5. Reset
	resetW := httptest.NewRecorder()
	router.ServeHTTP(resetW, httptest.NewRequest("POST", "/api/v1/chat/reset", nil))
	if resetW.Code != http.StatusOK {
		t.Fatalf("reset failed: %d", resetW.Code)
	}
}

func TestCorporateActionsConfirmAndReviewValidation(t *testing.T) {
	s := setupTestServer()
	router := s.Routes()

	// Missing request_id -> 400
	missW := httptest.NewRecorder()
	router.ServeHTTP(missW, httptest.NewRequest("POST", "/api/v1/corporate/actions/confirm", bytes.NewBufferString(`{"request_id":""}`)))
	if missW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing request_id, got %d", missW.Code)
	}

	// Non-existent request_id -> 404
	nfW := httptest.NewRecorder()
	router.ServeHTTP(nfW, httptest.NewRequest("POST", "/api/v1/corporate/actions/confirm", bytes.NewBufferString(`{"request_id":"missing"}`)))
	if nfW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown request_id, got %d", nfW.Code)
	}

	intern := domain.UserProfile{Role: domain.RoleIntern, ManagerName: "Дмитрий Волков", MentorName: "Мария Козлова"}
	draft, err := s.Executor.CreateAbsenceRequest(intern, "study_day_off", "2026-11-18", "2026-11-18", "Зачёт", false)
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}

	// Intern tries to review -> 403 Forbidden
	reviewInternReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/"+draft.ID+"/review", bytes.NewBufferString(`{"decision":"approved","comment":"ok"}`))
	reviewInternReq.Header.Set("X-User-Role", "intern")
	reviewInternW := httptest.NewRecorder()
	router.ServeHTTP(reviewInternW, reviewInternReq)
	if reviewInternW.Code != http.StatusForbidden {
		t.Errorf("intern review must return 403 Forbidden, got %d", reviewInternW.Code)
	}

	// Mentor sends invalid decision -> 400 Bad Request
	badDecReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/"+draft.ID+"/review", bytes.NewBufferString(`{"decision":"unknown"}`))
	badDecReq.Header.Set("X-User-Role", "mentor")
	badDecW := httptest.NewRecorder()
	router.ServeHTTP(badDecW, badDecReq)
	if badDecW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on invalid decision, got %d", badDecW.Code)
	}

	// Mentor reviews unknown request -> 404 Not Found
	unknownRevReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/missing-id/review", bytes.NewBufferString(`{"decision":"approved"}`))
	unknownRevReq.Header.Set("X-User-Role", "mentor")
	unknownRevW := httptest.NewRecorder()
	router.ServeHTTP(unknownRevW, unknownRevReq)
	if unknownRevW.Code != http.StatusNotFound {
		t.Errorf("expected 404 on unknown request review, got %d", unknownRevW.Code)
	}

	// Mentor approves valid request -> 200 OK
	reviewMentorReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/"+draft.ID+"/review", bytes.NewBufferString(`{"decision":"approved","comment":"Удачи"}`))
	reviewMentorReq.Header.Set("X-User-Role", "mentor")
	reviewMentorW := httptest.NewRecorder()
	router.ServeHTTP(reviewMentorW, reviewMentorReq)
	if reviewMentorW.Code != http.StatusOK {
		t.Errorf("mentor review must return 200 OK, got %d", reviewMentorW.Code)
	}
}

func TestKnowledgeAndProactiveHintAndCORSConfig(t *testing.T) {
	cfg := config.LoadWithLookup(func(k string) (string, bool) {
		switch k {
		case "CORS_ALLOWED_ORIGINS":
			return "http://localhost:5173,https://avito.local", true
		case "FF_ENABLE_PROACTIVE_CRON_HINTS":
			return "true", true
		default:
			return "", false
		}
	})

	store := mcp.NewStore()
	kb := rag.NewKnowledgeBase()
	kb.SetArticles([]domain.KnowledgeArticle{
		{ID: "kb-1", Title: "Доступы", Sensitivity: "intern", Tags: []string{"доступы"}},
		{ID: "kb-2", Title: "Зарплаты", Sensitivity: "manager_only", Tags: []string{"зарплаты"}},
	})
	tr := telemetry.NewTracker(true)
	exec := mcp.NewExecutorWithDeps(store, kb, tr)
	agent := ai.NewAgentOrchestratorWithDeps(ai.Config{Provider: "openrouter"}, kb, exec, exec, nil, tr, true)
	s := httpapi.NewServerWithConfig(cfg, store, exec, kb, agent, tr)
	router := s.Routes()

	// CORS allowed vs disallowed origin
	preReq := httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
	preReq.Header.Set("Origin", "http://localhost:5173")
	preW := httptest.NewRecorder()
	router.ServeHTTP(preW, preReq)
	if preW.Code != http.StatusOK || preW.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("expected matched CORS origin, got header=%q code=%d", preW.Header().Get("Access-Control-Allow-Origin"), preW.Code)
	}

	disallowedReq := httptest.NewRequest("GET", "/api/v1/health", nil)
	disallowedReq.Header.Set("Origin", "https://evil.example")
	disallowedW := httptest.NewRecorder()
	router.ServeHTTP(disallowedW, disallowedReq)
	if disallowedW.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected empty Access-Control-Allow-Origin for unlisted origin")
	}

	// Knowledge search & RBAC filtering
	kwW := httptest.NewRecorder()
	router.ServeHTTP(kwW, httptest.NewRequest("GET", "/api/v1/knowledge?q=доступы", nil))
	if kwW.Code != http.StatusOK {
		t.Fatalf("knowledge query failed: %d", kwW.Code)
	}
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, httptest.NewRequest("GET", "/api/v1/knowledge", nil))
	if listW.Code != http.StatusOK {
		t.Fatalf("knowledge list failed: %d", listW.Code)
	}

	// Proactive hint idempotent once per session
	hint1 := s.TriggerProactiveHint()
	hint2 := s.TriggerProactiveHint()
	if hint1 == nil || hint2 != nil {
		t.Fatalf("expected exactly one proactive hint emitted, got hint1=%v hint2=%v", hint1, hint2)
	}
}

func TestConcurrentAPIRequestsUnderRace(t *testing.T) {
	s := setupTestServer()
	router := s.Routes()

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/context", nil))
				if w.Code != http.StatusOK {
					t.Errorf("concurrent GET /context returned %d", w.Code)
				}
				w2 := httptest.NewRecorder()
				router.ServeHTTP(w2, httptest.NewRequest("GET", "/api/v1/knowledge", nil))
				if w2.Code != http.StatusOK {
					t.Errorf("concurrent GET /knowledge returned %d", w2.Code)
				}
			}
		}()
	}
	wg.Wait()
}
