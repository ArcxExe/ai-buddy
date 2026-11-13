import { beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { Header } from '../components/Header'
import { OnboardingRoadmap } from '../components/OnboardingRoadmap'
import { ActionCardView } from '../components/ActionCardView'
import { RequestsPage } from '../pages/RequestsPage'
import { DashboardPage } from '../pages/DashboardPage'
import { Markdown } from '../components/ui/Markdown'
import { formatDate, formatMessageTime } from '../lib/format'
import { createTestClient, renderWithClient } from './query-utils'
import { queryKeys } from '../api/queries'
import { context, draft } from './fixtures'
let client = createTestClient()
const render = (ui: React.ReactNode) => renderWithClient(ui, client)
describe('Product components', () => {
  beforeEach(() => { client = createTestClient() })
  it('renders safe Markdown with headings, emphasis, lists, links and code', () => {
    render(<Markdown>{'# Заголовок\n\n**Важное** и *курсив* с `inline`\n\n- Первый пункт\n- Второй пункт\n\n1. Шаг один\n2. Шаг два\n\n~~~ts\nconst ready = true\n~~~\n\n[Документация](https://example.com)\n\n<script>alert(1)</script>\n\n![image](https://example.com/pixel)\n\n[Опасная ссылка](javascript:alert%281%29)'}</Markdown>)
    expect(screen.getByRole('heading', { name: 'Заголовок' })).toBeInTheDocument()
    expect(screen.getByText('Важное').tagName).toBe('STRONG')
    expect(screen.getByText('курсив').tagName).toBe('EM')
    expect(screen.getByText('inline').tagName).toBe('CODE')
    expect(screen.getByText('const ready = true')).toBeInTheDocument()
    expect(screen.getAllByRole('list')).toHaveLength(2)
    expect(screen.getByRole('link', { name: 'Документация' })).toHaveAttribute('rel', 'noopener noreferrer')
    expect(document.querySelector('script')).toBeNull()
    expect(document.querySelector('img')).toBeNull()
    expect(screen.queryByRole('link', { name: 'Опасная ссылка' })).toBeNull()
  })
  it('formats local dates, today, yesterday and invalid values', () => {
    const now = new Date(2026, 10, 13, 14, 0)
    expect(formatMessageTime(new Date(2026, 10, 13, 12, 45).toISOString(), now)).toBe('Сегодня, 12:45')
    expect(formatMessageTime(new Date(2026, 10, 12, 18, 30).toISOString(), now)).toBe('Вчера, 18:30')
    expect(formatMessageTime(new Date(2026, 9, 1, 10, 0).toISOString(), now)).toContain('окт')
    expect(formatMessageTime('invalid')).toBe('Дата не указана')
    expect(formatDate('invalid')).toBe('Дата не указана')
    expect(formatDate('2026-11-13T08:00:00Z')).toContain('2026')
  })
  it('changes demo role through the header control', () => {
    const onRole = vi.fn()
    render(<Header onRoleChange={onRole} />)
    fireEvent.change(screen.getByLabelText('Демонстрационная роль'), { target: { value: 'mentor' } })
    expect(onRole).toHaveBeenCalledWith('mentor')
  })
  it('shows all checklist statuses, descriptions and suggested prompts', () => {
    const onStatus = vi.fn(), onPrompt = vi.fn()
    client.setQueryData(queryKeys.context, { ...context, onboarding: {
      ...context.onboarding, next_recommended_step: context.onboarding.checklist[0],
      checklist: [...context.onboarding.checklist, { ...context.onboarding.checklist[0], id: 'second', title: 'Второй шаг', status: 'done' }],
    } })
    render(<OnboardingRoadmap onSelectPrompt={onPrompt} onToggleChecklist={onStatus} />)
    fireEvent.change(screen.getByLabelText('Статус: Запросить доступы'), { target: { value: 'in_progress' } })
    expect(onStatus).toHaveBeenCalledWith('step-1', 'in_progress')
    fireEvent.click(screen.getAllByText('Обсудить с помощником')[0])
    expect(onPrompt).toHaveBeenCalledWith('Запроси доступ')
    fireEvent.click(screen.getByText('Спросить AI-buddy'))
    expect(screen.getByText('Второй шаг')).toBeInTheDocument()
    expect(screen.getAllByText('Готово').length).toBeGreaterThan(0)
  })
  it('shows actual dashboard metrics and the completed onboarding state', () => {
    client.setQueryData(queryKeys.context, { ...context, onboarding: { ...context.onboarding, overall_progress: 100, checklist: context.onboarding.checklist.map(item => ({ ...item, status: 'done' })), next_recommended_step: undefined } })
    render(<DashboardPage />)
    expect(screen.getByRole('heading', { name: /Привет, Алексей/ })).toBeInTheDocument()
    expect(screen.getByText('Все шаги завершены')).toBeInTheDocument()
    expect(screen.getByText(/расчётная оценка/)).toBeInTheDocument()
  })
  it('renders action details with null, arrays, numbers and comments', () => {
    const { rerender } = render(<ActionCardView card={{ ...draft, details: null }} />)
    expect(screen.queryByText('Детали заявки')).not.toBeInTheDocument()
    rerender(<ActionCardView card={{ ...draft, request_type: 'mentor_1on1_summary', reviewer_comment: 'Хороший план', details: { questions: ['Какие метрики?', { detail: true }], count: 2, reason: 'Подготовка', empty: null } }} />)
    fireEvent.click(screen.getByText('Детали заявки'))
    expect(screen.getByText('Какие метрики?')).toBeInTheDocument()
    expect(screen.getByText('Хороший план')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })
  it('shows RBAC rejection and its real summary', () => {
    render(<ActionCardView card={{ ...draft, request_type: 'rbac_blocked', status: 'blocked_by_rbac', summary: 'Политика запрещает доступ' }} />)
    expect(screen.getByText('Заблокировано правами')).toBeInTheDocument()
    expect(screen.getByText('Политика запрещает доступ')).toBeInTheDocument()
  })
  it('confirms drafts and disables repeated confirmation', () => {
    const onConfirm = vi.fn()
    render(<ActionCardView card={draft} onConfirm={onConfirm} />)
    fireEvent.click(screen.getByText('Подтвердить и отправить'))
    expect(onConfirm).toHaveBeenCalledWith(draft.id)
  })
  it('allows mentor review with the typed comment and supports status filters', () => {
    const request = { ...draft, status: 'pending_approval' as const }
    client.setQueryData(queryKeys.context, { ...context, active_role: 'mentor', corporate_requests: [request] })
    const onReview = vi.fn()
    render(<RequestsPage onConfirm={vi.fn()} onReview={onReview} isBusy={false} />)
    fireEvent.change(screen.getByPlaceholderText('Необязательно'), { target: { value: 'Согласовано' } })
    fireEvent.click(screen.getByText('Согласовать'))
    expect(onReview).toHaveBeenCalledWith(draft.id, 'approved', 'Согласовано')
    fireEvent.click(screen.getByText('Отклонить'))
    expect(onReview).toHaveBeenCalledWith(draft.id, 'rejected', 'Согласовано')
    fireEvent.change(screen.getByLabelText('Фильтр заявок'), { target: { value: 'completed' } })
    expect(screen.getByText('Заявок с таким статусом нет')).toBeInTheDocument()
  })
  it('hides review actions from interns and confirmation for another role', () => {
    client.setQueryData(queryKeys.context, { ...context, corporate_requests: [{ ...draft, status: 'pending_approval' }, { ...draft, id: 'other', created_by_role: 'mentor' }] })
    render(<RequestsPage onConfirm={vi.fn()} onReview={vi.fn()} isBusy={false} />)
    expect(screen.queryByText('Согласовать')).not.toBeInTheDocument()
    expect(screen.queryByText('Подтвердить и отправить')).not.toBeInTheDocument()
  })
})
