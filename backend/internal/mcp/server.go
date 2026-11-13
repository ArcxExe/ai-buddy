package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/rbac"
	"aibuddy/backend/internal/storage"
	"aibuddy/backend/internal/telemetry"
)

// Store provides thread-safe state management backed by a storage.Repository.
type Store struct {
	Mu                sync.RWMutex
	Onboarding        domain.OnboardingState
	CorporateRequests []domain.CorporateRequest
	Metrics           domain.ImpactMetrics
	Articles          []domain.KnowledgeArticle
	Users             map[domain.Role]domain.UserProfile
	ActiveRole        domain.Role
	Messages          []domain.ChatMessage

	repo storage.Repository
	seq  atomic.Uint64
}

// NewStore initializes a fresh in-memory Store matching the synthetic Avito team.
func NewStore() *Store {
	return NewStoreWithRepo(context.Background(), storage.NewMemoryRepository())
}

// NewStoreWithRepo creates a Store backed by repo and hydrates persisted state if present.
func NewStoreWithRepo(ctx context.Context, repo storage.Repository) *Store {
	if repo == nil {
		repo = storage.NewMemoryRepository()
	}

	state, err := repo.LoadState(ctx)
	if err != nil || state == nil {
		state = storage.DefaultAppState()
		_ = repo.SaveState(ctx, state)
	}
	if len(state.Users) == 0 {
		state.Users = storage.DefaultUsers()
	}
	if state.ActiveRole == "" {
		state.ActiveRole = domain.RoleIntern
	}
	storage.RecalculateOnboarding(&state.Onboarding)

	s := &Store{
		repo: repo,
	}
	s.applyAppStateLocked(state)
	return s
}

func (s *Store) applyAppStateLocked(st *storage.AppState) {
	cp := st.Clone()
	s.ActiveRole = cp.ActiveRole
	s.Users = cp.Users
	s.Onboarding = cp.Onboarding
	s.CorporateRequests = cp.CorporateRequests
	s.Messages = cp.Messages
	s.Metrics = cp.Metrics
	s.Articles = cp.Articles
}

func (s *Store) exportAppStateLocked() *storage.AppState {
	return (&storage.AppState{
		ActiveRole:        s.ActiveRole,
		Users:             s.Users,
		Onboarding:        s.Onboarding,
		CorporateRequests: s.CorporateRequests,
		Messages:          s.Messages,
		Metrics:           s.Metrics,
		Articles:          s.Articles,
	}).Clone()
}

func (s *Store) persistLocked() {
	if s.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.repo.SaveState(ctx, s.exportAppStateLocked())
}

func (s *Store) nextID(prefix string) string {
	n := s.seq.Add(1)
	return fmt.Sprintf("%s-%d-%03d", prefix, time.Now().UTC().UnixMilli(), n)
}

// RepositoryMode returns the active backing storage identifier ("memory", "postgres+redis", etc.).
func (s *Store) RepositoryMode() string {
	if s.repo == nil {
		return "memory"
	}
	return s.repo.Mode()
}

// SetArticles updates stored articles cleanly and persists the snapshot.
func (s *Store) SetArticles(articles []domain.KnowledgeArticle) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	out := make([]domain.KnowledgeArticle, len(articles))
	for i, a := range articles {
		out[i] = storage.CloneKnowledgeArticle(a)
	}
	s.Articles = out
	s.persistLocked()
}

// GetSnapshot safely retrieves deep copies of onboarding, corporate requests, and metrics.
func (s *Store) GetSnapshot() (domain.OnboardingState, []domain.CorporateRequest, domain.ImpactMetrics) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	reqs := make([]domain.CorporateRequest, len(s.CorporateRequests))
	for i, req := range s.CorporateRequests {
		reqs[i] = storage.CloneCorporateRequest(req)
	}
	return storage.CloneOnboardingState(s.Onboarding), reqs, s.Metrics
}

