import { useEffect, useRef } from 'react'
import { NavLink } from 'react-router-dom'
import { Home, MessagesSquare, ListChecks, BookOpen, Files, X, RotateCcw, ArrowUpRight } from 'lucide-react'
import { useAppContext } from '../api/queries'
import { useAppStore } from '../store/useAppStore'

export const navigation = [
  { to: '/', label: 'Главная', icon: Home, subtitle: 'Всё важное — под рукой' },
  { to: '/chat', label: 'ИИ-помощник', icon: MessagesSquare, subtitle: 'Разберёмся вместе' },
  { to: '/onboarding', label: 'Моя адаптация', icon: ListChecks, subtitle: 'Ваш путь в команде' },
  { to: '/knowledge', label: 'База знаний', icon: BookOpen, subtitle: 'Инструкции и ответы' },
  { to: '/requests', label: 'Заявки', icon: Files, subtitle: 'Корпоративные действия' },
]
export const roleLabels = { intern: 'Стажёр', mentor: 'Наставник', manager: 'Руководитель' }

export function Sidebar({ onReset, isBusy }: { onReset: () => void; isBusy: boolean }) {
  const { data } = useAppContext()
  const { isMobileNavOpen: open, closeMobileNav } = useAppStore()
  const ref = useRef<HTMLElement>(null)

  useEffect(() => {
    if (!open) return
    const previous = document.activeElement
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    ref.current?.querySelector<HTMLButtonElement>('button')?.focus()
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeMobileNav()
      if (event.key !== 'Tab') return
      const nodes = ref.current?.querySelectorAll<HTMLElement>('a[href],button:not(:disabled)')
      if (!nodes?.length) return
      const first = nodes[0], last = nodes[nodes.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', keyboard)
    return () => {
      document.body.style.overflow = overflow
      document.removeEventListener('keydown', keyboard)
      if (previous instanceof HTMLElement) previous.focus()
    }
  }, [open, closeMobileNav])

  useEffect(() => {
    const media = window.matchMedia?.('(min-width: 1024px)')
    const changed = () => { if (media?.matches) closeMobileNav() }
    media?.addEventListener('change', changed)
    return () => media?.removeEventListener('change', changed)
  }, [closeMobileNav])

  return <>
    {open && <button className="nav-scrim" aria-label="Закрыть навигацию" onClick={closeMobileNav} />}
    <aside id="app-navigation" ref={ref} className={'sidebar' + (open ? ' is-open' : '')} role={open ? 'dialog' : undefined} aria-modal={open || undefined} aria-label="Навигация приложения">
      <div className="sidebar-top">
        <NavLink to="/" className="brand" onClick={closeMobileNav} aria-label="AI-buddy — Главная">
          <span className="brand-symbol" aria-hidden="true"><i /><i /><i /><i /></span>
          <span><strong>AI-buddy<span className="brand-dot">.</span></strong><small>Avito Start</small></span>
        </NavLink>
        <button className="icon-button mobile-only" aria-label="Закрыть меню" onClick={closeMobileNav}><X size={20} /></button>
      </div>
      <div className="nav-caption">ВАШЕ ПРОСТРАНСТВО</div>
      <nav aria-label="Основная навигация">
        {navigation.map(({ to, label, icon: Icon }) => <NavLink end={to === '/'} key={to} to={to} onClick={closeMobileNav} className={({ isActive }) => 'nav-link' + (isActive ? ' active' : '')}>
          <Icon size={20} aria-hidden="true" /><span>{label}</span><ArrowUpRight className="nav-arrow" size={15} aria-hidden="true" />
        </NavLink>)}
      </nav>
      {data && <div className="sidebar-progress">
        <div><span>Адаптация</span><strong>{data.onboarding.overall_progress}%</strong></div>
        <div className="progress-track"><span style={{ width: data.onboarding.overall_progress + '%' }} /></div>
        <small>{data.onboarding.checklist.filter(item => item.status === 'done').length} из {data.onboarding.checklist.length} шагов завершено</small>
      </div>}
      <div className="sidebar-bottom">
        <div className="sidebar-person"><span className="avatar">{data?.current_user.avatar_initials || '…'}</span><div><strong>{data?.current_user.name || 'Загрузка профиля'}</strong><small>{data ? roleLabels[data.active_role] : 'Avito Start'}</small></div></div>
        {import.meta.env.DEV && <>
          <button className="reset-button" onClick={onReset} disabled={isBusy}><RotateCcw size={15} aria-hidden="true" />Сбросить демо</button>
          <span className="sidebar-footnote">Hack&Change 2026</span>
        </>}
      </div>
    </aside>
  </>
}
