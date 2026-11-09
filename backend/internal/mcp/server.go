package mcp

import (
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rbac"
	"fmt"
	"sync"
	"time"
)

// Store provides thread-safe in-memory state for checklist, corporate requests, and metrics
type Store struct {
	Mu                sync.RWMutex
	Onboarding        domain.OnboardingState
	CorporateRequests []domain.CorporateRequest
	Metrics           domain.ImpactMetrics
	Articles          []domain.KnowledgeArticle
}

// GetSnapshot safely retrieves current onboarding, requests and metrics
func (s *Store) GetSnapshot() (domain.OnboardingState, []domain.CorporateRequest, domain.ImpactMetrics) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.Onboarding, s.CorporateRequests, s.Metrics
}

// IncrementQuestionsResolved increments telemetry counter
func (s *Store) IncrementQuestionsResolved() (domain.OnboardingState, []domain.CorporateRequest, domain.ImpactMetrics) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Metrics.RoutineQuestionsResolved++
	return s.Onboarding, s.CorporateRequests, s.Metrics
}

// Reset restores fresh default state
func (s *Store) Reset() {
	fresh := NewStore()
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Onboarding = fresh.Onboarding
	s.CorporateRequests = fresh.CorporateRequests
	s.Metrics = fresh.Metrics
}

// NewStore initializes default state matching synthetic Avito team
func NewStore() *Store {
	checklist := []domain.ChecklistItem{
		{
			ID:              "step-01-access",
			StageID:         "stage-week-1",
			Title:           "Запросить доступы в Avito Access / IDM",
			Description:     "Оформить заявки на доступ к repo:avito-delivery-core, staging-окружению и Grafana.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryAccess,
			EstimatedTime:   "15 мин",
			SuggestedPrompt: "Какие доступы мне нужны для первой задачи в команде Авито Доставка?",
		},
		{
			ID:              "step-02-env-setup",
			StageID:         "stage-week-1",
			Title:           "Настройка окружения Go 1.27.3 и PaaS",
			Description:     "Установить зависимости, запустить docker compose и проверить сборку микросервиса.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryTools,
			EstimatedTime:   "45 мин",
			SuggestedPrompt: "Как локально собрать и запустить микросервис доставки через docker?",
		},
		{
			ID:              "step-03-absence-policy",
			StageID:         "stage-week-2",
			Title:           "Знакомство с регламентом отсутствий и отгулов",
			Description:     "Изучить порядок оформления учебных дней для сдачи сессии в университете.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryProcesses,
			EstimatedTime:   "10 мин",
			SuggestedPrompt: "Как в Авито оформить отгул на экзамен в вузе?",
		},
		{
			ID:              "step-04-first-pr",
			StageID:         "stage-week-2",
			Title:           "Первая задача: Pull Request и подготовка к 1:1",
			Description:     "Взять стартовый таск, оформить PR по стандартам Playbook и вынести архитектурные вопросы на 1:1 с наставником.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryFirstTask,
			EstimatedTime:   "2 дня",
			SuggestedPrompt: "Помоги подготовить повестку к встрече 1:1 с наставником Марией Козловой по первой задаче.",
		},
	}

	stages := []domain.OnboardingStage{
		{
			ID:              "stage-day-1",
			Title:           "День 1: Знакомство и техника",
			Period:          "1–2 день",
			Status:          "completed",
			ProgressPercent: 100,
		},
		{
			ID:              "stage-week-1",
			Title:           "Неделя 1: Доступы и окружение",
			Period:          "Неделя 1",
			Status:          "current",
			ProgressPercent: 0,
		},
		{
			ID:              "stage-week-2",
			Title:           "Неделя 2–3: Первая задача и процессы",
			Period:          "Неделя 2–3",
			Status:          "upcoming",
			ProgressPercent: 0,
		},
		{
			ID:              "stage-equator",
			Title:           "Экватор: Ревью компетенций",
			Period:          "Месяц 3",
			Status:          "upcoming",
			ProgressPercent: 0,
		},
	}

	s := &Store{
		Onboarding: domain.OnboardingState{
			OverallProgress:     25,
			CurrentStageID:      "stage-week-1",
			NextRecommendedStep: &checklist[0],
			Stages:              stages,
			Checklist:           checklist,
		},
		CorporateRequests: []domain.CorporateRequest{},
		Metrics: domain.ImpactMetrics{
			MentorHoursBaseline:      48.0,
			MentorHoursSaved:         0.0,
			RoutineQuestionsResolved: 0,
			MCPActionsExecuted:       0,
			RBACViolationsPrevented:  0,
			AvgLocalLatencyMs:        4,
		},
	}
	return s
}

// Executor coordinates MCP tool execution
type Executor struct {
	Store *Store
}

func NewExecutor(store *Store) *Executor {
	return &Executor{Store: store}
}

