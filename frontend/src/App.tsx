import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter, Link, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { Info, LoaderCircle } from 'lucide-react'
import { createQueryClient, useAppContext, useMessages, useWorkspaceActions } from './api/queries'
import { getErrorMessage } from './api/client'
import { Sidebar, navigation } from './components/Sidebar'
import { Header } from './components/Header'
import { useAppStore } from './store/useAppStore'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Notice } from './components/ui/Notice'
import { LoadingState } from './components/ui/LoadingState'

const DashboardPage = lazy(() => import('./pages/DashboardPage').then(module => ({ default: module.DashboardPage })))
const ChatPage = lazy(() => import('./pages/ChatPage').then(module => ({ default: module.ChatPage })))
const OnboardingRoadmap = lazy(() => import('./components/OnboardingRoadmap').then(module => ({ default: module.OnboardingRoadmap })))
const KnowledgePage = lazy(() => import('./pages/KnowledgePage').then(module => ({ default: module.KnowledgePage })))
const RequestsPage = lazy(() => import('./pages/RequestsPage').then(module => ({ default: module.RequestsPage })))

export function MainApp() {
  const contextQuery = useAppContext()
  const messagesQuery = useMessages()
  const action = useWorkspaceActions()
  const busy = action.isPending || action.requiresRefresh || !contextQuery.data || contextQuery.isError
  const chatBusy = busy || !messagesQuery.data || messagesQuery.isError
  const isSending = action.isPending && action.variables?.type === 'send'
  const location = useLocation()
  const navigate = useNavigate()
  const main = useRef<HTMLElement>(null)
  const mobileOpen = useAppStore(state => state.isMobileNavOpen)
  const closeNav = useAppStore(state => state.closeMobileNav)
  const role = contextQuery.data?.active_role
  useEffect(() => {
    document.title = (navigation.find(item => item.to === location.pathname)?.label || 'Страница не найдена') + ' · AI-buddy'
    main.current?.focus({ preventScroll: true })
    closeNav()
  }, [location.pathname, closeNav])
  const handleReset = () => {
    if (!busy && window.confirm('Сбросить историю чата и вернуть демо-состояние в исходный вид?')) action.execute({ type: 'reset' })
  }
  const error = action.error || contextQuery.error || messagesQuery.error
  const warning = action.requiresRefresh ? action.data?.warning || 'Результат операции не подтверждён. Обновите данные и проверьте результат перед повторным действием.' : undefined
  const canRefresh = action.requiresRefresh || contextQuery.isError || messagesQuery.isError
  const initialLoading = (!contextQuery.data || !messagesQuery.data) && (contextQuery.isFetching || messagesQuery.isFetching)
  const confirming = action.isPending && action.variables?.type === 'confirm' ? action.variables.requestId : undefined
  const reviewing = action.isPending && action.variables?.type === 'review' ? action.variables.requestId : undefined
  return <div className="app-shell" aria-busy={action.isPending}>
    <a className="skip-link" href="#main-content">Перейти к содержимому</a>
    <Sidebar onReset={handleReset} isBusy={busy} />
    <div className="app-workspace" inert={mobileOpen || undefined}>
      <Header onRoleChange={newRole => action.execute({ type: 'role', role: newRole })} isBusy={busy} />
      <main ref={main} tabIndex={-1} id="main-content">
        <div className="workspace-notices">
          {import.meta.env.DEV && <div className="demo-notice"><Info size={15} aria-hidden="true" /><span><strong>Демонстрационный прототип.</strong> Синтетические сотрудники, регламенты и сервисы · Hack&Change 2026</span></div>}
          {(error || warning) && <Notice tone={error ? 'error' : 'warning'} action={canRefresh && <button className="button button-secondary" disabled={action.isPending} onClick={() => action.execute({ type: 'refresh' })}>{action.requiresRefresh ? 'Обновить данные' : 'Повторить'}</button>}>
            {error && <span>{getErrorMessage(error)}</span>}{warning && <p>{warning}</p>}
          </Notice>}
          {(action.isPending && !isSending) || initialLoading ? <Notice tone="info"><LoaderCircle size={16} className="spin" aria-hidden="true" />{action.isPending ? action.variables?.type === 'refresh' ? 'Обновляем данные...' : 'Сохраняем изменения...' : 'Загружаем ваше пространство...'}</Notice> : null}
          {!action.isPending && !initialLoading && contextQuery.data && (contextQuery.isFetching || messagesQuery.isFetching) && <Notice tone="info">Обновляем данные. Последний загруженный результат остаётся на экране.</Notice>}
          {action.isSuccess && !error && !warning && action.variables?.type !== 'send' && <Notice tone={action.variables?.type === 'refresh' ? 'info' : 'success'}>{action.variables?.type === 'reset' ? 'Демо сброшено. Можно начать заново.' : action.variables?.type === 'refresh' ? 'Данные обновлены.' : 'Изменения сохранены.'}</Notice>}
        </div>
        <ErrorBoundary key={location.pathname}>
        <Suspense fallback={<div className="page"><LoadingState label="Загрузка раздела" /></div>}>
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/chat" element={<ChatPage onSendMessage={(content, hint) => action.execute({ type: 'send', content, hint })} onConfirmAction={requestId => action.execute({ type: 'confirm', requestId })} isSending={isSending} isBusy={chatBusy} confirmingRequestId={confirming} sendError={action.isError && action.variables?.type === 'send' ? getErrorMessage(action.error) : undefined} />} />
          <Route path="/onboarding" element={<OnboardingRoadmap onSelectPrompt={(prompt, hint) => navigate('/chat', { state: { prompt, hint } })} onToggleChecklist={(itemId, status) => action.execute({ type: 'checklist', itemId, status })} isBusy={busy} />} />
          <Route path="/knowledge" element={<KnowledgePage key={role} />} />
          <Route path="/requests" element={<RequestsPage onConfirm={requestId => action.execute({ type: 'confirm', requestId })} onReview={(requestId, decision, comment) => action.execute({ type: 'review', requestId, decision, comment })} isBusy={busy} confirmingRequestId={confirming} reviewingRequestId={reviewing} />} />
          <Route path="*" element={<div className="page"><div className="card empty-state"><h1>Страница не найдена</h1><p>Вернёмся к вашему пространству адаптации.</p><Link to="/" className="button button-dark">На главную</Link></div></div>} />
        </Routes>
        </Suspense>
        </ErrorBoundary>
      </main>
      <footer className="app-footer"><span>AI-buddy · Avito Start</span><span>Учиться новому проще вместе.</span></footer>
    </div>
  </div>
}

export default function App({ client }: { client?: QueryClient } = {}) {
  const [defaultClient] = useState(createQueryClient)
  return <QueryClientProvider client={client ?? defaultClient}><ErrorBoundary><BrowserRouter><MainApp /></BrowserRouter></ErrorBoundary></QueryClientProvider>
}