// GetFullSnapshot retrieves active user, ordered profiles, onboarding, requests, messages, and metrics atomically.
func (s *Store) GetFullSnapshot() (domain.UserProfile, []domain.UserProfile, domain.OnboardingState, []domain.CorporateRequest, []domain.ChatMessage, domain.ImpactMetrics) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()

	activeUser := storage.CloneUserProfile(s.Users[s.ActiveRole])
	users := storage.OrderedUsers(s.Users)
	reqs := make([]domain.CorporateRequest, len(s.CorporateRequests))
	for i, r := range s.CorporateRequests {
		reqs[i] = storage.CloneCorporateRequest(r)
	}
	msgs := make([]domain.ChatMessage, len(s.Messages))
	for i, m := range s.Messages {
		msgs[i] = storage.CloneChatMessage(m)
	}
	return activeUser, users, storage.CloneOnboardingState(s.Onboarding), reqs, msgs, s.Metrics
}

// GetActiveUser resolves user profile for roleOverride (or current ActiveRole if empty).
func (s *Store) GetActiveUser(roleOverride domain.Role) (domain.UserProfile, bool) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	role := s.ActiveRole
	if roleOverride != "" {
		role = roleOverride
	}
	u, ok := s.Users[role]
	if !ok {
		return domain.UserProfile{}, false
	}
	return storage.CloneUserProfile(u), true
}

// SetActiveRole switches active demo role if valid and persists cleanly.
func (s *Store) SetActiveRole(role domain.Role) (domain.UserProfile, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	u, ok := s.Users[role]
	if !ok {
		return domain.UserProfile{}, fmt.Errorf("%w: unknown role %q", ErrForbidden, role)
	}
	s.ActiveRole = role
	s.persistLocked()
	return storage.CloneUserProfile(u), nil
}

// AppendMessages adds chat messages safely and persists.
func (s *Store) AppendMessages(msgs ...domain.ChatMessage) {
	if len(msgs) == 0 {
		return
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for _, m := range msgs {
		s.Messages = append(s.Messages, storage.CloneChatMessage(m))
	}
	s.persistLocked()
}

// ListMessages returns deep copies of all chat messages.
func (s *Store) ListMessages() []domain.ChatMessage {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	out := make([]domain.ChatMessage, len(s.Messages))
	for i, m := range s.Messages {
		out[i] = storage.CloneChatMessage(m)
	}
	return out
}

// HasMessageID checks whether a chat message with id already exists.
func (s *Store) HasMessageID(id string) bool {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	for _, m := range s.Messages {
		if m.ID == id {
			return true
		}
	}
	return false
}

// SetAvgLatencyMs updates the telemetry rolling latency value in metrics.
func (s *Store) SetAvgLatencyMs(ms int) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if ms < 1 {
		ms = 1
	}
	s.Metrics.AvgLocalLatencyMs = ms
}

// IncrementQuestionsResolved increments routine questions metric and estimated buddy hours saved.
func (s *Store) IncrementQuestionsResolved() (domain.OnboardingState, []domain.CorporateRequest, domain.ImpactMetrics) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.Metrics.RoutineQuestionsResolved++
	s.Metrics.MentorHoursSaved += 0.35
	s.persistLocked()

	reqs := make([]domain.CorporateRequest, len(s.CorporateRequests))
	for i, req := range s.CorporateRequests {
		reqs[i] = storage.CloneCorporateRequest(req)
	}
	return storage.CloneOnboardingState(s.Onboarding), reqs, s.Metrics
}

// Reset restores fresh default demo state while keeping loaded knowledge articles.
func (s *Store) Reset() {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	articles := s.Articles
	fresh := storage.DefaultAppState()
	fresh.Articles = articles
	s.applyAppStateLocked(fresh)
	s.persistLocked()
}

// syncActionCardInMessagesLocked keeps chat ActionCards synchronized with CorporateRequests updates.
func (s *Store) syncActionCardInMessagesLocked(updated domain.CorporateRequest) {
	for i := range s.Messages {
		if s.Messages[i].ActionCard != nil && s.Messages[i].ActionCard.ID == updated.ID {
			cp := storage.CloneCorporateRequest(updated)
			s.Messages[i].ActionCard = &cp
		}
	}
}

// Executor coordinates MCP tool execution, KB lookups, RBAC checks, and Clickstream telemetry.
type Executor struct {
	Store   *Store
	KB      *rag.KnowledgeBase
	Tracker telemetry.ClickstreamTracker
}

// NewExecutor creates an Executor bound to store with a no-KB fallback.
func NewExecutor(store *Store) *Executor {
	return &Executor{Store: store}
}

