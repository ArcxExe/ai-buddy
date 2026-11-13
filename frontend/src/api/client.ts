import type {
  Role, AppContextResponse, ChatMessage, OnboardingState, ChecklistStatus,
  SwitchRoleRequest, SwitchRoleResponse, SendMessageRequest, ChatResponse,
  ConfirmActionRequest, ConfirmActionResponse, ReviewRequest, ReviewResponse,
  ReviewDecision, UpdateChecklistRequest, KnowledgeListResponse, ErrorResponse,
} from '../types'
import type { ZodType } from 'zod'
import {
  chatSchema, confirmSchema, contextSchema, knowledgeSchema, messagesSchema,
  onboardingSchema, reviewSchema, switchRoleSchema,
} from './schemas'

const API_BASE = '/api/v1'

export class ApiError extends Error {
  constructor(message: string, readonly code: string, readonly status: number) {
    super(message)
    this.name = 'ApiError'
  }
}

export function getErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status >= 500) return 'Сервис временно недоступен. Попробуйте позже.'
  if (error instanceof ApiError && error.status === 400) return 'Проверьте параметры запроса.'
  if (error instanceof ApiError && error.status === 403) return 'Операция отклонена сервером. Недостаточно прав.'
  if (error instanceof ApiError && error.status === 404) return 'Данные не найдены. Обновите данные и повторите поиск.'
  return error instanceof Error ? error.message : 'Не удалось выполнить запрос. Попробуйте ещё раз.'
}

async function readResponse<T>(response: Response, schema: ZodType<T>): Promise<T> {
  if (!response.ok) {
    let payload: unknown
    try {
      payload = await response.json()
    } catch {
      // Proxies and the standard Go router may return a non-JSON error body.
    }
    if (typeof payload === 'object' && payload !== null &&
        'code' in payload && typeof payload.code === 'string' &&
        'message' in payload && typeof payload.message === 'string') {
      const error: ErrorResponse = { code: payload.code, message: payload.message }
      throw new ApiError(error.message, error.code, response.status)
    }
    throw new ApiError(`Ошибка сервера (HTTP ${response.status}). Попробуйте ещё раз.`, 'HTTP_ERROR', response.status)
  }
  let payload: unknown
  try {
    payload = await response.json()
  } catch {
    throw new ApiError('Сервер вернул некорректный JSON. Попробуйте ещё раз.', 'INVALID_RESPONSE', response.status)
  }
  const result = schema.safeParse(payload)
  if (!result.success) {
    throw new ApiError('Сервер вернул данные неожиданного формата. Обновите данные или попробуйте позже.', 'INVALID_RESPONSE', response.status)
  }
  return result.data
}

async function request<T>(schema: ZodType<T>, path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, init)
  } catch {
    if (init?.signal?.aborted) throw init.signal.reason
    throw new ApiError('Не удалось связаться с сервером. Проверьте подключение и повторите запрос.', 'NETWORK_ERROR', 0)
  }
  try {
    return await readResponse(response, schema)
  } catch (error) {
    if (import.meta.env.DEV && import.meta.env.MODE !== 'test') console.error('API response error', error)
    throw error
  }
}

export async function fetchContext(role?: Role, signal?: AbortSignal): Promise<AppContextResponse> {
  return request(contextSchema, `/context`, {
    ...(role ? { headers: { 'X-User-Role': role } } : {}),
    ...(signal ? { signal } : {}),
  })
}

export async function switchRole(role: Role): Promise<SwitchRoleResponse> {
  const body: SwitchRoleRequest = { role }
  return request(switchRoleSchema, `/auth/switch-role`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function fetchMessages(signal?: AbortSignal): Promise<ChatMessage[]> {
  return request(messagesSchema, `/chat/messages`, signal ? { signal } : undefined)
}

export async function sendMessage(role: Role, content: string, scenarioHint?: string): Promise<ChatResponse> {
  const body: SendMessageRequest = { content, scenario_hint: scenarioHint }
  return request(chatSchema, `/chat/message`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-Role': role,
    },
    body: JSON.stringify(body),
  })
}

export async function resetChat(): Promise<AppContextResponse> {
  return request(contextSchema, `/chat/reset`, { method: 'POST' })
}

export async function confirmCorporateAction(role: Role, requestId: string): Promise<ConfirmActionResponse> {
  const body: ConfirmActionRequest = { request_id: requestId }
  return request(confirmSchema, `/corporate/actions/confirm`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-Role': role,
    },
    body: JSON.stringify(body),
  })
}

export async function reviewCorporateRequest(role: Role, requestId: string, decision: ReviewDecision, comment?: string): Promise<ReviewResponse> {
  const body: ReviewRequest = { decision, comment }
  return request(reviewSchema, `/corporate/requests/${encodeURIComponent(requestId)}/review`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-Role': role,
    },
    body: JSON.stringify(body),
  })
}

export async function updateChecklistStatus(itemId: string, status: ChecklistStatus): Promise<OnboardingState> {
  const body: UpdateChecklistRequest = { status }
  return request(onboardingSchema, `/onboarding/checklist/${encodeURIComponent(itemId)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export async function fetchKnowledge(role: Role, query?: string, signal?: AbortSignal): Promise<KnowledgeListResponse> {
  const path = query ? `/knowledge?q=${encodeURIComponent(query)}` : '/knowledge'
  return request(knowledgeSchema, path, {
    headers: { 'X-User-Role': role },
    ...(signal ? { signal } : {}),
  })
}
