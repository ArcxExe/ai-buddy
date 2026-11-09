import { describe, it, expect, beforeEach } from 'vitest'
import { useAppStore } from '../store/useAppStore'
import { Role, UserProfile, OnboardingState } from '../types'

describe('useAppStore', () => {
  beforeEach(() => {
    useAppStore.setState({
      activeRole: 'intern',
      currentUser: null,
      messages: [],
      isAgentTyping: false,
      isKnowledgeDrawerOpen: false,
    })
  })

  it('changes active role', () => {
    const { setActiveRole } = useAppStore.getState()
    setActiveRole('mentor')
    expect(useAppStore.getState().activeRole).toBe('mentor')
  })

  it('adds message to conversation', () => {
    const { addMessage } = useAppStore.getState()
    addMessage({
      id: 'm1',
      sender: 'user',
      role_context: 'intern',
      content: 'Hello AI',
      created_at: '2026-11-09 10:00:00',
    })
    expect(useAppStore.getState().messages).toHaveLength(1)
    expect(useAppStore.getState().messages[0].content).toBe('Hello AI')
  })

  it('toggles knowledge drawer', () => {
    const { toggleKnowledgeDrawer } = useAppStore.getState()
    expect(useAppStore.getState().isKnowledgeDrawerOpen).toBe(false)
    toggleKnowledgeDrawer()
    expect(useAppStore.getState().isKnowledgeDrawerOpen).toBe(true)
    toggleKnowledgeDrawer()
    expect(useAppStore.getState().isKnowledgeDrawerOpen).toBe(false)
  })

  it('updates typing state', () => {
    const { setAgentTyping } = useAppStore.getState()
    setAgentTyping(true, 'Reasoning...')
    expect(useAppStore.getState().isAgentTyping).toBe(true)
    expect(useAppStore.getState().currentAgentStep).toBe('Reasoning...')
  })
})
