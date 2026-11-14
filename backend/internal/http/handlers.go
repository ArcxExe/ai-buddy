package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/config"
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/telemetry"
)

// Server serves REST API endpoints matching specs/openapi.yaml.
type Server struct {
	mu             sync.RWMutex
	Store          *mcp.Store
	Executor       *mcp.Executor
	KB             *rag.KnowledgeBase
	Agent          *ai.AgentOrchestrator
	Tracker        telemetry.ClickstreamTracker
	Messages       []domain.ChatMessage
	Users          map[domain.Role]domain.UserProfile
	ActiveRole     domain.Role
	FeatureFlags   map[string]bool
	AllowedOrigins []string
	GoVersion      string
}

// NewServer constructs an HTTP API server using default runtime configuration.
func NewServer(store *mcp.Store, exec *mcp.Executor, kb *rag.KnowledgeBase, agent *ai.AgentOrchestrator) *Server {
	tracker := telemetry.NewTracker(true)
	if exec != nil && exec.Tracker == nil {
		exec.Tracker = tracker
	}
	if agent != nil && agent.Tracker == nil {
		agent.Tracker = tracker
	}
	return NewServerWithConfig(config.Load(), store, exec, kb, agent, tracker)
}

// NewServerWithConfig constructs an HTTP API server configured via config.Config and ClickstreamTracker.
func NewServerWithConfig(
	cfg config.Config,
	store *mcp.Store,
	exec *mcp.Executor,
	kb *rag.KnowledgeBase,
	agent *ai.AgentOrchestrator,
	tracker telemetry.ClickstreamTracker,
) *Server {
	if store == nil {
		store = mcp.NewStore()
	}
	if tracker == nil {
		tracker = telemetry.NewTracker(cfg.FeatureFlags.ClickstreamAnalytics)
	}
	if exec == nil {
		exec = mcp.NewExecutorWithDeps(store, kb, tracker)
	}

	activeUser, usersList, _, _, msgs, _ := store.GetFullSnapshot()
	usersMap := make(map[domain.Role]domain.UserProfile, len(usersList))
	for _, u := range usersList {
		usersMap[u.Role] = u
	}

	return &Server{
		Store:          store,
		Executor:       exec,
		KB:             kb,
		Agent:          agent,
		Tracker:        tracker,
		Messages:       msgs,
		Users:          usersMap,
		ActiveRole:     activeUser.Role,
		FeatureFlags:   cfg.FeatureFlags.ToMap(),
		AllowedOrigins: append([]string(nil), cfg.CORSAllowedOrigins...),
		GoVersion:      runtime.Version(),
	}
}

// Routes mounts all REST endpoints under /api/v1 and wraps them with CORS and latency telemetry.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/context", s.handleGetContext)
	mux.HandleFunc("POST /api/v1/auth/switch-role", s.handleSwitchRole)
	mux.HandleFunc("PATCH /api/v1/onboarding/checklist/{itemId}", s.handleUpdateChecklist)
	mux.HandleFunc("GET /api/v1/chat/messages", s.handleGetMessages)
	mux.HandleFunc("POST /api/v1/chat/message", s.handlePostMessage)
	mux.HandleFunc("POST /api/v1/chat/messages", s.handlePostMessage)
	mux.HandleFunc("POST /api/v1/chat/reset", s.handleReset)
	mux.HandleFunc("POST /api/v1/corporate/actions/confirm", s.handleConfirmAction)
	mux.HandleFunc("POST /api/v1/corporate/requests/{requestId}/review", s.handleReviewRequest)
	mux.HandleFunc("GET /api/v1/knowledge", s.handleKnowledge)

	return s.corsAndTelemetryMiddleware(mux)
}

// StartProactiveHintWorker periodically evaluates onboarding state and inserts non-intrusive
// reminder hints when FF_ENABLE_PROACTIVE_CRON_HINTS is enabled.
func (s *Server) StartProactiveHintWorker(ctx context.Context, interval time.Duration) {
	if !s.flagEnabled("proactive_cron_hints") || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.TriggerProactiveHint()
			}
		}
	}()
}

