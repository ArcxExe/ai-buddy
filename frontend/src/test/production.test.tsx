import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import App from '../App'
import * as api from '../api/client'
import { createTestClient } from './query-utils'
import { context, draft, knowledge } from './fixtures'
import { queryKeys } from '../api/queries'

beforeEach(() => {
  vi.stubEnv('DEV', false)
  vi.spyOn(api, 'fetchContext').mockResolvedValue(context)
  vi.spyOn(api, 'fetchMessages').mockResolvedValue([])
  vi.spyOn(api, 'fetchKnowledge').mockResolvedValue(knowledge)
})
afterEach(() => { vi.unstubAllEnvs(); vi.restoreAllMocks() })

describe('Production presentation', () => {
  it.each([
    ['/', /Привет, Алексей/], ['/chat', 'ИИ-помощник'], ['/onboarding', 'Моя адаптация'],
    ['/knowledge', 'База знаний'], ['/requests', 'Заявки'],
  ] as const)('renders %s without development controls and keeps the backend profile', async (route, title) => {
    window.history.replaceState({}, '', route)
    const client = createTestClient()
    render(<App client={client} />)
    expect(await screen.findByRole('heading', { level: 1, name: title })).toBeInTheDocument()
    expect(screen.queryByText('Демонстрационный прототип.')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Демонстрационная роль')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Сбросить демо' })).not.toBeInTheDocument()
    expect(screen.queryByText('Демо-метрики')).not.toBeInTheDocument()
    expect(screen.queryByText('Hack&Change 2026')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Роль пользователя')).toHaveTextContent('Стажёр')
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
  })

  it('keeps the three useful chat actions and hides the development RBAC attack preset', async () => {
    window.history.replaceState({}, '', '/chat')
    render(<App client={createTestClient()} />)
    await screen.findByLabelText('Сообщение AI-buddy')
    const actions = within(screen.getByLabelText('Быстрые сценарии'))
    expect(actions.getAllByRole('button')).toHaveLength(3)
    expect(actions.getByText('Получить доступ')).toBeInTheDocument()
    expect(actions.getByText('Оформить отгул')).toBeInTheDocument()
    expect(actions.getByText('Подготовить 1:1')).toBeInTheDocument()
    expect(actions.queryByText('Проверить права')).not.toBeInTheDocument()
    expect(screen.queryByText('Демо')).not.toBeInTheDocument()
    expect(screen.getByText('Проверяйте источники важных ответов')).toBeInTheDocument()
  })

  it('preserves server Mock names and explains their origin instead of presenting a live integration', async () => {
    window.history.replaceState({}, '', '/requests')
    const client = createTestClient()
    client.setQueryData(queryKeys.context, { ...context, corporate_requests: [draft] })
    render(<App client={client} />)
    expect(await screen.findByText(draft.service_name)).toBeInTheDocument()
    expect(screen.getByText('Карточки с пометкой Mock относятся к тестовым сервисам.')).toBeInTheDocument()
    expect(client.getQueryData(queryKeys.context)).toEqual({ ...context, corporate_requests: [draft] })
  })

  it('does not invent Mock labels for real service names and retains review rights from server context', async () => {
    window.history.replaceState({}, '', '/requests')
    const request = { ...draft, status: 'pending_approval', service_name: 'Corporate service' }
    const client = createTestClient()
    client.setQueryData(queryKeys.context, {
      ...context, active_role: 'mentor', current_user: { ...context.current_user, role: 'mentor' }, corporate_requests: [request],
    })
    render(<App client={client} />)
    expect(await screen.findByText('Corporate service')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Согласовать' })).toBeInTheDocument()
    expect(screen.queryByText(/Карточки с пометкой Mock/)).not.toBeInTheDocument()
    expect(screen.getByLabelText('Роль пользователя')).toHaveTextContent('Наставник')
  })

  it('keeps development role switching and reset available for existing local workflows', async () => {
    vi.stubEnv('DEV', true)
    window.history.replaceState({}, '', '/')
    render(<App client={createTestClient()} />)
    await screen.findByRole('heading', { name: /Привет, Алексей/ })
    expect(screen.getByLabelText('Демонстрационная роль')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Сбросить демо' })).toBeInTheDocument()
  })
})
