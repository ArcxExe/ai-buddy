import { StrictMode } from 'react'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import App from '../App'
import { ChatWorkspace } from '../components/ChatWorkspace'
import { ActionCardView } from '../components/ActionCardView'
import { draftKey, draftStorage, createDraftStorage } from '../lib/drafts'
import { queryKeys } from '../api/queries'
import * as api from '../api/client'
import { context, draft, message, knowledge } from './fixtures'
import { createTestClient, deferred, renderWithClient } from './query-utils'
import type { AppContextResponse, ChatResponse, ReviewResponse } from '../types'

const chatKey = draftKey(context, 'chat')!
const reviewKey = draftKey(context, 'review', draft.id)!
const pending = { ...draft, status: 'pending_approval' as const }
const reply: ChatResponse = {
  user_message: { ...message, id: 'user-1', sender: 'user', content: 'Мой вопрос' },
  assistant_message: message, onboarding: context.onboarding, corporate_requests: [draft], metrics: context.metrics,
}
const mentor: AppContextResponse = {
  ...context, active_role: 'mentor', current_user: { ...context.current_user, role: 'mentor' }, corporate_requests: [pending],
}
const chat = <ChatWorkspace onSendMessage={vi.fn()} onConfirmAction={vi.fn()} isSending={false} />

beforeEach(() => {
  vi.spyOn(api, 'fetchContext').mockResolvedValue(context)
  vi.spyOn(api, 'fetchMessages').mockResolvedValue([])
  vi.spyOn(api, 'fetchKnowledge').mockResolvedValue(knowledge)
})
afterEach(() => vi.restoreAllMocks())

describe('Draft storage', () => {
  it('stores only text, separates identities and request IDs, and creates no records for empty drafts', () => {
    const storage = createDraftStorage()
    expect(storage.read(chatKey)).toBe('')
    expect(window.sessionStorage.length).toBe(0)
    storage.write(chatKey, 'Только текст')
    expect(window.sessionStorage.getItem(chatKey)).toBe('Только текст')
    expect(draftKey(undefined, 'chat')).toBeUndefined()
    expect(draftKey(mentor, 'chat')).not.toBe(chatKey)
    expect(draftKey({ ...context, current_user: { ...context.current_user, id: 'other' } }, 'chat')).not.toBe(chatKey)
    expect(draftKey(context, 'review', 'other')).not.toBe(reviewKey)
    storage.write(undefined, 'Не сохранять без профиля')
    storage.write(reviewKey, '')
    expect(window.sessionStorage.length).toBe(1)
    storage.write(chatKey, '')
    expect(window.sessionStorage.length).toBe(0)
  })

  it('restores from sessionStorage after a fresh module instance, without a memory cache', () => {
    createDraftStorage().write(chatKey, 'После обновления страницы')
    expect(createDraftStorage().read(chatKey)).toBe('После обновления страницы')
  })

  it('survives denied reads and quota errors, and does not resurrect deleted text', () => {
    const storage = createDraftStorage()
    const get = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('Denied') })
    expect(storage.read(chatKey)).toBe('')
    const set = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Quota') })
    storage.write(chatKey, 'В памяти')
    expect(storage.read(chatKey)).toBe('В памяти')
    const remove = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { throw new Error('Denied') })
    storage.clearIfUnchanged(chatKey, 'В памяти')
    expect(storage.read(chatKey)).toBe('')
    get.mockRestore(); set.mockRestore(); remove.mockRestore()
  })

  it('survives an inaccessible sessionStorage property', () => {
    const storage = createDraftStorage()
    vi.spyOn(window, 'sessionStorage', 'get').mockImplementation(() => { throw new Error('SecurityError') })
    expect(storage.read(chatKey)).toBe('')
    storage.write(chatKey, 'Без хранилища')
    expect(storage.read(chatKey)).toBe('Без хранилища')
  })

  it('retains text when reads fail after a successful write', () => {
    const storage = createDraftStorage()
    storage.write(chatKey, 'Записано до сбоя чтения')
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('Read denied') })
    expect(storage.read(chatKey)).toBe('Записано до сбоя чтения')
    storage.write(chatKey, 'Новый текст во время сбоя')
    expect(storage.read(chatKey)).toBe('Новый текст во время сбоя')
  })
})