// TriggerProactiveHint generates a one-time onboarding nudge if the user has pending steps
// and the hint has not already been posted in the current session.
func (s *Server) TriggerProactiveHint() *domain.ChatMessage {
	if !s.flagEnabled("proactive_cron_hints") {
		return nil
	}
	const hintID = "msg-proactive-hint-01"
	if s.Store.HasMessageID(hintID) {
		return nil
	}

	activeUser, _, ob, _, _, _ := s.Store.GetFullSnapshot()
	if activeUser.Role != domain.RoleIntern || ob.NextRecommendedStep == nil {
		return nil
	}

	hint := domain.ChatMessage{
		ID:          hintID,
		Sender:      "assistant",
		RoleContext: domain.RoleIntern,
		Content: fmt.Sprintf(
			"💡 **Проактивная подсказка AI-buddy:** ваш текущий прогресс — **%d%%**. Следующий шаг чек-листа — *«%s»*. Если готовы, просто нажмите на карточку шага слева или спросите меня!",
			ob.OverallProgress, ob.NextRecommendedStep.Title,
		),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		ProcessingSteps: []string{"Proactive Cron Hint Engine: проверка прогресса онбординга"},
		EngineUsed:      "proactive_cron_hint",
	}
	s.Store.AppendMessages(hint)
	if s.Tracker != nil {
		s.Tracker.Record(telemetry.ClickstreamEvent{
			EventType: telemetry.EventProactiveHintCreated,
			ActorRole: activeUser.Role,
			Metadata:  map[string]string{"next_step": ob.NextRecommendedStep.ID},
		})
	}
	return &hint
}

func (s *Server) flagEnabled(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.FeatureFlags == nil {
		return false
	}
	return s.FeatureFlags[key]
}

func (s *Server) resolveRequestRole(r *http.Request) domain.Role {
	if hdr := strings.TrimSpace(r.Header.Get("X-User-Role")); hdr != "" {
		candidate := domain.Role(hdr)
		if _, ok := s.Store.GetActiveUser(candidate); ok {
			return candidate
		}
	}
	user, _ := s.Store.GetActiveUser("")
	if user.Role == "" {
		return domain.RoleIntern
	}
	return user.Role
}

func (s *Server) corsAndTelemetryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowOrigin := s.matchAllowedOrigin(origin); allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-Role")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		start := time.Now()
		next.ServeHTTP(w, r)

		// Record sub-second SLA telemetry for local API endpoints (excluding external LLM calls).
		isChatPost := r.Method == http.MethodPost && (r.URL.Path == "/api/v1/chat/message" || r.URL.Path == "/api/v1/chat/messages")
		if s.Tracker != nil && !isChatPost {
			elapsed := time.Since(start)
			s.Tracker.RecordLatency(r.URL.Path, s.resolveRequestRole(r), elapsed)
			avgMs := int(math.Round(s.Tracker.AvgLocalLatencyMs()))
			if avgMs < 1 {
				avgMs = 1
			}
			s.Store.SetAvgLatencyMs(avgMs)
		}
	})
}

func (s *Server) matchAllowedOrigin(origin string) string {
	if origin == "" {
		return "*"
	}
	if len(s.AllowedOrigins) == 0 {
		return "*"
	}
	for _, allowed := range s.AllowedOrigins {
		if allowed == "*" {
			return "*"
		}
		if strings.EqualFold(strings.TrimSpace(allowed), strings.TrimSpace(origin)) {
			return origin
		}
	}
	// Preserve seamless dev access when explicit list doesn't reject localhost dev servers.
	return ""
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	provider := "smart_fallback"
	model := "deterministic-sdd-engine"
	configured := false

	if s.Agent != nil {
		if s.Agent.Config.Provider != "" {
			provider = s.Agent.Config.Provider
		}
		if s.Agent.Config.Model != "" {
			model = s.Agent.Config.Model
		}
		configured = strings.TrimSpace(s.Agent.Config.APIKey) != ""
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":                "ok",
		"go_version":            s.GoVersion,
		"llm_provider":          provider,
		"llm_model":             model,
		"model":                 model,
		"fallback_ready":        true,
		"live_llm_configured":   configured,
		"openrouter_configured": configured,
		"storage_mode":          s.Store.RepositoryMode(),
	})
}