// NewExecutorWithDeps creates an Executor wired with KB search and Clickstream telemetry.
func NewExecutorWithDeps(store *Store, kb *rag.KnowledgeBase, tracker telemetry.ClickstreamTracker) *Executor {
	return &Executor{
		Store:   store,
		KB:      kb,
		Tracker: tracker,
	}
}

func (e *Executor) recordEvent(eventType string, role domain.Role, meta map[string]string) {
	if e.Tracker == nil {
		return
	}
	e.Tracker.Record(telemetry.ClickstreamEvent{
		EventType: eventType,
		ActorRole: role,
		Metadata:  meta,
	})
}

// ExecuteTool dispatches a named MCP tool call with strict RBAC validation before execution.
func (e *Executor) ExecuteTool(_ context.Context, user domain.UserProfile, toolName string, rawArgs json.RawMessage) (*ToolExecutionResult, error) {
	toolName = strings.TrimSpace(toolName)
	argsMap := make(map[string]interface{})
	if len(rawArgs) > 0 && string(rawArgs) != "null" {
		if err := json.Unmarshal(rawArgs, &argsMap); err != nil {
			return nil, fmt.Errorf("invalid tool arguments JSON: %w", err)
		}
	}

	targetResource, _ := argsMap["resource"].(string)
	if allowed, reason := rbac.CanExecuteMCPTool(user.Role, toolName, targetResource); !allowed {
		// If it's a blocked request_service_access, still materialize a blocked card for full UI audit trail.
		if toolName == "request_service_access" {
			level, _ := argsMap["access_level"].(string)
			why, _ := argsMap["reason"].(string)
			card, _ := e.RequestServiceAccess(user, targetResource, level, why)
			payload, _ := json.Marshal(map[string]interface{}{
				"allowed":         false,
				"blocked_by_rbac": true,
				"reason":          reason,
				"action_card":     card,
			})
			return &ToolExecutionResult{
				ToolName:       toolName,
				Service:        "Avito Access / IDM Mock",
				Allowed:        false,
				BlockedReason:  reason,
				OutputJSON:     string(payload),
				ActionCard:     card,
				ProcessingStep: fmt.Sprintf("RBAC Policy: доступ к %s заблокирован для роли %s", targetResource, user.Role),
			}, nil
		}

		e.recordEvent(telemetry.EventRBACAccessDenied, user.Role, map[string]string{
			"tool":   toolName,
			"reason": reason,
		})
		payload, _ := json.Marshal(map[string]interface{}{
			"allowed":         false,
			"blocked_by_rbac": true,
			"reason":          reason,
		})
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        "Avito RBAC Engine",
			Allowed:        false,
			BlockedReason:  reason,
			OutputJSON:     string(payload),
			ProcessingStep: fmt.Sprintf("RBAC Policy: вызов %s отклонён для роли %s", toolName, user.Role),
		}, nil
	}

	switch toolName {
	case "search_knowledge_base":
		var p struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal(rawArgs, &p)
		if p.Limit <= 0 {
			p.Limit = 3
		}
		sr := e.searchKnowledge(user.Role, p.Query, p.Limit)
		out, _ := json.Marshal(map[string]interface{}{
			"articles":            sr.Articles,
			"count":               len(sr.Articles),
			"hidden_by_rbac":      sr.HiddenByRBACCount,
			"total_articles":      sr.TotalArticlesCount,
		})
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        "Avito Knowledge Base / Playbook RAG",
			Allowed:        true,
			OutputJSON:     string(out),
			Citations:      sr.Citations,
			ProcessingStep: fmt.Sprintf("Knowledge Base: найдено %d статей (скрыто RBAC: %d)", len(sr.Articles), sr.HiddenByRBACCount),
		}, nil

	case "get_onboarding_status":
		ob, _, _ := e.Store.GetSnapshot()
		out, _ := json.Marshal(ob)
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        "Avito Start Onboarding Tracker",
			Allowed:        true,
			OutputJSON:     string(out),
			ProcessingStep: fmt.Sprintf("Onboarding Tracker: текущий прогресс %d%%", ob.OverallProgress),
		}, nil

	case "complete_checklist_item":
		var p struct {
			ItemID string                     `json:"item_id"`
			Status domain.ChecklistItemStatus `json:"status"`
		}
		_ = json.Unmarshal(rawArgs, &p)
		if strings.TrimSpace(p.ItemID) == "" {
			return nil, fmt.Errorf("%w: item_id", ErrMissingArgument)
		}
		if p.Status == "" {
			p.Status = domain.StatusDone
		}
		ob, err := e.UpdateChecklistItem(p.ItemID, p.Status)
		if err != nil {
			return nil, err
		}
		out, _ := json.Marshal(ob)
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        "Avito Start Onboarding Tracker",
			Allowed:        true,
			OutputJSON:     string(out),
			ProcessingStep: fmt.Sprintf("Чек-лист обновлён: %s -> %s (%d%%)", p.ItemID, p.Status, ob.OverallProgress),
		}, nil

	case "request_service_access":
		var p struct {
			Resource    string `json:"resource"`
			AccessLevel string `json:"access_level"`
			Reason      string `json:"reason"`
		}
		_ = json.Unmarshal(rawArgs, &p)
		if strings.TrimSpace(p.Resource) == "" {
			return nil, fmt.Errorf("%w: resource", ErrMissingArgument)
		}
		if p.AccessLevel == "" {
			p.AccessLevel = "read_write"
		}
		if p.Reason == "" {
			p.Reason = "Запрошено через AI-buddy для выполнения задач команды"
		}
		req, err := e.RequestServiceAccess(user, p.Resource, p.AccessLevel, p.Reason)
		if err != nil {
			return nil, err
		}
		out, _ := json.Marshal(req)
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        req.ServiceName,
			Allowed:        req.Status != domain.StatusBlockedByRBAC,
			OutputJSON:     string(out),
			ActionCard:     req,
			ProcessingStep: fmt.Sprintf("MCP (Avito Access/IDM): заявка %s -> %s", req.TargetResource, req.Status),
		}, nil

	case "create_absence_request":
		var p struct {
			AbsenceType     string `json:"absence_type"`
			StartDate       string `json:"start_date"`
			EndDate         string `json:"end_date"`
			Reason          string `json:"reason"`
			ConfirmedByUser bool   `json:"confirmed_by_user"`
		}
		_ = json.Unmarshal(rawArgs, &p)
		if p.AbsenceType == "" {
			p.AbsenceType = "study_day_off"
		}
		if p.StartDate == "" {
			p.StartDate = time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")
		}
		if p.EndDate == "" {
			p.EndDate = p.StartDate
		}
		if p.Reason == "" {
			p.Reason = "Учебный/личный day-off по согласованию с наставником"
		}
		req, err := e.CreateAbsenceRequest(user, p.AbsenceType, p.StartDate, p.EndDate, p.Reason, p.ConfirmedByUser)
		if err != nil {
			return nil, err
		}
		out, _ := json.Marshal(req)
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        req.ServiceName,
			Allowed:        true,
			OutputJSON:     string(out),
			ActionCard:     req,
			ProcessingStep: fmt.Sprintf("MCP (Avito Absence): создан черновик %s (%s)", req.ID, req.Status),
		}, nil

	case "prepare_mentor_1on1_summary":
		var p struct {
			Topic       string   `json:"topic"`
			WhatChecked []string `json:"what_checked"`
			Questions   []string `json:"questions"`
		}
		_ = json.Unmarshal(rawArgs, &p)
		if p.Topic == "" {
			p.Topic = "Первая задача стажёра и вопросы по архитектуре"
		}
		if len(p.WhatChecked) == 0 {
			p.WhatChecked = []string{"Playbook и README сервиса", "Локальный запуск и тесты"}
		}
		if len(p.Questions) == 0 {
			p.Questions = []string{"Соответствует ли подход стандартам команды?", "Какие edge-cases проверить перед PR?"}
		}
		req, err := e.PrepareMentor1on1Summary(user, p.Topic, p.WhatChecked, p.Questions)
		if err != nil {
			return nil, err
		}
		out, _ := json.Marshal(req)
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        req.ServiceName,
			Allowed:        true,
			OutputJSON:     string(out),
			ActionCard:     req,
			ProcessingStep: fmt.Sprintf("MCP (Buddy Sync): сформирована повестка 1:1 (%s)", p.Topic),
		}, nil

	case "review_corporate_request":
		var p struct {
			RequestID string `json:"request_id"`
			Decision  string `json:"decision"`
			Comment   string `json:"comment"`
		}
		_ = json.Unmarshal(rawArgs, &p)
		req, err := e.ReviewRequest(user.Role, p.RequestID, p.Decision, p.Comment)
		if err != nil {
			return nil, err
		}
		out, _ := json.Marshal(req)
		return &ToolExecutionResult{
			ToolName:       toolName,
			Service:        req.ServiceName,
			Allowed:        true,
			OutputJSON:     string(out),
			ActionCard:     req,
			ProcessingStep: fmt.Sprintf("MCP (Workflow): заявка %s переведена в %s", req.ID, req.Status),
		}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, toolName)
	}
}

