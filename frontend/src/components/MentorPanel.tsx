import React from 'react'
import { CorporateRequest } from '../types'
import { useAppStore } from '../store/useAppStore'
import { Shield, Clock, CheckCircle2, XCircle, Award, BarChart3, AlertCircle } from 'lucide-react'

interface MentorPanelProps {
  onReview: (requestId: string, decision: 'approved' | 'rejected', comment: string) => void
  isReviewing: boolean
}

export const MentorPanel: React.FC<MentorPanelProps> = ({ onReview, isReviewing }) => {
  const { corporateRequests, activeRole, currentUser, metrics } = useAppStore()

  const pendingRequests = corporateRequests.filter((r) => r.status === 'pending_approval')
  const historyRequests = corporateRequests.filter((r) => r.status !== 'pending_approval')

  return (
    <div className="bg-white rounded-2xl border border-avito-border p-5 shadow-sm space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-gray-100 pb-4">
        <div>
          <div className="flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded-full bg-blue-500" />
            <h2 className="font-bold text-base text-avito-dark">
              {activeRole === 'mentor' ? 'Рабочее место наставника (Мария Козлова)' : 'Панель руководителя (Дмитрий Волков)'}
            </h2>
          </div>
          <p className="text-xs text-gray-500 mt-0.5">
            Согласование заявок стажёра и метрики эффективности адаптации в реальном времени
          </p>
        </div>
        <span className="text-xs font-semibold px-2.5 py-1 rounded-lg bg-blue-50 text-avito-blue border border-blue-200">
          RBAC: {currentUser?.role_title}
        </span>
      </div>

      {/* Real-time Business Metrics */}
      {metrics && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          <div className="p-3 bg-emerald-50/60 rounded-xl border border-emerald-100">
            <div className="text-[11px] font-semibold text-emerald-800">Сэкономлено времени buddy</div>
            <div className="text-lg font-extrabold text-emerald-700 mt-0.5">
              +{metrics.mentor_hours_saved.toFixed(1)} ч
            </div>
            <div className="text-[10px] text-emerald-600">из 48 ч норматива</div>
          </div>

          <div className="p-3 bg-blue-50/60 rounded-xl border border-blue-100">
            <div className="text-[11px] font-semibold text-blue-800">Рутинных вопросов закрыто</div>
            <div className="text-lg font-extrabold text-avito-blue mt-0.5">
              {metrics.routine_questions_resolved}
            </div>
            <div className="text-[10px] text-blue-600">без отвлечения наставника</div>
          </div>

          <div className="p-3 bg-purple-50/60 rounded-xl border border-purple-100">
            <div className="text-[11px] font-semibold text-purple-800">MCP действий выполнено</div>
            <div className="text-lg font-extrabold text-avito-purple mt-0.5">
              {metrics.mcp_actions_executed}
            </div>
            <div className="text-[10px] text-purple-600">IDM, Absence, 1:1 Sync</div>
          </div>

          <div className="p-3 bg-amber-50/60 rounded-xl border border-amber-100">
            <div className="text-[11px] font-semibold text-amber-800">RBAC перехватов</div>
            <div className="text-lg font-extrabold text-amber-700 mt-0.5">
              {metrics.rbac_violations_prevented}
            </div>
            <div className="text-[10px] text-amber-600">блокировок опасных ресурсов</div>
          </div>
        </div>
      )}

      {/* Pending Approval Section */}
      <div className="space-y-3">
        <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider flex items-center gap-1.5">
          <Clock className="w-3.5 h-3.5 text-avito-blue" />
          <span>Ожидают согласования ({pendingRequests.length})</span>
        </h3>

        {pendingRequests.length === 0 ? (
          <div className="p-6 text-center border border-dashed border-gray-200 rounded-xl text-xs text-gray-400">
            Нет активных заявок, требующих согласования.
            <div className="mt-1 text-[11px] text-gray-500">
              Переключись на роль <b>Стажёра</b> и нажми кнопку <i>«Оформить отгул на экзамен»</i>, чтобы создать заявку!
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {pendingRequests.map((req) => (
              <div key={req.id} className="p-4 rounded-xl border border-blue-200 bg-blue-50/30 space-y-3">
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <span className="text-[10px] font-mono font-bold text-avito-blue px-2 py-0.5 bg-blue-100 rounded-md">
                      {req.id}
                    </span>
                    <h4 className="text-xs font-bold text-avito-dark mt-1">{req.title}</h4>
                    <p className="text-xs text-gray-600 mt-0.5">{req.summary}</p>
                  </div>
                </div>

                <div className="flex items-center gap-2 pt-2 border-t border-blue-100">
                  <button
                    onClick={() => onReview(req.id, 'approved', 'Согласовано наставником. Удачи на экзамене!')}
                    disabled={isReviewing}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-avito-green hover:bg-avito-green-hover text-white rounded-lg text-xs font-bold transition shadow-2xs"
                  >
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    <span>Согласовать</span>
                  </button>
                  <button
                    onClick={() => onReview(req.id, 'rejected', 'Необходимо перенести на другой день')}
                    disabled={isReviewing}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-100 hover:bg-red-50 text-gray-700 hover:text-red-700 rounded-lg text-xs font-semibold transition"
                  >
                    <XCircle className="w-3.5 h-3.5" />
                    <span>Отклонить</span>
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* History and Executed Actions */}
      <div className="space-y-2">
        <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">
          Журнал корпоративных действий ({historyRequests.length})
        </h3>
        <div className="space-y-2 max-h-60 overflow-y-auto">
          {historyRequests.map((req) => (
            <div key={req.id} className="p-3 rounded-xl border border-gray-100 bg-gray-50/60 flex items-center justify-between text-xs">
              <div>
                <span className="font-semibold text-gray-800">{req.title}</span>
                <span className="text-[10px] text-gray-400 block font-mono">{req.id} • {req.service_name}</span>
              </div>
              <span className={`text-[10px] font-bold px-2 py-0.5 rounded-full ${
                req.status === 'approved' || req.status === 'completed'
                  ? 'bg-emerald-50 text-emerald-700'
                  : req.status === 'blocked_by_rbac'
                  ? 'bg-red-50 text-red-700'
                  : 'bg-gray-200 text-gray-700'
              }`}>
                {req.status}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
