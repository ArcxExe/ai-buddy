import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertCircle } from 'lucide-react'

// Rendering recovery is independent of authentication and server mutations.
// Retrying remounts the UI without resetting the backend or clearing Query data.
export class ErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; reloadRequired: boolean }> {
  state = { hasError: false, reloadRequired: false }

  static getDerivedStateFromError(error: unknown) {
    const reloadRequired = error instanceof Error && /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|Unable to preload CSS/i.test(error.message)
    return { hasError: true, reloadRequired }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    if (import.meta.env.DEV && import.meta.env.MODE !== 'test') console.error('UI rendering error', error, info.componentStack)
  }

  render() {
    if (!this.state.hasError) return this.props.children

    return <div className="page">
      <section className="card empty-state" role="alert" aria-labelledby="application-error-title">
        <AlertCircle size={32} aria-hidden="true" />
        <h1 id="application-error-title">Не удалось отобразить страницу</h1>
        <p>{this.state.reloadRequired ? 'Не удалось загрузить раздел. Проверьте соединение и перезагрузите страницу.' : 'Попробуйте открыть её снова. Повторные заявки отправляться не будут.'}</p>
        <div className="button-row">
          {this.state.reloadRequired
            ? <a className="button button-dark" href={window.location.href}>Перезагрузить страницу</a>
            : <button className="button button-dark" onClick={() => this.setState({ hasError: false, reloadRequired: false })}>Попробовать снова</button>}
          <a className="button button-secondary" href="/">На главную</a>
        </div>
      </section>
    </div>
  }
}
