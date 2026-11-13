import { Check, Clock3, ShieldAlert, FilePenLine, X } from 'lucide-react'
import type { RequestStatus, ChecklistStatus } from '../../types'

export const checklistLabels: Record<ChecklistStatus, string> = { todo: 'Не начато', in_progress: 'В процессе', done: 'Готово' }
export const requestLabels: Record<RequestStatus, string> = {
  draft_confirmation_required: 'Требует подтверждения', pending_approval: 'Ожидает согласования',
  approved: 'Одобрено', completed: 'Выполнено', rejected: 'Отклонено', blocked_by_rbac: 'Заблокировано правами',
}
export function StatusBadge({ status }: { status: RequestStatus }) {
  const Icon = status === 'blocked_by_rbac' ? ShieldAlert : status === 'rejected' ? X
    : status === 'draft_confirmation_required' ? FilePenLine : status === 'pending_approval' ? Clock3 : Check
  return <span className={'status-badge status-' + status}><Icon size={13} aria-hidden="true" />{requestLabels[status]}</span>
}
