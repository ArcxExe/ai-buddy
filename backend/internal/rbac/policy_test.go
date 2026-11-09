package rbac_test

import (
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rbac"
	"testing"
)

func TestCanAccessKnowledge(t *testing.T) {
	tests := []struct {
		role        domain.Role
		sensitivity string
		expected    bool
	}{
		{domain.RoleIntern, "public", true},
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
	}

	for _, tt := range tests {
		got := rbac.CanAccessKnowledge(tt.role, tt.sensitivity)
		if got != tt.expected {
			t.Errorf("CanAccessKnowledge(%s, %s) = %v; want %v", tt.role, tt.sensitivity, got, tt.expected)
		}
	}
}

func TestCanExecuteMCPTool(t *testing.T) {
	// Intern allowed standard tools
	allowed, reason := rbac.CanExecuteMCPTool(domain.RoleIntern, "request_service_access", "repo:avito-delivery-core")
	if !allowed || reason != "" {
		t.Fatalf("expected allowed access to repo, got allowed=%v, reason=%s", allowed, reason)
	}

	// Intern blocked from privileged resource
	allowed, reason = rbac.CanExecuteMCPTool(domain.RoleIntern, "request_service_access", "prod-billing-db")
	if allowed || reason == "" {
		t.Fatalf("expected blocked access to prod-billing-db for intern, got allowed=%v", allowed)
	}

	// Intern blocked from reviewing corporate requests
	allowed, reason = rbac.CanExecuteMCPTool(domain.RoleIntern, "review_corporate_request", "")
	if allowed || reason == "" {
		t.Fatalf("expected blocked review_corporate_request for intern, got allowed=%v", allowed)
	}

	// Mentor allowed to review
	allowed, reason = rbac.CanExecuteMCPTool(domain.RoleMentor, "review_corporate_request", "")
	if !allowed || reason != "" {
		t.Fatalf("expected allowed review_corporate_request for mentor, got allowed=%v, reason=%s", allowed, reason)
	}

	// Manager allowed to review
	allowed, reason = rbac.CanExecuteMCPTool(domain.RoleManager, "review_corporate_request", "")
	if !allowed || reason != "" {
		t.Fatalf("expected allowed review_corporate_request for manager, got allowed=%v, reason=%s", allowed, reason)
	}

	// Unknown tool rejected
	allowed, reason = rbac.CanExecuteMCPTool(domain.RoleManager, "non_existent_tool", "")
	if allowed {
		t.Fatalf("expected rejection for unknown tool")
	}
}

func TestCanReviewRequest(t *testing.T) {
	if rbac.CanReviewRequest(domain.RoleIntern) {
		t.Errorf("intern should not be able to review requests")
	}
	if !rbac.CanReviewRequest(domain.RoleMentor) {
		t.Errorf("mentor should be able to review requests")
	}
	if !rbac.CanReviewRequest(domain.RoleManager) {
		t.Errorf("manager should be able to review requests")
	}
}
