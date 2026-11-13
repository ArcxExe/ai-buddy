export function LoadingState({ label, failed = false }: { label: string; failed?: boolean }) {
  if (failed) return <div className="card empty-state query-placeholder"><p>Данные пока недоступны. Повторите загрузку с помощью кнопки выше.</p></div>
  return <div className="loading-skeleton query-placeholder" role="status" aria-label={label}>
    <span className="sr-only">{label}</span>
    <div aria-hidden="true"><div className="skeleton-line" /><div className="skeleton-card" /><div className="skeleton-card" /></div>
  </div>
}
