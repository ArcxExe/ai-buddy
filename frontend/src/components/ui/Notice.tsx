import type { ReactNode } from 'react'
import { AlertCircle, CheckCircle2, Info, TriangleAlert } from 'lucide-react'

export function Notice({ tone, children, action }: {
  tone: 'success' | 'error' | 'warning' | 'info'
  children: ReactNode
  action?: ReactNode
}) {
  const Icon = tone === 'error' ? AlertCircle : tone === 'success' ? CheckCircle2 : tone === 'warning' ? TriangleAlert : Info
  return <div className={'notice-banner notice-' + tone} role={tone === 'error' ? 'alert' : 'status'} aria-atomic="true">
    <Icon size={18} aria-hidden="true" /><div className="notice-content">{children}</div>{action}
  </div>
}
