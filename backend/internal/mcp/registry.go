package mcp

import (
	"aibuddy/backend/internal/domain"
	"aibuddy/backend/internal/rbac"
)

// ToolCatalog returns the 7 registered MCP tools matching specs/mcp-tools.json.
func ToolCatalog() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:          "search_knowledge_base",
			Service:       "Avito Knowledge Base / Playbook RAG",
			Description:   "Поиск по внутренней базе знаний AvitoTech, Playbook стажёра, регламентам доступов и отгулов с автоматической фильтрацией по правам роли.",
			RequiredRoles: []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager},
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Поисковый запрос на русском или английском языке",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Максимальное количество статей (по умолчанию 3)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:          "get_onboarding_status",
			Service:       "Avito Start Onboarding Tracker",
			Description:   "Возвращает текущий прогресс адаптации стажёра, активный этап и следующий рекомендованный шаг чек-листа.",
			RequiredRoles: []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager},
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"include_completed": map[string]interface{}{
						"type":        "boolean",
						"description": "Включать ли уже выполненные шаги",
					},
				},
			},
		},
		{
			Name:          "complete_checklist_item",
			Service:       "Avito Start Onboarding Tracker",
			Description:   "Обновляет статус шага чек-листа адаптации (todo, in_progress, done) и пересчитывает общий прогресс.",
			RequiredRoles: []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager},
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"item_id": map[string]interface{}{
						"type":        "string",
						"description": "ID шага, например step-01-access, step-02-env-setup, step-03-absence-policy, step-04-first-pr",
					},
					"status": map[string]interface{}{
						"type": "string",
						"enum": []string{"todo", "in_progress", "done"},
					},
				},
				"required": []string{"item_id"},
			},
		},
		{
			Name:          "request_service_access",
			Service:       "Avito Access / IDM Mock",
			Description:   "Оформляет заявку на доступ к репозиторию, staging или инструменту команды. Проверяет RBAC-политику ИБ до создания заявки.",
			RequiredRoles: []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager},
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource": map[string]interface{}{
						"type":        "string",
						"description": "Идентификатор ресурса (например: repo:avito-delivery-core, env:staging-delivery, grafana:delivery-dashboard)",
					},
					"access_level": map[string]interface{}{
						"type": "string",
						"enum": []string{"read", "read_write", "admin"},
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Обоснование для чего нужен доступ",
					},
				},
				"required": []string{"resource", "access_level", "reason"},
			},
		},
		{
			Name:                 "create_absence_request",
			Service:              "Avito Absence Service Mock",
			Description:          "Создаёт черновик заявки на отгул/day-off в системе Avito Absence. По умолчанию требует подтверждения пользователя перед отправкой руководителю.",
			RequiredRoles:        []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager},
			RequiresConfirmation: true,
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"absence_type": map[string]interface{}{
						"type": "string",
						"enum": []string{"study_day_off", "personal_day_off", "vacation"},
					},
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Дата начала YYYY-MM-DD",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "Дата окончания YYYY-MM-DD",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Причина отсутствия",
					},
					"confirmed_by_user": map[string]interface{}{
						"type":        "boolean",
						"description": "Всегда false для первичного черновика",
					},
				},
				"required": []string{"absence_type", "start_date", "end_date", "reason"},
			},
		},
		{
			Name:          "prepare_mentor_1on1_summary",
			Service:       "Avito Buddy Sync Mock",
			Description:   "Формирует структурированную повестку и summary для встречи 1:1 с наставником (Buddy).",
			RequiredRoles: []domain.Role{domain.RoleIntern, domain.RoleMentor, domain.RoleManager},
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"topic": map[string]interface{}{
						"type":        "string",
						"description": "Ключевая тема или задача для обсуждения",
					},
					"what_checked": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"questions": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"topic", "what_checked", "questions"},
			},
		},
		{
			Name:          "review_corporate_request",
			Service:       "Avito Corporate Workflow Mock",
			Description:   "Позволяет наставнику (mentor) или руководителю (manager) согласовать или отклонить заявку стажёра.",
			RequiredRoles: []domain.Role{domain.RoleMentor, domain.RoleManager},
			ParametersSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"request_id": map[string]interface{}{
						"type": "string",
					},
					"decision": map[string]interface{}{
						"type": "string",
						"enum": []string{"approved", "rejected"},
					},
					"comment": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"request_id", "decision"},
			},
		},
	}
}

// DefinitionsForRole returns the MCP tools permitted for a specific user role.
func (e *Executor) DefinitionsForRole(role domain.Role) []ToolDefinition {
	all := ToolCatalog()
	out := make([]ToolDefinition, 0, len(all))
	for _, def := range all {
		if ok, _ := rbac.CanExecuteMCPTool(role, def.Name, ""); ok {
			out = append(out, def)
		}
	}
	return out
}

// OpenAITools returns the OpenAI/OpenRouter tool schemas for function calling.
func (e *Executor) OpenAITools() []OpenAITool {
	all := ToolCatalog()
	out := make([]OpenAITool, 0, len(all))
	for _, def := range all {
		out = append(out, OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.ParametersSchema,
			},
		})
	}
	return out
}
