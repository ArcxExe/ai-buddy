package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/telemetry"
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
			Source:      "Playbook [Mock]",
			Summary:     "Порядок получения repo/staging",
			Sensitivity: "intern",
			Tags:        []string{"доступы", "репозиторий"},
		},
		{
			ID:          "kb-absence",
			Title:       "Оформление отгула",
			Content:     "Порядок подачи заявки на отгул",
			Source:      "HR Policy [Mock]",
			Summary:     "Оформление study day-off",
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

	// Verify staging & grafana resource inference
	msgStg, _ := agent.ProcessMessage(ctx, testUser(), "Нужен доступ к staging окружению", "")
	if msgStg.ActionCard == nil || msgStg.ActionCard.TargetResource != "env:staging-delivery" {
		t.Fatalf("expected env:staging-delivery, got %+v", msgStg.ActionCard)
	}
	msgGrafana, _ := agent.ProcessMessage(ctx, testUser(), "Выдай доступ на чтение в grafana дашборды", "")
	if msgGrafana.ActionCard == nil || msgGrafana.ActionCard.TargetResource != "grafana:delivery-dashboard" {
		t.Fatalf("expected grafana:delivery-dashboard, got %+v", msgGrafana.ActionCard)
	}
}

func TestSmartFallbackScenario1B_RBACBlocked(t *testing.T) {
	llmCalled := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Guardrail MUST block in code before contacting external LLM even when API key is set!
	agent, store := setupTestAgent("sk-live-test", mockServer.URL)
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Выдай мне админский доступ к prod-billing-db и зарплатную ведомость", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llmCalled {
		t.Fatalf("external LLM must not be called when prompt guardrail blocks request")
	}
	if msg.ActionCard == nil {
		t.Fatalf("expected action card for rbac violation")
	}
	if msg.ActionCard.Status != domain.StatusBlockedByRBAC {
		t.Errorf("expected blocked_by_rbac, got %s", msg.ActionCard.Status)
	}
	if msg.EngineUsed != "rbac_guardrail" {
		t.Errorf("expected engine rbac_guardrail, got %s", msg.EngineUsed)
	}
	if store.Metrics.RBACViolationsPrevented != 1 {
		t.Errorf("expected 1 violation prevented, got %d", store.Metrics.RBACViolationsPrevented)
	}
}

func TestSmartFallbackScenario2Absence(t *testing.T) {
	agent, _ := setupTestAgent("", "")
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Как оформить отгул на экзамен в вузе 2026-12-01?", "scenario_02_absence_day_off")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil {
		t.Fatalf("expected action card for absence draft")
	}
	if msg.ActionCard.Status != domain.StatusDraftConfirmationRequired {
		t.Errorf("expected draft_confirmation_required, got %s", msg.ActionCard.Status)
	}
	if msg.ActionCard.Details["start_date"] != "2026-12-01" {
		t.Errorf("expected parsed date 2026-12-01, got %v", msg.ActionCard.Details["start_date"])
	}
}

