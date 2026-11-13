import { z } from 'zod'
import type {
  AppContextResponse, ChatMessage, ChatResponse, ChecklistItem, ConfirmActionResponse,
  CorporateRequest, ImpactMetrics, KnowledgeArticle, KnowledgeCitation,
  KnowledgeListResponse, OnboardingStage, OnboardingState, ReviewResponse,
  SwitchRoleResponse, UserProfile,
} from '../types'

// Validate the implemented Go contracts. Unknown fields are preserved for additive
// server changes; required fields are never replaced with defaults or coerced.
const roleSchema = z.enum(['intern', 'mentor', 'manager'])
const checklistStatusSchema = z.enum(['todo', 'in_progress', 'done'])

const userSchema: z.ZodType<UserProfile> = z.object({
  id: z.string(), name: z.string(), role: roleSchema, role_title: z.string(),
  team: z.string(), department: z.string(), mentor_name: z.string(),
  manager_name: z.string(), avatar_initials: z.string(), permissions: z.array(z.string()),
}).passthrough()

const checklistItemSchema: z.ZodType<ChecklistItem> = z.object({
  id: z.string(), stage_id: z.string(), title: z.string(), description: z.string(),
  status: checklistStatusSchema, category: z.enum(['access', 'tools', 'processes', 'first_task']),
  estimated_time: z.string(), suggested_prompt: z.string(),
}).passthrough()

const stageSchema: z.ZodType<OnboardingStage> = z.object({
  id: z.string(), title: z.string(), period: z.string(),
  status: z.enum(['completed', 'current', 'upcoming']), progress_percent: z.number().int(),
}).passthrough()

export const onboardingSchema: z.ZodType<OnboardingState> = z.object({
  overall_progress: z.number().int(), current_stage_id: z.string(),
  next_recommended_step: checklistItemSchema.optional(),
  stages: z.array(stageSchema), checklist: z.array(checklistItemSchema),
}).passthrough()

const requestSchema: z.ZodType<CorporateRequest> = z.object({
  id: z.string(), service_name: z.string(), tool_name: z.string(),
  request_type: z.enum(['access_request', 'absence_request', 'mentor_1on1_summary', 'rbac_blocked']),
  title: z.string(), summary: z.string(),
  status: z.enum(['draft_confirmation_required', 'pending_approval', 'approved', 'completed', 'blocked_by_rbac', 'rejected']),
  created_by_role: roleSchema, target_resource: z.string(), reviewer_comment: z.string().optional(),
  created_at: z.string(), details: z.record(z.unknown()).nullable(),
}).passthrough()

const metricsSchema: z.ZodType<ImpactMetrics> = z.object({
  mentor_hours_baseline: z.number().finite(), mentor_hours_saved: z.number().finite(),
  routine_questions_resolved: z.number().int(), mcp_actions_executed: z.number().int(),
  rbac_violations_prevented: z.number().int(), avg_local_latency_ms: z.number().int(),
}).passthrough()

const citationSchema: z.ZodType<KnowledgeCitation> = z.object({
  id: z.string(), title: z.string(), source: z.string(), sensitivity: z.string(),
}).passthrough()

const messageSchema: z.ZodType<ChatMessage> = z.object({
  id: z.string(), sender: z.enum(['user', 'assistant']), role_context: roleSchema,
  content: z.string(), created_at: z.string(), processing_steps: z.array(z.string()).optional(),
  citations: z.array(citationSchema).optional(), action_card: requestSchema.optional(),
  engine_used: z.string().optional(),
}).passthrough()

const articleSchema: z.ZodType<KnowledgeArticle> = z.object({
  id: z.string(), title: z.string(), category: z.string(), sensitivity: z.string(),
  source: z.string(), summary: z.string(), content: z.string(), tags: z.array(z.string()).nullable(),
}).passthrough()

export const contextSchema: z.ZodType<AppContextResponse> = z.object({
  active_role: roleSchema, current_user: userSchema, available_roles: z.array(userSchema),
  onboarding: onboardingSchema, corporate_requests: z.array(requestSchema), metrics: metricsSchema,
  feature_flags: z.record(z.boolean()), disclaimer: z.string(),
}).passthrough()

export const messagesSchema = z.array(messageSchema)

export const switchRoleSchema: z.ZodType<SwitchRoleResponse> = z.object({
  active_role: roleSchema, current_user: userSchema,
}).passthrough()

export const chatSchema: z.ZodType<ChatResponse> = z.object({
  user_message: messageSchema, assistant_message: messageSchema, onboarding: onboardingSchema,
  corporate_requests: z.array(requestSchema), metrics: metricsSchema,
}).passthrough()

export const confirmSchema: z.ZodType<ConfirmActionResponse> = z.object({
  request: requestSchema, onboarding: onboardingSchema,
  corporate_requests: z.array(requestSchema), metrics: metricsSchema,
}).passthrough()

export const reviewSchema: z.ZodType<ReviewResponse> = z.object({
  request: requestSchema, corporate_requests: z.array(requestSchema),
}).passthrough()

export const knowledgeSchema: z.ZodType<KnowledgeListResponse> = z.object({
  items: z.array(articleSchema), hidden_by_rbac_count: z.number().int(),
  active_role: roleSchema, disclaimer: z.string(),
}).passthrough()
