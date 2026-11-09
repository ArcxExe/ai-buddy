package rbac

import (
	"aibuddy/backend/internal/domain"
	"strings"
)

// RestrictedResources lists system resources forbidden for interns and requiring elevation
var RestrictedResources = map[string]bool{
	"prod-billing-db":           true,
	"prod-users-pii":            true,
	"k8s-cluster-admin":         true,
	"hr-salary-grades":          true,
	"mentor-private-evaluation": true,
}

// CanAccessKnowledge checks if role is allowed to view article by sensitivity level
func CanAccessKnowledge(role domain.Role, sensitivity string) bool {
	switch role {
	case domain.RoleIntern:
		return sensitivity == "public" || sensitivity == "intern"
	case domain.RoleMentor:
		return sensitivity == "public" || sensitivity == "intern" || sensitivity == "mentor_only"
	case domain.RoleManager:
		return true
	default:
		return sensitivity == "public"
	}
}

// CanExecuteMCPTool checks tool permission and validates target resource against RBAC guardrails
func CanExecuteMCPTool(role domain.Role, toolName string, targetResource string) (bool, string) {
	// 1. Tool-level permissions
	switch toolName {
	case "review_corporate_request":
		if role != domain.RoleMentor && role != domain.RoleManager {
			return false, "Согласование корпоративных заявок доступно только наставнику (mentor) или руководителю (manager)."
		}
	case "search_knowledge_base", "get_onboarding_status", "complete_checklist_item",
		"request_service_access", "create_absence_request", "prepare_mentor_1on1_summary":
		// Allowed for all roles
	default:
		return false, "Неизвестный MCP-инструмент: " + toolName
	}

	// 2. Resource-level guardrails
	if targetResource != "" {
		resLower := strings.ToLower(targetResource)
		for restricted := range RestrictedResources {
			if strings.Contains(resLower, restricted) {
				if role == domain.RoleIntern {
					return false, "Доступ к привилегированному ресурсу '" + restricted + "' заблокирован политикой RBAC для роли стажёра."
				}
			}
		}
	}

	return true, ""
}

// CanReviewRequest verifies if user can approve/reject corporate requests
func CanReviewRequest(role domain.Role) bool {
	return role == domain.RoleMentor || role == domain.RoleManager
}
