import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from '../App'
import * as api from '../api/client'
import { queryKeys } from '../api/queries'
import { KnowledgePage } from '../pages/KnowledgePage'
import { context, knowledge, message } from './fixtures'
import { createTestClient, deferred, renderWithClient } from './query-utils'
import type { AppContextResponse, ChatMessage, KnowledgeListResponse } from '../types'

beforeEach(() => {
  vi.spyOn(api, 'fetchContext').mockResolvedValue(context)
  vi.spyOn(api, 'fetchMessages').mockResolvedValue([])
  vi.spyOn(api, 'fetchKnowledge').mockResolvedValue(knowledge)
})
afterEach(() => vi.restoreAllMocks())
function mount(path: string, client = createTestClient()) {
  window.history.replaceState({}, '', path)
  render(<App client={client} />)
  return client
}

describe('Page loading and recovery', () => {
  it.each([
    ['/', 'Загрузка главной страницы'], ['/chat', 'Загрузка истории сообщений'],
    ['/onboarding', 'Загрузка адаптации'], ['/knowledge', 'Загрузка базы знаний'], ['/requests', 'Загрузка заявок'],
  ])('shows an accessible skeleton for %s, without a premature empty result', async (path, label) => {
    const load = deferred<AppContextResponse>()
    const history = deferred<ChatMessage[]>()
    vi.spyOn(api, 'fetchContext').mockReturnValue(load.promise)
    vi.spyOn(api, 'fetchMessages').mockReturnValue(history.promise)
    const client = createTestClient(false)
    mount(path, client)
    expect(await screen.findByRole('status', { name: label })).toBeInTheDocument()
    expect(screen.queryByText('Пока нет заявок')).not.toBeInTheDocument()
    expect(screen.queryByText('Начнём с вашего вопроса?')).not.toBeInTheDocument()
    expect(screen.getByRole('navigation')).toBeInTheDocument()
    await act(async () => { load.resolve(context); history.resolve([]) })
    await waitFor(() => expect(screen.queryByRole('status', { name: label })).not.toBeInTheDocument())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps other sections and checklist writes usable when only history fails', async () => {
    const client = createTestClient()
    client.removeQueries({ queryKey: queryKeys.messages })
    vi.spyOn(api, 'fetchMessages').mockRejectedValue(new api.ApiError('Не удалось связаться с сервером', 'NETWORK_ERROR', 0))
    const update = vi.spyOn(api, 'updateChecklistStatus').mockResolvedValue({ ...context.onboarding, overall_progress: 100 })
    mount('/onboarding', client)
    await screen.findByRole('alert')
    expect(await screen.findByLabelText('Статус: Запросить доступы')).not.toBeDisabled()
    fireEvent.change(screen.getByLabelText('Статус: Запросить доступы'), { target: { value: 'done' } })
    await waitFor(() => expect(update).toHaveBeenCalledTimes(1))
    await screen.findAllByText('100%')
    expect(screen.queryByText('Изменения сохранены.')).not.toBeInTheDocument()
    expect(client.getQueryData<AppContextResponse>(queryKeys.context)?.onboarding.overall_progress).toBe(100)
  })

  it.each([400, 403, 404, 500])('shows HTTP %s failures without a false success and retries only reads', async status => {
    const client = createTestClient(false)
    const text = status === 400 ? 'Проверьте параметры запроса' : status === 403 ? 'Недостаточно прав' : status === 404 ? 'Данные не найдены' : 'SQL private stack trace'
    vi.spyOn(api, 'fetchContext').mockRejectedValueOnce(new api.ApiError(text, 'SERVER_ERROR', status)).mockResolvedValue(context)
    mount('/', client)
    expect(await screen.findByRole('alert')).toHaveTextContent(status === 500 ? 'Сервис временно недоступен' : text)
    expect(screen.queryByText('SQL private stack trace')).not.toBeInTheDocument()
    expect(screen.queryByText('Изменения сохранены.')).not.toBeInTheDocument()
    expect(screen.queryByRole('status', { name: 'Загрузка главной страницы' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }))
    await screen.findByRole('heading', { name: /Привет, Алексей/ })
    expect(api.fetchContext).toHaveBeenCalledTimes(2)
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
  })

  it('retains the last history during a refetch failure, then recovers via the read button', async () => {
    const client = createTestClient()
    client.setQueryData(queryKeys.messages, [message])
    vi.spyOn(api, 'fetchMessages').mockRejectedValueOnce(new api.ApiError('Связь прервана', 'NETWORK_ERROR', 0)).mockResolvedValue([message])
    mount('/chat', client)
    await act(async () => { await client.refetchQueries({ queryKey: queryKeys.messages }) })
    expect(await screen.findByRole('alert')).toHaveTextContent('Связь прервана')
    expect(screen.getByText(message.content)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Отправить сообщение' })).toBeDisabled()
    fireEvent.click(screen.getByText('Повторить'))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
    expect(screen.getByText(message.content)).toBeInTheDocument()
  })

  it.each([400, 403, 404, 500])('keeps internal HTTP %s details out of the public message', status => {
    const error = new api.ApiError('private SQL/resource identifier and stack trace', 'INTERNAL_DETAIL', status)
    expect(api.getErrorMessage(error)).not.toContain('private')
    expect(api.getErrorMessage(error)).not.toContain('SQL')
    // Keep the original exception available for development diagnostics.
    expect(error.message).toContain('private SQL')
  })

  it('keeps same-role articles and the open reader through background refresh and failure', async () => {
    const update = deferred<KnowledgeListResponse>()
    vi.spyOn(api, 'fetchKnowledge').mockResolvedValueOnce(knowledge).mockReturnValueOnce(update.promise).mockResolvedValue(knowledge)
    renderWithClient(<KnowledgePage />, createTestClient(), '/knowledge?article=kb-1')
    await screen.findByText('Полный текст')
    fireEvent.click(screen.getByText('Найти'))
    await screen.findByText('Обновляем материалы. Последний результат остаётся доступен.')
    expect(screen.getByText('Полный текст')).toBeInTheDocument()
    await act(async () => update.reject(new api.ApiError('Internal KB exception', 'KB_ERROR', 500)))
    expect(await screen.findByRole('alert')).toHaveTextContent('Сервис временно недоступен')
    expect(screen.getByText('Полный текст')).toBeInTheDocument()
    expect(screen.queryByText('Internal KB exception')).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('Повторить'))
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('keeps role-filtered knowledge during safe context refresh but hides it if identity cannot be refreshed', async () => {
    const load = deferred<AppContextResponse>()
    const client = createTestClient()
    renderWithClient(<KnowledgePage />, client)
    await screen.findByText(knowledge.items[0].title)
    vi.spyOn(api, 'fetchContext').mockReturnValueOnce(load.promise)
    let read!: Promise<void>
    act(() => { read = client.refetchQueries({ queryKey: queryKeys.context }) })
    expect(screen.getByText(knowledge.items[0].title)).toBeInTheDocument()
    await act(async () => { load.reject(new Error('Context unavailable')); await read })
    await waitFor(() => expect(screen.queryByText(knowledge.items[0].title)).not.toBeInTheDocument())
  })

  it('does not call an unassigned checklist completed', () => {
    const client = createTestClient()
    client.setQueryData(queryKeys.context, { ...context, onboarding: { ...context.onboarding, checklist: [], stages: [] } })
    mount('/', client)
    expect(screen.getByText('Чек-лист пока не назначен')).toBeInTheDocument()
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Моя адаптация'))
    expect(screen.getByText('Чек-лист пока не назначен')).toBeInTheDocument()
    expect(screen.queryByText('Все шаги завершены')).not.toBeInTheDocument()
  })
})

describe('Mutation reconciliation and notices', () => {
  it.each(['role', 'reset'] as const)('keeps previous-role documents hidden after an uncertain %s until reconciliation', async operation => {
    const client = createTestClient()
    const manager = { ...context, active_role: 'manager' as const, current_user: { ...context.current_user, role: 'manager' as const } }
    client.setQueryData(queryKeys.context, manager)
    vi.spyOn(api, 'fetchKnowledge').mockImplementation(async role => ({ ...knowledge, active_role: role, items: role === 'manager' ? [{ ...knowledge.items[0], title: 'Закрытая статья', sensitivity: 'manager_only' }] : [] }))
    vi.spyOn(api, 'switchRole').mockRejectedValue(new api.ApiError('Связь прервана', 'NETWORK_ERROR', 0))
    vi.spyOn(api, 'resetChat').mockRejectedValue(new api.ApiError('Связь прервана', 'NETWORK_ERROR', 0))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    mount('/knowledge', client)
    await screen.findByText('Закрытая статья')
    if (operation === 'role') fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'intern' } })
    else fireEvent.click(screen.getByText('Сбросить демо'))
    await screen.findByText(/Результат операции не подтверждён/)
    expect(screen.queryByText('Закрытая статья')).not.toBeInTheDocument()
    expect(screen.queryByRole('status', { name: 'Загрузка базы знаний' })).not.toBeInTheDocument()
    expect(api.fetchKnowledge).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByText('Обновить данные'))
    await screen.findByText('Ничего не найдено')
    expect(screen.queryByText('Закрытая статья')).not.toBeInTheDocument()
    expect(api.fetchKnowledge).toHaveBeenLastCalledWith('intern', '', expect.any(AbortSignal))
  })

  it('distinguishes a successful role change from failed reads and refreshes without switching again', async () => {
    const mentor = { ...context, active_role: 'mentor' as const, current_user: { ...context.current_user, role: 'mentor' as const } }
    vi.spyOn(api, 'switchRole').mockResolvedValue({ active_role: 'mentor', current_user: mentor.current_user })
    vi.spyOn(api, 'fetchContext').mockRejectedValueOnce(new api.ApiError('DB down', 'ERROR', 500)).mockResolvedValue(mentor)
    mount('/')
    fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'mentor' } })
    await screen.findByText('Роль изменена, но не все данные удалось загрузить. Обновите данные перед следующим действием.')
    expect(screen.getByLabelText('Демонстрационная роль')).toHaveValue('mentor')
    expect(screen.queryByText('Изменения сохранены.')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Демонстрационная роль')).toBeDisabled()
    fireEvent.click(screen.getByText('Обновить данные'))
    await screen.findByText('Данные обновлены.')
    expect(api.switchRole).toHaveBeenCalledTimes(1)
    expect(screen.getByLabelText('Демонстрационная роль')).not.toBeDisabled()
  })

  it('handles a missing history after a successful role switch as partial success', async () => {
    vi.spyOn(api, 'switchRole').mockResolvedValue({ active_role: 'intern', current_user: context.current_user })
    vi.spyOn(api, 'fetchMessages').mockRejectedValueOnce(new api.ApiError('Network', 'NETWORK_ERROR', 0)).mockResolvedValue([])
    mount('/')
    fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'intern' } })
    await screen.findByText(/Роль изменена, но не все данные/)
    fireEvent.click(screen.getByText('Обновить данные'))
    await screen.findByText('Данные обновлены.')
    expect(api.switchRole).toHaveBeenCalledTimes(1)
  })

  it('requires read reconciliation after an ambiguous send and never resends it automatically', async () => {
    const send = vi.spyOn(api, 'sendMessage').mockRejectedValue(new api.ApiError('Связь прервана', 'NETWORK_ERROR', 0))
    const client = mount('/chat')
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: 'Не терять этот вопрос' } })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    await screen.findByText(/Результат операции не подтверждён/)
    fireEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    expect(send).toHaveBeenCalledTimes(1)
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Не терять этот вопрос')
    expect(client.getQueryData(queryKeys.messages)).toEqual([])
    vi.spyOn(api, 'fetchMessages').mockResolvedValue([message])
    fireEvent.click(screen.getByText('Обновить данные'))
    await screen.findByText('Данные обновлены.')
    expect(client.getQueryData(queryKeys.messages)).toEqual([message])
    expect(send).toHaveBeenCalledTimes(1)
    expect(screen.queryByText('Изменения сохранены.')).not.toBeInTheDocument()
  })

  it('keeps writes locked when reconciliation also fails, then allows another read retry', async () => {
    vi.spyOn(api, 'resetChat').mockRejectedValue(new api.ApiError('Internal exception', 'RESET_ERROR', 500))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(api, 'fetchContext').mockRejectedValueOnce(new api.ApiError('Still down', 'ERROR', 500)).mockResolvedValue(context)
    mount('/')
    fireEvent.click(screen.getByText('Сбросить демо'))
    await screen.findByText(/Результат операции не подтверждён/)
    fireEvent.click(screen.getByText('Обновить данные'))
    await waitFor(() => expect(api.fetchContext).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(screen.getByText('Обновить данные')).not.toBeDisabled())
    expect(screen.getByText('Сбросить демо')).toBeDisabled()
    fireEvent.click(screen.getByText('Обновить данные'))
    await screen.findByText('Данные обновлены.')
    expect(api.resetChat).toHaveBeenCalledTimes(1)
  })

  it('shows one success notice and removes it when a later read fails', async () => {
    const client = mount('/onboarding')
    vi.spyOn(api, 'updateChecklistStatus').mockResolvedValue({ ...context.onboarding, overall_progress: 100 })
    fireEvent.change(screen.getByLabelText('Статус: Запросить доступы'), { target: { value: 'done' } })
    await screen.findByText('Изменения сохранены.')
    act(() => client.setQueryData(queryKeys.context, { ...context, onboarding: { ...context.onboarding, overall_progress: 100 } }))
    expect(screen.getAllByText('Изменения сохранены.')).toHaveLength(1)
    vi.spyOn(api, 'fetchContext').mockRejectedValue(new api.ApiError('Disconnected', 'NETWORK_ERROR', 0))
    await act(async () => { await client.refetchQueries({ queryKey: queryKeys.context }) })
    expect(await screen.findByRole('alert')).toHaveTextContent('Disconnected')
    expect(screen.queryByText('Изменения сохранены.')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Статус: Запросить доступы')).toBeDisabled()
  })

  it('isolates a page rendering failure and recovers by navigation without losing the shell', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const client = createTestClient()
    // A render fault deliberately bypasses the API validator to exercise the boundary.
    client.setQueryData(queryKeys.context, { ...context, metrics: null })
    mount('/', client)
    expect(screen.getByRole('alert')).toHaveTextContent('Не удалось отобразить страницу')
    expect(screen.getByRole('navigation')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Попробовать снова'))
    expect(screen.getByRole('alert')).toHaveTextContent('Не удалось отобразить страницу')
    fireEvent.click(within(screen.getByRole('navigation')).getByText('База знаний'))
    await screen.findByText(knowledge.items[0].title)
    expect(screen.queryByText('Не удалось отобразить страницу')).not.toBeInTheDocument()
    act(() => client.setQueryData(queryKeys.context, context))
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Главная'))
    expect(screen.getByRole('heading', { name: /Привет, Алексей/ })).toBeInTheDocument()
  })
})
