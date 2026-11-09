package domain

// Role represents system user roles in Avito
type Role string

const (
	RoleIntern  Role = "intern"
	RoleMentor  Role = "mentor"
	RoleManager Role = "manager"
)

// UserProfile represents synthetic Avito employee
type UserProfile struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Role           Role     `json:"role"`
	RoleTitle      string   `json:"role_title"`
	Team           string   `json:"team"`
	Department     string   `json:"department"`
	MentorName     string   `json:"mentor_name"`
	ManagerName    string   `json:"manager_name"`
	AvatarInitials string   `json:"avatar_initials"`
	Permissions    []string `json:"permissions"`
}

// ChecklistItemCategory groups onboarding tasks
type ChecklistItemCategory string

const (
	CategoryAccess    ChecklistItemCategory = "access"
	CategoryTools     ChecklistItemCategory = "tools"
	CategoryProcesses ChecklistItemCategory = "processes"
	CategoryFirstTask ChecklistItemCategory = "first_task"
)

// ChecklistItemStatus represents progress of a task
type ChecklistItemStatus string

const (
	StatusTodo       ChecklistItemStatus = "todo"
	StatusInProgress ChecklistItemStatus = "in_progress"
	StatusDone       ChecklistItemStatus = "done"
)

// ChecklistItem represents single onboarding step
type ChecklistItem struct {
	ID              string                `json:"id"`
	StageID         string                `json:"stage_id"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Status          ChecklistItemStatus   `json:"status"`
	Category        ChecklistItemCategory `json:"category"`
	EstimatedTime   string                `json:"estimated_time"`
	SuggestedPrompt string                `json:"suggested_prompt"`
}

// OnboardingStage represents lifecycle stage
type OnboardingStage struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Period          string `json:"period"`
	Status          string `json:"status"` // "completed", "current", "upcoming"
	ProgressPercent int    `json:"progress_percent"`
}

// OnboardingState is full roadmap snapshot
type OnboardingState struct {
	OverallProgress     int               `json:"overall_progress"`
	CurrentStageID      string            `json:"current_stage_id"`
	NextRecommendedStep *ChecklistItem    `json:"next_recommended_step,omitempty"`
	Stages              []OnboardingStage `json:"stages"`
	Checklist           []ChecklistItem   `json:"checklist"`
}

// RequestType represents corporate action kind
type RequestType string

const (
	TypeAccessRequest      RequestType = "access_request"
	TypeAbsenceRequest     RequestType = "absence_request"
	TypeMentor1on1Summary  RequestType = "mentor_1on1_summary"
	TypeRBACBlocked        RequestType = "rbac_blocked"
)

// RequestStatus represents status in corporate system
type RequestStatus string

const (
	StatusDraftConfirmationRequired RequestStatus = "draft_confirmation_required"
	StatusPendingApproval           RequestStatus = "pending_approval"
	StatusApproved                  RequestStatus = "approved"
	StatusCompleted                 RequestStatus = "completed"
	StatusBlockedByRBAC             RequestStatus = "blocked_by_rbac"
	StatusRejected                  RequestStatus = "rejected"
)

// CorporateRequest represents an action or request in Avito systems (Absence, IDM, MentorSync)
type CorporateRequest struct {
	ID              string                 `json:"id"`
	ServiceName     string                 `json:"service_name"`
	ToolName        string                 `json:"tool_name"`
	RequestType     RequestType            `json:"request_type"`
	Title           string                 `json:"title"`
	Summary         string                 `json:"summary"`
	Status          RequestStatus          `json:"status"`
	CreatedByRole   Role                   `json:"created_by_role"`
	TargetResource  string                 `json:"target_resource"`
	ReviewerComment string                 `json:"reviewer_comment,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	Details         map[string]interface{} `json:"details"`
}

// KnowledgeCitation represents source reference
type KnowledgeCitation struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Source      string `json:"source"`
	Sensitivity string `json:"sensitivity"`
}

// ChatMessage represents conversation item
type ChatMessage struct {
	ID              string              `json:"id"`
	Sender          string              `json:"sender"` // "user" or "assistant"
	RoleContext     Role                `json:"role_context"`
	Content         string              `json:"content"`
	CreatedAt       string              `json:"created_at"`
	ProcessingSteps []string            `json:"processing_steps,omitempty"`
	Citations       []KnowledgeCitation `json:"citations,omitempty"`
	ActionCard      *CorporateRequest   `json:"action_card,omitempty"`
	EngineUsed      string              `json:"engine_used,omitempty"`
}

// ImpactMetrics represents real-time analytics
type ImpactMetrics struct {
	MentorHoursBaseline      float64 `json:"mentor_hours_baseline"`
	MentorHoursSaved         float64 `json:"mentor_hours_saved"`
	RoutineQuestionsResolved int     `json:"routine_questions_resolved"`
	MCPActionsExecuted       int     `json:"mcp_actions_executed"`
	RBACViolationsPrevented  int     `json:"rbac_violations_prevented"`
	AvgLocalLatencyMs        int     `json:"avg_local_latency_ms"`
}

// KnowledgeArticle represents seeded and indexed article
type KnowledgeArticle struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Sensitivity string   `json:"sensitivity"` // "public", "intern", "mentor_only", "manager_only"
	Source      string   `json:"source"`
	Summary     string   `json:"summary"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
}
