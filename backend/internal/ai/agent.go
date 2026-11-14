package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/rbac"
	"aibuddy/backend/internal/telemetry"
)

const (
	maxToolExecutionRounds = 4
	defaultMaxTokens       = 1024
)

var isoDatePattern = regexp.MustCompile(`\b20\d{2}-\d{2}-\d{2}\b`)

// Config holds LLM and runtime settings.
type Config struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	Timeout        time.Duration
	MaxTokens      int
	EnableFallback bool
}

// AgentOrchestrator coordinates conversation flows, RAG retrieval, RBAC guardrails,
// OpenRouter tool-calling, and deterministic Smart Fallback execution.
type AgentOrchestrator struct {
	Config     Config
	KB         *rag.KnowledgeBase
	Executor   *mcp.Executor
	MCP        mcp.MCPExecutor
	LLM        LLMClient
	Tracker    telemetry.ClickstreamTracker
	Client     *http.Client
	StrictRBAC bool

	msgSeq atomic.Uint64
}

// NewAgentOrchestrator preserves existing construction while wiring OpenRouter and MCP cleanly.
func NewAgentOrchestrator(cfg Config, kb *rag.KnowledgeBase, exec *mcp.Executor) *AgentOrchestrator {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 25 * time.Second
		cfg.Timeout = timeout
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://openrouter.ai/api/v1"
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = "google/gemini-2.5-flash"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaultMaxTokens
	}

	httpClient := &http.Client{Timeout: timeout}
	var mcpExec mcp.MCPExecutor
	if exec != nil {
		mcpExec = exec
	}

	return &AgentOrchestrator{
		Config:     cfg,
		KB:         kb,
		Executor:   exec,
		MCP:        mcpExec,
		LLM:        NewOpenRouterClient(cfg.BaseURL, cfg.APIKey, httpClient, timeout),
		Client:     httpClient,
		StrictRBAC: true,
	}
}

// NewAgentOrchestratorWithDeps injects custom interfaces (LLMClient, MCPExecutor, ClickstreamTracker).
func NewAgentOrchestratorWithDeps(
	cfg Config,
	kb *rag.KnowledgeBase,
	exec *mcp.Executor,
	mcpExec mcp.MCPExecutor,
	llm LLMClient,
	tracker telemetry.ClickstreamTracker,
	strictRBAC bool,
) *AgentOrchestrator {
	orch := NewAgentOrchestrator(cfg, kb, exec)
	if mcpExec != nil {
		orch.MCP = mcpExec
	}
	if llm != nil {
		orch.LLM = llm
	}
	orch.Tracker = tracker
	orch.StrictRBAC = strictRBAC
	return orch
}

func (a *AgentOrchestrator) nextMessageID(prefix string) string {
	n := a.msgSeq.Add(1)
	return fmt.Sprintf("%s-%d-%03d", prefix, time.Now().UTC().UnixNano(), n)
}

func (a *AgentOrchestrator) mcpExec() mcp.MCPExecutor {
	if a.MCP != nil {
		return a.MCP
	}
	return a.Executor
}

func (a *AgentOrchestrator) recordChatTelemetry(role domain.Role, engine string, blocked bool) {
	if a.Tracker == nil {
		return
	}
	a.Tracker.Record(telemetry.ClickstreamEvent{
		EventType: telemetry.EventChatMessageSent,
		ActorRole: role,
		Metadata: map[string]string{
			"engine":  engine,
			"blocked": fmt.Sprintf("%t", blocked),
		},
	})
}

