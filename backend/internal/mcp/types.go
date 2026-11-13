package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"aibuddy/backend/internal/domain"
)

// Sentinel errors mapped by HTTP handlers to status codes cleanly.
var (
	ErrForbidden       = errors.New("mcp: forbidden by RBAC policy")
	ErrNotFound        = errors.New("mcp: entity not found")
	ErrInvalidDecision = errors.New("mcp: decision must be approved or rejected")
	ErrInvalidStatus   = errors.New("mcp: invalid checklist status")
	ErrUnknownTool     = errors.New("mcp: unknown tool")
	ErrMissingArgument = errors.New("mcp: missing required argument")
)

// ToolDefinition mirrors an entry in specs/mcp-tools.json.
type ToolDefinition struct {
	Name                 string                 `json:"name"`
	Service              string                 `json:"service"`
	Description          string                 `json:"description"`
	RequiredRoles        []domain.Role          `json:"required_roles"`
	RequiresConfirmation bool                   `json:"requires_confirmation"`
	ParametersSchema     map[string]interface{} `json:"parameters"`
}

// OpenAITool is the OpenRouter/OpenAI function-calling schema envelope.
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction describes an executable tool signature for LLM function calling.
type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolExecutionResult captures the outcome of running an MCP tool call.
type ToolExecutionResult struct {
	ToolName       string                     `json:"tool_name"`
	Service        string                     `json:"service"`
	Allowed        bool                       `json:"allowed"`
	BlockedReason  string                     `json:"blocked_reason,omitempty"`
	OutputJSON     string                     `json:"output_json"`
	Citations      []domain.KnowledgeCitation `json:"citations,omitempty"`
	ActionCard     *domain.CorporateRequest   `json:"action_card,omitempty"`
	ProcessingStep string                     `json:"processing_step"`
}

// MCPExecutor defines the corporate action execution contract (AGENTS.md §5.3).
type MCPExecutor interface {
	DefinitionsForRole(role domain.Role) []ToolDefinition
	OpenAITools() []OpenAITool
	ExecuteTool(ctx context.Context, user domain.UserProfile, toolName string, rawArgs json.RawMessage) (*ToolExecutionResult, error)
	RequestServiceAccess(user domain.UserProfile, resource string, level string, reason string) (*domain.CorporateRequest, error)
	CreateAbsenceRequest(user domain.UserProfile, absenceType string, startDate string, endDate string, reason string, confirmed bool) (*domain.CorporateRequest, error)
	PrepareMentor1on1Summary(user domain.UserProfile, topic string, whatChecked []string, questions []string) (*domain.CorporateRequest, error)
	ConfirmAction(requestID string) (*domain.CorporateRequest, error)
	ReviewRequest(role domain.Role, requestID string, decision string, comment string) (*domain.CorporateRequest, error)
	UpdateChecklistItem(itemID string, status domain.ChecklistItemStatus) (*domain.OnboardingState, error)
}
