import React from 'react'
import { CorporateRequest } from '../types'
import { ShieldAlert, CheckCircle, Clock, Calendar, Key, MessageSquare, Send } from 'lucide-react'

interface ActionCardViewProps {
  card: CorporateRequest
  onConfirm?: (requestId: string) => void
  isConfirming?: boolean
}

export const ActionCardView: React.FC<ActionCardViewProps> = ({ card, onConfirm, isConfirming }) => {
  const isRBACBlocked = card.status === 'blocked_by_rbac' || card.request_type === 'rbac_blocked'
  const isDraft = card.status === 'draft_confirmation_required'
  const isApproved = card.status === 'approved' || card.status === 'completed'
  const isPending = card.status === 'pending_approval'

  if (isRBACBlocked) {
    return (
      <div className="mt-3 p-3.5 rounded-xl border border-red-200 bg-red-50/70 text-red-900 space-y-2">
        <div className="flex items-center gap-2">
          <ShieldAlert className="w-4 h-4 text-red-600 flex-shrink-0" />
          <span className="text-xs font-bold uppercase tracking-wider text-red-700">
            RBAC Guardrail • Доступ ограничен
          </span>
        </div>
        <div className="text-xs font-semibold">{card.title}</div>
        <p className="text-xs text-red-700/90 leading-relaxed">{card.summary}</p>
        <div className="text-[10px] text-red-500 font-mono">
          Ресурс: {card.target_resource} | Сервис: {card.service_name}
        </div>
      </div>
    )
  }

  return (
    <div className="mt-3 p-4 rounded-xl border border-avito-border bg-white shadow-2xs space-y-3">
      {/* Service badge & Status */}
      <div className="flex items-center justify-between gap-2 border-b border-gray-100 pb-2">
        <div className="flex items-center gap-2">
          {card.request_type === 'access_request' && <Key className="w-4 h-4 text-avito-blue" />}
          {card.request_type === 'absence_request' && <Calendar className="w-4 h-4 text-avito-green" />}
          {card.request_type === 'mentor_1on1_summary' && <MessageSquare className="w-4 h-4 text-avito-purple" />}
          <span className="text-xs font-bold text-gray-800">{card.service_name}</span>
        </div>
        <span
          className={`text-[11px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1 ${
            isApproved
              ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
              : isDraft
              ? 'bg-amber-50 text-amber-700 border border-amber-200'
              : isPending
              ? 'bg-blue-50 text-blue-700 border border-blue-200'
              : 'bg-gray-100 text-gray-700'
          }`}
        >
          {isApproved && <CheckCircle className="w-3 h-3 text-emerald-600" />}
          {isPending && <Clock className="w-3 h-3 text-blue-600 animate-spin" />}
          {card.status === 'draft_confirmation_required' && 'Черновик (требует подтверждения)'}
          {card.status === 'pending_approval' && 'На согласовании'}
          {card.status === 'approved' && 'Одобрено'}
          {card.status === 'completed' && 'Зафиксировано'}
          {card.status === 'rejected' && 'Отклонено'}
        </span>
      </div>

      {/* Title & Summary */}
      <div>
        <h4 className="text-xs font-bold text-avito-dark">{card.title}</h4>
        <p className="text-xs text-gray-600 mt-1 leading-relaxed">{card.summary}</p>
      </div>

      {/* 1:1 Questions Details */}
      {card.request_type === 'mentor_1on1_summary' && card.details?.questions && (
        <div className="space-y-1.5 bg-purple-50/50 p-2.5 rounded-lg border border-purple-100 text-xs">
          <span className="font-semibold text-purple-900 block">Вопросы для синка с Марией:</span>
          <ul className="list-disc list-inside space-y-1 text-purple-800 text-[11px]">
            {(card.details.questions as string[]).map((q, idx) => (
              <li key={idx}>{q}</li>
            ))}
          </ul>
        </div>
      )}

      {/* Confirmation Action for Human-in-the-Loop (Slide 4 Case) */}
      {isDraft && onConfirm && (
        <div className="pt-1 flex items-center justify-between gap-3">
          <span className="text-[11px] text-gray-500">
            Действие не выполнится без твоего согласия
          </span>
          <button
            onClick={() => onConfirm(card.id)}
            disabled={isConfirming}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-avito-green hover:bg-avito-green-hover text-white rounded-lg text-xs font-bold shadow-xs transition"
          >
            <Send className="w-3 h-3" />
            <span>{isConfirming ? 'Отправка...' : 'Подтвердить и отправить'}</span>
          </button>
        </div>
      )}

      {card.reviewer_comment && (
        <div className="text-[11px] bg-gray-50 p-2 rounded-lg border border-gray-200 text-gray-700">
          <span className="font-semibold">Комментарий согласующего:</span> {card.reviewer_comment}
        </div>
      )}
    </div>
  )
}
