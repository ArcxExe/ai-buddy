import { Role, AppContextResponse, ChatMessage, OnboardingState, CorporateRequest, KnowledgeArticle } from '../types'

const API_BASE = '/api/v1'

export async function fetchContext(role: Role): Promise<AppContextResponse> {
  const res = await fetch(`${API_BASE}/context`, {
    headers: { 'X-User-Role': role },
  })
  if (!res.ok) throw new Error('Failed to load context')
  return res.json()
}

export async function switchRole(role: Role) {
  const res = await fetch(`${API_BASE}/auth/switch-role`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ role }),
  })
  if (!res.ok) throw new Error('Failed to switch role')
  return res.json()
}

export async function fetchMessages(): Promise<ChatMessage[]> {
  const res = await fetch(`${API_BASE}/chat/messages`)
  if (!res.ok) throw new Error('Failed to fetch messages')
  return res.json()
}

export async function sendMessage(role: Role, content: string, scenarioHint?: string) {
  const res = await fetch(`${API_BASE}/chat/message`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-Role': role,
    },
    body: JSON.stringify({ content, scenario_hint: scenarioHint }),
  })
  if (!res.ok) throw new Error('Failed to send message')
  return res.json()
}

export async function resetChat(): Promise<AppContextResponse> {
  const res = await fetch(`${API_BASE}/chat/reset`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to reset chat')
  return res.json()
}

export async function confirmCorporateAction(role: Role, requestId: string): Promise<CorporateRequest> {
  const res = await fetch(`${API_BASE}/corporate/actions/confirm`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-Role': role,
    },
    body: JSON.stringify({ request_id: requestId }),
  })
  if (!res.ok) throw new Error('Failed to confirm action')
  return res.json()
}

export async function reviewCorporateRequest(role: Role, requestId: string, decision: 'approved' | 'rejected', comment: string): Promise<CorporateRequest> {
  const res = await fetch(`${API_BASE}/corporate/requests/${requestId}/review`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-Role': role,
    },
    body: JSON.stringify({ decision, comment }),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error || 'Review failed')
  }
  return res.json()
}

export async function updateChecklistStatus(itemId: string, status: string): Promise<OnboardingState> {
  const res = await fetch(`${API_BASE}/onboarding/checklist/${itemId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  })
  if (!res.ok) throw new Error('Failed to update checklist item')
  return res.json()
}

export async function fetchKnowledge(role: Role, query?: string): Promise<{
  role: string
  visible_articles: KnowledgeArticle[]
  hidden_by_rbac_count: number
}> {
  const url = query ? `${API_BASE}/knowledge?q=${encodeURIComponent(query)}` : `${API_BASE}/knowledge`
  const res = await fetch(url, {
    headers: { 'X-User-Role': role },
  })
  if (!res.ok) throw new Error('Failed to load knowledge articles')
  return res.json()
}