describe('Chat drafts', () => {
  it('saves immediately, restores through StrictMode/unmount and never sends automatically', () => {
    const send = vi.fn()
    const client = createTestClient()
    const view = renderWithClient(<StrictMode><ChatWorkspace onSendMessage={send} onConfirmAction={vi.fn()} isSending={false} /></StrictMode>, client)
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: '  Мой черновик\nВторая строка' } })
    expect(sessionStorage.getItem(chatKey)).toBe('  Мой черновик\nВторая строка')
    expect(client.getQueryData(queryKeys.messages)).toEqual([])
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
    view.unmount()
    renderWithClient(<ChatWorkspace onSendMessage={send} onConfirmAction={vi.fn()} isSending={false} />, createTestClient())
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('  Мой черновик\nВторая строка')
    expect(send).not.toHaveBeenCalled()
  })

  it('restores a browser-stored draft after a fresh application mount', async () => {
    sessionStorage.setItem(chatKey, 'Сохранено до обновления')
    window.history.replaceState({}, '', '/chat')
    render(<App client={createTestClient()} />)
    expect(await screen.findByLabelText('Сообщение AI-buddy')).toHaveValue('Сохранено до обновления')
    expect(sessionStorage.getItem(chatKey)).toBe('Сохранено до обновления')
  })

  it('keeps drafts through navigation and consumes route suggestions without overwriting them', async () => {
    window.history.replaceState({}, '', '/chat')
    render(<App client={createTestClient()} />)
    fireEvent.change(await screen.findByLabelText('Сообщение AI-buddy'), { target: { value: 'Мой черновик' } })
    const nav = within(screen.getByRole('navigation'))
    fireEvent.click(nav.getByText('Моя адаптация'))
    fireEvent.click(await screen.findByText('Обсудить с помощником'))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Мой черновик')
    fireEvent.click(nav.getByText('Главная'))
    fireEvent.click(nav.getByText('ИИ-помощник'))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Мой черновик')
  })

  it('consumes a suggested prompt once and does not restore it after a successful send', async () => {
    vi.spyOn(api, 'sendMessage').mockResolvedValue(reply)
    window.history.replaceState({}, '', '/onboarding')
    const client = createTestClient()
    render(<App client={client} />)
    fireEvent.click(await screen.findByText('Обсудить с помощником'))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Запроси доступ')
    expect(api.sendMessage).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    await waitFor(() => expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue(''))
    expect(sessionStorage.getItem(chatKey)).toBeNull()
    const nav = within(screen.getByRole('navigation'))
    fireEvent.click(nav.getByText('Главная'))
    await act(async () => window.history.back())
    await waitFor(() => expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue(''))
    expect(api.sendMessage).toHaveBeenCalledTimes(1)
  })

  it('does not erase a draft while the user context is loading, and isolates profile and role changes', async () => {
    sessionStorage.setItem(chatKey, 'Стажёр')
    const client = createTestClient(false)
    const load = deferred<AppContextResponse>()
    vi.spyOn(api, 'fetchContext').mockReturnValue(load.promise)
    renderWithClient(chat, client)
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('')
    expect(sessionStorage.getItem(chatKey)).toBe('Стажёр')
    await act(async () => load.resolve(context))
    await waitFor(() => expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Стажёр'))
    await act(async () => client.setQueryData(queryKeys.context, mentor))
    await waitFor(() => expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue(''))
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: 'Наставник' } })
    await act(async () => client.setQueryData(queryKeys.context, context))
    await waitFor(() => expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Стажёр'))
  })

  it('clears only after success even when chat is unmounted during the request', async () => {
    const result = deferred<ChatResponse>()
    vi.spyOn(api, 'sendMessage').mockReturnValue(result.promise)
    window.history.replaceState({}, '', '/chat')
    render(<App client={createTestClient()} />)
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: '  Мой вопрос  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    await waitFor(() => expect(api.sendMessage).toHaveBeenCalledTimes(1))
    expect(sessionStorage.getItem(chatKey)).toBe('  Мой вопрос  ')
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Главная'))
    await act(async () => result.resolve(reply))
    expect(sessionStorage.getItem(chatKey)).toBeNull()
    fireEvent.click(within(screen.getByRole('navigation')).getByText('ИИ-помощник'))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('')
  })

  it('preserves a failed send across navigation and allows an explicit retry', async () => {
    vi.spyOn(api, 'sendMessage').mockRejectedValueOnce(new Error('Сеть недоступна')).mockResolvedValue(reply)
    window.history.replaceState({}, '', '/chat')
    render(<App client={createTestClient()} />)
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: 'Мой вопрос' } })
    fireEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    await screen.findByText('Сеть недоступна')
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Заявки'))
    fireEvent.click(within(screen.getByRole('navigation')).getByText('ИИ-помощник'))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Мой вопрос')
    fireEvent.click(screen.getByRole('button', { name: 'Отправить сообщение' }))
    await waitFor(() => expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue(''))
    expect(api.sendMessage).toHaveBeenCalledTimes(2)
  })

  it.each([false, true])('preserves unrelated text after a preset or a newer edit (new edit: %s)', async newer => {
    const result = deferred<ChatResponse>()
    vi.spyOn(api, 'sendMessage').mockReturnValue(result.promise)
    window.history.replaceState({}, '', '/chat')
    render(<App client={createTestClient()} />)
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: 'Мой вопрос' } })
    fireEvent.click(newer ? screen.getByRole('button', { name: 'Отправить сообщение' }) : screen.getByText('Оформить отгул'))
    await waitFor(() => expect(api.sendMessage).toHaveBeenCalled())
    if (newer) act(() => draftStorage.write(chatKey, 'Более новый текст'))
    await act(async () => result.resolve(reply))
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue(newer ? 'Более новый текст' : 'Мой вопрос')
  })

  it('keeps editing and remounting usable when sessionStorage is blocked', () => {
    const blocked = vi.spyOn(window, 'sessionStorage', 'get').mockImplementation(() => { throw new Error('Blocked') })
    const view = renderWithClient(chat, createTestClient())
    fireEvent.change(screen.getByLabelText('Сообщение AI-buddy'), { target: { value: 'Резерв в памяти' } })
    view.unmount()
    renderWithClient(chat, createTestClient())
    expect(screen.getByLabelText('Сообщение AI-buddy')).toHaveValue('Резерв в памяти')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    blocked.mockRestore()
    act(() => draftStorage.write(chatKey, ''))
  })
})

