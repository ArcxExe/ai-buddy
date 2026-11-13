import { Menu, ChevronRight } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { useAppContext } from '../api/queries'
import { useAppStore } from '../store/useAppStore'
import type { Role } from '../types'
import { navigation, roleLabels } from './Sidebar'

interface HeaderProps {
  onRoleChange: (role: Role) => void
  isBusy?: boolean
}
export function Header({ onRoleChange, isBusy = false }: HeaderProps) {
  const { data } = useAppContext()
  const location = useLocation()
  const current = navigation.find(item => item.to === location.pathname)
  const toggle = useAppStore(state => state.toggleMobileNav)
  const mobileOpen = useAppStore(state => state.isMobileNavOpen)
  return <header className="app-header">
    <div className="header-location">
      <button className="icon-button mobile-only" onClick={toggle} aria-label="Открыть меню" aria-controls="app-navigation" aria-expanded={mobileOpen}><Menu size={22} /></button>
      <span className="header-product">Avito Start</span><ChevronRight size={14} className="header-chevron" aria-hidden="true" />
      <span className="header-title">{current?.label || 'Страница не найдена'}</span>
    </div>
    <div className="header-actions">
      {import.meta.env.DEV && <label className="role-picker"><span>Демо-роль</span><select aria-label="Демонстрационная роль" value={data?.active_role || 'intern'} onChange={event => onRoleChange(event.target.value as Role)} disabled={isBusy}>
        <option value="intern">Стажёр</option><option value="mentor">Наставник</option><option value="manager">Руководитель</option>
      </select></label>}
      {!import.meta.env.DEV && data && <span className="neutral-badge" aria-label="Роль пользователя">{roleLabels[data.active_role]}</span>}
      <span className="avatar header-avatar" title={data?.current_user.name} aria-label={data?.current_user.name}>{data?.current_user.avatar_initials || '…'}</span>
    </div>
  </header>
}
