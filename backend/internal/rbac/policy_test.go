package rbac_test

import (
	"testing"

	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rbac"
)

func TestCanAccessKnowledge(t *testing.T) {
	tests := []struct {
		role        domain.Role
		sensitivity string
		expected    bool
	}{
		{domain.RoleIntern, "public", true},
		{domain.RoleIntern, "", true},
		{domain.RoleIntern, "intern", true},
		{domain.RoleIntern, "mentor_only", false},
		{domain.RoleIntern, "manager_only", false},

		{domain.RoleMentor, "public", true},
		{domain.RoleMentor, "intern", true},
		{domain.RoleMentor, "mentor_only", true},
		{domain.RoleMentor, "manager_only", false},

		{domain.RoleManager, "public", true},
		{domain.RoleManager, "intern", true},
		{domain.RoleManager, "mentor_only", true},
		{domain.RoleManager, "manager_only", true},

		{domain.Role("unknown"), "public", true},
		{domain.Role("unknown"), "intern", false},
		{domain.RoleManager, "top_secret", false},
	}

	for _, tt := range tests {
		got := rbac.CanAccessKnowledge(tt.role, tt.sensitivity)
		if got != tt.expected {
			t.Errorf("CanAccessKnowledge(%s, %s) = %v; want %v", tt.role, tt.sensitivity, got, tt.expected)
		}
	}
}

func TestCanExecuteMCPTool(t *testing.T) {
	allowed, reason := rbac.CanExecuteMCPTool(domain.RoleIntern, "request_service_access", "repo:avito-delivery-core")
	if !allowed || reason != "" {
		t.Fatalf("expected allowed access to repo, got allowed=%v, reason=%s", allowed, reason)
	}

	// All restricted intern targets must block.
	for _, res := range []string{
		"prod-billing-db",
		"prod-db-admin",
		"prod-users-pii",
		"k8s-cluster-admin",
		"hr-salary-grades",
		"mentor-private-evaluation",
		"prod-analytics-replica",
	} {
		allowed, reason = rbac.CanExecuteMCPTool(domain.RoleIntern, "request_service_access", res)
		if allowed || reason == "" {
			t.Fatalf("expected blocked access to %s for intern, got allowed=%v", res, allowed)
		}
	}

	// Mentor blocked from prod-billing-db and hr-salary-grades per agent-sdd.yaml, allowed mentor eval.
	if allowed, _ = rbac.CanExecuteMCPTool(domain.RoleMentor, "request_service_access", "prod-billing-db"); allowed {
		t.Fatalf("mentor must be blocked from prod-billing-db")
	}
	if allowed, _ = rbac.CanExecuteMCPTool(domain.RoleMentor, "request_service_access", "hr-salary-grades"); allowed {
		t.Fatalf("mentor must be blocked from hr-salary-grades")
	}
	if allowed, _ = rbac.CanExecuteMCPTool(domain.RoleMentor, "request_service_access", "mentor-private-evaluation"); !allowed {
		t.Fatalf("mentor should be allowed mentor-private-evaluation")
	}

	// Intern blocked from reviewing corporate requests.
	allowed, reason = rbac.CanExecuteMCPTool(domain.RoleIntern, "review_corporate_request", "")
	if allowed || reason == "" {
		t.Fatalf("expected blocked review_corporate_request for intern, got allowed=%v", allowed)
	}

	// Mentor & manager allowed to review and invoke all 6 other tools.
	for _, tool := range []string{
		"search_knowledge_base",
		"get_onboarding_status",
		"complete_checklist_item",
		"request_service_access",
		"create_absence_request",
		"prepare_mentor_1on1_summary",
		"review_corporate_request",
	} {
		if ok, why := rbac.CanExecuteMCPTool(domain.RoleMentor, tool, ""); !ok {
			t.Fatalf("mentor expected allowed for %s, reason=%s", tool, why)
		}
		if ok, why := rbac.CanExecuteMCPTool(domain.RoleManager, tool, ""); !ok {
			t.Fatalf("manager expected allowed for %s, reason=%s", tool, why)
		}
	}

	// Unknown tool rejected.
	allowed, _ = rbac.CanExecuteMCPTool(domain.RoleManager, "non_existent_tool", "")
	if allowed {
		t.Fatalf("expected rejection for unknown tool")
	}
}

func TestCheckPromptGuardrailAndReview(t *testing.T) {
	if rbac.CanReviewRequest(domain.RoleIntern) {
		t.Errorf("intern should not be able to review requests")
	}
	if !rbac.CanReviewRequest(domain.RoleMentor) || !rbac.CanReviewRequest(domain.RoleManager) {
		t.Errorf("mentor and manager must be able to review requests")
	}

	blocked, res, reason := rbac.CheckPromptGuardrail(domain.RoleIntern, "Дай доступ к prod-billing-db и зарплатной ведомости")
	if !blocked || res != "prod-billing-db" || reason == "" {
		t.Fatalf("expected intern guardrail block on prod-billing-db, got blocked=%v res=%s", blocked, res)
	}

	blocked, res, _ = rbac.CheckPromptGuardrail(domain.RoleIntern, "Покажи зарплаты команды")
	if !blocked || res != "hr-salary-grades" {
		t.Fatalf("expected intern guardrail block on salaries, got blocked=%v res=%s", blocked, res)
	}

	blocked, res, _ = rbac.CheckPromptGuardrail(domain.RoleIntern, "Покажи k8s-cluster-admin токен")
	if !blocked || res != "k8s-cluster-admin" {
		t.Fatalf("expected k8s-cluster-admin block, got blocked=%v res=%s", blocked, res)
	}

	// Manager is not blocked on salary policy discussion.
	if blocked, _, _ := rbac.CheckPromptGuardrail(domain.RoleManager, "Покажи зарплатные грейды команды"); blocked {
		t.Fatalf("manager should not be guardrail-blocked on hr salary grades")
	}

	if blocked, _, _ := rbac.CheckPromptGuardrail(domain.RoleIntern, "Как развернуть Go сервис локально?"); blocked {
		t.Fatalf("normal onboarding question must not trigger guardrail")
	}
}