// RequestServiceAccess creates or blocks an access request in Avito Access/IDM
func (e *Executor) RequestServiceAccess(user domain.UserProfile, resource string, level string, reason string) (*domain.CorporateRequest, error) {
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	allowed, blockReason := rbac.CanExecuteMCPTool(user.Role, "request_service_access", resource)
	now := time.Now().Format("2006-01-02 15:04:05")
	reqID := fmt.Sprintf("ACC-2026-%03d", len(e.Store.CorporateRequests)+101)

	if !allowed {
		e.Store.Metrics.RBACViolationsPrevented++
		req := domain.CorporateRequest{
			ID:             reqID,
			ServiceName:    "Avito Access / IDM Service",
			ToolName:       "request_service_access",
			RequestType:    domain.TypeRBACBlocked,
			Title:          "Заблокирована попытка запроса: " + resource,
			Summary:        blockReason,
			Status:         domain.StatusBlockedByRBAC,
			CreatedByRole:  user.Role,
			TargetResource: resource,
			CreatedAt:      now,
			Details: map[string]interface{}{
				"resource":     resource,
				"access_level": level,
				"error":        blockReason,
			},
		}
		e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
		return &req, nil
	}

	req := domain.CorporateRequest{
		ID:             reqID,
		ServiceName:    "Avito Access / IDM Service",
		ToolName:       "request_service_access",
		RequestType:    domain.TypeAccessRequest,
		Title:          "Заявка на доступ: " + resource,
		Summary:        fmt.Sprintf("Запрошен уровень '%s' для ресурса '%s'. Обоснование: %s", level, resource, reason),
		Status:         domain.StatusApproved,
		CreatedByRole:  user.Role,
		TargetResource: resource,
		CreatedAt:      now,
		Details: map[string]interface{}{
			"resource":     resource,
			"access_level": level,
			"justification": reason,
		},
	}

	e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
	e.Store.Metrics.MCPActionsExecuted++
	e.Store.Metrics.MentorHoursSaved += 0.5

	// Auto complete step-01-access if applicable
	for i := range e.Store.Onboarding.Checklist {
		if e.Store.Onboarding.Checklist[i].ID == "step-01-access" {
			e.Store.Onboarding.Checklist[i].Status = domain.StatusDone
		}
	}
	e.recalculateProgressLocked()

	return &req, nil
}

// CreateAbsenceRequest creates draft or finalized absence request in Avito Absence Service
func (e *Executor) CreateAbsenceRequest(user domain.UserProfile, absenceType string, startDate string, endDate string, reason string, confirmed bool) (*domain.CorporateRequest, error) {
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	reqID := fmt.Sprintf("ABS-2026-%03d", len(e.Store.CorporateRequests)+201)
	now := time.Now().Format("2006-01-02 15:04:05")

	status := domain.StatusDraftConfirmationRequired
	summary := fmt.Sprintf("Черновик заявки на отгул с %s по %s. Нажмите 'Подтвердить', чтобы отправить руководителю (%s).", startDate, endDate, user.ManagerName)
	if confirmed {
		status = domain.StatusPendingApproval
		summary = fmt.Sprintf("Заявка на отгул отправлена на согласование руководителю (%s). Наставник (%s) уведомлен.", user.ManagerName, user.MentorName)
		e.Store.Metrics.MCPActionsExecuted++
		e.Store.Metrics.MentorHoursSaved += 0.4
	}

	req := domain.CorporateRequest{
		ID:             reqID,
		ServiceName:    "Avito Absence Service (Система отсутствий)",
		ToolName:       "create_absence_request",
		RequestType:    domain.TypeAbsenceRequest,
		Title:          fmt.Sprintf("Учебный отгул: %s (%s)", startDate, reason),
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
			"confirmed":    confirmed,
			"approver":     user.ManagerName,
			"notifier":     user.MentorName,
		},
	}

	// Update existing if it was draft
	found := false
	for i, existing := range e.Store.CorporateRequests {
		if existing.ID == reqID || (existing.RequestType == domain.TypeAbsenceRequest && existing.Status == domain.StatusDraftConfirmationRequired && confirmed) {
			req.ID = existing.ID
			e.Store.CorporateRequests[i] = req
			found = true
			break
		}
	}
	if !found {
		e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
	}

	if confirmed {
		for i := range e.Store.Onboarding.Checklist {
			if e.Store.Onboarding.Checklist[i].ID == "step-03-absence-policy" {
				e.Store.Onboarding.Checklist[i].Status = domain.StatusDone
			}
		}
		e.recalculateProgressLocked()
	}

	return &req, nil
}

