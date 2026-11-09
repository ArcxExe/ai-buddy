import React from 'react'
import { Role } from '../types'
import { useAppStore } from '../store/useAppStore'
import { Shield, BookOpen, RotateCcw, UserCheck, ShieldAlert, Award } from 'lucide-react'

interface HeaderProps {
  onReset: () => void
  onRoleChange: (role: Role) => void
}

export const Header: React.FC<HeaderProps> = ({ onReset, onRoleChange }) => {
  const { activeRole, currentUser, toggleKnowledgeDrawer, metrics } = useAppStore()

  const roles: { role: Role; label: string; icon: React.ReactNode; color: string }[] = [
    { role: 'intern', label: 'Стажёр (Алексей)', icon: <UserCheck className="w-4 h-4" />, color: 'bg-emerald-500' },
    { role: 'mentor', label: 'Наставник (Мария)', icon: <Award className="w-4 h-4" />, color: 'bg-blue-500' },
    { role: 'manager', label: 'Руководитель (Дмитрий)', icon: <Shield className="w-4 h-4" />, color: 'bg-purple-600' },
  ]

  return (
    <header className="bg-white border-b border-avito-border sticky top-0 z-40 px-4 lg:px-8 py-3 shadow-sm">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-3">
        {/* Brand & Title */}
        <div className="flex items-center gap-3">
          <div className="flex items-center -space-x-1">
            <div className="w-4 h-4 rounded-full bg-avito-green" />
            <div className="w-4 h-4 rounded-full bg-avito-blue" />
            <div className="w-4 h-4 rounded-full bg-avito-purple" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-extrabold text-xl tracking-tight text-avito-dark">AI-buddy</span>
              <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-emerald-50 text-avito-green border border-emerald-200">
                Avito Start
              </span>
            </div>
            <p className="text-xs text-avito-muted hidden sm:block">
              ИИ-наставник стажёров • Hack&Change 2026 Prototype
            </p>
          </div>
        </div>

        {/* Role Switcher for Hackathon Judges */}
        <div className="flex items-center bg-gray-100 p-1 rounded-xl border border-gray-200">
          <span className="text-xs font-semibold text-gray-500 px-2 hidden lg:inline">Роль для проверки:</span>
          {roles.map((r) => {
            const isActive = activeRole === r.role
            return (
              <button
                key={r.role}
                onClick={() => onRoleChange(r.role)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                  isActive
                    ? 'bg-white text-avito-dark shadow-sm font-semibold'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <span className={`w-2 h-2 rounded-full ${r.color}`} />
                {r.label}
              </button>
            )
          })}
        </div>

        {/* Actions & Metrics pill */}
        <div className="flex items-center gap-2">
          {metrics && (
            <div className="hidden xl:flex items-center gap-1.5 bg-emerald-50 text-emerald-800 text-xs px-2.5 py-1.5 rounded-lg border border-emerald-200">
              <span className="font-bold">+{metrics.mentor_hours_saved.toFixed(1)} ч</span>
              <span className="text-emerald-600">сэкономлено buddy</span>
            </div>
          )}

          <button
            onClick={toggleKnowledgeDrawer}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition"
            title="Открыть базу знаний AvitoTech"
          >
            <BookOpen className="w-3.5 h-3.5 text-avito-blue" />
            <span className="hidden sm:inline">База знаний</span>
          </button>

          <button
            onClick={onReset}
            className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium text-gray-500 hover:text-red-600 hover:bg-red-50 rounded-lg transition border border-transparent hover:border-red-200"
            title="Сбросить историю и данные демо"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Сброс</span>
          </button>

          {currentUser && (
            <div className="flex items-center gap-2 pl-2 border-l border-gray-200">
              <div className="w-7 h-7 rounded-full bg-avito-purple text-white flex items-center justify-center font-bold text-xs">
                {currentUser.avatar_initials}
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