func (s *Server) handleGetContext(w http.ResponseWriter, r *http.Request) {
	_, availableRoles, onboarding, requests, _, metrics := s.Store.GetFullSnapshot()
	activeRole := s.resolveRequestRole(r)
	currentUser, ok := s.Store.GetActiveUser(activeRole)
	if !ok && len(availableRoles) > 0 {
		currentUser = availableRoles[0]
		activeRole = currentUser.Role
	}

	s.mu.RLock()
	ff := make(map[string]bool, len(s.FeatureFlags))
	for k, v := range s.FeatureFlags {
		ff[k] = v
	}
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_role":        activeRole,
		"current_user":       currentUser,
		"available_roles":    availableRoles,
		"onboarding":         onboarding,
		"corporate_requests": requests,
		"metrics":            metrics,
		"feature_flags":      ff,
		"disclaimer":         "Демо-прототип AI-buddy (Hack&Change 2026): регламенты, сотрудники и корпоративные сервисы (Absence, IDM) являются синтетическими имитациями.",
	})
}

func (s *Server) handleSwitchRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role domain.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Некорректный формат запроса")
		return
	}

	user, err := s.Store.SetActiveRole(body.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ROLE", "Неизвестная роль: "+string(body.Role))
		return
	}

	s.mu.Lock()
	s.ActiveRole = user.Role
	s.mu.Unlock()

	if s.Tracker != nil {
		s.Tracker.Record(telemetry.ClickstreamEvent{
			EventType: telemetry.EventRoleSwitched,
			ActorRole: user.Role,
			ActorID:   user.ID,
			Route:     r.URL.Path,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_role":  user.Role,
		"current_user": user,
	})
}

func (s *Server) handleUpdateChecklist(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	var body struct {
		Status domain.ChecklistItemStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Некорректное тело запроса")
		return
	}

	state, err := s.Executor.UpdateChecklistItem(itemID, body.Status)
	if err != nil {
		switch {
		case errors.Is(err, mcp.ErrInvalidStatus):
			writeError(w, http.StatusBadRequest, "INVALID_STATUS", err.Error())
		case errors.Is(err, mcp.ErrNotFound):
			writeError(w, http.StatusNotFound, "ITEM_NOT_FOUND", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "CHECKLIST_ERROR", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleGetMessages(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Store.ListMessages())
}

func (s *Server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content      string      `json:"content"`
		ScenarioHint string      `json:"scenario_hint"`
		Role         domain.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Некорректный JSON")
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		writeError(w, http.StatusBadRequest, "EMPTY_MESSAGE", "Сообщение не может быть пустым")
		return
	}

	activeRole := s.resolveRequestRole(r)
	if body.Role != "" {
		if _, ok := s.Store.GetActiveUser(body.Role); ok {
			activeRole = body.Role
		}
	}
	user, _ := s.Store.GetActiveUser(activeRole)

	userMsg := domain.ChatMessage{
		ID:          fmt.Sprintf("msg-usr-%d", time.Now().UTC().UnixNano()),
		Sender:      "user",
		RoleContext: activeRole,
		Content:     strings.TrimSpace(body.Content),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	botMsg, err := s.Agent.ProcessMessage(r.Context(), user, body.Content, body.ScenarioHint)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AGENT_ERROR", err.Error())
		return
	}

	s.Store.AppendMessages(userMsg, *botMsg)

	onboarding, requests, metrics := s.Store.GetSnapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_message":       userMsg,
		"assistant_message":  botMsg,
		"onboarding":         onboarding,
		"corporate_requests": requests,
		"metrics":            metrics,
	})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	s.Store.Reset()
	if s.Tracker != nil {
		s.Tracker.Reset()
	}

	s.mu.Lock()
	s.ActiveRole = domain.RoleIntern
	s.Messages = nil
	s.mu.Unlock()

	s.handleGetContext(w, r)
}

