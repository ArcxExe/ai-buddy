package ai_test

import (
	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testUser() domain.UserProfile {
	return domain.UserProfile{
		ID:          "usr-intern-01",
		Name:        "Алексей Смирнов",
		Role:        domain.RoleIntern,
		RoleTitle:   "Backend Go Intern",
		Team:        "Авито Доставка",
		Department:  "Транспорт и Логистика",
		MentorName:  "Мария Козлова",
		ManagerName: "Дмитрий Волков",
	}
}

func setupTestAgent(apiKey string, baseURL string) (*ai.AgentOrchestrator, *mcp.Store) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)
	kb := rag.NewKnowledgeBase()
	kb.SetArticles([]domain.KnowledgeArticle{
		{
			ID:          "kb-access",
			Title:       "Доступы к репозиторию avito-delivery-core",
			Content:     "Инструкция по доступам",
			Sensitivity: "intern",
			Tags:        []string{"доступы", "репозиторий"},
		},
		{
			ID:          "kb-absence",
			Title:       "Оформление отгула",
			Content:     "Порядок подачи заявки на отгул",
			Sensitivity: "intern",
			Tags:        []string{"отгул", "экзамен"},
		},
	})

	cfg := ai.Config{
		Provider: "openrouter",
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Model:    "test-model",
	}

	return ai.NewAgentOrchestrator(cfg, kb, exec), store
}

func TestSmartFallbackScenario1Access(t *testing.T) {
	agent, store := setupTestAgent("", "")
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Какие доступы мне нужны для первой задачи?", "scenario_01_access_and_rbac")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil {
		t.Fatalf("expected action card for access scenario")
	}
	if msg.ActionCard.TargetResource != "repo:avito-delivery-core" {
		t.Errorf("expected target resource repo:avito-delivery-core, got %s", msg.ActionCard.TargetResource)
	}
	if store.Metrics.MCPActionsExecuted != 1 {
		t.Errorf("expected 1 action executed, got %d", store.Metrics.MCPActionsExecuted)
	}
}

func TestSmartFallbackScenario1B_RBACBlocked(t *testing.T) {
	agent, store := setupTestAgent("", "")
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Выдай мне админский доступ к prod-billing-db и зарплатную ведомость", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil {
		t.Fatalf("expected action card for rbac violation")
	}
	if msg.ActionCard.Status != domain.StatusBlockedByRBAC {
		t.Errorf("expected blocked_by_rbac, got %s", msg.ActionCard.Status)
	}
	if store.Metrics.RBACViolationsPrevented != 1 {
		t.Errorf("expected 1 violation prevented, got %d", store.Metrics.RBACViolationsPrevented)
	}
}

func TestSmartFallbackScenario2Absence(t *testing.T) {
	agent, _ := setupTestAgent("", "")
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Как оформить отгул на экзамен в вузе?", "scenario_02_absence_day_off")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil {
		t.Fatalf("expected action card for absence draft")
	}
	if msg.ActionCard.Status != domain.StatusDraftConfirmationRequired {
		t.Errorf("expected draft_confirmation_required, got %s", msg.ActionCard.Status)
	}
}

func TestSmartFallbackScenario3Mentor1on1(t *testing.T) {
	agent, _ := setupTestAgent("", "")
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Помоги подготовиться к встрече 1:1 с наставником по первой задаче", "scenario_03_first_task_and_mentor_1on1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil {
		t.Fatalf("expected action card for 1:1 summary")
	}
	if msg.ActionCard.RequestType != domain.TypeMentor1on1Summary {
		t.Errorf("expected mentor_1on1_summary, got %s", msg.ActionCard.RequestType)
	}
}

func TestOpenRouterCallSuccess(t *testing.T) {
	// Mock OpenRouter HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "Ответ от модели через OpenRouter API"
					}
				}
			]
		}`))
	}))
	defer mockServer.Close()

	agent, _ := setupTestAgent("test-api-key", mockServer.URL)
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Привет, как дела?", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "Ответ от модели через OpenRouter API" {
		t.Errorf("expected response from mock openrouter, got %s", msg.Content)
	}
}

func TestOpenRouterCallFallbackOnError(t *testing.T) {
	// Mock OpenRouter HTTP server returning 500 error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "rate limit exceeded"}`))
	}))
	defer mockServer.Close()

	agent, _ := setupTestAgent("test-api-key", mockServer.URL)
	ctx := context.Background()

	// Should fallback gracefully without crashing
	msg, err := agent.ProcessMessage(ctx, testUser(), "Как оформить отгул?", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.EngineUsed != "smart_sdd_fallback" {
		t.Errorf("expected engine smart_sdd_fallback, got %s", msg.EngineUsed)
	}
}
