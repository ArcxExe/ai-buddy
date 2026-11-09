import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import React from 'react'
import { ChatWorkspace } from '../components/ChatWorkspace'
import { KnowledgeDrawer } from '../components/KnowledgeDrawer'
import { useAppStore } from '../store/useAppStore'
import * as api from '../api/client'

describe('ChatWorkspace and KnowledgeDrawer', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    useAppStore.setState({
      messages: [],
      isAgentTyping: false,
      isKnowledgeDrawerOpen: false,
      activeRole: 'intern',
    })
  })

  it('renders quick presets and triggers send on click', () => {
    const onSend = vi.fn()
    const onConfirm = vi.fn()

    render(<ChatWorkspace onSendMessage={onSend} onConfirmAction={onConfirm} isSending={false} />)

    const presetBtn = screen.getByText(/Сценарий 1: Доступы в IDM/i)
    fireEvent.click(presetBtn)

    expect(onSend).toHaveBeenCalledWith(
      expect.stringContaining('Какие доступы'),
      'scenario_01_access_and_rbac',
    )
  })

  it('submits text from input field', () => {
    const onSend = vi.fn()
    const onConfirm = vi.fn()

    render(<ChatWorkspace onSendMessage={onSend} onConfirmAction={onConfirm} isSending={false} />)

    const input = screen.getByPlaceholderText(/Задай вопрос/i)
    fireEvent.change(input, { target: { value: 'Тестовый вопрос' } })

    const submitBtn = screen.getByText('Отправить')
    fireEvent.click(submitBtn)

    expect(onSend).toHaveBeenCalledWith('Тестовый вопрос')
  })

  it('displays message with processing steps and citations', () => {
    useAppStore.setState({
      messages: [
        {
          id: 'msg-1',
          sender: 'assistant',
          role_context: 'intern',
          content: 'Ответ помощника',
          created_at: '2026-11-09 12:00:00',
          processing_steps: ['Шаг 1: Поиск в Playbook'],
          citations: [
            { id: 'c1', title: 'AvitoTech Guide', source: 'Playbook', sensitivity: 'intern' },
          ],
        },
      ],
    })

    render(<ChatWorkspace onSendMessage={vi.fn()} onConfirmAction={vi.fn()} isSending={false} />)

    expect(screen.getByText('Ответ помощника')).toBeInTheDocument()
    expect(screen.getByText('Шаг 1: Поиск в Playbook')).toBeInTheDocument()
    expect(screen.getByText('AvitoTech Guide')).toBeInTheDocument()
  })

  it('renders KnowledgeDrawer and fetches articles', async () => {
    vi.spyOn(api, 'fetchKnowledge').mockResolvedValueOnce({
      role: 'intern',
      visible_articles: [
        {
          id: 'kb-1',
          title: 'Стандарты Go в Авито',
          category: 'engineering',
          sensitivity: 'intern',
          source: 'Playbook',
          summary: 'Гайд по Go 1.27.3',
          content: 'Полный текст',
          tags: ['go'],
        },
      ],
      hidden_by_rbac_count: 2,
    })

    useAppStore.setState({ isKnowledgeDrawerOpen: true })

    render(<KnowledgeDrawer />)

    expect(screen.getByText('База знаний AvitoTech')).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText('Стандарты Go в Авито')).toBeInTheDocument()
      expect(screen.getByText(/Скрыто политикой/i)).toBeInTheDocument()
    })
  })
})