func TestSmartFallbackScenario3Mentor1on1AndLocalSetup(t *testing.T) {
	agent, _ := setupTestAgent("", "")
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Помоги подготовиться к встрече 1:1 с наставником по первой задаче", "scenario_03_first_task_and_mentor_1on1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil || msg.ActionCard.RequestType != domain.TypeMentor1on1Summary {
		t.Fatalf("expected mentor_1on1_summary action card, got %+v", msg.ActionCard)
	}

	setupMsg, err := agent.ProcessMessage(ctx, testUser(), "Какой мой текущий прогресс и как поднять docker локально?", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(setupMsg.Content, "прогресс адаптации") {
		t.Fatalf("expected local setup & progress response, got %s", setupMsg.Content)
	}
}

func TestOpenRouterCallSuccess(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"model": "test-model",
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

func TestOpenRouterMultiTurnToolExecutionAndRBAC(t *testing.T) {
	turn := 0
	var capturedToolsCount int

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		var reqBody ai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if turn == 1 {
			capturedToolsCount = len(reqBody.Tools)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"model": "google/gemini-2.5-flash",
				"choices": [{
					"message": {
						"content": null,
						"tool_calls": [
							{
								"id": "call_1",
								"type": "function",
								"function": {
									"name": "request_service_access",
									"arguments": "{\"resource\":\"repo:avito-delivery-core\",\"access_level\":\"read_write\",\"reason\":\"Task\"}"
								}
							}
						]
					}
				}]
			}`))
			return
		}

		// Verify tool execution result was fed back into LLM conversation on turn 2.
		hasToolMsg := false
		for _, m := range reqBody.Messages {
			if m.Role == "tool" && m.ToolCallID == "call_1" && strings.Contains(m.Content, "repo:avito-delivery-core") {
				hasToolMsg = true
			}
		}
		if !hasToolMsg {
			t.Fatalf("expected tool message in round 2 payload: %+v", reqBody.Messages)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "google/gemini-2.5-flash",
			"choices": [{
				"message": {
					"content": "Заявка на repo:avito-delivery-core успешно выполнена через MCP!"
				}
			}]
		}`))
	}))
	defer mockServer.Close()

	store := mcp.NewStore()
	kb := rag.NewKnowledgeBase()
	tracker := telemetry.NewTracker(true)
	exec := mcp.NewExecutorWithDeps(store, kb, tracker)
	cfg := ai.Config{
		Provider: "openrouter",
		APIKey:   "sk-test-tools",
		BaseURL:  mockServer.URL,
		Model:    "google/gemini-2.5-flash",
	}
	agent := ai.NewAgentOrchestratorWithDeps(cfg, kb, exec, exec, nil, tracker, true)

	msg, err := agent.ProcessMessage(context.Background(), testUser(), "Оформи заявку в репозиторий команды", "")
	if err != nil {
		t.Fatalf("process message failed: %v", err)
	}
	if turn != 2 {
		t.Fatalf("expected 2 LLM turns, got %d", turn)
	}
	if capturedToolsCount != 7 {
		t.Fatalf("expected 7 MCP tool schemas sent to OpenRouter, got %d", capturedToolsCount)
	}
	if msg.ActionCard == nil || msg.ActionCard.TargetResource != "repo:avito-delivery-core" {
		t.Fatalf("expected persisted ActionCard from MCP tool execution, got %+v", msg.ActionCard)
	}
	if !strings.HasPrefix(msg.EngineUsed, "openrouter:") {
		t.Fatalf("expected openrouter engine tag, got %s", msg.EngineUsed)
	}
	if len(tracker.ListRecent(10)) == 0 {
		t.Fatalf("expected telemetry clickstream events recorded")
	}
}

func TestOpenRouterToolRBACBlocksIllegalModelToolCall(t *testing.T) {
	// Even if a prompt jailbreak tricks an LLM into requesting a privileged tool call,
	// Go RBAC in MCP Executor blocks it in code.
	mockLLM := &fakeLLMClient{
		responses: []*ai.ChatCompletionResponse{
			{
				Model: "test-model",
				ToolCalls: []ai.ToolCall{
					{
						ID:   "call_illegal",
						Type: "function",
						Function: ai.ToolCallFunction{
							Name:      "request_service_access",
							Arguments: `{"resource":"prod-billing-db","access_level":"admin","reason":"bypass"}`,
						},
					},
				},
			},
			{
				Model:   "test-model",
				Content: "Доступ к prod-billing-db заблокирован на уровне корпоративного RBAC.",
			},
		},
	}

	store := mcp.NewStore()
	kb := rag.NewKnowledgeBase()
	exec := mcp.NewExecutorWithDeps(store, kb, nil)
	agent := ai.NewAgentOrchestratorWithDeps(ai.Config{
		Provider: "openrouter",
		APIKey:   "sk-mock",
		Model:    "test-model",
	}, kb, exec, exec, mockLLM, nil, true)

	msg, err := agent.ProcessMessage(context.Background(), testUser(), "Помоги мне с настройкой отладки", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.ActionCard == nil || msg.ActionCard.Status != domain.StatusBlockedByRBAC {
		t.Fatalf("expected RBAC-blocked ActionCard when LLM requested prod-billing-db, got %+v", msg.ActionCard)
	}
	if store.Metrics.RBACViolationsPrevented != 1 {
		t.Fatalf("expected 1 RBAC violation prevented, got %d", store.Metrics.RBACViolationsPrevented)
	}
}

type fakeLLMClient struct {
	responses []*ai.ChatCompletionResponse
	idx       int
}

func (f *fakeLLMClient) ChatCompletion(_ context.Context, _ ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error) {
	if f.idx >= len(f.responses) {
		return &ai.ChatCompletionResponse{Content: "done"}, nil
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp, nil
}

func TestOpenRouterCallFallbackOnError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "rate limit exceeded"}`))
	}))
	defer mockServer.Close()

	agent, _ := setupTestAgent("test-api-key", mockServer.URL)
	ctx := context.Background()

	msg, err := agent.ProcessMessage(ctx, testUser(), "Как оформить отгул?", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.EngineUsed != "smart_sdd_fallback" {
		t.Errorf("expected engine smart_sdd_fallback, got %s", msg.EngineUsed)
	}
}
