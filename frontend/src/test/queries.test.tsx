import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react'
import App from '../App'
import * as api from '../api/client'
import { createQueryClient, queryKeys, useWorkspaceActions } from '../api/queries'
import { useAppStore } from '../store/useAppStore'
import { context, draft, message, knowledge } from './fixtures'
import { createTestClient, deferred, withClient } from './query-utils'
import type { AppContextResponse, ChatMessage, ChatResponse, ConfirmActionResponse, KnowledgeListResponse, SwitchRoleResponse } from '../types'

describe('Query cache consistency and request ordering', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.history.replaceState({}, '', '/chat')
    useAppStore.setState(useAppStore.getInitialState(), true)
    vi.spyOn(api, 'fetchContext').mockResolvedValue(context)
    vi.spyOn(api, 'fetchMessages').mockResolvedValue([])
    vi.spyOn(api, 'fetchKnowledge').mockResolvedValue({ ...knowledge, items: [] })
  })

  it('configures writes without automatic retries', () => {
    const client = createQueryClient()
    expect(client.getDefaultOptions().mutations?.retry).toBe(false)
  })

  it('takes the initial role and profile from the global backend context', async () => {
    const client = createTestClient(false)
    vi.spyOn(api, 'fetchContext').mockResolvedValue({
      ...context, active_role: 'manager', current_user: { ...context.current_user, role: 'manager', avatar_initials: 'ДВ' },
    })
    render(<App client={client} />)
    expect((await screen.findAllByText('ДВ')).length).toBeGreaterThan(0)
    expect(screen.getByLabelText('Демонстрационная роль')).toHaveValue('manager')
    expect(api.fetchContext).toHaveBeenCalledWith(undefined, expect.any(AbortSignal))
    expect(client.getQueryCache().findAll({ queryKey: queryKeys.context })).toHaveLength(1)
  })

  it('blocks duplicate clicks synchronously and serializes writes during a long LLM call', async () => {
    const client = createTestClient()
    const pending = deferred<ChatResponse>()
    const send = vi.spyOn(api, 'sendMessage').mockReturnValue(pending.promise)
    const reset = vi.spyOn(api, 'resetChat')
    const { result } = renderHook(useWorkspaceActions, { wrapper: withClient(client) })
    act(() => {
      result.current.execute({ type: 'send', content: 'Вопрос' })
      result.current.execute({ type: 'send', content: 'Вопрос' })
      result.current.execute({ type: 'reset' })
    })
    await waitFor(() => expect(send).toHaveBeenCalledTimes(1))
    expect(reset).not.toHaveBeenCalled()
    expect(client.getQueryData(queryKeys.messages)).toEqual([])
    await act(async () => pending.resolve({
      user_message: { ...message, id: 'user-1', sender: 'user', action_card: undefined },
      assistant_message: message, onboarding: context.onboarding, corporate_requests: [draft], metrics: context.metrics,
    }))
    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(client.getQueryData<ChatMessage[]>(queryKeys.messages)).toHaveLength(2)
  })

  it('disables all write controls while LLM is pending and shows no streaming steps', async () => {
    const client = createTestClient()
    const pending = deferred<ChatResponse>()
    vi.spyOn(api, 'sendMessage').mockReturnValue(pending.promise)
    render(<App client={client} />)
    fireEvent.click(screen.getByText('Оформить отгул'))
    await screen.findByText('AI-buddy готовит ответ')
    expect(screen.getByLabelText('Демонстрационная роль')).toBeDisabled()
    expect(screen.getByText('Сбросить демо').closest('button')).toBeDisabled()
    expect(screen.getByLabelText('Сообщение AI-buddy')).toBeDisabled()
    expect(screen.queryByText(/Как подготовлен ответ/)).not.toBeInTheDocument()
    await act(async () => pending.reject(new api.ApiError('LLM недоступен', 'AGENT_ERROR', 500)))
    expect(await screen.findByRole('alert')).toHaveTextContent('Сервис временно недоступен')
    expect(screen.queryByText('LLM недоступен')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Демонстрационная роль')).toBeDisabled()
    fireEvent.click(screen.getByText('Обновить данные'))
    await screen.findByText('Данные обновлены.')
    expect(screen.getByLabelText('Демонстрационная роль')).not.toBeDisabled()
    expect(api.sendMessage).toHaveBeenCalledTimes(1)
  })

  it('does not retry failed mutations even when the client default allows retries', async () => {
    const client = createTestClient()
    client.setDefaultOptions({ mutations: { retry: 3, retryDelay: 0 }, queries: { staleTime: Infinity, retry: false } })
    const update = vi.spyOn(api, 'updateChecklistStatus').mockRejectedValue(new Error('Ошибка изменения'))
    const { result } = renderHook(useWorkspaceActions, { wrapper: withClient(client) })
    act(() => result.current.execute({ type: 'checklist', itemId: 'step-1', status: 'done' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(update).toHaveBeenCalledTimes(1)
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
  })

  it('keeps navigation and knowledge usable during a long LLM request', async () => {
    const client = createTestClient()
    const pending = deferred<ChatResponse>()
    vi.spyOn(api, 'sendMessage').mockReturnValue(pending.promise)
    vi.spyOn(api, 'fetchKnowledge').mockResolvedValue(knowledge)
    render(<App client={client} />)
    fireEvent.click(screen.getByText('Оформить отгул'))
    await screen.findByText('AI-buddy готовит ответ')
    fireEvent.click(screen.getByRole('link', { name: 'База знаний' }))
    await screen.findByText(knowledge.items[0].title)
    expect(screen.getByLabelText('Поиск по базе знаний')).not.toBeDisabled()
    await act(async () => pending.resolve({
      user_message: { ...message, id: 'user-long', sender: 'user', content: 'Как оформить отгул на экзамен в университете?', action_card: undefined },
      assistant_message: message, onboarding: context.onboarding, corporate_requests: [draft], metrics: context.metrics,
    }))
    fireEvent.click(screen.getByRole('link', { name: 'ИИ-помощник' }))
    await screen.findByText(message.content)
    expect(client.getQueryData<ChatMessage[]>(queryKeys.messages)).toHaveLength(2)
  })

  it('prevents writes until context and history have loaded', () => {
    const client = createTestClient(false)
    const send = vi.spyOn(api, 'sendMessage')
    const { result } = renderHook(useWorkspaceActions, { wrapper: withClient(client) })
    act(() => result.current.execute({ type: 'send', content: 'hello' }))
    expect(send).not.toHaveBeenCalled()
  })

  it('does not restore an old context after a checklist mutation', async () => {
    const client = createTestClient()
    const old = deferred<AppContextResponse>()
    vi.spyOn(api, 'fetchContext').mockReturnValue(old.promise)
    vi.spyOn(api, 'updateChecklistStatus').mockResolvedValue({ ...context.onboarding, overall_progress: 100 })
    render(<App client={client} />)
    let refetch!: Promise<void>
    act(() => { refetch = client.refetchQueries({ queryKey: queryKeys.context }) })
    await waitFor(() => expect(api.fetchContext).toHaveBeenCalled())
    fireEvent.click(screen.getByRole('link', { name: 'Моя адаптация' }))
    fireEvent.change(await screen.findByLabelText('Статус: Запросить доступы'), { target: { value: 'done' } })
    await screen.findAllByText('100%')
    await act(async () => { old.resolve(context); await refetch })
    expect(client.getQueryData<AppContextResponse>(queryKeys.context)?.onboarding.overall_progress).toBe(100)
    expect(screen.queryByText('25%')).not.toBeInTheDocument()
  })

  it('reset replaces all snapshots and discards late context, history and knowledge responses', async () => {
    const client = createTestClient()
    const manager: AppContextResponse = {
      ...context, active_role: 'manager', current_user: { ...context.current_user, role: 'manager', avatar_initials: 'ДВ' },
      corporate_requests: [draft],
    }
    client.setQueryData(queryKeys.context, manager)
    client.setQueryData(queryKeys.messages, [message])
    const oldContext = deferred<AppContextResponse>()
    const oldHistory = deferred<ChatMessage[]>()
    const oldKnowledge = deferred<KnowledgeListResponse>()
    vi.spyOn(api, 'fetchContext').mockReturnValueOnce(oldContext.promise)
    vi.spyOn(api, 'fetchMessages').mockReturnValueOnce(oldHistory.promise)
    vi.spyOn(api, 'fetchKnowledge').mockReturnValueOnce(oldKnowledge.promise)
    const fresh: AppContextResponse = {
      ...context, onboarding: { ...context.onboarding, overall_progress: 0 },
      metrics: { ...context.metrics, mentor_hours_saved: 0 }, corporate_requests: [],
    }
    vi.spyOn(api, 'resetChat').mockResolvedValue(fresh)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    window.history.replaceState({}, '', '/knowledge')
    render(<App client={client} />)
    await waitFor(() => expect(api.fetchKnowledge).toHaveBeenCalled())
    let contextRead!: Promise<void>, historyRead!: Promise<void>
    act(() => {
      contextRead = client.refetchQueries({ queryKey: queryKeys.context })
      historyRead = client.refetchQueries({ queryKey: queryKeys.messages })
    })
    fireEvent.click(screen.getByText('Сбросить демо'))
    await screen.findAllByText('0%')
    expect(screen.getAllByText('АС').length).toBeGreaterThan(0)
    expect(client.getQueryData(queryKeys.context)).toEqual(fresh)
    expect(client.getQueryData(queryKeys.messages)).toEqual([])
    await act(async () => {
      oldContext.resolve(manager)
      oldHistory.resolve([message])
      oldKnowledge.resolve({ ...knowledge, active_role: 'manager' })
      await Promise.all([contextRead, historyRead])
    })
    expect(client.getQueryData(queryKeys.context)).toEqual(fresh)
    expect(client.getQueryData(queryKeys.messages)).toEqual([])
    expect(screen.queryByText(message.content)).not.toBeInTheDocument()
    expect(screen.queryByText(knowledge.items[0].title)).not.toBeInTheDocument()
    expect(client.getQueryCache().findAll({ queryKey: ['knowledge', 'manager'] })).toHaveLength(0)
  })

  it('failed reset preserves the profile, snapshots and chat', async () => {
    const client = createTestClient()
    client.setQueryData(queryKeys.messages, [message])
    vi.spyOn(api, 'resetChat').mockRejectedValue(new Error('Сброс недоступен'))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<App client={client} />)
    fireEvent.click(screen.getByText('Сбросить демо'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Сброс недоступен')
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
    expect(client.getQueryData(queryKeys.messages)).toEqual([message])
    expect(screen.getByText(message.content)).toBeInTheDocument()
  })

  it('cancels a read started while reset was already pending', async () => {
    const client = createTestClient()
    const pendingReset = deferred<AppContextResponse>()
    const oldRead = deferred<AppContextResponse>()
    vi.spyOn(api, 'resetChat').mockReturnValue(pendingReset.promise)
    vi.spyOn(api, 'fetchContext').mockReturnValue(oldRead.promise)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<App client={client} />)
    fireEvent.click(screen.getByText('Сбросить демо'))
    await waitFor(() => expect(api.resetChat).toHaveBeenCalledTimes(1))
    let read!: Promise<unknown>
    act(() => {
      read = client.fetchQuery({
        queryKey: queryKeys.context, staleTime: 0,
        queryFn: ({ signal }) => api.fetchContext(undefined, signal),
      }).catch(error => error)
    })
    await waitFor(() => expect(api.fetchContext).toHaveBeenCalledTimes(1))
    // An explicit cache refresh can start even though UI observers are disabled.
    const fresh = { ...context, onboarding: { ...context.onboarding, overall_progress: 0 } }
    await act(async () => pendingReset.resolve(fresh))
    await screen.findAllByText('0%')
    await act(async () => { oldRead.reject(new Error('Late stale error')) })
    await read
    expect(vi.mocked(api.fetchContext).mock.calls[0][1]?.aborted).toBe(true)
    expect(client.getQueryData(queryKeys.context)).toEqual(fresh)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows confirmation progress only on the target card and blocks repeated confirmation', async () => {
    const client = createTestClient()
    client.setQueryData(queryKeys.messages, [message, {
      ...message, id: 'msg-2', action_card: { ...draft, id: 'abs-2' },
    }])
    const pending = deferred<ConfirmActionResponse>()
    const confirm = vi.spyOn(api, 'confirmCorporateAction').mockReturnValue(pending.promise)
    render(<App client={client} />)
    fireEvent.click(screen.getAllByText('Подтвердить и отправить')[0])
    expect(await screen.findByText('Отправка...')).toBeInTheDocument()
    const second = screen.getByText('Подтвердить и отправить')
    expect(second.closest('button')).toBeDisabled()
    fireEvent.click(second)
    expect(confirm).toHaveBeenCalledTimes(1)
    const request = { ...draft, status: 'pending_approval' as const }
    await act(async () => pending.resolve({
      request, onboarding: context.onboarding, corporate_requests: [request], metrics: context.metrics,
    }))
    await screen.findByText('Ожидает согласования')
    expect(screen.queryByText('Отправка...')).not.toBeInTheDocument()
  })

  it('hides previous-role knowledge throughout a role switch and uses the backend response role', async () => {
    const client = createTestClient()
    const manager = { ...context, active_role: 'manager' as const, current_user: { ...context.current_user, role: 'manager' as const } }
    client.setQueryData(queryKeys.context, manager)
    vi.spyOn(api, 'fetchKnowledge').mockResolvedValueOnce({ ...knowledge, active_role: 'manager' })
    const pending = deferred<SwitchRoleResponse>()
    vi.spyOn(api, 'switchRole').mockReturnValue(pending.promise)
    window.history.replaceState({}, '', '/knowledge')
    render(<App client={client} />)
    await screen.findByText(knowledge.items[0].title)
    fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'intern' } })
    await waitFor(() => expect(screen.queryByText(knowledge.items[0].title)).not.toBeInTheDocument())
    await act(async () => pending.resolve({ active_role: 'intern', current_user: context.current_user }))
    await waitFor(() => expect(client.getQueryData<AppContextResponse>(queryKeys.context)?.active_role).toBe('intern'))
    await screen.findByText('Ничего не найдено')
    expect(api.fetchContext).toHaveBeenCalledWith(undefined, expect.any(AbortSignal))
    expect(api.fetchKnowledge).toHaveBeenLastCalledWith('intern', '', expect.any(AbortSignal))
  })

  it('discards a previous-role context refetch and refreshes global state after switching', async () => {
    const client = createTestClient()
    const old = deferred<AppContextResponse>()
    const mentor: AppContextResponse = {
      ...context, active_role: 'mentor', current_user: { ...context.current_user, role: 'mentor', avatar_initials: 'МК' },
      metrics: { ...context.metrics, mentor_hours_saved: 8 },
    }
    vi.spyOn(api, 'fetchContext').mockReturnValueOnce(old.promise).mockResolvedValue(mentor)
    vi.spyOn(api, 'switchRole').mockResolvedValue({ active_role: 'mentor', current_user: mentor.current_user })
    render(<App client={client} />)
    let read!: Promise<void>
    act(() => { read = client.refetchQueries({ queryKey: queryKeys.context }) })
    fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'mentor' } })
    await screen.findAllByText('МК')
    await waitFor(() => expect(client.getQueryData<AppContextResponse>(queryKeys.context)?.metrics.mentor_hours_saved).toBe(8))
    await act(async () => { old.resolve(context); await read })
    expect(client.getQueryData(queryKeys.context)).toEqual(mentor)
    expect(screen.queryAllByText('АС')).toHaveLength(0)
  })

  it('keeps the last successful screen on a refetch error and recovers without reloading', async () => {
    const client = createTestClient()
    vi.spyOn(api, 'fetchContext').mockRejectedValueOnce(new Error('Backend недоступен')).mockResolvedValue(context)
    render(<App client={client} />)
    await act(async () => { await client.refetchQueries({ queryKey: queryKeys.context }) })
    expect(await screen.findByRole('alert')).toHaveTextContent('Backend недоступен')
    expect(screen.getAllByText('25%').length).toBeGreaterThan(0)
    await act(async () => { await client.refetchQueries({ queryKey: queryKeys.context }) })
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })
})
