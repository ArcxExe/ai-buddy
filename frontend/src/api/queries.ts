import { useRef, useState } from 'react'
import { QueryClient, useIsMutating, useMutation, useMutationState, useQuery, useQueryClient } from '@tanstack/react-query'
import * as api from './client'
import type { AppContextResponse, ChatMessage, ChecklistStatus, CorporateRequest, ReviewDecision, Role } from '../types'
import { draftKey, draftStorage } from '../lib/drafts'

export const queryKeys = {
  context: ['context'] as const,
  messages: ['messages'] as const,
  knowledge: ['knowledge'] as const,
  knowledgeSearch: (role: Role | undefined, query: string) => ['knowledge', role, query] as const,
}
export const operationKey = ['workspace-operation'] as const

function uncertainResult(error: unknown) {
  return error instanceof api.ApiError && (error.status === 0 || error.status >= 500 || error.code === 'INVALID_RESPONSE')
}

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: 5000, retry: false, refetchOnWindowFocus: true },
      mutations: { retry: false },
    },
  })
}

export function useWorkspaceBusy() {
  return useIsMutating({ mutationKey: operationKey }) > 0
}

export function useAppContext() {
  const busy = useWorkspaceBusy()
  return useQuery({
    queryKey: queryKeys.context,
    // No role override: the backend owns the single shared demo role.
    queryFn: ({ signal }) => api.fetchContext(undefined, signal),
    enabled: !busy,
  })
}

export function useMessages() {
  const busy = useWorkspaceBusy()
  return useQuery({
    queryKey: queryKeys.messages,
    queryFn: ({ signal }) => api.fetchMessages(signal),
    enabled: !busy,
  })
}

export function useKnowledge(query: string, open: boolean) {
  const context = useAppContext()
  const writing = useWorkspaceBusy()
  const role = context.data?.active_role
  const transitions = useMutationState({
    filters: { mutationKey: operationKey },
    select: mutation => {
      const operation = mutation.state.variables as WorkspaceOperation | undefined
      const changesIdentity = operation?.type === 'role' || operation?.type === 'reset'
      return (changesIdentity && (mutation.state.status === 'pending' ||
        (mutation.state.status === 'error' && uncertainResult(mutation.state.error)))) ||
        (operation?.type === 'refresh' && (mutation.state.status === 'pending' || mutation.state.status === 'error'))
    },
  })
  // Older failed mutations remain in the mutation cache. Only the latest
  // operation defines whether identity is currently uncertain/reconciling.
  const busy = transitions.at(-1) ?? false
  const result = useQuery({
    queryKey: queryKeys.knowledgeSearch(role, query),
    queryFn: ({ signal }) => {
      if (!role) throw new Error('Сначала загрузите контекст пользователя.')
      return api.fetchKnowledge(role, query, signal)
    },
    enabled: open && !!role && !busy && !context.isFetching && !context.isError,
    // Never carry results across roles or searches.
    placeholderData: undefined,
    gcTime: 0,
  })
  return { ...result, data: busy || context.isError ? undefined : result.data, isIdentityUncertain: busy && !writing }
}

export type WorkspaceOperation =
  | { type: 'send'; content: string; hint?: string }
  | { type: 'checklist'; itemId: string; status: ChecklistStatus }
  | { type: 'role'; role: Role }
  | { type: 'confirm'; requestId: string }
  | { type: 'review'; requestId: string; decision: ReviewDecision; comment: string }
  | { type: 'reset' }
  | { type: 'refresh' }

