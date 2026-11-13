import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react'
import App from '../App'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { queryKeys, useWorkspaceActions } from '../api/queries'
import { context, message } from './fixtures'
import { createTestClient, withClient } from './query-utils'

afterEach(() => vi.restoreAllMocks())

describe('Application recovery', () => {
  it('offers an explicit page reload for a cached lazy import failure instead of a futile render retry', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    window.history.replaceState({}, '', '/chat')
    function FailedRoute(): never { throw new Error('Failed to fetch dynamically imported module: /assets/ChatPage.js') }
    render(<ErrorBoundary><FailedRoute /></ErrorBoundary>)
    expect(screen.getByRole('alert')).toHaveTextContent('Проверьте соединение')
    expect(screen.getByRole('link', { name: 'Перезагрузить страницу' })).toHaveAttribute('href', window.location.href)
    expect(screen.queryByRole('button', { name: 'Попробовать снова' })).not.toBeInTheDocument()
    expect(screen.queryByText(/\/assets\/ChatPage/)).not.toBeInTheDocument()
  })

  it('renders normally without a failure', () => {
    render(<ErrorBoundary><p>Страница работает</p></ErrorBoundary>)
    expect(screen.getByText('Страница работает')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('catches rendering failures, hides technical details and recovers only on user request', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    let broken = true
    function Page() {
      if (broken) throw new Error('Private implementation details')
      return <p>Страница восстановлена</p>
    }
    render(<ErrorBoundary><Page /></ErrorBoundary>)
    expect(screen.getByRole('alert')).toHaveTextContent('Не удалось отобразить страницу')
    expect(screen.queryByText(/Private implementation/)).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/')
    expect(screen.queryByText('Страница восстановлена')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Попробовать снова' }))
    expect(screen.getByRole('alert')).toHaveTextContent('Не удалось отобразить страницу')
    broken = false
    fireEvent.click(screen.getByRole('button', { name: 'Попробовать снова' }))
    expect(screen.getByText('Страница восстановлена')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('handles malformed context as a recoverable API error and never caches it', async () => {
    window.history.replaceState({}, '', '/')
    const client = createTestClient(false)
    vi.spyOn(globalThis, 'fetch').mockImplementation(async input => new Response(JSON.stringify(
      String(input).endsWith('/context') ? { ...context, onboarding: null } : [],
    )))
    render(<App client={client} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Сервер вернул данные неожиданного формата')
    expect(screen.queryByText('Не удалось отобразить страницу')).not.toBeInTheDocument()
    expect(client.getQueryData(queryKeys.context)).toBeUndefined()
    expect(screen.getByRole('navigation', { name: 'Основная навигация' })).toBeInTheDocument()
    vi.mocked(globalThis.fetch).mockImplementation(async input => new Response(JSON.stringify(
      String(input).endsWith('/context') ? context : [],
    )))
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }))
    expect(await screen.findByRole('heading', { name: /Привет, Алексей/ })).toBeInTheDocument()
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
    client.clear()
  })

  it('keeps cached server data and reports failure when a mutation returns an invalid snapshot', async () => {
    const client = createTestClient()
    const fetch = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      user_message: { ...message, sender: 'user' }, assistant_message: message,
      onboarding: { ...context.onboarding, overall_progress: '100' },
      corporate_requests: [], metrics: context.metrics,
    })))
    const { result } = renderHook(useWorkspaceActions, { wrapper: withClient(client) })
    act(() => result.current.execute({ type: 'send', content: 'Отгул' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toMatchObject({ code: 'INVALID_RESPONSE' })
    expect(result.current.isSuccess).toBe(false)
    expect(client.getQueryData(queryKeys.context)).toEqual(context)
    expect(client.getQueryData(queryKeys.messages)).toEqual([])
    expect(fetch).toHaveBeenCalledTimes(1)
    client.clear()
  })
})
