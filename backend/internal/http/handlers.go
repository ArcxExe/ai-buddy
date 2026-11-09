package httpapi

import (
	"aibuddy/backend/internal/ai"
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/rbac"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mu           sync.RWMutex
	Store        *mcp.Store
	Executor     *mcp.Executor
	KB           *rag.KnowledgeBase
	Agent        *ai.AgentOrchestrator
	Messages     []domain.ChatMessage
	Users        map[domain.Role]domain.UserProfile
	ActiveRole   domain.Role
	FeatureFlags map[string]bool
	GoVersion    string
}

func NewServer(store *mcp.Store, exec *mcp.Executor, kb *rag.KnowledgeBase, agent *ai.AgentOrchestrator) *Server {
	users := map[domain.Role]domain.UserProfile{
		domain.RoleIntern: {
			ID:             "usr-intern-01",
			Name:           "Алексей Смирнов",
			Role:           domain.RoleIntern,
			RoleTitle:      "Backend Go Intern",
			Team:           "Авито Доставка",
			Department:     "Транспорт и Логистика",
			MentorName:     "Мария Козлова",
			ManagerName:    "Дмитрий Волков",
			AvatarInitials: "АС",
			Permissions:    []string{"read_own_checklist", "request_access", "request_absence", "sync_1on1"},
		},
		domain.RoleMentor: {
			ID:             "usr-mentor-01",
			Name:           "Мария Козлова",
			Role:           domain.RoleMentor,
			RoleTitle:      "Senior Go Developer / Buddy",
			Team:           "Авито Доставка",
			Department:     "Транспорт и Логистика",
			MentorName:     "-",
			ManagerName:    "Дмитрий Волков",
			AvatarInitials: "МК",
			Permissions:    []string{"read_all_checklists", "review_requests", "view_intern_metrics"},
		},
		domain.RoleManager: {
			ID:             "usr-manager-01",
			Name:           "Дмитрий Волков",
			Role:           domain.RoleManager,
			RoleTitle:      "Engineering Manager",
			Team:           "Авито Доставка",
			Department:     "Транспорт и Логистика",
			MentorName:     "-",
			ManagerName:    "-",
			AvatarInitials: "ДВ",
			Permissions:    []string{"admin_all", "approve_absence", "approve_access", "view_headcount"},
		},
	}

	return &Server{
		Store:      store,
		Executor:   exec,
		KB:         kb,
		Agent:      agent,
		Messages:   []domain.ChatMessage{},
		Users:      users,
		ActiveRole: domain.RoleIntern,
		FeatureFlags: map[string]bool{
			"mcp_corporate_actions":    true,
			"rbac_strict_enforcement":  true,
			"clickstream_telemetry":    true,
			"proactive_onboarding_bot": true,
		},
		GoVersion: "go1.27.3",
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/context", s.handleGetContext)
	mux.HandleFunc("POST /api/v1/auth/switch-role", s.handleSwitchRole)
	mux.HandleFunc("PATCH /api/v1/onboarding/checklist/{itemId}", s.handleUpdateChecklist)
	mux.HandleFunc("GET /api/v1/chat/messages", s.handleGetMessages)
	mux.HandleFunc("POST /api/v1/chat/message", s.handleSendMessage)
	mux.HandleFunc("POST /api/v1/chat/reset", s.handleReset)
	mux.HandleFunc("POST /api/v1/corporate/actions/confirm", s.handleConfirmAction)
	mux.HandleFunc("POST /api/v1/corporate/requests/{requestId}/review", s.handleReviewRequest)
	mux.HandleFunc("GET /api/v1/knowledge", s.handleListKnowledge)

	return s.corsMiddleware(mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-Role")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) getCallerUser(r *http.Request) domain.UserProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	headerRole := domain.Role(r.Header.Get("X-User-Role"))
	if u, ok := s.Users[headerRole]; ok {
		return u
	}
	return s.Users[s.ActiveRole]
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := map[string]interface{}{
		"status":                "ok",
		"go_version":            s.GoVersion,
		"llm_provider":          s.Agent.Config.Provider,
		"llm_model":             s.Agent.Config.Model,
		"live_llm_configured":   s.Agent.Config.APIKey != "",
		"local_latency_ms":      1,
		"disclaimer":            "Синтетический прототип для хакатона Avito Start (Hack&Change 2026)",
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetContext(w http.ResponseWriter, r *http.Request) {
	user := s.getCallerUser(r)

	onboarding, corporateRequests, metrics := s.Store.GetSnapshot()

	var availableRoles []domain.UserProfile
	s.mu.RLock()
	for _, u := range s.Users {
		availableRoles = append(availableRoles, u)
	}
	ff := s.FeatureFlags
	s.mu.RUnlock()

	resp := map[string]interface{}{
		"current_user":       user,
		"available_roles":    availableRoles,
		"onboarding":         onboarding,
		"corporate_requests": corporateRequests,
		"metrics":            metrics,
		"feature_flags":      ff,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSwitchRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role domain.Role `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}

	s.mu.Lock()
	if _, ok := s.Users[body.Role]; !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown role"})
		return
	}
	s.ActiveRole = body.Role
	updatedUser := s.Users[body.Role]
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, updatedUser)
}

func (s *Server) handleUpdateChecklist(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	var body struct {
		Status domain.ChecklistItemStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	updatedState, err := s.Executor.UpdateChecklistItem(itemID, body.Status)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, updatedState)
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	msgs := make([]domain.ChatMessage, len(s.Messages))
	copy(msgs, s.Messages)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	user := s.getCallerUser(r)

	var body struct {
		Content      string `json:"content"`
		ScenarioHint string `json:"scenario_hint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Поле 'content' обязательно"})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	userMsg := domain.ChatMessage{
		ID:          fmt.Sprintf("msg-user-%d", time.Now().UnixNano()),
		Sender:      "user",
		RoleContext: user.Role,
		Content:     body.Content,
		CreatedAt:   now,
	}

	botMsg, err := s.Agent.ProcessMessage(r.Context(), user, body.Content, body.ScenarioHint)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.mu.Lock()
	s.Messages = append(s.Messages, userMsg, *botMsg)
	s.mu.Unlock()

	onboarding, corporateRequests, metrics := s.Store.IncrementQuestionsResolved()

	resp := map[string]interface{}{
		"user_message":       userMsg,
		"assistant_message":  botMsg,
		"onboarding":         onboarding,
		"corporate_requests": corporateRequests,
		"metrics":            metrics,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.Messages = []domain.ChatMessage{}
	s.ActiveRole = domain.RoleIntern
	s.mu.Unlock()

	s.Store.Reset()

	s.handleGetContext(w, r)
}

func (s *Server) handleConfirmAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request_id is required"})
		return
	}

	updated, err := s.Executor.ConfirmAction(body.RequestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleReviewRequest(w http.ResponseWriter, r *http.Request) {
	user := s.getCallerUser(r)
	requestID := r.PathValue("requestId")

	var body struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if !rbac.CanReviewRequest(user.Role) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "Недостаточно прав: стажёр не может согласовать заявку",
			"code":  "RBAC_FORBIDDEN",
			"role":  string(user.Role),
		})
		return
	}

	updated, err := s.Executor.ReviewRequest(user.Role, requestID, body.Decision, body.Comment)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleListKnowledge(w http.ResponseWriter, r *http.Request) {
	user := s.getCallerUser(r)
	query := r.URL.Query().Get("q")

	if query != "" {
		res := s.KB.Search(user.Role, query, 10)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"role":                  user.Role,
			"visible_articles":      res.Articles,
			"hidden_by_rbac_count":  res.HiddenByRBACCount,
			"total_articles_count":  res.TotalArticlesCount,
		})
		return
	}

	visible, hidden := s.KB.ListAllVisible(user.Role)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"role":                 user.Role,
		"visible_articles":     visible,
		"hidden_by_rbac_count": hidden,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
