package mcp_test

import (
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"testing"
)

func sampleUser(role domain.Role) domain.UserProfile {
	return domain.UserProfile{
		ID:          "usr-01",
		Name:        "Алексей Смирнов",
		Role:        role,
		RoleTitle:   "Backend Go Intern",
		Team:        "Авито Доставка",
		Department:  "Транспорт и Логистика",
		MentorName:  "Мария Козлова",
		ManagerName: "Дмитрий Волков",
	}
}

func TestRequestServiceAccessAllowedAndBlocked(t *testing.T) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)
	intern := sampleUser(domain.RoleIntern)

	// 1. Allowed request
	req, err := exec.RequestServiceAccess(intern, "repo:avito-delivery-core", "read_write", "Нужно для задачи")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Status != domain.StatusApproved {
		t.Errorf("expected approved status for repo access, got %s", req.Status)
	}
	if store.Metrics.MCPActionsExecuted != 1 {
		t.Errorf("expected 1 action executed, got %d", store.Metrics.MCPActionsExecuted)
	}

	// 2. Blocked request (RBAC)
	reqBlocked, err := exec.RequestServiceAccess(intern, "prod-billing-db", "admin", "Хочу посмотреть")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqBlocked.Status != domain.StatusBlockedByRBAC {
		t.Errorf("expected blocked status for prod-billing-db, got %s", reqBlocked.Status)
	}
	if store.Metrics.RBACViolationsPrevented != 1 {
		t.Errorf("expected 1 violation prevented, got %d", store.Metrics.RBACViolationsPrevented)
	}
}

func TestAbsenceRequestFlow(t *testing.T) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)
	intern := sampleUser(domain.RoleIntern)

	// Draft
	draft, err := exec.CreateAbsenceRequest(intern, "study_day_off", "2026-11-18", "2026-11-18", "Сдача зачёта", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft.Status != domain.StatusDraftConfirmationRequired {
		t.Errorf("expected draft status, got %s", draft.Status)
	}

	// Confirm
	confirmed, err := exec.ConfirmAction(draft.ID)
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if confirmed.Status != domain.StatusPendingApproval {
		t.Errorf("expected pending_approval, got %s", confirmed.Status)
	}

	// Non-existent confirm
	_, err = exec.ConfirmAction("unknown-id")
	if err == nil {
		t.Errorf("expected error confirming unknown id")
	}
}

func TestPrepareMentor1on1Summary(t *testing.T) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)
	intern := sampleUser(domain.RoleIntern)

	req, err := exec.PrepareMentor1on1Summary(intern, "Кэширование", []string{"Playbook"}, []string{"Какой TTL выбрать?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Status != domain.StatusCompleted {
		t.Errorf("expected completed status, got %s", req.Status)
	}
}

func TestReviewRequest(t *testing.T) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)
	intern := sampleUser(domain.RoleIntern)

	draft, _ := exec.CreateAbsenceRequest(intern, "study_day_off", "2026-11-18", "2026-11-18", "Экзамен", true)

	// Intern cannot review
	_, err := exec.ReviewRequest(domain.RoleIntern, draft.ID, "approved", "ok")
	if err == nil {
		t.Errorf("intern must not be allowed to review request")
	}

	// Mentor can approve
	approved, err := exec.ReviewRequest(domain.RoleMentor, draft.ID, "approved", "Отлично, удачи на экзамене")
	if err != nil {
		t.Fatalf("mentor review failed: %v", err)
	}
	if approved.Status != domain.StatusApproved {
		t.Errorf("expected approved, got %s", approved.Status)
	}

	// Manager can reject
	draft2, _ := exec.CreateAbsenceRequest(intern, "personal_day_off", "2026-11-20", "2026-11-20", "Личное", true)
	rejected, err := exec.ReviewRequest(domain.RoleManager, draft2.ID, "rejected", "В этот день общий демо-день")
	if err != nil {
		t.Fatalf("manager review failed: %v", err)
	}
	if rejected.Status != domain.StatusRejected {
		t.Errorf("expected rejected, got %s", rejected.Status)
	}

	// Review non-existent
	_, err = exec.ReviewRequest(domain.RoleManager, "non-existent", "approved", "")
	if err == nil {
		t.Errorf("expected error on non-existent request")
	}
}

func TestUpdateChecklistItem(t *testing.T) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)

	state, err := exec.UpdateChecklistItem("step-02-env-setup", domain.StatusDone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatalf("expected state")
	}

	// Not found item
	_, err = exec.UpdateChecklistItem("non-existent-step", domain.StatusDone)
	if err == nil {
		t.Errorf("expected error on non-existent item")
	}
}
