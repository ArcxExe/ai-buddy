import React from 'react'
import { ChecklistItem, ChecklistStatus } from '../types'
import { useAppStore } from '../store/useAppStore'
import { CheckCircle2, Circle, Clock, ArrowRight, Sparkles, AlertCircle } from 'lucide-react'

interface OnboardingRoadmapProps {
  onSelectPrompt: (prompt: string, hint?: string) => void
  onToggleChecklist: (itemId: string, newStatus: ChecklistStatus) => void
}

export const OnboardingRoadmap: React.FC<OnboardingRoadmapProps> = ({ onSelectPrompt, onToggleChecklist }) => {
  const { onboarding, currentUser } = useAppStore()

  if (!onboarding) return null

  const getStatusIcon = (status: ChecklistStatus) => {
    switch (status) {
      case 'done':
        return <CheckCircle2 className="w-4 h-4 text-avito-green flex-shrink-0" />
      case 'in_progress':
        return <Clock className="w-4 h-4 text-avito-blue animate-pulse flex-shrink-0" />
      default:
        return <Circle className="w-4 h-4 text-gray-300 flex-shrink-0" />
    }
  }

  return (
    <div className="bg-white rounded-2xl border border-avito-border p-5 shadow-sm space-y-5">
      {/* Header & Overall Progress */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <h2 className="font-bold text-base text-avito-dark flex items-center gap-2">
            <span>Дорожная карта адаптации</span>
            <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-blue-50 text-avito-blue">
              {currentUser?.team || 'Авито Доставка'}
            </span>
          </h2>
          <span className="text-sm font-extrabold text-avito-green">{onboarding.overall_progress}%</span>
        </div>
        <div className="w-full bg-gray-100 rounded-full h-2.5 overflow-hidden">
          <div
            className="bg-avito-green h-2.5 rounded-full transition-all duration-500 ease-out"
            style={{ width: `${onboarding.overall_progress}%` }}
          />
        </div>
      </div>

      {/* Recommended Next Step Banner */}
      {onboarding.next_recommended_step && (
        <div className="bg-gradient-to-r from-emerald-50 to-blue-50 border border-emerald-200/80 rounded-xl p-3.5 space-y-2">
          <div className="flex items-center gap-1.5 text-xs font-bold text-emerald-800">
            <Sparkles className="w-3.5 h-3.5 text-avito-green" />
            <span>Рекомендуемый следующий шаг</span>
          </div>
          <p className="text-xs text-gray-700 font-medium">
            {onboarding.next_recommended_step.title}
          </p>
          <button
            onClick={() => onSelectPrompt(onboarding.next_recommended_step!.suggested_prompt)}
            className="flex items-center gap-1.5 text-xs font-semibold text-emerald-700 hover:text-emerald-900 bg-white/80 hover:bg-white px-2.5 py-1.5 rounded-lg border border-emerald-300/60 shadow-2xs transition"
          >
            <span>Спросить AI-buddy</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Stages horizontal steps */}
      <div className="grid grid-cols-4 gap-2 pt-1">
        {onboarding.stages.map((st) => {
          const isCompleted = st.status === 'completed'
          const isCurrent = st.status === 'current'
          return (
            <div
              key={st.id}
              className={`p-2 rounded-xl text-center border text-xs transition ${
                isCurrent
                  ? 'border-avito-blue bg-blue-50/50 font-bold text-avito-dark'
                  : isCompleted
                  ? 'border-emerald-200 bg-emerald-50/40 text-emerald-800'
                  : 'border-gray-100 bg-gray-50/50 text-gray-400'
              }`}
            >
              <div className="font-semibold truncate">{st.period}</div>
              <div className="text-[10px] truncate text-gray-500">{st.title.split(':')[0]}</div>
            </div>
          )
        })}
      </div>

      {/* Checklist items list */}
      <div className="space-y-2">
        <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
          Чек-лист задач стажёра
        </h3>
        <div className="space-y-2">
          {onboarding.checklist.map((item) => {
            const isDone = item.status === 'done'
            return (
              <div
                key={item.id}
                className={`p-3 rounded-xl border transition flex items-start gap-3 ${
                  isDone
                    ? 'border-gray-100 bg-gray-50/60 text-gray-500'
                    : 'border-avito-border hover:border-blue-200 bg-white'
                }`}
              >
                <button
                  onClick={() => onToggleChecklist(item.id, isDone ? 'todo' : 'done')}
                  className="mt-0.5 focus:outline-hidden"
                  title="Отметить статус шага (<1 с)"
                >
                  {getStatusIcon(item.status)}
                </button>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <span className={`text-xs font-semibold ${isDone ? 'line-through text-gray-400' : 'text-gray-900'}`}>
                      {item.title}
                    </span>
                    <span className="text-[10px] text-gray-400 font-medium whitespace-nowrap">
                      {item.estimated_time}
                    </span>
                  </div>
                  <p className="text-[11px] text-gray-500 mt-0.5 line-clamp-2">
                    {item.description}
                  </p>
                  {!isDone && (
                    <button
                      onClick={() => onSelectPrompt(item.suggested_prompt)}
                      className="text-[11px] font-medium text-avito-blue hover:underline mt-1.5 flex items-center gap-1"
                    >
                      <span>Развернуть в чате</span>
                      <ArrowRight className="w-2.5 h-2.5" />
                    </button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
