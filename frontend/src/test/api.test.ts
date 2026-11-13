import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as api from '../api/client'
import { context, draft, message, knowledge } from './fixtures'

const mockResponse = (body: unknown, status = 200) =>
  vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(JSON.stringify(body), { status }))

describe('API Client: Go HTTP contracts', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('reads the global role without an override and forwards query cancellation signals', async () => {
    const controller = new AbortController()
    mockResponse(context)
    await api.fetchContext(undefined, controller.signal)
    expect(globalThis.fetch).toHaveBeenLastCalledWith('/api/v1/context', { signal: controller.signal })
    mockResponse([])
    await api.fetchMessages(controller.signal)
    expect(globalThis.fetch).toHaveBeenLastCalledWith('/api/v1/chat/messages', { signal: controller.signal })
    mockResponse(knowledge)
    await api.fetchKnowledge('intern', '', controller.signal)
    expect(globalThis.fetch).toHaveBeenLastCalledWith('/api/v1/knowledge', {
      headers: { 'X-User-Role': 'intern' }, signal: controller.signal,
    })
  })

  it('preserves intentional cancellation instead of showing a network error', async () => {
    const controller = new AbortController()
    controller.abort()
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(controller.signal.reason)
    await expect(api.fetchMessages(controller.signal)).rejects.toBe(controller.signal.reason)
  })

  it('loads context with role header', async () => {
    const fetch = mockResponse(context)
    expect(await api.fetchContext('intern')).toEqual(context)
    expect(fetch).toHaveBeenCalledWith('/api/v1/context', { headers: { 'X-User-Role': 'intern' } })
  })

  it('switches role using the response envelope', async () => {
    const body = { active_role: 'mentor', current_user: { ...context.current_user, role: 'mentor' } }
    const fetch = mockResponse(body)
    expect(await api.switchRole('mentor')).toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/switch-role', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ role: 'mentor' }),
    })
  })

  it('loads history with optional fields omitted', async () => {
    mockResponse([{ ...message, action_card: undefined }])
    const result = await api.fetchMessages()
    expect(result).toHaveLength(1)
    expect(result[0].action_card).toBeUndefined()
  })

  it('sends to singular /chat/message and reads the full snapshot', async () => {
    const body = { user_message: {
      id: 'msg-user', sender: 'user', role_context: 'intern', content: 'Отгул', created_at: message.created_at,
    }, assistant_message: message,
      onboarding: context.onboarding, corporate_requests: [draft], metrics: context.metrics }
    const fetch = mockResponse(body)
    expect(await api.sendMessage('intern', 'Отгул', 'scenario_02_absence_day_off')).toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/chat/message', {
      method: 'POST', headers: { 'Content-Type': 'application/json', 'X-User-Role': 'intern' },
      body: JSON.stringify({ content: 'Отгул', scenario_hint: 'scenario_02_absence_day_off' }),
    })
  })

  it('resets the demo and returns context', async () => {
    const fetch = mockResponse(context)
    expect(await api.resetChat()).toEqual(context)
    expect(fetch).toHaveBeenCalledWith('/api/v1/chat/reset', { method: 'POST' })
  })

  it('confirms and returns request, onboarding, requests and metrics', async () => {
    const request = { ...draft, status: 'pending_approval' }
    const body = { request, onboarding: context.onboarding, corporate_requests: [request], metrics: context.metrics }
    const fetch = mockResponse(body)
    expect(await api.confirmCorporateAction('intern', draft.id)).toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/corporate/actions/confirm', {
      method: 'POST', headers: { 'Content-Type': 'application/json', 'X-User-Role': 'intern' },
      body: JSON.stringify({ request_id: draft.id }),
    })
  })

  it('reviews using request and corporate_requests', async () => {
    const request = { ...draft, status: 'approved', reviewer_comment: 'ok' }
    const body = { request, corporate_requests: [request] }
    const fetch = mockResponse(body)
    expect(await api.reviewCorporateRequest('mentor', draft.id, 'approved', 'ok')).toEqual(body)
    expect(fetch).toHaveBeenCalledWith('/api/v1/corporate/requests/abs-1/review', {
      method: 'POST', headers: { 'Content-Type': 'application/json', 'X-User-Role': 'mentor' },
      body: JSON.stringify({ decision: 'approved', comment: 'ok' }),
    })
  })

  it('updates checklist with next_recommended_step omitted', async () => {
    const fetch = mockResponse(context.onboarding)
    expect(await api.updateChecklistStatus('step-1', 'done')).toEqual(context.onboarding)
    expect(fetch).toHaveBeenCalledWith('/api/v1/onboarding/checklist/step-1', {
      method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status: 'done' }),
    })
  })

  it('reads items and encodes search query', async () => {
    const fetch = mockResponse(knowledge)
    expect(await api.fetchKnowledge('intern', 'Go & доступ')).toEqual(knowledge)
    expect(fetch).toHaveBeenCalledWith('/api/v1/knowledge?q=' + encodeURIComponent('Go & доступ'), {
      headers: { 'X-User-Role': 'intern' },
    })
    mockResponse({ ...knowledge, items: [] })
    expect((await api.fetchKnowledge('mentor')).items).toEqual([])
    expect(globalThis.fetch).toHaveBeenLastCalledWith('/api/v1/knowledge', { headers: { 'X-User-Role': 'mentor' } })
  })

  it.each([
    () => api.fetchContext('intern'), () => api.switchRole('mentor'), () => api.fetchMessages(),
    () => api.sendMessage('intern', 'hello'), () => api.resetChat(),
    () => api.confirmCorporateAction('intern', 'missing'),
    () => api.reviewCorporateRequest('intern', 'abs-1', 'approved'),
    () => api.updateChecklistStatus('missing', 'done'), () => api.fetchKnowledge('intern'),
  ])('preserves backend errors for every operation', async (operation) => {
    mockResponse({ code: 'RBAC_FORBIDDEN', message: 'Недостаточно прав' }, 403)
    await expect(operation()).rejects.toMatchObject({
      name: 'ApiError', message: 'Недостаточно прав', code: 'RBAC_FORBIDDEN', status: 403,
    })
  })

  it('handles non-JSON and malformed error responses', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('Bad Gateway', { status: 502 }))
    await expect(api.fetchMessages()).rejects.toMatchObject({ code: 'HTTP_ERROR', status: 502 })
    mockResponse({ error: 'old shape' }, 500)
    await expect(api.fetchMessages()).rejects.toThrow('HTTP 500')
  })

  it('reports network failure without fabricated success', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new TypeError('Failed to fetch'))
    await expect(api.fetchMessages()).rejects.toMatchObject({ code: 'NETWORK_ERROR', status: 0 })
    expect(api.getErrorMessage(new Error('Ошибка'))).toBe('Ошибка')
    expect(api.getErrorMessage(null)).toContain('Не удалось')
  })

  it('rejects invalid JSON in a successful HTTP response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('invalid', { status: 200 }))
    await expect(api.fetchKnowledge('intern')).rejects.toMatchObject({ code: 'INVALID_RESPONSE', status: 200 })
  })

  it.each([
    ['context', () => api.fetchContext(), { ...context, current_user: { ...context.current_user, permissions: 'read' } }],
    ['history', () => api.fetchMessages(), [{ ...message, sender: 'bot' }]],
    ['chat', () => api.sendMessage('intern', 'Отгул'), {
      user_message: { ...message, sender: 'user' }, assistant_message: { ...message, action_card: { ...draft, status: 'sent' } },
      onboarding: context.onboarding, corporate_requests: [draft], metrics: context.metrics,
    }],
    ['checklist', () => api.updateChecklistStatus('step-1', 'done'), {
      ...context.onboarding, checklist: [{ ...context.onboarding.checklist[0], status: 'completed' }],
    }],
    ['knowledge', () => api.fetchKnowledge('intern'), { ...knowledge, items: null }],
    ['role', () => api.switchRole('mentor'), { active_role: 'owner', current_user: context.current_user }],
    ['confirm', () => api.confirmCorporateAction('intern', draft.id), {
      request: draft, onboarding: context.onboarding, corporate_requests: [draft], metrics: { ...context.metrics, mentor_hours_saved: '1.5' },
    }],
    ['review', () => api.reviewCorporateRequest('mentor', draft.id, 'approved'), { request: draft }],
    ['reset', () => api.resetChat(), { ...context, feature_flags: { mcp_enabled: 'true' } }],
  ] as const)('rejects invalid %s response structure without retrying', async (_name, operation, body) => {
    const fetch = mockResponse(body)
    await expect(operation()).rejects.toMatchObject({
      code: 'INVALID_RESPONSE', status: 200, message: expect.stringContaining('неожиданного формата'),
    })
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('accepts Go nullable fields, omitted optional fields and additive fields without defaults', async () => {
    mockResponse([{ ...message, action_card: { ...draft, details: null }, extra_field: 'server extension' }])
    expect(await api.fetchMessages()).toEqual([
      { ...message, action_card: { ...draft, details: null }, extra_field: 'server extension' },
    ])
    mockResponse({ ...knowledge, items: [{ ...knowledge.items[0], tags: null }] })
    expect((await api.fetchKnowledge('intern')).items[0].tags).toBeNull()
    mockResponse({ ...context, onboarding: { ...context.onboarding, next_recommended_step: context.onboarding.checklist[0] } })
    expect((await api.fetchContext()).onboarding.next_recommended_step).toEqual(context.onboarding.checklist[0])
    mockResponse([{ ...message, action_card: undefined, processing_steps: ['Поиск завершён'],
      citations: [{ id: 'kb-1', title: 'Статья', source: 'Playbook', sensitivity: 'intern' }], engine_used: 'smart_sdd_fallback' }])
    expect((await api.fetchMessages())[0].citations).toHaveLength(1)
  })

  it('rejects null or missing required fields instead of inventing empty results', async () => {
    mockResponse({ ...knowledge, items: undefined })
    await expect(api.fetchKnowledge('intern')).rejects.toMatchObject({ code: 'INVALID_RESPONSE' })
    mockResponse({ ...context.onboarding, next_recommended_step: null })
    await expect(api.updateChecklistStatus('step-1', 'done')).rejects.toMatchObject({ code: 'INVALID_RESPONSE' })
    mockResponse({ ...knowledge, items: [{ ...knowledge.items[0], tags: undefined }] })
    await expect(api.fetchKnowledge('intern')).rejects.toMatchObject({ code: 'INVALID_RESPONSE' })
    mockResponse([{ ...message, action_card: { ...draft, details: 'not an object' } }])
    await expect(api.fetchMessages()).rejects.toMatchObject({ code: 'INVALID_RESPONSE' })
  })
})
