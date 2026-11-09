import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import React from 'react'
import { Header } from '../components/Header'
import { OnboardingRoadmap } from '../components/OnboardingRoadmap'
import { ActionCardView } from '../components/ActionCardView'
import { MentorPanel } from '../components/MentorPanel'
import { useAppStore } from '../store/useAppStore'
import { CorporateRequest } from '../types'

describe('Component Tests', () => {
  it('renders Header with brand and role buttons', () => {
    const handleRole = vi.fn()
    const handleReset = vi.fn()
    render(<Header onRoleChange={handleRole} onReset={handleReset} />)

    expect(screen.getByText('AI-buddy')).toBeInTheDocument()
    expect(screen.getByText('Avito Start')).toBeInTheDocument()

    const mentorBtn = screen.getByText('Наставник (Мария)')
    fireEvent.click(mentorBtn)
    expect(handleRole).toHaveBeenCalledWith('mentor')
  })

  it('renders OnboardingRoadmap with progress and checklist', () => {
    useAppStore.setState({
      onboarding: {
        overall_progress: 40,
        current_stage_id: 'stage-1',
        stages: [
          { id: 's1', title: 'День 1', period: 'День 1', status: 'completed', progress_percent: 100 },
        ],
        checklist: [
          {
            id: 'item-1',
            stage_id: 's1',
            title: 'Запросить доступы',
            description: 'В Avito Access',
            status: 'todo',
            category: 'access',
            estimated_time: '10 мин',
            suggested_prompt: 'Запроси доступ',
          },
        ],
      },
    })

    const onPrompt = vi.fn()
    const onToggle = vi.fn()
    render(<OnboardingRoadmap onSelectPrompt={onPrompt} onToggleChecklist={onToggle} />)

    expect(screen.getByText('Дорожная карта адаптации')).toBeInTheDocument()
    expect(screen.getByText('40%')).toBeInTheDocument()
    expect(screen.getByText('Запросить доступы')).toBeInTheDocument()
  })

  it('renders ActionCardView for RBAC blocked resource', () => {
    const card: CorporateRequest = {
      id: 'req-rbac',
      service_name: 'Avito Access / IDM',
      tool_name: 'request_service_access',
      request_type: 'rbac_blocked',
      title: 'Заблокирован доступ к prod-billing-db',
      summary: 'Политика RBAC запрещает доступ стажёрам',
      status: 'blocked_by_rbac',
      created_by_role: 'intern',
      target_resource: 'prod-billing-db',
      created_at: '2026-11-09 12:00:00',
      details: {},
    }

    render(<ActionCardView card={card} />)
    expect(screen.getByText(/RBAC Guardrail/i)).toBeInTheDocument()
    expect(screen.getByText('Заблокирован доступ к prod-billing-db')).toBeInTheDocument()
  })

  it('renders ActionCardView draft and triggers confirm', () => {
    const onConfirm = vi.fn()
    const card: CorporateRequest = {
      id: 'abs-1',
      service_name: 'Avito Absence',
      tool_name: 'create_absence_request',
      request_type: 'absence_request',
      title: 'Учебный отгул на 18 ноября',
      summary: 'Экзамен в вузе',
      status: 'draft_confirmation_required',
      created_by_role: 'intern',
      target_resource: 'absence:study_day_off',
      created_at: '2026-11-09 12:00:00',
      details: {},
    }

    render(<ActionCardView card={card} onConfirm={onConfirm} />)
    expect(screen.getByText('Черновик (требует подтверждения)')).toBeInTheDocument()

    const confirmBtn = screen.getByText('Подтвердить и отправить')
    fireEvent.click(confirmBtn)
    expect(onConfirm).toHaveBeenCalledWith('abs-1')
  })

  it('renders MentorPanel with approval buttons', () => {
    const onReview = vi.fn()
    useAppStore.setState({
      activeRole: 'mentor',
      corporateRequests: [
        {
          id: 'abs-pending',
          service_name: 'Avito Absence',
          tool_name: 'create_absence_request',
          request_type: 'absence_request',
          title: 'Заявка на отгул',
          summary: 'Прошу согласовать',
          status: 'pending_approval',
          created_by_role: 'intern',
          target_resource: 'absence',
          created_at: '2026-11-09 12:00:00',
          details: {},
        },
      ],
      metrics: {
        mentor_hours_baseline: 48,
        mentor_hours_saved: 2.5,
        routine_questions_resolved: 4,
        mcp_actions_executed: 3,
        rbac_violations_prevented: 1,
        avg_local_latency_ms: 2,
      },
    })

    render(<MentorPanel onReview={onReview} isReviewing={false} />)
    expect(screen.getByText('Рабочее место наставника (Мария Козлова)')).toBeInTheDocument()
    expect(screen.getByText('+2.5 ч')).toBeInTheDocument()

    const approveBtn = screen.getByText('Согласовать')
    fireEvent.click(approveBtn)
    expect(onReview).toHaveBeenCalledWith('abs-pending', 'approved', expect.any(String))
  })
})