func (e *Executor) searchKnowledge(role domain.Role, query string, limit int) rag.SearchResult {
	if e.KB != nil {
		return e.KB.Search(role, query, limit)
	}
	temp := rag.NewKnowledgeBase()
	e.Store.Mu.RLock()
	articles := make([]domain.KnowledgeArticle, len(e.Store.Articles))
	copy(articles, e.Store.Articles)
	e.Store.Mu.RUnlock()
	temp.SetArticles(articles)
	return temp.Search(role, query, limit)
}

// RequestServiceAccess creates or blocks an access request in Avito Access/IDM.
func (e *Executor) RequestServiceAccess(user domain.UserProfile, resource string, level string, reason string) (*domain.CorporateRequest, error) {
	allowed, denyReason := rbac.CanExecuteMCPTool(user.Role, "request_service_access", resource)
	now := time.Now().UTC().Format(time.RFC3339)

	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	if !allowed {
		req := domain.CorporateRequest{
			ID:             e.Store.nextID("req-rbac"),
			ServiceName:    "Avito Access / IDM (Mock)",
			ToolName:       "request_service_access",
			RequestType:    domain.TypeRBACBlocked,
			Title:          "Доступ заблокирован политикой RBAC: " + resource,
			Summary:        denyReason,
			Status:         domain.StatusBlockedByRBAC,
			CreatedByRole:  user.Role,
			TargetResource: resource,
			CreatedAt:      now,
			Details: map[string]interface{}{
				"resource":     resource,
				"access_level": level,
				"reason":       reason,
				"policy_rule":  "RBAC_GUARDRAIL_DENIED",
			},
		}
		e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
		e.Store.Metrics.RBACViolationsPrevented++
		e.Store.persistLocked()
		e.recordEvent(telemetry.EventRBACAccessDenied, user.Role, map[string]string{"resource": resource})
		cp := storage.CloneCorporateRequest(req)
		return &cp, nil
	}

	status := domain.StatusApproved
	reviewerNote := "Автоматически одобрено по базовому профилю команды " + user.Team
	if level == "admin" {
		status = domain.StatusPendingApproval
		reviewerNote = "Требует подтверждения руководителя: " + user.ManagerName
	}

	req := domain.CorporateRequest{
		ID:             e.Store.nextID("req-idm"),
		ServiceName:    "Avito Access / IDM (Mock)",
		ToolName:       "request_service_access",
		RequestType:    domain.TypeAccessRequest,
		Title:          fmt.Sprintf("Доступ (%s): %s", level, resource),
		Summary:        reviewerNote,
		Status:         status,
		CreatedByRole:  user.Role,
		TargetResource: resource,
		CreatedAt:      now,
		Details: map[string]interface{}{
			"resource":     resource,
			"access_level": level,
			"reason":       reason,
			"sla":          "Доступ выдаётся в течение 15 минут",
			"synthetic":    true,
		},
	}

	e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
	e.Store.Metrics.MCPActionsExecuted++
	e.Store.Metrics.MentorHoursSaved += 0.5

	if strings.Contains(strings.ToLower(resource), "repo:") || strings.Contains(strings.ToLower(resource), "staging") {
		for i := range e.Store.Onboarding.Checklist {
			if e.Store.Onboarding.Checklist[i].ID == "step-01-access" && e.Store.Onboarding.Checklist[i].Status != domain.StatusDone {
				e.Store.Onboarding.Checklist[i].Status = domain.StatusDone
			}
		}
		storage.RecalculateOnboarding(&e.Store.Onboarding)
	}

	e.Store.persistLocked()
	e.recordEvent(telemetry.EventMCPActionCompleted, user.Role, map[string]string{
		"tool":     "request_service_access",
		"resource": resource,
		"status":   string(status),
	})

	cp := storage.CloneCorporateRequest(req)
	return &cp, nil
}

