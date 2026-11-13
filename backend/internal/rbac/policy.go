package rbac

import (
	"fmt"
	"strings"

	"aibuddy/backend/internal/domain"
)

// RestrictedResources lists system resources forbidden for interns and requiring elevation.
var RestrictedResources = map[string]bool{
	"prod-billing-db":           true,
	"prod-db-admin":             true,
	"prod-users-pii":            true,
	"k8s-cluster-admin":         true,
	"hr-salary-grades":          true,
	"mentor-private-evaluation": true,
}

// deniedResourcesByRole encodes hard RBAC denial boundaries aligned with specs/agent-sdd.yaml.
var deniedResourcesByRole = map[domain.Role]map[string]bool{
	domain.RoleIntern: {
		"prod-billing-db":           true,
		"prod-db-admin":             true,
		"prod-users-pii":            true,
		"k8s-cluster-admin":         true,
		"hr-salary-grades":          true,
		"mentor-private-evaluation": true,
	},
	domain.RoleMentor: {
		"prod-billing-db":  true,
		"prod-db-admin":    true,
		"k8s-cluster-admin": true,
		"hr-salary-grades": true,
	},
	domain.RoleManager: {},
}

// toolRolePolicy defines which roles may invoke each MCP tool from specs/mcp-tools.json.
var toolRolePolicy = map[string]map[domain.Role]bool{
	"search_knowledge_base":      {domain.RoleIntern: true, domain.RoleMentor: true, domain.RoleManager: true},
	"get_onboarding_status":      {domain.RoleIntern: true, domain.RoleMentor: true, domain.RoleManager: true},
	"complete_checklist_item":    {domain.RoleIntern: true, domain.RoleMentor: true, domain.RoleManager: true},
	"request_service_access":     {domain.RoleIntern: true, domain.RoleMentor: true, domain.RoleManager: true},
	"create_absence_request":     {domain.RoleIntern: true, domain.RoleMentor: true, domain.RoleManager: true},
	"prepare_mentor_1on1_summary": {domain.RoleIntern: true, domain.RoleMentor: true, domain.RoleManager: true},
	"review_corporate_request":   {domain.RoleMentor: true, domain.RoleManager: true},
}

type guardrailRule struct {
	keywords       []string
	targetResource string
	blockedRoles   map[domain.Role]bool
	reason         string
}

var promptGuardrails = []guardrailRule{
	{
		keywords:       []string{"prod-billing-db", "prod-db-admin", "production admin", "prod db", "прод-бд", "продакшн бд", "billing-db"},
		targetResource: "prod-billing-db",
		blockedRoles:   map[domain.Role]bool{domain.RoleIntern: true, domain.RoleMentor: true},
		reason:         "Доступ к production-БД биллинга и административным правам запрещён политикой ИБ без отдельного SEC-тикетирования и SLA выше стажёрского/buddy контура.",
	},
	{
		keywords:       []string{"prod-users-pii", "чужие персональные данные", "персональные данные пользователей", "pii"},
		targetResource: "prod-users-pii",
		blockedRoles:   map[domain.Role]bool{domain.RoleIntern: true, domain.RoleMentor: true},
		reason:         "Доступ к персональным данным пользователей (PII) в production запрещён политикой безопасности Авито.",
	},
	{
		keywords:       []string{"k8s-cluster-admin", "cluster-admin"},
		targetResource: "k8s-cluster-admin",
		blockedRoles:   map[domain.Role]bool{domain.RoleIntern: true, domain.RoleMentor: true},
		reason:         "Привилегия k8s-cluster-admin недоступна по RBAC; для деплоя используйте team-namespace в Staging.",
	},
	{
		keywords:       []string{"hr-salary-grades", "зарплат", "оклад", "компенсаци", "salary", "ведомост"},
		targetResource: "hr-salary-grades",
		blockedRoles:   map[domain.Role]bool{domain.RoleIntern: true, domain.RoleMentor: true},
		reason:         "Зарплатные грейды и компенсационные ведомости относятся к уровню manager_only.",
	},
	{
		keywords:       []string{"mentor-private-evaluation", "закрытая оценка", "приватн", "отзыв ментора о"},
		targetResource: "mentor-private-evaluation",
		blockedRoles:   map[domain.Role]bool{domain.RoleIntern: true},
		reason:         "Приватные HR/buddy-оценки закрыты для роли стажёра.",
	},
}

// CanAccessKnowledge checks if role is allowed to view an article by sensitivity level.
func CanAccessKnowledge(role domain.Role, sensitivity string) bool {
	switch strings.ToLower(strings.TrimSpace(sensitivity)) {
	case "", "public":
		return true
	case "intern":
		return role == domain.RoleIntern || role == domain.RoleMentor || role == domain.RoleManager
	case "mentor_only":
		return role == domain.RoleMentor || role == domain.RoleManager
	case "manager_only":
		return role == domain.RoleManager
	default:
		return false
	}
}

// IsResourceDenied reports whether targetResource is blocked for role under RBAC policy.
func IsResourceDenied(role domain.Role, targetResource string) (bool, string) {
	norm := strings.ToLower(strings.TrimSpace(targetResource))
	if norm == "" {
		return false, ""
	}

	if denied := deniedResourcesByRole[role]; denied[norm] {
		return true, fmt.Sprintf("RBAC policy denied access for role %q to restricted resource %q", role, targetResource)
	}

	if role == domain.RoleIntern {
		for _, kw := range []string{"prod-", "production", "billing", "salary", "pii", "cluster-admin", "зарплат", "оклад"} {
			if strings.Contains(norm, kw) {
				return true, fmt.Sprintf("RBAC policy denied intern access to sensitive resource %q", targetResource)
			}
		}
	}
	return false, ""
}

// CanExecuteMCPTool checks tool permission and validates target resource against RBAC guardrails.
func CanExecuteMCPTool(role domain.Role, toolName string, targetResource string) (bool, string) {
	allowedRoles, exists := toolRolePolicy[strings.TrimSpace(toolName)]
	if !exists {
		return false, fmt.Sprintf("unknown MCP tool %q", toolName)
	}
	if !allowedRoles[role] {
		return false, fmt.Sprintf("role %q is not permitted to invoke MCP tool %q", role, toolName)
	}

	if denied, reason := IsResourceDenied(role, targetResource); denied {
		return false, reason
	}
	return true, ""
}

// CanReviewRequest verifies if user can approve/reject corporate requests.
func CanReviewRequest(role domain.Role) bool {
	return role == domain.RoleMentor || role == domain.RoleManager
}

// CheckPromptGuardrail evaluates incoming user text against SDD RBAC triggers before calling external LLMs.
func CheckPromptGuardrail(role domain.Role, prompt string) (blocked bool, resource string, reason string) {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if lower == "" {
		return false, "", ""
	}

	for _, rule := range promptGuardrails {
		if !rule.blockedRoles[role] {
			continue
		}
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				return true, rule.targetResource, rule.reason
			}
		}
	}
	return false, "", ""
}