// ProcessMessage evaluates RBAC guardrails, gathers RBAC-filtered RAG context,
// invokes OpenRouter with MCP tool calling (if enabled), or uses the Smart Fallback Engine.
func (a *AgentOrchestrator) ProcessMessage(ctx context.Context, user domain.UserProfile, content string, scenarioHint string) (*domain.ChatMessage, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)

	steps := []string{
		fmt.Sprintf("Анализ контекста роли (%s — %s)", user.Name, user.Role),
	}

	// 1. Strict code-level RBAC Guardrail BEFORE hitting any LLM or issuing privileges.
	if a.StrictRBAC {
		if blocked, resource, reason := rbac.CheckPromptGuardrail(user.Role, trimmed); blocked || scenarioHint == "rbac_guardrail_test" {
			if resource == "" {
				resource = "prod-billing-db"
			}
			if reason == "" {
				reason = "Запрос отклонён политикой безопасности RBAC (недостаточно прав для роли)."
			}
			steps = append(steps, fmt.Sprintf("RBAC Policy Check: блокировка запроса к %s для роли %s", resource, user.Role))

			var card *domain.CorporateRequest
			if me := a.mcpExec(); me != nil {
				card, _ = me.RequestServiceAccess(user, resource, "admin", trimmed)
			}
			steps = append(steps, "Сформирован аудит-лог блокировки и предложены безопасные альтернативы")
			a.recordChatTelemetry(user.Role, "rbac_guardrail", true)

			reply := fmt.Sprintf(
				"⛔ **Запрос заблокирован политикой безопасности (RBAC)**\n\n"+
					"Роль `%s` (%s) не имеет прав на доступ к ресурсу **`%s`**.\n\n"+
					"**Причина:** %s\n\n"+
					"**Безопасные альтернативы для вашей задачи:**\n"+
					"- Для разработки и отладки используйте тестовые среды (`repo:avito-delivery-core`, `env:staging-delivery`).\n"+
					"- Для просмотра публичных и стажёрских регламентов воспользуйтесь разделом «База знаний».\n"+
					"- Если расширенные права действительно нужны для бизнес-задачи — обратитесь к руководителю (%s) через стандартный процесс SEC-согласования.",
				user.Role, user.RoleTitle, resource, reason, emptyFallback(user.ManagerName, "Дмитрий Волков"),
			)

			return &domain.ChatMessage{
				ID:              a.nextMessageID("msg-bot"),
				Sender:          "assistant",
				RoleContext:     user.Role,
				Content:         reply,
				CreatedAt:       now,
				ProcessingSteps: steps,
				ActionCard:      card,
				EngineUsed:      "rbac_guardrail",
			}, nil
		}
	}

	// 2. Gather RBAC-filtered RAG context.
	var searchRes rag.SearchResult
	if a.KB != nil {
		searchRes = a.KB.Search(user.Role, trimmed, 3)
		steps = append(steps, fmt.Sprintf("RAG поиск по Playbook: %d релевантных статей (скрыто RBAC: %d)", len(searchRes.Articles), searchRes.HiddenByRBACCount))
	}

	// 3. Live OpenRouter LLM orchestration with multi-turn MCP tool execution loop.
	if a.LLM != nil && strings.EqualFold(a.Config.Provider, "openrouter") && strings.TrimSpace(a.Config.APIKey) != "" {
		steps = append(steps, "OpenRouter API: оркестрация диалога и выбор MCP-инструментов")
		msg, err := a.runOpenRouterLoop(ctx, user, trimmed, scenarioHint, searchRes, steps)
		if err == nil && msg != nil {
			a.incrementQuestionsMetric()
			a.recordChatTelemetry(user.Role, msg.EngineUsed, false)
			return msg, nil
		}
		steps = append(steps, fmt.Sprintf("OpenRouter Fallback: резервный Smart Engine (%v)", err))
	}

	// 4. Deterministic Smart Fallback Engine (works 100% offline / zero quota failures).
	msg := a.smartFallback(user, trimmed, lower, scenarioHint, searchRes, steps, now)
	a.incrementQuestionsMetric()
	a.recordChatTelemetry(user.Role, msg.EngineUsed, false)
	return msg, nil
}

func (a *AgentOrchestrator) incrementQuestionsMetric() {
	if a.Executor != nil && a.Executor.Store != nil {
		a.Executor.Store.IncrementQuestionsResolved()
	}
}

