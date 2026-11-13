import { Check, Send, X, ShieldAlert, KeyRound, CalendarDays, MessagesSquare } from 'lucide-react'
import type { CorporateRequest, ReviewDecision } from '../types'
import { StatusBadge } from './ui/StatusBadge'
import { formatDate } from '../lib/format'
import { useAppContext } from '../api/queries'
import { draftKey, useDraft } from '../lib/drafts'

interface ActionCardViewProps {
  card: CorporateRequest
  onConfirm?: (requestId: string) => void
  isConfirming?: boolean
  isDisabled?: boolean
  onReview?: (decision: ReviewDecision, comment: string) => void
  isReviewing?: boolean
}
const detailLabels: Record<string, string> = {
  resource: 'Ресурс', access_level: 'Уровень доступа', reason: 'Причина', start_date: 'Начало', end_date: 'Окончание',
  absence_type: 'Тип отсутствия', approver: 'Согласующий', buddy: 'Наставник', topic: 'Тема',
  what_checked: 'Что уже проверено', questions: 'Вопросы к встрече', sla: 'Срок обработки', policy_rule: 'Правило доступа',
}
const detailValues: Record<string, string> = { study_day_off: 'Учебный отгул', personal_day_off: 'Личный отгул', vacation: 'Отпуск', read: 'Чтение', read_write: 'Чтение и запись', admin: 'Администрирование' }
const requestTypes = { access_request: 'Доступ', absence_request: 'Отсутствие', mentor_1on1_summary: 'Встреча 1:1', rbac_blocked: 'Ограничение доступа' }
function DetailValue({ value }: { value: unknown }) {
  if (Array.isArray(value)) return <ul>{value.map((item: unknown, index) => <li key={index}>{typeof item === 'string' ? item : JSON.stringify(item)}</li>)}</ul>
  if (typeof value === 'string') return <>{detailValues[value] || value}</>
  return <>{JSON.stringify(value)}</>
}
export function ActionCardView({ card, onConfirm, isConfirming, isDisabled, onReview, isReviewing }: ActionCardViewProps) {
  const { data: context } = useAppContext()
  const [comment, setComment] = useDraft(draftKey(context, 'review', card.id))
  const blocked = card.status === 'blocked_by_rbac'
  const Icon = blocked ? ShieldAlert : card.request_type === 'access_request' ? KeyRound : card.request_type === 'absence_request' ? CalendarDays : MessagesSquare
  const details = Object.entries(card.details ?? {}).filter(([key, value]) => key !== 'synthetic' && value !== null && value !== undefined)
  return <article className={'action-card' + (blocked ? ' action-blocked' : '')}>
    <div className="action-top"><span className="action-service"><Icon size={17} aria-hidden="true" />{card.service_name}</span><StatusBadge status={card.status} /></div>
    <h3>{card.title}</h3><p>{card.summary}</p>
    <div className="action-meta"><span>{requestTypes[card.request_type]}</span><span>{formatDate(card.created_at)}</span><span>{card.id}</span></div>
    {details.length > 0 && <details className="action-details"><summary>Детали заявки</summary><dl>{details.map(([key, value]) => <div key={key}><dt>{detailLabels[key] || key}</dt><dd><DetailValue value={value} /></dd></div>)}</dl></details>}
    {card.reviewer_comment && <div className="reviewer-comment"><strong>Комментарий согласующего</strong><p>{card.reviewer_comment}</p></div>}
    {card.status === 'draft_confirmation_required' && onConfirm && <div className="action-footer"><span>Отправим только после вашего подтверждения</span><button className="button button-primary" disabled={isDisabled || isConfirming} onClick={() => onConfirm(card.id)}><Send size={15} />{isConfirming ? 'Отправка...' : 'Подтвердить и отправить'}</button></div>}
    {card.status === 'pending_approval' && onReview && <div className="review-form"><label>Комментарий к решению<textarea value={comment} onChange={event => setComment(event.target.value)} rows={2} disabled={isDisabled} placeholder="Необязательно" /></label><div className="button-row"><button className="button button-primary" disabled={isDisabled} onClick={() => onReview('approved', comment)}><Check size={16} />{isReviewing ? 'Сохранение...' : 'Согласовать'}</button><button className="button button-secondary" disabled={isDisabled} onClick={() => onReview('rejected', comment)}><X size={16} />Отклонить</button></div></div>}
  </article>
}
