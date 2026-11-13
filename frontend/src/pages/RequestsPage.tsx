import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Files, ArrowRight } from 'lucide-react'
import { useAppContext } from '../api/queries'
import type { ReviewDecision, RequestStatus } from '../types'
import { ActionCardView } from '../components/ActionCardView'
import { requestLabels } from '../components/ui/StatusBadge'
import { LoadingState } from '../components/ui/LoadingState'

interface RequestsPageProps {
  onConfirm: (id: string) => void
  onReview: (id: string, decision: ReviewDecision, comment: string) => void
  isBusy: boolean
  confirmingRequestId?: string
  reviewingRequestId?: string
}
export function RequestsPage({ onConfirm, onReview, isBusy, confirmingRequestId, reviewingRequestId }: RequestsPageProps) {
  const { data, isError } = useAppContext()
  const [filter, setFilter] = useState<RequestStatus | 'all'>('all')
  if (!data) return <div className="page"><LoadingState label="Загрузка заявок" failed={isError} /></div>
  const requests = data?.corporate_requests ?? []
  const visible = requests.filter(request => filter === 'all' || request.status === filter)
  const canReview = data?.active_role === 'mentor' || data?.active_role === 'manager'
  return <div className="page requests-page"><div className="page-intro"><span className="eyebrow">ВСЁ ПОД КОНТРОЛЕМ</span><h1>Заявки</h1><p>Доступы, отсутствие и встречи — история ваших корпоративных действий.</p></div>
    <div className="requests-toolbar"><span>{requests.length} в журнале · {requests.filter(item => item.status === 'pending_approval').length} ожидают согласования</span><label className="filter-label">Статус<select aria-label="Фильтр заявок" value={filter} onChange={event => setFilter(event.target.value as RequestStatus | 'all')}><option value="all">Все заявки</option>{Object.entries(requestLabels).map(([value, text]) => <option key={value} value={value}>{text}</option>)}</select></label></div>
    {visible.length === 0 ? <div className="card empty-state"><span className="empty-icon"><Files size={30} /></span><h2>{requests.length ? 'Заявок с таким статусом нет' : 'Пока нет заявок'}</h2><p>Помощник может запросить доступ, подготовить отгул или повестку встречи.</p><Link className="button button-dark" to="/chat">Открыть помощника<ArrowRight size={16} /></Link></div> : <div className="requests-list">{visible.map(request => <ActionCardView key={request.id} card={request} onConfirm={data?.active_role === request.created_by_role ? onConfirm : undefined} isConfirming={confirmingRequestId === request.id} isDisabled={isBusy} onReview={canReview ? (decision, comment) => onReview(request.id, decision, comment) : undefined} isReviewing={reviewingRequestId === request.id} />)}</div>}
    {requests.some(request => /\bmock\b/i.test(request.service_name)) && <p className="fine-print">Карточки с пометкой Mock относятся к тестовым сервисам.</p>}
  </div>
}
