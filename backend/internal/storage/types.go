package storage

import (
	"context"
	"errors"
	"sort"

	"aibuddy/backend/internal/domain"
)

// ErrNotFound is returned when persisted state does not exist yet in the backing store.
var ErrNotFound = errors.New("storage: state not found")

// ClickstreamRecord represents a telemetry event persisted into long-term storage.
type ClickstreamRecord struct {
	ID        string            `json:"id"`
	EventType string            `json:"event_type"`
	ActorRole domain.Role       `json:"actor_role"`
	ActorID   string            `json:"actor_id,omitempty"`
	Route     string            `json:"route,omitempty"`
	LatencyMs float64           `json:"latency_ms,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// AppState is the canonical snapshot persisted by Repository implementations.
type AppState struct {
	ActiveRole        domain.Role                  `json:"active_role"`
	Users             map[domain.Role]domain.UserProfile `json:"users"`
	Onboarding        domain.OnboardingState       `json:"onboarding"`
	CorporateRequests []domain.CorporateRequest    `json:"corporate_requests"`
	Messages          []domain.ChatMessage         `json:"messages"`
	Metrics           domain.ImpactMetrics         `json:"metrics"`
	Articles          []domain.KnowledgeArticle    `json:"articles"`
}

// Repository abstracts state persistence across memory, Redis cache, and PostgreSQL 16.
type Repository interface {
	LoadState(ctx context.Context) (*AppState, error)
	SaveState(ctx context.Context, state *AppState) error
	AppendClickstream(ctx context.Context, record ClickstreamRecord) error
	Mode() string
	Healthy(ctx context.Context) bool
	Close() error
}

// RoleOrder defines the canonical display order for demo personas.
var RoleOrder = []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager}

// DefaultUsers returns the 3 synthetic Avito Start demo personas without real PII.
func DefaultUsers() map[domain.Role]domain.UserProfile {
	return map[domain.Role]domain.UserProfile{
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
			Permissions: []string{
				"knowledge:read_intern",
				"onboarding:checklist_self",
				"mcp:request_access_safe",
				"mcp:request_absence",
				"mcp:prepare_1on1",
			},
		},
		domain.RoleMentor: {
			ID:             "usr-mentor-01",
			Name:           "Мария Козлова",
			Role:           domain.RoleMentor,
			RoleTitle:      "Senior Go Developer / Buddy",
			Team:           "Авито Доставка",
			Department:     "Транспорт и Логистика",
			MentorName:     "Дмитрий Волков",
			ManagerName:    "Дмитрий Волков",
			AvatarInitials: "МК",
			Permissions: []string{
				"knowledge:read_mentor",
				"onboarding:view_mentee",
				"mcp:review_requests",
				"mcp:request_access_safe",
				"mcp:request_absence",
			},
		},
		domain.RoleManager: {
			ID:             "usr-manager-01",
			Name:           "Дмитрий Волков",
			Role:           domain.RoleManager,
			RoleTitle:      "Engineering Manager",
			Team:           "Авито Доставка",
			Department:     "Транспорт и Логистика",
			MentorName:     "",
			ManagerName:    "",
			AvatarInitials: "ДВ",
			Permissions: []string{
				"knowledge:read_all",
				"onboarding:admin",
				"mcp:review_requests",
				"metrics:view_all",
				"rbac:admin",
			},
		},
	}
}

// OrderedUsers converts a user map into a deterministically sorted slice.
func OrderedUsers(users map[domain.Role]domain.UserProfile) []domain.UserProfile {
	out := make([]domain.UserProfile, 0, len(users))
	seen := make(map[domain.Role]bool, len(users))
	for _, r := range RoleOrder {
		if u, ok := users[r]; ok {
			out = append(out, CloneUserProfile(u))
			seen[r] = true
		}
	}
	var extras []string
	for r := range users {
		if !seen[r] {
			extras = append(extras, string(r))
		}
	}
	sort.Strings(extras)
	for _, r := range extras {
		out = append(out, CloneUserProfile(users[domain.Role(r)]))
	}
	return out
}

// DefaultOnboardingState builds the initial Avito Start onboarding roadmap.
func DefaultOnboardingState() domain.OnboardingState {
	checklist := []domain.ChecklistItem{
		{
			ID:              "step-01-access",
			StageID:         "stage-week-1",
			Title:           "Получить базовые доступы (Git, VPN, Staging)",
			Description:     "Оформить доступы к репозиторию avito-delivery-core и тестовым сервисам через Avito Access/IDM.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryAccess,
			EstimatedTime:   "15 мин",
			SuggestedPrompt: "Какие доступы мне нужны для первой задачи в команде Авито Доставка? Помоги оформить безопасную заявку.",
		},
		{
			ID:              "step-02-env-setup",
			StageID:         "stage-week-1",
			Title:           "Настроить локальное Go-окружение",
			Description:     "Установить Go, Docker, клонировать сервис и поднять зависимости по внутреннему Playbook.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryTools,
			EstimatedTime:   "40 мин",
			SuggestedPrompt: "Как развернуть локальную среду разработки Go и запустить интеграционные тесты нашего сервиса?",
		},
		{
			ID:              "step-03-absence-policy",
			StageID:         "stage-week-2",
			Title:           "Изучить процесс отгулов и отпусков",
			Description:     "Разобраться, как стажёр оформляет учебный отгул или day-off в Avito Absence.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryProcesses,
			EstimatedTime:   "10 мин",
			SuggestedPrompt: "Как оформить отгул на экзамен в вузе на следующей неделе?",
		},
		{
			ID:              "step-04-first-pr",
			StageID:         "stage-week-2",
			Title:           "Подготовить первый Pull Request и 1:1 с наставником",
			Description:     "Разобрать первую таску, проверить критерии Definition of Done и сформулировать вопросы к Buddy.",
			Status:          domain.StatusTodo,
			Category:        domain.CategoryFirstTask,
			EstimatedTime:   "2 часа",
			SuggestedPrompt: "Помоги подготовиться к встрече 1:1 с наставником Марией по моей первой задаче в кэшировании Redis.",
		},
	}

	state := domain.OnboardingState{
		CurrentStageID: "stage-week-1",
		Stages: []domain.OnboardingStage{
			{
				ID:     "stage-week-1",
				Title:  "Неделя 1: Доступы и среда",
				Period: "Дни 1–5",
				Status: "current",
			},
			{
				ID:     "stage-week-2",
				Title:  "Неделя 2: Процессы и первая задача",
				Period: "Дни 6–10",
				Status: "upcoming",
			},
			{
				ID:     "stage-week-3",
				Title:  "Недели 3–4: Самостоятельные фичи",
				Period: "Дни 11–30",
				Status: "upcoming",
			},
		},
		Checklist: checklist,
	}
	RecalculateOnboarding(&state)
	return state
}

// DefaultMetrics returns baseline hackathon impact metrics.
func DefaultMetrics() domain.ImpactMetrics {
	return domain.ImpactMetrics{
		MentorHoursBaseline:      14.0,
		MentorHoursSaved:         0,
		RoutineQuestionsResolved: 0,
		MCPActionsExecuted:       0,
		RBACViolationsPrevented:  0,
		AvgLocalLatencyMs:        12,
	}
}

// DefaultAppState constructs a complete fresh workspace state.
func DefaultAppState() *AppState {
	return &AppState{
		ActiveRole:        domain.RoleIntern,
		Users:             DefaultUsers(),
		Onboarding:        DefaultOnboardingState(),
		CorporateRequests: make([]domain.CorporateRequest, 0),
		Messages:          make([]domain.ChatMessage, 0),
		Metrics:           DefaultMetrics(),
		Articles:          make([]domain.KnowledgeArticle, 0),
	}
}

// RecalculateOnboarding recomputes stage progress percentages, stage lifecycle states,
// overall progress, and next recommended step deterministically from checklist items.
func RecalculateOnboarding(state *domain.OnboardingState) {
	if state == nil {
		return
	}

	total := len(state.Checklist)
	doneCount := 0
	var nextStep *domain.ChecklistItem

	stageTotal := make(map[string]int, len(state.Stages))
	stageDone := make(map[string]int, len(state.Stages))

	for i := range state.Checklist {
		item := state.Checklist[i]
		stageTotal[item.StageID]++
		if item.Status == domain.StatusDone {
			doneCount++
			stageDone[item.StageID]++
		} else if nextStep == nil {
			cp := item
			nextStep = &cp
		}
	}

	if total > 0 {
		state.OverallProgress = (doneCount * 100) / total
	} else {
		state.OverallProgress = 0
	}
	state.NextRecommendedStep = nextStep

	assignedCurrent := false
	for i := range state.Stages {
		st := &state.Stages[i]
		stTotal := stageTotal[st.ID]
		stDone := stageDone[st.ID]

		if stTotal > 0 {
			st.ProgressPercent = (stDone * 100) / stTotal
			if stDone == stTotal {
				st.Status = "completed"
				continue
			}
		} else {
			if doneCount == total && total > 0 {
				st.ProgressPercent = 100
				st.Status = "completed"
				continue
			}
			st.ProgressPercent = 0
		}

		if !assignedCurrent {
			st.Status = "current"
			state.CurrentStageID = st.ID
			assignedCurrent = true
		} else {
			st.Status = "upcoming"
		}
	}

	if !assignedCurrent && len(state.Stages) > 0 {
		state.CurrentStageID = state.Stages[len(state.Stages)-1].ID
	}
}

// Clone creates a deep copy of AppState safe for concurrent mutations across goroutines.
func (s *AppState) Clone() *AppState {
	if s == nil {
		return nil
	}
	out := &AppState{
		ActiveRole:        s.ActiveRole,
		Users:             make(map[domain.Role]domain.UserProfile, len(s.Users)),
		Onboarding:        CloneOnboardingState(s.Onboarding),
		CorporateRequests: make([]domain.CorporateRequest, len(s.CorporateRequests)),
		Messages:          make([]domain.ChatMessage, len(s.Messages)),
		Metrics:           s.Metrics,
		Articles:          make([]domain.KnowledgeArticle, len(s.Articles)),
	}
	for k, u := range s.Users {
		out.Users[k] = CloneUserProfile(u)
	}
	for i, req := range s.CorporateRequests {
		out.CorporateRequests[i] = CloneCorporateRequest(req)
	}
	for i, msg := range s.Messages {
		out.Messages[i] = CloneChatMessage(msg)
	}
	for i, art := range s.Articles {
		out.Articles[i] = CloneKnowledgeArticle(art)
	}
	return out
}

// CloneUserProfile deep-copies a UserProfile slice fields.
func CloneUserProfile(u domain.UserProfile) domain.UserProfile {
	if u.Permissions != nil {
		perms := make([]string, len(u.Permissions))
		copy(perms, u.Permissions)
		u.Permissions = perms
	}
	return u
}

// CloneOnboardingState deep-copies OnboardingState and all child slices/pointers.
func CloneOnboardingState(in domain.OnboardingState) domain.OnboardingState {
	out := in
	if in.NextRecommendedStep != nil {
		cp := *in.NextRecommendedStep
		out.NextRecommendedStep = &cp
	}
	if in.Stages != nil {
		out.Stages = make([]domain.OnboardingStage, len(in.Stages))
		copy(out.Stages, in.Stages)
	}
	if in.Checklist != nil {
		out.Checklist = make([]domain.ChecklistItem, len(in.Checklist))
		copy(out.Checklist, in.Checklist)
	}
	return out
}

// CloneCorporateRequest deep-copies a CorporateRequest including its Details map.
func CloneCorporateRequest(in domain.CorporateRequest) domain.CorporateRequest {
	out := in
	out.Details = cloneDetailsMap(in.Details)
	return out
}

// CloneChatMessage deep-copies a ChatMessage including citations, steps, and ActionCard.
func CloneChatMessage(in domain.ChatMessage) domain.ChatMessage {
	out := in
	if in.ProcessingSteps != nil {
		out.ProcessingSteps = make([]string, len(in.ProcessingSteps))
		copy(out.ProcessingSteps, in.ProcessingSteps)
	}
	if in.Citations != nil {
		out.Citations = make([]domain.KnowledgeCitation, len(in.Citations))
		copy(out.Citations, in.Citations)
	}
	if in.ActionCard != nil {
		cp := CloneCorporateRequest(*in.ActionCard)
		out.ActionCard = &cp
	}
	return out
}

// CloneKnowledgeArticle deep-copies a KnowledgeArticle and its tags.
func CloneKnowledgeArticle(in domain.KnowledgeArticle) domain.KnowledgeArticle {
	out := in
	if in.Tags != nil {
		out.Tags = make([]string, len(in.Tags))
		copy(out.Tags, in.Tags)
	}
	return out
}

func cloneDetailsMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v interface{}) interface{} {
	switch typed := v.(type) {
	case map[string]interface{}:
		return cloneDetailsMap(typed)
	case []string:
		cp := make([]string, len(typed))
		copy(cp, typed)
		return cp
	case []interface{}:
		cp := make([]interface{}, len(typed))
		for i, item := range typed {
			cp[i] = cloneAny(item)
		}
		return cp
	default:
		return typed
	}
}
