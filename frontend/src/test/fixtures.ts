import type { AppContextResponse, ChatMessage, CorporateRequest, KnowledgeListResponse } from '../types'

export const context: AppContextResponse = {
  active_role: 'intern',
  current_user: {
    id: 'usr-01', name: 'Алексей Смирнов', role: 'intern', role_title: 'Backend Go Intern',
    team: 'Авито Доставка', department: 'Engineering', mentor_name: 'Мария Козлова',
    manager_name: 'Дмитрий Волков', avatar_initials: 'АС', permissions: ['read'],
  },
  available_roles: [],
  onboarding: {
    overall_progress: 25, current_stage_id: 's1',
    stages: [{ id: 's1', title: 'День 1', period: 'День 1', status: 'current', progress_percent: 25 }],
    checklist: [{
      id: 'step-1', stage_id: 's1', title: 'Запросить доступы', description: 'В Avito Access',
      status: 'todo', category: 'access', estimated_time: '10 мин', suggested_prompt: 'Запроси доступ',
    }],
  },
  corporate_requests: [],
  metrics: {
    mentor_hours_baseline: 48, mentor_hours_saved: 1.5, routine_questions_resolved: 2,
    mcp_actions_executed: 1, rbac_violations_prevented: 0, avg_local_latency_ms: 3,
  },
  feature_flags: { mcp_enabled: true }, disclaimer: 'Синтетические данные прототипа',
}
export const draft: CorporateRequest = {
  id: 'abs-1', service_name: 'Avito Absence (Mock)', tool_name: 'create_absence_request',
  request_type: 'absence_request', title: 'Учебный отгул', summary: 'Черновик готов',
  status: 'draft_confirmation_required', created_by_role: 'intern',
  target_resource: 'absence:study_day_off', created_at: '2026-11-13T08:00:00Z',
  details: { absence_type: 'study_day_off', start_date: '2026-11-18', end_date: '2026-11-18', reason: 'Экзамен', synthetic: true },
}
export const message: ChatMessage = {
  id: 'msg-1', sender: 'assistant', role_context: 'intern', content: 'Подготовлен черновик',
  created_at: '2026-11-13T08:00:00Z', action_card: draft,
}
export const knowledge: KnowledgeListResponse = {
  active_role: 'intern', disclaimer: 'Синтетическая база знаний', hidden_by_rbac_count: 2,
  items: [{
    id: 'kb-1', title: 'Стандарты Go в Авито', category: 'engineering', sensitivity: 'intern',
    source: 'Playbook', summary: 'Гайд по Go', content: 'Полный текст', tags: ['go'],
  }],
}
