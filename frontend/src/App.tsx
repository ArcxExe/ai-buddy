import React, { useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider, useQuery, useMutation } from '@tanstack/react-query'
import { useAppStore } from './store/useAppStore'
import { fetchContext, fetchMessages, sendMessage, resetChat, confirmCorporateAction, reviewCorporateRequest, updateChecklistStatus, switchRole } from './api/client'
import { Role, ChecklistStatus } from './types'
import { Header } from './components/Header'
import { OnboardingRoadmap } from './components/OnboardingRoadmap'
import { ChatWorkspace } from './components/ChatWorkspace'
import { MentorPanel } from './components/MentorPanel'
import { KnowledgeDrawer } from './components/KnowledgeDrawer'
import { ShieldCheck, Info } from 'lucide-react'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      staleTime: 5000,
    },
  },
})

function MainApp() {
  const {
    activeRole,
    setActiveRole,
    setContext,
    setMessages,
    addMessage,
    setAgentTyping,
    updateOnboarding,
    updateCorporateRequests,
    updateMetrics,
  } = useAppStore()

  const [activeTab, setActiveTab] = useState<'onboarding' | 'mentor'>('onboarding')

  // Context Query
  const contextQuery = useQuery({
    queryKey: ['context', activeRole],
    queryFn: () => fetchContext(activeRole),
  })

  // Messages Query
  const messagesQuery = useQuery({
    queryKey: ['messages'],
    queryFn: fetchMessages,
  })

  useEffect(() => {
    if (contextQuery.data) {
      setContext(contextQuery.data)
      if (activeRole !== 'intern' && activeTab === 'onboarding') {
        setActiveTab('mentor')
      } else if (activeRole === 'intern' && activeTab === 'mentor') {
        setActiveTab('onboarding')
      }
    }
  }, [contextQuery.data, activeRole])

  useEffect(() => {
    if (messagesQuery.data) {
      setMessages(messagesQuery.data)
    }
  }, [messagesQuery.data])

  // Send Message Mutation
  const sendMutation = useMutation({
    mutationFn: ({ content, hint }: { content: string; hint?: string }) => {
      setAgentTyping(true, 'Анализирую запрос и сверяюсь со стандартами команды...')
      return sendMessage(activeRole, content, hint)
    },
    onSuccess: (data) => {
      setAgentTyping(false)
      addMessage(data.user_message)
      addMessage(data.assistant_message)
      updateOnboarding(data.onboarding)
      updateCorporateRequests(data.corporate_requests)
      updateMetrics(data.metrics)
    },
    onError: (err: any) => {
      setAgentTyping(false)
      alert('Ошибка при отправке: ' + err.message)
    },
  })

  // Confirm Action Mutation
  const confirmMutation = useMutation({
    mutationFn: (requestId: string) => confirmCorporateAction(activeRole, requestId),
    onSuccess: () => {
      contextQuery.refetch()
      messagesQuery.refetch()
    },
  })

  // Review Action Mutation (for Mentor/Manager)
  const reviewMutation = useMutation({
    mutationFn: ({ id, decision, comment }: { id: string; decision: 'approved' | 'rejected'; comment: string }) =>
      reviewCorporateRequest(activeRole, id, decision, comment),
    onSuccess: () => {
      contextQuery.refetch()
    },
  })

  // Update Checklist Item Mutation
  const checklistMutation = useMutation({
    mutationFn: ({ itemId, status }: { itemId: string; status: ChecklistStatus }) =>
      updateChecklistStatus(itemId, status),
    onSuccess: (updatedState) => {
      updateOnboarding(updatedState)
    },
  })

  const handleRoleChange = async (role: Role) => {
    try {
      await switchRole(role)
      setActiveRole(role)
      if (role === 'intern') {
        setActiveTab('onboarding')
      } else {
        setActiveTab('mentor')
      }
    } catch (e) {
      console.error(e)
    }
  }

  const handleReset = async () => {
    if (window.confirm('Сбросить историю чата и вернуть демо-состояние в исходный вид?')) {
      const fresh = await resetChat()
      setContext(fresh)
      setMessages([])
    }
  }

  return (
    <div className="min-h-screen bg-[#F8F9FA] flex flex-col">
      <Header onReset={handleReset} onRoleChange={handleRoleChange} />

      {/* Synthetic Disclaimer Banner */}
      <div className="bg-blue-50/80 border-b border-blue-100 px-4 py-1.5 text-center text-xs text-blue-900 font-medium flex items-center justify-center gap-2">
        <Info className="w-3.5 h-3.5 text-avito-blue flex-shrink-0" />
        <span>
          <b>Демонстрационный прототип (Hack&Change 2026):</b> Все регламенты, сервисы (Absence, IDM) и сотрудники являются синтетическими имитациями.
        </span>
      </div>

      {/* Main Workspace */}
      <main className="flex-1 max-w-7xl w-full mx-auto p-4 lg:p-6 grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Column: Roadmap or Mentor Panel */}
        <div className="lg:col-span-5 space-y-4">
          {/* Mobile Tab Switcher */}
          <div className="flex lg:hidden bg-gray-200 p-1 rounded-xl text-xs font-semibold">
            <button
              onClick={() => setActiveTab('onboarding')}
              className={`flex-1 py-1.5 rounded-lg ${activeTab === 'onboarding' ? 'bg-white shadow-xs' : 'text-gray-600'}`}
            >
              Чек-лист стажёра
            </button>
            <button
              onClick={() => setActiveTab('mentor')}
              className={`flex-1 py-1.5 rounded-lg ${activeTab === 'mentor' ? 'bg-white shadow-xs' : 'text-gray-600'}`}
            >
              Панель наставника
            </button>
          </div>

          <div className="hidden lg:block">
            {activeRole === 'intern' ? (
              <OnboardingRoadmap
                onSelectPrompt={(p, hint) => sendMutation.mutate({ content: p, hint })}
                onToggleChecklist={(id, status) => checklistMutation.mutate({ itemId: id, status })}
              />
            ) : (
              <MentorPanel
                onReview={(id, decision, comment) => reviewMutation.mutate({ id, decision, comment })}
                isReviewing={reviewMutation.isPending}
              />
            )}
          </div>

          <div className="block lg:hidden">
            {activeTab === 'onboarding' ? (
              <OnboardingRoadmap
                onSelectPrompt={(p, hint) => sendMutation.mutate({ content: p, hint })}
                onToggleChecklist={(id, status) => checklistMutation.mutate({ itemId: id, status })}
              />
            ) : (
              <MentorPanel
                onReview={(id, decision, comment) => reviewMutation.mutate({ id, decision, comment })}
                isReviewing={reviewMutation.isPending}
              />
            )}
          </div>
        </div>

        {/* Right Column: Chat Workspace */}
        <div className="lg:col-span-7">
          <ChatWorkspace
            onSendMessage={(content, hint) => sendMutation.mutate({ content, hint })}
            onConfirmAction={(requestId) => confirmMutation.mutate(requestId)}
            isSending={sendMutation.isPending}
          />
        </div>
      </main>

      <KnowledgeDrawer />
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <MainApp />
    </QueryClientProvider>
  )
}