describe('Request comment drafts', () => {
  it('restores a stored comment after reload without creating records for other cards', () => {
    const key = draftKey(mentor, 'review', draft.id)!
    sessionStorage.setItem(key, 'До обновления страницы')
    const client = createTestClient()
    client.setQueryData(queryKeys.context, mentor)
    renderWithClient(<><ActionCardView card={pending} onReview={vi.fn()} /><ActionCardView card={{ ...pending, id: 'untouched' }} onReview={vi.fn()} /></>, client)
    expect(screen.getAllByLabelText('Комментарий к решению')[0]).toHaveValue('До обновления страницы')
    expect(screen.getAllByLabelText('Комментарий к решению')[1]).toHaveValue('')
    expect(sessionStorage.length).toBe(1)
  })

  it('isolates requests, restores on remount, and separates both role and user ID', async () => {
    const client = createTestClient()
    client.setQueryData(queryKeys.context, mentor)
    const view = renderWithClient(<><ActionCardView card={pending} onReview={vi.fn()} /><ActionCardView card={{ ...pending, id: 'abs-2' }} onReview={vi.fn()} /></>, client)
    const inputs = screen.getAllByLabelText('Комментарий к решению')
    expect(sessionStorage.length).toBe(0)
    fireEvent.change(inputs[0], { target: { value: 'Первая заявка' } })
    fireEvent.change(inputs[1], { target: { value: 'Вторая заявка' } })
    expect(sessionStorage.length).toBe(2)
    await act(async () => client.setQueryData(queryKeys.context, { ...mentor, active_role: 'manager' }))
    await waitFor(() => expect(inputs[0]).toHaveValue(''))
    fireEvent.change(inputs[0], { target: { value: 'Руководитель' } })
    await act(async () => client.setQueryData(queryKeys.context, { ...mentor, current_user: { ...mentor.current_user, id: 'another-mentor' } }))
    await waitFor(() => expect(inputs[0]).toHaveValue(''))
    await act(async () => client.setQueryData(queryKeys.context, mentor))
    await waitFor(() => expect(inputs[0]).toHaveValue('Первая заявка'))
    expect(inputs[1]).toHaveValue('Вторая заявка')
    view.unmount()
    renderWithClient(<ActionCardView card={pending} onReview={vi.fn()} />, client)
    expect(screen.getByLabelText('Комментарий к решению')).toHaveValue('Первая заявка')
    expect(client.getQueryData(queryKeys.context)).toEqual(mentor)
  })

  it.each(['approved', 'rejected'] as const)('clears only the relevant comment after successful %s while away', async decision => {
    const result = deferred<ReviewResponse>()
    vi.spyOn(api, 'reviewCorporateRequest').mockReturnValue(result.promise)
    const client = createTestClient()
    client.setQueryData(queryKeys.context, mentor)
    window.history.replaceState({}, '', '/requests')
    render(<App client={client} />)
    const key = draftKey(mentor, 'review', draft.id)!
    const other = draftKey(mentor, 'review', 'abs-other')!
    draftStorage.write(other, 'Другой комментарий')
    fireEvent.change(screen.getByLabelText('Комментарий к решению'), { target: { value: 'Решение наставника' } })
    fireEvent.click(screen.getByText(decision === 'approved' ? 'Согласовать' : 'Отклонить'))
    await waitFor(() => expect(api.reviewCorporateRequest).toHaveBeenCalledWith('mentor', draft.id, decision, 'Решение наставника'))
    expect(sessionStorage.getItem(key)).toBe('Решение наставника')
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Главная'))
    const request = { ...pending, status: decision, reviewer_comment: 'Решение наставника' }
    await act(async () => result.resolve({ request, corporate_requests: [request] }))
    expect(sessionStorage.getItem(key)).toBeNull()
    expect(sessionStorage.getItem(other)).toBe('Другой комментарий')
  })

  it('retains a comment on API failure and restores it after filtering and navigation', async () => {
    vi.spyOn(api, 'reviewCorporateRequest').mockRejectedValue(new Error('Согласование недоступно'))
    const client = createTestClient()
    client.setQueryData(queryKeys.context, mentor)
    window.history.replaceState({}, '', '/requests')
    render(<App client={client} />)
    fireEvent.change(screen.getByLabelText('Комментарий к решению'), { target: { value: 'Повторить позже' } })
    fireEvent.click(screen.getByText('Согласовать'))
    await screen.findByText('Согласование недоступно')
    fireEvent.change(screen.getByLabelText('Фильтр заявок'), { target: { value: 'approved' } })
    fireEvent.change(screen.getByLabelText('Фильтр заявок'), { target: { value: 'all' } })
    expect(screen.getByLabelText('Комментарий к решению')).toHaveValue('Повторить позже')
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Главная'))
    fireEvent.click(within(screen.getByRole('navigation')).getByText('Заявки'))
    expect(screen.getByLabelText('Комментарий к решению')).toHaveValue('Повторить позже')
    expect(client.getQueryData(queryKeys.context)).toEqual(mentor)
  })

  it('does not clear newer comments or another role when an older review finishes', async () => {
    const result = deferred<ReviewResponse>()
    vi.spyOn(api, 'reviewCorporateRequest').mockReturnValue(result.promise)
    const client = createTestClient()
    client.setQueryData(queryKeys.context, mentor)
    window.history.replaceState({}, '', '/requests')
    render(<App client={client} />)
    const key = draftKey(mentor, 'review', draft.id)!
    const managerKey = draftKey({ ...mentor, active_role: 'manager' }, 'review', draft.id)!
    fireEvent.change(screen.getByLabelText('Комментарий к решению'), { target: { value: 'Старый комментарий' } })
    fireEvent.click(screen.getByText('Согласовать'))
    await waitFor(() => expect(api.reviewCorporateRequest).toHaveBeenCalled())
    act(() => {
      draftStorage.write(key, 'Новый комментарий')
      draftStorage.write(managerKey, 'Руководитель')
    })
    const request = { ...pending, status: 'approved' as const }
    await act(async () => result.resolve({ request, corporate_requests: [request] }))
    expect(sessionStorage.getItem(key)).toBe('Новый комментарий')
    expect(sessionStorage.getItem(managerKey)).toBe('Руководитель')
  })
})