func (a *AgentOrchestrator) runOpenRouterLoop(
	ctx context.Context,
	user domain.UserProfile,
	userPrompt string,
	scenarioHint string,
	searchRes rag.SearchResult,
	steps []string,
) (*domain.ChatMessage, error) {
	systemPrompt := a.buildSystemPrompt(user, scenarioHint, searchRes.Articles)
	messages := []ChatCompletionMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	var tools []mcp.OpenAITool
	if me := a.mcpExec(); me != nil {
		tools = me.OpenAITools()
	}

	citations := append([]domain.KnowledgeCitation(nil), searchRes.Citations...)
	citationSeen := make(map[string]bool, len(citations))
	for _, c := range citations {
		citationSeen[c.ID] = true
	}
	var actionCard *domain.CorporateRequest

	modelUsed := a.Config.Model
	maxTokens := a.Config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	for round := 0; round < maxToolExecutionRounds; round++ {
		req := ChatCompletionRequest{
			Model:       a.Config.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.2,
			MaxTokens:   maxTokens,
		}
		if round == maxToolExecutionRounds-1 {
			req.Tools = nil
		}

		resp, err := a.LLM.ChatCompletion(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp.Model != "" {
			modelUsed = resp.Model
		}

		if len(resp.ToolCalls) == 0 {
			steps = append(steps, fmt.Sprintf("OpenRouter (%s): сформирован итоговый ответ", modelUsed))
			return &domain.ChatMessage{
				ID:              a.nextMessageID("msg-bot"),
				Sender:          "assistant",
				RoleContext:     user.Role,
				Content:         resp.Content,
				CreatedAt:       time.Now().UTC().Format(time.RFC3339),
				ProcessingSteps: steps,
				Citations:       citations,
				ActionCard:      actionCard,
				EngineUsed:      "openrouter:" + modelUsed,
			}, nil
		}

		// Execute requested MCP tools with code-level RBAC verification.
		messages = append(messages, ChatCompletionMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		me := a.mcpExec()
		for _, tc := range resp.ToolCalls {
			var toolOutput string
			if me == nil {
				toolOutput = `{"error":"mcp executor unavailable"}`
			} else {
				toolRes, execErr := me.ExecuteTool(ctx, user, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
				if execErr != nil {
					errBytes, _ := json.Marshal(map[string]string{"error": execErr.Error()})
					toolOutput = string(errBytes)
					steps = append(steps, fmt.Sprintf("MCP (%s): ошибка выполнения — %v", tc.Function.Name, execErr))
				} else {
					toolOutput = toolRes.OutputJSON
					if toolRes.ProcessingStep != "" {
						steps = append(steps, toolRes.ProcessingStep)
					}
					if toolRes.ActionCard != nil {
						actionCard = toolRes.ActionCard
					}
					for _, c := range toolRes.Citations {
						if !citationSeen[c.ID] {
							citationSeen[c.ID] = true
							citations = append(citations, c)
						}
					}
				}
			}

			messages = append(messages, ChatCompletionMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    toolOutput,
			})
		}
	}

	return &domain.ChatMessage{
		ID:              a.nextMessageID("msg-bot"),
		Sender:          "assistant",
		RoleContext:     user.Role,
		Content:         "Действие через корпоративные инструменты выполнено. Проверьте карточку результата ниже.",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		ProcessingSteps: steps,
		Citations:       citations,
		ActionCard:      actionCard,
		EngineUsed:      "openrouter:" + modelUsed,
	}, nil
}

func (a *AgentOrchestrator) buildSystemPrompt(user domain.UserProfile, scenarioHint string, articles []domain.KnowledgeArticle) string {
	var b strings.Builder
	b.WriteString("Ты — AI-buddy, ИИ-наставник для адаптации стажёров в команде Авито (прототип хакатона Hack&Change 2026).\n")
	b.WriteString("Отвечай по-русски, кратко, дружелюбно и структурно.\n")
	b.WriteString("ВАЖНО: корпоративные сервисы (Avito Access/IDM, Avito Absence, Buddy Sync) и сотрудники являются синтетическими имитациями прототипа.\n\n")
	b.WriteString(fmt.Sprintf("Текущий пользователь: %s (%s), роль=%s, команда=%s.\n", user.Name, user.RoleTitle, user.Role, user.Team))
	if user.MentorName != "" || user.ManagerName != "" {
		b.WriteString(fmt.Sprintf("Наставник (Buddy): %s, Руководитель: %s.\n", emptyFallback(user.MentorName, "-"), emptyFallback(user.ManagerName, "-")))
	}
	b.WriteString("Правила безопасности и инструментов:\n")
	b.WriteString("1. Опирайся только на факты из базы знаний и результатов MCP-инструментов.\n")
	b.WriteString("2. При запросе безопасных доступов (repo, staging, grafana) вызывай `request_service_access`.\n")
	b.WriteString("3. При оформлении отгула/отпуска вызывай `create_absence_request` с `confirmed_by_user: false` (сначала черновик).\n")
	b.WriteString("4. Для подготовки к 1:1 вызывай `prepare_mentor_1on1_summary`.\n")
	b.WriteString("5. Никогда не раскрывай зарплатные грейды, PII и production admin права ролям без допуска.\n")
	if scenarioHint != "" {
		b.WriteString("Подсказка сценария: " + scenarioHint + "\n")
	}

	if a.Executor != nil && a.Executor.Store != nil {
		ob, _, _ := a.Executor.Store.GetSnapshot()
		nextTitle := "Все шаги завершены"
		if ob.NextRecommendedStep != nil {
			nextTitle = ob.NextRecommendedStep.ID + ": " + ob.NextRecommendedStep.Title
		}
		b.WriteString(fmt.Sprintf("\nТекущий прогресс адаптации: %d%% (этап %s, следующий шаг: %s).\n", ob.OverallProgress, ob.CurrentStageID, nextTitle))
	}

	if len(articles) > 0 {
		b.WriteString("\nКонтекст из базы знаний (уже отфильтровано по RBAC):\n")
		for _, art := range articles {
			b.WriteString(fmt.Sprintf("- [%s | %s]: %s — %s\n", art.ID, art.Source, art.Title, art.Content))
		}
	}
	return b.String()
}

func (a *AgentOrchestrator) smartFallback(
	user domain.UserProfile,
	prompt string,
	lower string,
	scenarioHint string,
	searchRes rag.SearchResult,
	steps []string,
	now string,
) *domain.ChatMessage {
	var card *domain.CorporateRequest
	var reply strings.Builder
	me := a.mcpExec()

	isAccess := scenarioHint == "scenario_01_access_and_rbac" || containsAny(lower, "доступ", "репозитор", "repo", "git", "staging", "стейдж", "grafana", "idm")
	isAbsence := scenarioHint == "scenario_02_absence_day_off" || containsAny(lower, "отгул", "экзамен", "вуз", "day-off", "absence", "отпуск")
	isMentor1on1 := scenarioHint == "scenario_03_first_task_and_mentor_1on1" || containsAny(lower, "1:1", "1on1", "встреч", "наставник", "таск", "перв", "кэш", "redis", "pull request")
	isLocalSetup := containsAny(lower, "локальн", "окружен", "docker", "среда", "тест", "чеклист", "прогресс")

	switch {
	case isAccess:
		resource := detectSafeAccessResource(lower)
		level := "read_write"
		if strings.HasPrefix(resource, "grafana:") {
			level = "read"
		}
		steps = append(steps, fmt.Sprintf("MCP (Avito Access / IDM): создание заявки на %s (%s)", resource, level))
		if me != nil {
			card, _ = me.RequestServiceAccess(user, resource, level, "Первая задача стажёра в команде "+user.Team)
		}
		reply.WriteString(fmt.Sprintf("Привет, %s! Для старта работы в команде **%s** я проверил регламент безопасности и уже оформил заявку через **Avito Access/IDM (Mock)**:\n\n", user.Name, user.Team))
		reply.WriteString(fmt.Sprintf("1. **Ресурс:** `%s` (`%s`)\n", resource, level))
		reply.WriteString("2. **Что входит в базовый пакет Week-1:** репозиторий `repo:avito-delivery-core`, тестовая среда `env:staging-delivery` и дашборды `grafana:delivery-dashboard`.\n")
		reply.WriteString("3. **Шаг чек-листа:** `step-01-access` автоматически отмечен выполненным.")

	case isAbsence:
		absType := "study_day_off"
		if strings.Contains(lower, "отпуск") {
			absType = "vacation"
		} else if strings.Contains(lower, "личн") {
			absType = "personal_day_off"
		}
		targetDate := extractOrDefaultFutureDate(prompt, 5)
		reason := "Сдача экзамена / учебная сессия в вузе"
		if absType != "study_day_off" {
			reason = "Личный day-off по согласованию с наставником"
		}
		steps = append(steps, "MCP (Avito Absence Mock): подготовка черновика (Human-in-the-loop)")
		if me != nil {
			card, _ = me.CreateAbsenceRequest(user, absType, targetDate, targetDate, reason, false)
		}
		reply.WriteString("По синтетическому регламенту **Avito Start** учебные и личные day-off согласуются заранее с наставником и руководителем.\n\n")
		reply.WriteString(fmt.Sprintf("Я подготовил **черновик заявки** в системе *Avito Absence (Mock)*:\n- **Дата:** `%s`\n- **Согласующий:** %s\n\n", targetDate, emptyFallback(user.ManagerName, "Дмитрий Волков")))
		reply.WriteString("⚠️ Заявка **не отправлена без вашего согласия**. Нажмите кнопку **«Подтвердить отправку»** в карточке ниже.")

	case isMentor1on1:
		topic := "Кэширование и первая задача в avito-delivery-core"
		if !strings.Contains(lower, "кэш") && !strings.Contains(lower, "redis") {
			topic = "Первая задача стажёра и подготовка к Pull Request"
		}
		whatChecked := []string{
			"Изучен Playbook сервиса и Definition of Done",
			"Проверены локальные unit-тесты и схема данных",
		}
		questions := []string{
			"Соответствует ли выбранный подход стандартам команды Авито Доставка?",
			"Какие corner-cases и метрики важно покрыть до открытия PR?",
		}
		steps = append(steps, "MCP (Buddy Sync Mock): генерация структурированной повестки 1:1")
		if me != nil {
			card, _ = me.PrepareMentor1on1Summary(user, topic, whatChecked, questions)
		}
		reply.WriteString(fmt.Sprintf("Отличный подход, %s! Чтобы встреча 1:1 с наставником (**%s**) прошла максимально продуктивно, я подготовил структурированную повестку:\n\n", user.Name, emptyFallback(user.MentorName, "Мария Козлова")))
		reply.WriteString("- **Тема:** " + topic + "\n")
		reply.WriteString("- **Что уже проверено:** Playbook, структура сервиса и базовые тесты\n")
		reply.WriteString("- **Вопросы для Buddy:** проверка архитектуры решения, edge-cases и критерии PR-ревью.\n\n")
		reply.WriteString("Чек-лист `step-04-first-pr` переведён в статус `in_progress`.")

	case isLocalSetup:
		steps = append(steps, "Проверка статуса чек-листа адаптации и технических гайдов")
		progress := 0
		nextStep := "step-01-access"
		if a.Executor != nil && a.Executor.Store != nil {
			ob, _, _ := a.Executor.Store.GetSnapshot()
			progress = ob.OverallProgress
			if ob.NextRecommendedStep != nil {
				nextStep = ob.NextRecommendedStep.Title
			}
		}
		reply.WriteString(fmt.Sprintf("Ваш текущий прогресс адаптации: **%d%%** (следующий шаг: *%s*).\n\n", progress, nextStep))
		reply.WriteString("Для локальной Go-разработки используйте Go 1.24+, Docker Compose (`postgres:16-alpine`, `redis:7-alpine`) и команды `make test` / `make coverage` перед каждым PR.")

	default:
		steps = append(steps, "Синтез ответа с опорой на RBAC-проверенные статьи Playbook")
		if len(searchRes.Articles) > 0 {
			reply.WriteString("По найденным материалам вашей базы знаний:\n\n")
			for _, art := range searchRes.Articles {
				reply.WriteString(fmt.Sprintf("- **%s** (*%s*): %s\n", art.Title, art.Source, art.Summary))
			}
			reply.WriteString("\n*(Примечание: используются синтетические регламенты прототипа Avito Start)*")
		} else {
			reply.WriteString(fmt.Sprintf("Я **AI-buddy**, ваш ИИ-наставник в команде **%s**. Могу помочь с доступами (IDM), настройкой окружения, оформлением отгула в Absence и подготовкой к 1:1 с наставником %s.", user.Team, emptyFallback(user.MentorName, "Марией")))
		}
	}

	return &domain.ChatMessage{
		ID:              a.nextMessageID("msg-bot"),
		Sender:          "assistant",
		RoleContext:     user.Role,
		Content:         reply.String(),
		CreatedAt:       now,
		ProcessingSteps: steps,
		Citations:       searchRes.Citations,
		ActionCard:      card,
		EngineUsed:      "smart_sdd_fallback",
	}
}

func detectSafeAccessResource(lower string) string {
	switch {
	case containsAny(lower, "staging", "стейдж"):
		return "env:staging-delivery"
	case containsAny(lower, "grafana", "дашборд", "мониторинг"):
		return "grafana:delivery-dashboard"
	default:
		return "repo:avito-delivery-core"
	}
}

func extractOrDefaultFutureDate(prompt string, addDays int) string {
	if match := isoDatePattern.FindString(prompt); match != "" {
		return match
	}
	return time.Now().UTC().AddDate(0, 0, addDays).Format("2006-01-02")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func emptyFallback(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}
