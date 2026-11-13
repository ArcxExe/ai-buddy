export type Role = 'intern' | 'mentor' | 'manager'

export interface UserProfile {
  id: string
  name: string
  role: Role
  role_title: string
  team: string
  department: string
  mentor_name: string
  manager_name: string
  avatar_initials: string
  permissions: string[]
}

export type ChecklistCategory = 'access' | 'tools' | 'processes' | 'first_task'
export type ChecklistStatus = 'todo' | 'in_progress' | 'done'

export interface ChecklistItem {
  id: string
  stage_id: string
  title: string
  description: string
  status: ChecklistStatus
  category: ChecklistCategory
  estimated_time: string
  suggested_prompt: string
}

export interface OnboardingStage {
  id: string
  title: string
  period: string
  status: 'completed' | 'current' | 'upcoming'
  progress_percent: number
}

export interface OnboardingState {
  overall_progress: number
  current_stage_id: string
  next_recommended_step?: ChecklistItem
  stages: OnboardingStage[]
  checklist: ChecklistItem[]
}

export type RequestType = 'access_request' | 'absence_request' | 'mentor_1on1_summary' | 'rbac_blocked'
export type RequestStatus = 'draft_confirmation_required' | 'pending_approval' | 'approved' | 'completed' | 'blocked_by_rbac' | 'rejected'

export interface CorporateRequest {
  id: string
  service_name: string
  tool_name: string
  request_type: RequestType
  title: string
  summary: string
  status: RequestStatus
  created_by_role: Role
  target_resource: string
  reviewer_comment?: string
  created_at: string
  details: Record<string, unknown> | null
}

export interface KnowledgeCitation {
  id: string
  title: string
  source: string
  sensitivity: string
}

export interface ChatMessage {
  id: string
  sender: 'user' | 'assistant'
  role_context: Role
  content: string
  created_at: string
  processing_steps?: string[]
  citations?: KnowledgeCitation[]
  action_card?: CorporateRequest
  engine_used?: string
}

export interface ImpactMetrics {
  mentor_hours_baseline: number
  mentor_hours_saved: number
  routine_questions_resolved: number
  mcp_actions_executed: number
  rbac_violations_prevented: number
  avg_local_latency_ms: number
}

export interface AppContextResponse {
  active_role: Role
  current_user: UserProfile
  available_roles: UserProfile[]
  onboarding: OnboardingState
  corporate_requests: CorporateRequest[]
  metrics: ImpactMetrics
  feature_flags: Record<string, boolean>
  disclaimer: string
}

export interface KnowledgeArticle {
  id: string
  title: string
  category: string
  sensitivity: string
  source: string
  summary: string
  content: string
  tags: string[] | null
}

export interface SwitchRoleRequest { role: Role }
export interface SwitchRoleResponse { active_role: Role; current_user: UserProfile }
export interface SendMessageRequest { content: string; scenario_hint?: string; role?: Role }
export interface ChatResponse {
  user_message: ChatMessage
  assistant_message: ChatMessage
  onboarding: OnboardingState
  corporate_requests: CorporateRequest[]
  metrics: ImpactMetrics
}
export interface ConfirmActionRequest { request_id: string }
export interface ConfirmActionResponse {
  request: CorporateRequest
  onboarding: OnboardingState
  corporate_requests: CorporateRequest[]
  metrics: ImpactMetrics
}
export type ReviewDecision = 'approved' | 'rejected'
export interface ReviewRequest { decision: ReviewDecision; comment?: string }
export interface ReviewResponse { request: CorporateRequest; corporate_requests: CorporateRequest[] }
export interface UpdateChecklistRequest { status: ChecklistStatus }
export interface KnowledgeListResponse {
  items: KnowledgeArticle[]
  hidden_by_rbac_count: number
  active_role: Role
  disclaimer: string
}
export interface ErrorResponse { code: string; message: string }
