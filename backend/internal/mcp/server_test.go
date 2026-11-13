package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/mcp"
	"aibuddy/backend/internal/rag"
	"aibuddy/backend/internal/telemetry"
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

	// 2. Admin request waits for manager approval
	reqAdmin, err := exec.RequestServiceAccess(intern, "grafana:delivery-dashboard", "admin", "Алерты")
	if err != nil || reqAdmin.Status != domain.StatusPendingApproval {
		t.Fatalf("expected pending_approval for admin request, got %+v err=%v", reqAdmin, err)
	}

	// 3. Blocked request (RBAC)
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

func TestAbsenceRequestFlowAndActionCardSync(t *testing.T) {
	store := mcp.NewStore()
	tr := telemetry.NewTracker(true)
	exec := mcp.NewExecutorWithDeps(store, nil, tr)
	intern := sampleUser(domain.RoleIntern)

	draft, err := exec.CreateAbsenceRequest(intern, "study_day_off", "2026-11-18", "2026-11-18", "Сдача зачёта", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft.Status != domain.StatusDraftConfirmationRequired {
		t.Errorf("expected draft status, got %s", draft.Status)
	}

	// Save a chat message containing the action card and confirm sync works cleanly.
	store.AppendMessages(domain.ChatMessage{
		ID:         "msg-draft",
		Sender:     "assistant",
		ActionCard: draft,
	})

	confirmed, err := exec.ConfirmAction(draft.ID)
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if confirmed.Status != domain.StatusPendingApproval {
		t.Errorf("expected pending_approval, got %s", confirmed.Status)
	}

	msgs := store.ListMessages()
	if msgs[0].ActionCard.Status != domain.StatusPendingApproval {
		t.Fatalf("expected ActionCard in chat synced to pending_approval, got %s", msgs[0].ActionCard.Status)
	}

	// Review approval also syncs chat ActionCard.
	reviewed, err := exec.ReviewRequest(domain.RoleMentor, draft.ID, "approved", "Удачи на экзамене")
	if err != nil {
		t.Fatalf("review failed: %v", err)
	}
	if reviewed.Status != domain.StatusApproved {
		t.Fatalf("expected approved, got %s", reviewed.Status)
	}
	msgsAfter := store.ListMessages()
	if msgsAfter[0].ActionCard.Status != domain.StatusApproved || msgsAfter[0].ActionCard.ReviewerComment != "Удачи на экзамене" {
		t.Fatalf("expected synced review state in chat message, got %+v", msgsAfter[0].ActionCard)
	}

	// Non-existent confirm
	_, err = exec.ConfirmAction("unknown-id")
	if !errors.Is(err, mcp.ErrNotFound) {
		t.Errorf("expected ErrNotFound confirming unknown id, got %v", err)
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

	if _, err := exec.ReviewRequest(domain.RoleIntern, draft.ID, "approved", "ok"); !errors.Is(err, mcp.ErrForbidden) {
		t.Errorf("expected ErrForbidden for intern, got %v", err)
	}

	if _, err := exec.ReviewRequest(domain.RoleMentor, draft.ID, "maybe", "ok"); !errors.Is(err, mcp.ErrInvalidDecision) {
		t.Errorf("expected ErrInvalidDecision, got %v", err)
	}

	approved, err := exec.ReviewRequest(domain.RoleMentor, draft.ID, "approved", "Отлично, удачи на экзамене")
	if err != nil {
		t.Fatalf("mentor review failed: %v", err)
	}
	if approved.Status != domain.StatusApproved {
		t.Errorf("expected approved, got %s", approved.Status)
	}

	draft2, _ := exec.CreateAbsenceRequest(intern, "personal_day_off", "2026-11-20", "2026-11-20", "Личное", true)
	rejected, err := exec.ReviewRequest(domain.RoleManager, draft2.ID, "rejected", "В этот день общий демо-день")
	if err != nil {
		t.Fatalf("manager review failed: %v", err)
	}
	if rejected.Status != domain.StatusRejected {
		t.Errorf("expected rejected, got %s", rejected.Status)
	}

	if _, err = exec.ReviewRequest(domain.RoleManager, "non-existent", "approved", ""); !errors.Is(err, mcp.ErrNotFound) {
		t.Errorf("expected ErrNotFound on non-existent request, got %v", err)
	}
}

func TestUpdateChecklistItemAndStageProgression(t *testing.T) {
	store := mcp.NewStore()
	exec := mcp.NewExecutor(store)

	_, _ = exec.UpdateChecklistItem("step-01-access", domain.StatusDone)
	state, err := exec.UpdateChecklistItem("step-02-env-setup", domain.StatusDone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.OverallProgress != 50 || state.CurrentStageID != "stage-week-2" || state.Stages[0].Status != "completed" {
		t.Fatalf("expected stage 1 completed and stage 2 active, got progress=%d stage=%s stages=%+v", state.OverallProgress, state.CurrentStageID, state.Stages)
	}

	// Roll back step-02 -> stage 1 returns to current
	state, err = exec.UpdateChecklistItem("step-02-env-setup", domain.StatusTodo)
	if err != nil || state.CurrentStageID != "stage-week-1" || state.Stages[0].Status != "current" {
		t.Fatalf("expected stage 1 rolled back to current, got %+v err=%v", state, err)
	}

	if _, err = exec.UpdateChecklistItem("step-01-access", domain.ChecklistItemStatus("invalid")); !errors.Is(err, mcp.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
	if _, err = exec.UpdateChecklistItem("non-existent-step", domain.StatusDone); !errors.Is(err, mcp.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestExecuteAllSevenMCPToolsAndRegistry(t *testing.T) {
	store := mcp.NewStore()
	kb := rag.NewKnowledgeBase()
	kb.SetArticles([]domain.KnowledgeArticle{
		{
			ID:          "kb-1",
			Title:       "Доступы и IDM",
			Content:     "Запрос доступа к repo:avito-delivery-core",
			Sensitivity: "intern",
			Source:      "Playbook [Mock]",
			Tags:        []string{"idm", "repo", "доступ"},
		},
	})
	tr := telemetry.NewTracker(true)
	exec := mcp.NewExecutorWithDeps(store, kb, tr)
	intern := sampleUser(domain.RoleIntern)
	mentor := sampleUser(domain.RoleMentor)
	ctx := context.Background()

	if len(exec.OpenAITools()) != 7 {
		t.Fatalf("expected 7 OpenAI tools, got %d", len(exec.OpenAITools()))
	}
	if len(exec.DefinitionsForRole(domain.RoleIntern)) != 6 {
		t.Fatalf("expected 6 tools permitted for intern, got %d", len(exec.DefinitionsForRole(domain.RoleIntern)))
	}
	if len(exec.DefinitionsForRole(domain.RoleMentor)) != 7 {
		t.Fatalf("expected 7 tools permitted for mentor, got %d", len(exec.DefinitionsForRole(domain.RoleMentor)))
	}

	// 1. search_knowledge_base
	res, err := exec.ExecuteTool(ctx, intern, "search_knowledge_base", json.RawMessage(`{"query":"доступ idm","limit":2}`))
	if err != nil || !res.Allowed || len(res.Citations) != 1 {
		t.Fatalf("search_knowledge_base failed: res=%+v err=%v", res, err)
	}

	// 2. get_onboarding_status
	res, err = exec.ExecuteTool(ctx, intern, "get_onboarding_status", nil)
	if err != nil || !res.Allowed || res.OutputJSON == "" {
		t.Fatalf("get_onboarding_status failed: %+v err=%v", res, err)
	}

	// 3. complete_checklist_item
	res, err = exec.ExecuteTool(ctx, intern, "complete_checklist_item", json.RawMessage(`{"item_id":"step-02-env-setup","status":"done"}`))
	if err != nil || !res.Allowed {
		t.Fatalf("complete_checklist_item failed: %+v err=%v", res, err)
	}

	// 4. request_service_access (allowed + blocked)
	res, err = exec.ExecuteTool(ctx, intern, "request_service_access", json.RawMessage(`{"resource":"repo:avito-delivery-core","access_level":"read_write","reason":"Task 1"}`))
	if err != nil || !res.Allowed || res.ActionCard == nil {
		t.Fatalf("request_service_access allowed failed: %+v err=%v", res, err)
	}
	resBlocked, err := exec.ExecuteTool(ctx, intern, "request_service_access", json.RawMessage(`{"resource":"prod-billing-db","access_level":"admin","reason":"debug"}`))
	if err != nil || resBlocked.Allowed || resBlocked.ActionCard == nil || resBlocked.ActionCard.Status != domain.StatusBlockedByRBAC {
		t.Fatalf("request_service_access block failed: %+v err=%v", resBlocked, err)
	}

	// 5. create_absence_request
	resAbs, err := exec.ExecuteTool(ctx, intern, "create_absence_request", json.RawMessage(`{"absence_type":"study_day_off","start_date":"2026-11-22","end_date":"2026-11-22","reason":"Exam"}`))
	if err != nil || !resAbs.Allowed || resAbs.ActionCard == nil || resAbs.ActionCard.Status != domain.StatusDraftConfirmationRequired {
		t.Fatalf("create_absence_request failed: %+v err=%v", resAbs, err)
	}

	// 6. prepare_mentor_1on1_summary
	res1on1, err := exec.ExecuteTool(ctx, intern, "prepare_mentor_1on1_summary", json.RawMessage(`{"topic":"Redis TTL","what_checked":["docs"],"questions":["TTL?"]}`))
	if err != nil || !res1on1.Allowed || res1on1.ActionCard == nil {
		t.Fatalf("prepare_mentor_1on1_summary failed: %+v err=%v", res1on1, err)
	}

	// 7. review_corporate_request: intern blocked, mentor allowed
	resRevIntern, err := exec.ExecuteTool(ctx, intern, "review_corporate_request", json.RawMessage(`{"request_id":"`+resAbs.ActionCard.ID+`","decision":"approved"}`))
	if err != nil || resRevIntern.Allowed {
		t.Fatalf("expected intern review tool call blocked cleanly, got %+v err=%v", resRevIntern, err)
	}
	resRevMentor, err := exec.ExecuteTool(ctx, mentor, "review_corporate_request", json.RawMessage(`{"request_id":"`+resAbs.ActionCard.ID+`","decision":"approved","comment":"OK"}`))
	if err != nil || !resRevMentor.Allowed || resRevMentor.ActionCard.Status != domain.StatusApproved {
		t.Fatalf("expected mentor review tool call succeeded, got %+v err=%v", resRevMentor, err)
	}

	// Unknown tool & invalid JSON error checks
	resUnknown, err := exec.ExecuteTool(ctx, mentor, "drop_prod_tables", nil)
	if err != nil || resUnknown.Allowed {
		t.Fatalf("expected unknown tool blocked by RBAC policy, got %+v err=%v", resUnknown, err)
	}
	if _, err := exec.ExecuteTool(ctx, intern, "get_onboarding_status", json.RawMessage(`{invalid`)); err == nil {
		t.Fatalf("expected error on invalid JSON")
	}

	// IncrementQuestionsResolved, Reset, Role switch helpers
	store.IncrementQuestionsResolved()
	if _, err := store.SetActiveRole(domain.RoleMentor); err != nil {
		t.Fatalf("switch role failed: %v", err)
	}
	store.SetAvgLatencyMs(5)
	store.Reset()
	_, reqs, metrics := store.GetSnapshot()
	if len(reqs) != 0 || metrics.RoutineQuestionsResolved != 0 {
		t.Fatalf("expected clean state after Reset")
	}
}
