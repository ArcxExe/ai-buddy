import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as api from '../api/client'

describe('API Client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('fetchContext calls /api/v1/context with header', async () => {
    const mockData = { current_user: { id: 'u1' } }
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => mockData,
    } as Response)

    const res = await api.fetchContext('intern')
    expect(res).toEqual(mockData)
    expect(global.fetch).toHaveBeenCalledWith('/api/v1/context', {
      headers: { 'X-User-Role': 'intern' },
    })
  })

  it('switchRole posts role payload', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: 'u2', role: 'mentor' }),
    } as Response)

    const res = await api.switchRole('mentor')
    expect(res.role).toBe('mentor')
  })

  it('fetchMessages gets messages array', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => [{ id: 'm1', content: 'test' }],
    } as Response)

    const res = await api.fetchMessages()
    expect(res).toHaveLength(1)
  })

  it('sendMessage posts prompt and scenario hint', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ user_message: { id: 'u' }, assistant_message: { id: 'a' } }),
    } as Response)

    const res = await api.sendMessage('intern', 'Отгул', 'scenario_02')
    expect(res.assistant_message.id).toBe('a')
  })

  it('resetChat calls /api/v1/chat/reset', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ status: 'reset' }),
    } as Response)

    const res = await api.resetChat()
    expect(res).toBeDefined()
  })

  it('confirmCorporateAction posts request_id', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: 'abs-1', status: 'pending_approval' }),
    } as Response)

    const res = await api.confirmCorporateAction('intern', 'abs-1')
    expect(res.status).toBe('pending_approval')
  })

  it('reviewCorporateRequest approves request', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: 'abs-1', status: 'approved' }),
    } as Response)

    const res = await api.reviewCorporateRequest('mentor', 'abs-1', 'approved', 'ok')
    expect(res.status).toBe('approved')
  })

  it('updateChecklistStatus updates item status', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ overall_progress: 50 }),
    } as Response)

    const res = await api.updateChecklistStatus('step-1', 'done')
    expect(res.overall_progress).toBe(50)
  })

  it('fetchKnowledge gets knowledge articles and query', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => ({ visible_articles: [], hidden_by_rbac_count: 1 }),
    } as Response)

    const res = await api.fetchKnowledge('intern', 'отгул')
    expect(res.hidden_by_rbac_count).toBe(1)
  })
})
