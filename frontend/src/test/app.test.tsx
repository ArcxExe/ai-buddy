import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import React from 'react'
import App from '../App'
import * as api from '../api/client'

describe('App Root Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(api, 'fetchContext').mockResolvedValue({
      current_user: {
        id: 'usr-01',
        name: 'Алексей Смирнов',
        role: 'intern',
        role_title: 'Backend Go Intern',
        team: 'Авито Доставка',
        department: 'Транспорт и Логистика',
        mentor_name: 'Мария Козлова',
        manager_name: 'Дмитрий Волков',
        avatar_initials: 'АС',
        permissions: ['read'],
      },
      available_roles: [],
      onboarding: {
        overall_progress: 25,
        current_stage_id: 's1',
        stages: [
          { id: 's1', title: 'День 1', period: 'День 1', status: 'completed', progress_percent: 100 },
        ],
        checklist: [],
      },
      corporate_requests: [],
      metrics: {
        mentor_hours_baseline: 48,
        mentor_hours_saved: 1.5,
        routine_questions_resolved: 2,
        mcp_actions_executed: 1,
        rbac_violations_prevented: 0,
        avg_local_latency_ms: 3,
      },
      feature_flags: { mcp_enabled: true },
    })

    vi.spyOn(api, 'fetchMessages').mockResolvedValue([])
  })

  it('renders complete app without crash', async () => {
    render(<App />)

    expect(screen.getByText('AI-buddy')).toBeInTheDocument()
    expect(screen.getByText(/Демонстрационный прототип/i)).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getAllByText('25%')[0]).toBeInTheDocument()
    })
  })
})
