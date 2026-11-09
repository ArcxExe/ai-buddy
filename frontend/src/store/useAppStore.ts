import { create } from 'zustand'
import { Role, UserProfile, OnboardingState, CorporateRequest, ImpactMetrics, ChatMessage } from '../types'

interface AppStore {
  activeRole: Role
  currentUser: UserProfile | null
  availableRoles: UserProfile[]
  onboarding: OnboardingState | null
  corporateRequests: CorporateRequest[]
  metrics: ImpactMetrics | null
  featureFlags: Record<string, boolean>
  messages: ChatMessage[]
  isAgentTyping: boolean
  currentAgentStep: string | null
  isKnowledgeDrawerOpen: boolean

  // Actions
  setActiveRole: (role: Role) => void
  setContext: (data: {
    current_user: UserProfile
    available_roles: UserProfile[]
    onboarding: OnboardingState
    corporate_requests: CorporateRequest[]
    metrics: ImpactMetrics
    feature_flags: Record<string, boolean>
  }) => void
  setMessages: (messages: ChatMessage[]) => void
  addMessage: (message: ChatMessage) => void
  setAgentTyping: (typing: boolean, step?: string | null) => void
  updateOnboarding: (onboarding: OnboardingState) => void
  updateCorporateRequests: (requests: CorporateRequest[]) => void
  updateMetrics: (metrics: ImpactMetrics) => void
  toggleKnowledgeDrawer: () => void
}

export const useAppStore = create<AppStore>((set) => ({
  activeRole: 'intern',
  currentUser: null,
  availableRoles: [],
  onboarding: null,
  corporateRequests: [],
  metrics: null,
  featureFlags: {},
  messages: [],
  isAgentTyping: false,
  currentAgentStep: null,
  isKnowledgeDrawerOpen: false,

  setActiveRole: (role) => set({ activeRole: role }),
  setContext: (data) =>
    set({
      currentUser: data.current_user,
      availableRoles: data.available_roles,
      onboarding: data.onboarding,
      corporateRequests: data.corporate_requests,
      metrics: data.metrics,
      featureFlags: data.feature_flags,
    }),
  setMessages: (messages) => set({ messages }),
  addMessage: (message) => set((state) => ({ messages: [...state.messages, message] })),
  setAgentTyping: (typing, step = null) => set({ isAgentTyping: typing, currentAgentStep: step }),
  updateOnboarding: (onboarding) => set({ onboarding }),
  updateCorporateRequests: (corporateRequests) => set({ corporateRequests }),
  updateMetrics: (metrics) => set({ metrics }),
  toggleKnowledgeDrawer: () => set((state) => ({ isKnowledgeDrawerOpen: !state.isKnowledgeDrawerOpen })),
}))