// PrepareMentor1on1Summary creates agenda for 1:1 meeting with buddy
func (e *Executor) PrepareMentor1on1Summary(user domain.UserProfile, topic string, whatChecked []string, questions []string) (*domain.CorporateRequest, error) {
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	reqID := fmt.Sprintf("SYNC-2026-%03d", len(e.Store.CorporateRequests)+301)
	now := time.Now().Format("2006-01-02 15:04:05")

	req := domain.CorporateRequest{
		ID:             reqID,
		ServiceName:    "Avito Mentor Sync & Calendar",
		ToolName:       "prepare_mentor_1on1_summary",
		RequestType:    domain.TypeMentor1on1Summary,
		Title:          "Повестка 1:1 с наставником: " + topic,
		Summary:        fmt.Sprintf("Сформировано структурированное саммари для наставника (%s): 3 точечных вопроса вместо часового погружения.", user.MentorName),
		Status:         domain.StatusCompleted,
		CreatedByRole:  user.Role,
		TargetResource: "mentor-sync:" + user.MentorName,
		CreatedAt:      now,
		Details: map[string]interface{}{
			"mentor_name":  user.MentorName,
			"topic":        topic,
			"what_checked": whatChecked,
			"questions":    questions,
		},
	}

	e.Store.CorporateRequests = append([]domain.CorporateRequest{req}, e.Store.CorporateRequests...)
	e.Store.Metrics.MCPActionsExecuted++
	e.Store.Metrics.MentorHoursSaved += 0.75
	e.recalculateProgressLocked()

	return &req, nil
}

// ConfirmAction confirms a pending draft action
func (e *Executor) ConfirmAction(requestID string) (*domain.CorporateRequest, error) {
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	for i, req := range e.Store.CorporateRequests {
		if req.ID == requestID {
			if req.Status == domain.StatusDraftConfirmationRequired {
				e.Store.CorporateRequests[i].Status = domain.StatusPendingApproval
				e.Store.CorporateRequests[i].Summary = "Заявка успешно подтверждена и отправлена на согласование руководителю."
				e.Store.Metrics.MCPActionsExecuted++
				e.Store.Metrics.MentorHoursSaved += 0.4

				for c := range e.Store.Onboarding.Checklist {
					if e.Store.Onboarding.Checklist[c].ID == "step-03-absence-policy" {
						e.Store.Onboarding.Checklist[c].Status = domain.StatusDone
					}
				}
				e.recalculateProgressLocked()
				return &e.Store.CorporateRequests[i], nil
			}
			return &e.Store.CorporateRequests[i], nil
		}
	}
	return nil, fmt.Errorf("заявка с ID '%s' не найдена", requestID)
}

// ReviewRequest allows mentor or manager to approve or reject
func (e *Executor) ReviewRequest(role domain.Role, requestID string, decision string, comment string) (*domain.CorporateRequest, error) {
	e.Store.Mu.Lock()
	defer e.Store.Mu.Unlock()

	if !rbac.CanReviewRequest(role) {
		return nil, fmt.Errorf("недостаточно прав: согласование доступно только для mentor или manager")
	}

	for i, req := range e.Store.CorporateRequests {
		if req.ID == requestID {
			if decision == "approved" {
				e.Store.CorporateRequests[i].Status = domain.StatusApproved
			} else {
				e.Store.CorporateRequests[i].Status = domain.StatusRejected
			}
			e.Store.CorporateRequests[i].ReviewerComment = comment
			return &e.Store.CorporateRequests[i], nil
		}
	}
	return nil, fmt.Errorf("заявка с ID '%s' не найдена", requestID)
}

// UpdateChecklistItem updates a checklist task status and recalculates progress
func (e *Executor) UpdateChecklistItem(itemID string, status domain.ChecklistItemStatus) (*domain.OnboardingState, error) {
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
		return nil, fmt.Errorf("задача '%s' не найдена в чек-листе", itemID)
	}

	e.recalculateProgressLocked()
	return &e.Store.Onboarding, nil
}

func (e *Executor) recalculateProgressLocked() {
	doneCount := 0
	var nextStep *domain.ChecklistItem
	for i := range e.Store.Onboarding.Checklist {
		if e.Store.Onboarding.Checklist[i].Status == domain.StatusDone {
			doneCount++
		} else if nextStep == nil && e.Store.Onboarding.Checklist[i].Status != domain.StatusDone {
			nextStep = &e.Store.Onboarding.Checklist[i]
		}
	}
	total := len(e.Store.Onboarding.Checklist)
	if total > 0 {
		e.Store.Onboarding.OverallProgress = (doneCount * 100) / total
	}
	e.Store.Onboarding.NextRecommendedStep = nextStep

	// Update stage progresses
	if e.Store.Onboarding.OverallProgress >= 50 {
		e.Store.Onboarding.CurrentStageID = "stage-week-2"
		for i := range e.Store.Onboarding.Stages {
			if e.Store.Onboarding.Stages[i].ID == "stage-week-1" {
				e.Store.Onboarding.Stages[i].Status = "completed"
				e.Store.Onboarding.Stages[i].ProgressPercent = 100
			}
			if e.Store.Onboarding.Stages[i].ID == "stage-week-2" {
				e.Store.Onboarding.Stages[i].Status = "current"
				e.Store.Onboarding.Stages[i].ProgressPercent = 50
			}
		}
	}
}
