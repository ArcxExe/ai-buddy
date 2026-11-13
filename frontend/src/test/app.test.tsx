import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from '../App'
import * as api from '../api/client'
import { useAppStore } from '../store/useAppStore'
import { createTestClient } from './query-utils'
import { context, draft, message, knowledge } from './fixtures'
import { queryKeys } from '../api/queries'
import type { AppContextResponse, ChatMessage, CorporateRequest } from '../types'

let client = createTestClient()
const renderApp = (path = '/') => { window.history.replaceState({}, '', path); return render(<App client={client} />) }
const getContext = () => client.getQueryData<AppContextResponse>(queryKeys.context)
const getMessages = () => client.getQueryData<ChatMessage[]>(queryKeys.messages)
const error = () => new api.ApiError('Операция отклонена сервером', 'RBAC_FORBIDDEN', 403)
const send = () => fireEvent.click(screen.getByText('Оформить отгул'))
describe('Application routes and backend integration', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    client = createTestClient()
    useAppStore.setState(useAppStore.getInitialState(), true)
    vi.spyOn(api, 'fetchContext').mockResolvedValue(context)
    vi.spyOn(api, 'fetchMessages').mockResolvedValue([])
    vi.spyOn(api, 'fetchKnowledge').mockResolvedValue(knowledge)
  })
  it.each([['/', /Привет, Алексей/], ['/chat', 'ИИ-помощник'], ['/onboarding', 'Моя адаптация'], ['/knowledge', 'База знаний'], ['/requests', 'Заявки']] as const)(
    'renders direct route %s', async (path, title) => {
      renderApp(path)
      expect(await screen.findByRole('heading', { level: 1, name: title })).toBeInTheDocument()
      expect(screen.getByText('Демонстрационный прототип.')).toBeInTheDocument()
      expect(screen.getByRole('navigation', { name: 'Основная навигация' })).toBeInTheDocument()
    },
  )
  it('navigates all five sections, handles back/forward and unknown routes', async () => {
    renderApp()
    const nav = screen.getByRole('navigation')
    fireEvent.click(within(nav).getByText('ИИ-помощник'))
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('ИИ-помощник')
    fireEvent.click(within(nav).getByText('База знаний'))
    await screen.findByText(knowledge.items[0].title)
    await act(async () => window.history.back())
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('ИИ-помощник'))
    await act(async () => window.history.forward())
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('База знаний'))
    fireEvent.click(within(nav).getByText('Заявки'))
    expect(screen.getByText('Пока нет заявок')).toBeInTheDocument()
  })
  it('offers a working return link on an unknown URL', () => {
    renderApp('/missing')
    fireEvent.click(screen.getByRole('link', { name: 'На главную' }))
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Привет, Алексей')
  })
  it('opens and closes accessible mobile navigation with keyboard and links', () => {
    renderApp()
    const menu = screen.getByLabelText('Открыть меню')
    menu.focus()
    fireEvent.click(menu)
    const dialog = screen.getByRole('dialog', { name: 'Навигация приложения' })
    expect(within(dialog).getByLabelText('Закрыть меню')).toHaveFocus()
    const reset = within(dialog).getByText('Сбросить демо')
    reset.focus()
    fireEvent.keyDown(document, { key: 'Tab' })
    expect(within(dialog).getByRole('link', { name: /AI-buddy/ })).toHaveFocus()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(menu).toHaveFocus()
    fireEvent.click(menu)
    fireEvent.click(within(screen.getByRole('dialog')).getByText('Моя адаптация'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Моя адаптация')
    fireEvent.click(screen.getByLabelText('Открыть меню'))
    fireEvent.click(screen.getByLabelText('Закрыть навигацию'))
    expect(useAppStore.getState().isMobileNavOpen).toBe(false)
  })
  it('prepares a suggested prompt in chat without prematurely sending a request', () => {
    const sendMessage = vi.spyOn(api, 'sendMessage')
    renderApp('/onboarding')
    fireEvent.click(screen.getByText('Обсудить с помощником'))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Запроси доступ')
    expect(sendMessage).not.toHaveBeenCalled()
  })
  it('sends chat and consumes the response snapshots', async () => {
    vi.spyOn(api, 'sendMessage').mockResolvedValue({
      user_message: { ...message, id: 'user-1', sender: 'user', action_card: undefined },
      assistant_message: message, onboarding: { ...context.onboarding, overall_progress: 50 },
      corporate_requests: [draft], metrics: { ...context.metrics, mentor_hours_saved: 2 },
    })
    renderApp('/chat')
    send()
    await screen.findByText('Подтвердить и отправить')
    expect(getContext()?.onboarding.overall_progress).toBe(50)
    expect(getContext()?.corporate_requests).toEqual([draft])
    expect(screen.queryByText('AI-buddy готовит ответ')).not.toBeInTheDocument()
  })
  it('confirms from requests and updates the cached chat card', async () => {
    client.setQueryData(queryKeys.context, { ...context, corporate_requests: [draft] })
    client.setQueryData(queryKeys.messages, [message])
    const request: CorporateRequest = { ...draft, status: 'pending_approval' }
    vi.spyOn(api, 'confirmCorporateAction').mockResolvedValue({
      request, corporate_requests: [request], onboarding: { ...context.onboarding, overall_progress: 50 },
      metrics: { ...context.metrics, mentor_hours_saved: 3 },
    })
    renderApp('/requests')
    fireEvent.click(screen.getByText('Подтвердить и отправить'))
    await screen.findByText('Ожидает согласования')
    expect(getContext()?.corporate_requests).toEqual([request])
    expect(getMessages()?.[0].action_card).toEqual(request)
    expect(getContext()?.metrics.mentor_hours_saved).toBe(3)
  })
  it.each(['approved', 'rejected'] as const)('mentor reviews a request: %s', async decision => {
    const pending: CorporateRequest = { ...draft, status: 'pending_approval' }
    const request: CorporateRequest = { ...pending, status: decision, reviewer_comment: 'Решение' }
    client.setQueryData(queryKeys.context, { ...context, active_role: 'mentor', corporate_requests: [pending] })
    client.setQueryData(queryKeys.messages, [{ ...message, action_card: pending }])
    vi.spyOn(api, 'reviewCorporateRequest').mockResolvedValue({ request, corporate_requests: [request] })
    renderApp('/requests')
    fireEvent.click(screen.getByText(decision === 'approved' ? 'Согласовать' : 'Отклонить'))
    await waitFor(() => expect(getContext()?.corporate_requests).toEqual([request]))
    expect(getMessages()?.[0].action_card).toEqual(request)
  })
  it('uses the role and profile returned by the backend', async () => {
    const user = { ...context.current_user, role: 'mentor' as const, avatar_initials: 'МК' }
    vi.spyOn(api, 'switchRole').mockResolvedValue({ active_role: 'mentor', current_user: user })
    vi.spyOn(api, 'fetchContext').mockResolvedValue({ ...context, active_role: 'mentor', current_user: user })
    renderApp()
    fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'mentor' } })
    await waitFor(() => expect(getContext()?.active_role).toBe('mentor'))
    expect(getContext()?.current_user).toEqual(user)
    expect(screen.getByLabelText('Демонстрационная роль')).toHaveValue('mentor')
  })
  it('updates the checklist through all real statuses', async () => {
    const next = { ...context.onboarding, overall_progress: 100, checklist: context.onboarding.checklist.map(item => ({ ...item, status: 'done' as const })) }
    vi.spyOn(api, 'updateChecklistStatus').mockResolvedValue(next)
    renderApp('/onboarding')
    fireEvent.change(screen.getByLabelText('Статус: Запросить доступы'), { target: { value: 'done' } })
    await waitFor(() => expect(getContext()?.onboarding).toEqual(next))
    expect(screen.getByLabelText('Статус: Запросить доступы')).toHaveValue('done')
  })
  it('resets all server data without reloading or losing the current route', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    client.setQueryData(queryKeys.messages, [message])
    vi.spyOn(api, 'resetChat').mockResolvedValue({ ...context, onboarding: { ...context.onboarding, overall_progress: 0 } })
    renderApp('/chat')
    fireEvent.click(screen.getByText('Сбросить демо'))
    await screen.findByText('Демо сброшено. Можно начать заново.')
    expect(getMessages()).toEqual([])
    expect(getContext()?.onboarding.overall_progress).toBe(0)
    expect(window.location.pathname).toBe('/chat')
  })
  it('does not reset if the user cancels confirmation', () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const reset = vi.spyOn(api, 'resetChat')
    renderApp()
    fireEvent.click(screen.getByText('Сбросить демо'))
    expect(reset).not.toHaveBeenCalled()
  })
  it.each(['send', 'confirm', 'review', 'checklist', 'role', 'reset'] as const)(
    'shows the server error for %s without applying success state', async operation => {
      vi.spyOn(api, 'sendMessage').mockRejectedValue(error())
      vi.spyOn(api, 'confirmCorporateAction').mockRejectedValue(error())
      vi.spyOn(api, 'reviewCorporateRequest').mockRejectedValue(error())
      vi.spyOn(api, 'updateChecklistStatus').mockRejectedValue(error())
      vi.spyOn(api, 'switchRole').mockRejectedValue(error())
      vi.spyOn(api, 'resetChat').mockRejectedValue(error())
      vi.spyOn(window, 'confirm').mockReturnValue(true)
      if (operation === 'confirm') client.setQueryData(queryKeys.messages, [message])
      if (operation === 'review') client.setQueryData(queryKeys.context, { ...context, active_role: 'mentor', corporate_requests: [{ ...draft, status: 'pending_approval' }] })
      renderApp(operation === 'review' ? '/requests' : operation === 'checklist' ? '/onboarding' : '/chat')
      if (operation === 'send') send()
      if (operation === 'confirm') fireEvent.click(screen.getByText('Подтвердить и отправить'))
      if (operation === 'review') fireEvent.click(screen.getByText('Согласовать'))
      if (operation === 'checklist') fireEvent.change(screen.getByLabelText('Статус: Запросить доступы'), { target: { value: 'done' } })
      if (operation === 'role') fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'mentor' } })
      if (operation === 'reset') fireEvent.click(screen.getByText('Сбросить демо'))
      expect(await screen.findByRole('alert')).toHaveTextContent('Операция отклонена сервером')
      expect(getContext()?.onboarding).toEqual(context.onboarding)
      expect(screen.queryByText('Изменения сохранены.')).not.toBeInTheDocument()
    },
  )
  it.each(['context', 'messages'] as const)('reports initial query error: %s', async operation => {
    client = createTestClient(false)
    if (operation === 'context') vi.spyOn(api, 'fetchContext').mockRejectedValue(error())
    else vi.spyOn(api, 'fetchMessages').mockRejectedValue(error())
    renderApp()
    expect(await screen.findByRole('alert')).toHaveTextContent('Операция отклонена сервером')
  })
})