// CreateAbsenceRequest creates a draft or finalized absence request in Avito Absence Service.
func (e *Executor) CreateAbsenceRequest(user domain.UserProfile, absenceType string, startDate string, endDate string, reason string, confirmed bool) (*domain.CorporateRequest, error) {
	allowed, denyReason := rbac.CanExecuteMCPTool(user.Role, "create_absence_request", "")
	if !allowed {
		return nil, fmt.Errorf("%w: %s", ErrForbidden, denyReason)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := domain.StatusDraftConfirmationRequired
	summary := "Черновик готов. Проверьте даты и нажмите «Подтвердить отправку» для направления руководителю."
	if confirmed {
		status = domain.StatusPendingApproval
		summary = "Заявка отправлена на согласование: " + user.ManagerName
	}

	typeTitle := "Учебный day-off"
	switch absenceType {
	case "personal_day_off":
		typeTitle = "Личный отгул"
	case "vacation":
		typeTitle = "Отпуск"
	}

	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	req := domain.CorporateRequest{
		ID:             e.Store.nextID("req-abs"),
		ServiceName:    "Avito Absence (Mock)",
		ToolName:       "create_absence_request",
		RequestType:    domain.TypeAbsenceRequest,
		Title:          fmt.Sprintf("%s: %s", typeTitle, startDate),
		Summary:        summary,
		Status:         status,
		CreatedByRole:  user.Role,
		TargetResource: "absence:" + absenceType,
		CreatedAt:      now,
		Details: map[string]interface{}{
			"absence_type": absenceType,
			"start_date":   startDate,
			"end_date":     endDate,
			"reason":       reason,
			"approver":     user.ManagerName,
			"buddy":        user.MentorName,
			"synthetic":    true,
		},
	}

	e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
	e.Store.Metrics.MCPActionsExecuted++
	e.Store.Metrics.MentorHoursSaved += 0.4
	e.Store.persistLocked()

	e.recordEvent(telemetry.EventMCPActionCompleted, user.Role, map[string]string{
		"tool":   "create_absence_request",
		"status": string(status),
	})

	cp := storage.CloneCorporateRequest(req)
	return &cp, nil
}

// PrepareMentor1on1Summary creates an agenda summary for a 1:1 meeting with Buddy.
func (e *Executor) PrepareMentor1on1Summary(user domain.UserProfile, topic string, whatChecked []string, questions []string) (*domain.CorporateRequest, error) {
	allowed, denyReason := rbac.CanExecuteMCPTool(user.Role, "prepare_mentor_1on1_summary", "")
	if !allowed {
		return nil, fmt.Errorf("%w: %s", ErrForbidden, denyReason)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	req := domain.CorporateRequest{
		ID:             e.Store.nextID("req-1on1"),
		ServiceName:    "Avito Buddy Sync (Mock)",
		ToolName:       "prepare_mentor_1on1_summary",
		RequestType:    domain.TypeMentor1on1Summary,
		Title:          "Повестка 1:1 с наставником: " + topic,
		Summary:        fmt.Sprintf("Сформирована структура для %s (%d вопросов)", user.MentorName, len(questions)),
		Status:         domain.StatusCompleted,
		CreatedByRole:  user.Role,
		TargetResource: "mentor:" + user.MentorName,
		CreatedAt:      now,
		Details: map[string]interface{}{
			"topic":        topic,
			"what_checked": whatChecked,
			"questions":    questions,
			"buddy":        user.MentorName,
			"synthetic":    true,
		},
	}

	e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
	e.Store.Metrics.MCPActionsExecuted++
	e.Store.Metrics.MentorHoursSaved += 0.6

	for i := range e.Store.Onboarding.Checklist {
		if e.Store.Onboarding.Checklist[i].ID == "step-04-first-pr" && e.Store.Onboarding.Checklist[i].Status == domain.StatusTodo {
			e.Store.Onboarding.Checklist[i].Status = domain.StatusInProgress
		}
	}
	storage.RecalculateOnboarding(&e.Store.Onboarding)
	e.Store.persistLocked()

	e.recordEvent(telemetry.EventMCPActionCompleted, user.Role, map[string]string{
		"tool":  "prepare_mentor_1on1_summary",
		"topic": topic,
	})

	cp := storage.CloneCorporateRequest(req)
	return &cp, nil
}

// ConfirmAction confirms a pending draft action, updates onboarding, and syncs chat ActionCards.
func (e *Executor) ConfirmAction(requestID string) (*domain.CorporateRequest, error) {
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	for i := range e.Store.CorporateRequests {
		req := &e.Store.CorporateRequests[i]
		if req.ID != requestID {
			continue
		}
		if req.Status == domain.StatusDraftConfirmationRequired {
			req.Status = domain.StatusPendingApproval
			approver := "Дмитрий Волков"
			if app, ok := req.Details["approver"].(string); ok && app != "" {
				approver = app
			}
			req.Summary = "Подтверждено стажёром и отправлено на согласование (" + approver + ")"
		}
		if req.RequestType == domain.TypeAbsenceRequest {
			for j := range e.Store.Onboarding.Checklist {
				if e.Store.Onboarding.Checklist[j].ID == "step-03-absence-policy" && e.Store.Onboarding.Checklist[j].Status != domain.StatusDone {
					e.Store.Onboarding.Checklist[j].Status = domain.StatusDone
				}
			}
			storage.RecalculateOnboarding(&e.Store.Onboarding)
		}
		e.Store.syncActionCardInMessagesLocked(*req)
		e.Store.persistLocked()
		e.recordEvent(telemetry.EventRequestConfirmed, req.CreatedByRole, map[string]string{"request_id": req.ID})
		cp := storage.CloneCorporateRequest(*req)
		return &cp, nil
	}

	return nil, fmt.Errorf("%w: request %s", ErrNotFound, requestID)
}

// ReviewRequest allows mentor or manager to approve or reject a corporate request and syncs chat ActionCards.
func (e *Executor) ReviewRequest(role domain.Role, requestID string, decision string, comment string) (*domain.CorporateRequest, error) {
	if !rbac.CanReviewRequest(role) {
		e.recordEvent(telemetry.EventRBACAccessDenied, role, map[string]string{
			"tool":       "review_corporate_request",
			"request_id": requestID,
		})
		return nil, fmt.Errorf("%w: role %s cannot review corporate requests", ErrForbidden, role)
	}

	var targetStatus domain.RequestStatus
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approved":
		targetStatus = domain.StatusApproved
	case "rejected":
		targetStatus = domain.StatusRejected
	default:
		return nil, fmt.Errorf("%w: got %q", ErrInvalidDecision, decision)
	}

	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	for i := range e.Store.CorporateRequests {
		req := &e.Store.CorporateRequests[i]
		if req.ID != requestID {
			continue
		}
		req.Status = targetStatus
		req.ReviewerComment = comment
		if targetStatus == domain.StatusApproved {
			req.Summary = fmt.Sprintf("Одобрено (%s)", role)
		} else {
			req.Summary = fmt.Sprintf("Отклонено (%s)", role)
		}
		if comment != "" {
			req.Summary += ": " + comment
		}
		e.Store.syncActionCardInMessagesLocked(*req)
		e.Store.persistLocked()
		e.recordEvent(telemetry.EventRequestReviewed, role, map[string]string{
			"request_id": requestID,
			"decision":   string(targetStatus),
		})
		cp := storage.CloneCorporateRequest(*req)
		return &cp, nil
	}

	return nil, fmt.Errorf("%w: request %s", ErrNotFound, requestID)
}

// UpdateChecklistItem updates a checklist task status, recalculates stage & overall progress, and persists.
func (e *Executor) UpdateChecklistItem(itemID string, status domain.ChecklistItemStatus) (*domain.OnboardingState, error) {
	switch status {
	case domain.StatusTodo, domain.StatusInProgress, domain.StatusDone:
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, status)
	}

	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	found := false
	for i := range e.Store.Onboarding.Checklist {
		if e.Store.Onboarding.Checklist[i].ID == itemID {
			e.Store.Onboarding.Checklist[i].Status = status
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: checklist item %s", ErrNotFound, itemID)
	}

	storage.RecalculateOnboarding(&e.Store.Onboarding)
	e.Store.persistLocked()
	e.recordEvent(telemetry.EventChecklistUpdated, e.Store.ActiveRole, map[string]string{
		"item_id": itemID,
		"status":  string(status),
	})

	cp := storage.CloneOnboardingState(e.Store.Onboarding)
	return &cp, nil
}