export function useWorkspaceActions() {
  const client = useQueryClient()
  const locked = useRef(false)
  const [requiresRefresh, setRequiresRefresh] = useState(false)

  const refresh = async () => {
    const reads = await Promise.allSettled([
      client.fetchQuery({ queryKey: queryKeys.context, staleTime: 0, queryFn: ({ signal }) => api.fetchContext(undefined, signal) }),
      client.fetchQuery({ queryKey: queryKeys.messages, staleTime: 0, queryFn: ({ signal }) => api.fetchMessages(signal) }),
    ])
    const failure = reads.find(read => read.status === 'rejected')
    if (failure) throw failure.reason
  }

  const updateContext = (patch: Partial<AppContextResponse>) => {
    client.setQueryData<AppContextResponse>(queryKeys.context, old => old && { ...old, ...patch })
  }
  const syncCard = (request: CorporateRequest) => {
    client.setQueryData<ChatMessage[]>(queryKeys.messages, old => old?.map(message =>
      message.action_card?.id === request.id ? { ...message, action_card: request } : message,
    ))
  }

  const mutation = useMutation({
    mutationKey: operationKey,
    retry: false,
    mutationFn: async (operation: WorkspaceOperation) => {
      // Cancellation also discards a late result when a mocked/server request ignores AbortSignal.
      await client.cancelQueries({ predicate: query => operation.type === 'role' || operation.type === 'reset' || query.queryKey[0] !== 'knowledge' })
      if (operation.type === 'refresh') {
        await refresh()
        setRequiresRefresh(false)
        return
      }
      const context = client.getQueryData<AppContextResponse>(queryKeys.context)
      if (!context) throw new Error('Контекст пользователя ещё не загружен.')
      const role = context.active_role
      switch (operation.type) {
        case 'send': {
          const key = draftKey(context, 'chat')
          const submitted = draftStorage.read(key)
          const data = await api.sendMessage(role, operation.content, operation.hint)
          updateContext({ onboarding: data.onboarding, corporate_requests: data.corporate_requests, metrics: data.metrics })
          client.setQueryData<ChatMessage[]>(queryKeys.messages, old => [
            ...(old ?? []), data.user_message, data.assistant_message,
          ])
          // Presets may be sent while a different message is still being drafted.
          if (submitted.trim() === operation.content.trim()) draftStorage.clearIfUnchanged(key, submitted)
          break
        }
        case 'checklist':
          updateContext({ onboarding: await api.updateChecklistStatus(operation.itemId, operation.status) })
          break
        case 'confirm': {
          const data = await api.confirmCorporateAction(role, operation.requestId)
          updateContext({ onboarding: data.onboarding, corporate_requests: data.corporate_requests, metrics: data.metrics })
          syncCard(data.request)
          break
        }
        case 'review': {
          const key = draftKey(context, 'review', operation.requestId)
          const data = await api.reviewCorporateRequest(role, operation.requestId, operation.decision, operation.comment)
          updateContext({ corporate_requests: data.corporate_requests })
          syncCard(data.request)
          draftStorage.clearIfUnchanged(key, operation.comment)
          break
        }
        case 'role': {
          const data = await api.switchRole(operation.role)
          await client.cancelQueries()
          client.removeQueries({ queryKey: queryKeys.knowledge })
          updateContext({ active_role: data.active_role, current_user: data.current_user })
          // Explicit fetches also work when React batches a fast mutation without rendering pending.
          // Keep writes locked until the shared backend context/history have been reconciled.
          // The write already succeeded. A subsequent read failure must not be
          // presented as a failed role switch or cause that write to be repeated.
          const reads = await Promise.allSettled([
            client.fetchQuery({ queryKey: queryKeys.context, staleTime: 0, queryFn: ({ signal }) => api.fetchContext(undefined, signal) }),
            client.fetchQuery({ queryKey: queryKeys.messages, staleTime: 0, queryFn: ({ signal }) => api.fetchMessages(signal) }),
          ])
          if (reads.some(read => read.status === 'rejected')) {
            setRequiresRefresh(true)
            return { warning: 'Роль изменена, но не все данные удалось загрузить. Обновите данные перед следующим действием.' }
          }
          break
        }
        case 'reset': {
          const data = await api.resetChat()
          await client.cancelQueries()
          // Keep the old screen on failure; replace caches only after successful reset.
          client.removeQueries({ queryKey: queryKeys.knowledge })
          client.setQueryData(queryKeys.context, data)
          client.setQueryData<ChatMessage[]>(queryKeys.messages, [])
          break
        }
      }
    },
    onError: (error, operation) => {
      if (operation.type !== 'refresh' && uncertainResult(error)) setRequiresRefresh(true)
    },
    onSettled: () => { locked.current = false },
  })

  const execute = (operation: WorkspaceOperation) => {
    // The synchronous lock covers repeated clicks before React rerenders pending state.
    if (locked.current || client.isMutating({ mutationKey: operationKey }) > 0) return
    if (operation.type !== 'refresh') {
      if (requiresRefresh || !client.getQueryData(queryKeys.context)) return
      if (client.getQueryState(queryKeys.context)?.status === 'error') return
      if (operation.type === 'send' && (!client.getQueryData(queryKeys.messages) || client.getQueryState(queryKeys.messages)?.status === 'error')) return
    }
    locked.current = true
    mutation.mutate(operation)
  }
  return { ...mutation, execute, requiresRefresh }
}