func (s *Server) handleConfirmAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Некорректный JSON")
		return
	}
	if strings.TrimSpace(body.RequestID) == "" {
		writeError(w, http.StatusBadRequest, "MISSING_REQUEST_ID", "Не указан request_id")
		return
	}

	updated, err := s.Executor.ConfirmAction(body.RequestID)
	if err != nil {
		if errors.Is(err, mcp.ErrNotFound) {
			writeError(w, http.StatusNotFound, "ACTION_NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "CONFIRM_ERROR", err.Error())
		return
	}

	onboarding, requests, metrics := s.Store.GetSnapshot()
	payload := corporateRequestResponse(updated, requests)
	payload["onboarding"] = onboarding
	payload["metrics"] = metrics
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleReviewRequest(w http.ResponseWriter, r *http.Request) {
	requestID := r.PathValue("requestId")
	var body struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Некорректный JSON")
		return
	}

	activeRole := s.resolveRequestRole(r)
	updated, err := s.Executor.ReviewRequest(activeRole, requestID, body.Decision, body.Comment)
	if err != nil {
		switch {
		case errors.Is(err, mcp.ErrForbidden):
			writeError(w, http.StatusForbidden, "RBAC_FORBIDDEN", err.Error())
		case errors.Is(err, mcp.ErrInvalidDecision):
			writeError(w, http.StatusBadRequest, "INVALID_DECISION", err.Error())
		case errors.Is(err, mcp.ErrNotFound):
			writeError(w, http.StatusNotFound, "REQUEST_NOT_FOUND", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "REVIEW_ERROR", err.Error())
		}
		return
	}

	_, requests, _ := s.Store.GetSnapshot()
	writeJSON(w, http.StatusOK, corporateRequestResponse(updated, requests))
}

func corporateRequestResponse(updated *domain.CorporateRequest, requests []domain.CorporateRequest) map[string]interface{} {
	payload := map[string]interface{}{
		"request":            updated,
		"corporate_requests": requests,
	}
	if updated != nil {
		payload["id"] = updated.ID
		payload["service_name"] = updated.ServiceName
		payload["tool_name"] = updated.ToolName
		payload["request_type"] = updated.RequestType
		payload["title"] = updated.Title
		payload["summary"] = updated.Summary
		payload["status"] = updated.Status
		payload["created_by_role"] = updated.CreatedByRole
		payload["target_resource"] = updated.TargetResource
		payload["reviewer_comment"] = updated.ReviewerComment
		payload["created_at"] = updated.CreatedAt
		payload["details"] = updated.Details
	}
	return payload
}

func (s *Server) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	activeRole := s.resolveRequestRole(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	var articles []domain.KnowledgeArticle
	var hiddenCount int

	if query != "" {
		res := s.KB.Search(activeRole, query, 10)
		articles = res.Articles
		hiddenCount = res.HiddenByRBACCount
	} else {
		articles, hiddenCount = s.KB.ListAllVisible(activeRole)
	}

	if articles == nil {
		articles = make([]domain.KnowledgeArticle, 0)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"role":                 activeRole,
		"active_role":          activeRole,
		"visible_articles":     articles,
		"items":                articles,
		"hidden_by_rbac_count": hiddenCount,
		"disclaimer":           "Синтетическая база знаний прототипа AI-buddy с RBAC-фильтрацией чувствительности.",
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"code":    code,
		"error":   message,
		"message": message,
	})
}
