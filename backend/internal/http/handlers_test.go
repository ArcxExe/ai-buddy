package httpapi_test

import (
	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/domain"
	httpapi "aibuddy/backend/internal/http"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestContextEndpoint(t *testing.T) {
	s := setupTestServer()
	req := httptest.NewRequest("GET", "/api/v1/context", nil)
	w := httptest.NewRecorder()

	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSwitchRole(t *testing.T) {
	s := setupTestServer()

	// 1. Switch to mentor
	body := bytes.NewBufferString(`{"role":"mentor"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/switch-role", body)
	w := httptest.NewRecorder()

	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// 2. Switch to invalid role
	badBody := bytes.NewBufferString(`{"role":"superadmin"}`)
	badReq := httptest.NewRequest("POST", "/api/v1/auth/switch-role", badBody)
	badW := httptest.NewRecorder()

	s.Routes().ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad role, got %d", badW.Code)
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

	// Not found item
	reqNotFound := httptest.NewRequest("PATCH", "/api/v1/onboarding/checklist/non-existent", bytes.NewBufferString(`{"status":"done"}`))
	wNotFound := httptest.NewRecorder()
	s.Routes().ServeHTTP(wNotFound, reqNotFound)
	if wNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", wNotFound.Code)
	}
}

func TestChatEndpoints(t *testing.T) {
	s := setupTestServer()

	// 1. Send message
	msgBody := bytes.NewBufferString(`{"content":"Как оформить отгул на экзамен?","scenario_hint":"scenario_02_absence_day_off"}`)
	req := httptest.NewRequest("POST", "/api/v1/chat/message", msgBody)
	w := httptest.NewRecorder()

	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send message failed: %d, body: %s", w.Code, w.Body.String())
	}

	// 2. Get messages
	getReq := httptest.NewRequest("GET", "/api/v1/chat/messages", nil)
	getW := httptest.NewRecorder()
	s.Routes().ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get messages failed: %d", getW.Code)
	}

	var msgs []domain.ChatMessage
	if err := json.NewDecoder(getW.Body).Decode(&msgs); err != nil {
		t.Fatalf("failed decoding msgs: %v", err)
	}
	if len(msgs) != 2 { // user + bot
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}

	// 3. Reset
	resetReq := httptest.NewRequest("POST", "/api/v1/chat/reset", nil)
	resetW := httptest.NewRecorder()
	s.Routes().ServeHTTP(resetW, resetReq)
	if resetW.Code != http.StatusOK {
		t.Fatalf("reset failed: %d", resetW.Code)
	}
}

func TestCorporateActionsConfirmAndReview(t *testing.T) {
	s := setupTestServer()

	// Create absence draft
	intern := domain.UserProfile{
		Role:        domain.RoleIntern,
		ManagerName: "Дмитрий Волков",
		MentorName:  "Мария Козлова",
	}
	draft, err := s.Executor.CreateAbsenceRequest(intern, "study_day_off", "2026-11-18", "2026-11-18", "Зачёт", false)
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}

	// Confirm draft
	confirmBody := bytes.NewBufferString(`{"request_id":"` + draft.ID + `"}`)
	confirmReq := httptest.NewRequest("POST", "/api/v1/corporate/actions/confirm", confirmBody)
	confirmW := httptest.NewRecorder()
	s.Routes().ServeHTTP(confirmW, confirmReq)
	if confirmW.Code != http.StatusOK {
		t.Fatalf("confirm action failed: %d", confirmW.Code)
	}

	// Intern tries to review (RBAC Forbidden)
	reviewBody := bytes.NewBufferString(`{"decision":"approved","comment":"Сам себе одобрил"}`)
	reviewInternReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/"+draft.ID+"/review", reviewBody)
	reviewInternReq.Header.Set("X-User-Role", "intern")
	reviewInternW := httptest.NewRecorder()
	s.Routes().ServeHTTP(reviewInternW, reviewInternReq)
	if reviewInternW.Code != http.StatusForbidden {
		t.Errorf("intern review must return 403 Forbidden, got %d", reviewInternW.Code)
	}

	// Mentor reviews (Allowed)
	reviewMentorReq := httptest.NewRequest("POST", "/api/v1/corporate/requests/"+draft.ID+"/review", bytes.NewBufferString(`{"decision":"approved","comment":"Удачи"}`))
	reviewMentorReq.Header.Set("X-User-Role", "mentor")
	reviewMentorW := httptest.NewRecorder()
	s.Routes().ServeHTTP(reviewMentorW, reviewMentorReq)
	if reviewMentorW.Code != http.StatusOK {
		t.Errorf("mentor review must return 200 OK, got %d", reviewMentorW.Code)
	}
}

func TestKnowledgeEndpoint(t *testing.T) {
	s := setupTestServer()

	// 1. List all
	req := httptest.NewRequest("GET", "/api/v1/knowledge", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("knowledge list failed: %d", w.Code)
	}

	// 2. Query search
	queryReq := httptest.NewRequest("GET", "/api/v1/knowledge?q=доступы", nil)
	queryW := httptest.NewRecorder()
	s.Routes().ServeHTTP(queryW, queryReq)
	if queryW.Code != http.StatusOK {
		t.Fatalf("knowledge query failed: %d", queryW.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	s := setupTestServer()
	req := httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for CORS OPTIONS, got %d", w.Code)
	}
}
