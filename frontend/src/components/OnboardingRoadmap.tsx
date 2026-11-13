import { ArrowRight, Clock3, Check, Circle, Sparkles } from 'lucide-react'
import { useAppContext } from '../api/queries'
import type { ChecklistStatus } from '../types'
import { checklistLabels } from './ui/StatusBadge'
import { LoadingState } from './ui/LoadingState'

interface OnboardingRoadmapProps {
  onSelectPrompt: (prompt: string, hint?: string) => void
  onToggleChecklist: (itemId: string, newStatus: ChecklistStatus) => void
  isBusy?: boolean
}
export function OnboardingRoadmap({ onSelectPrompt, onToggleChecklist, isBusy = false }: OnboardingRoadmapProps) {
  const { data, isError } = useAppContext()
  const onboarding = data?.onboarding
  if (!onboarding) return <div className="page"><LoadingState label="Загрузка адаптации" failed={isError} /></div>
  if (onboarding.checklist.length === 0) return <div className="page"><div className="page-intro"><h1>Моя адаптация</h1></div><div className="card empty-state"><h2>Чек-лист пока не назначен</h2><p>Шаги адаптации появятся здесь после назначения.</p></div></div>
  const done = onboarding.checklist.filter(item => item.status === 'done').length
  return <div className="page">
    <div className="page-intro"><span className="eyebrow">ШАГ ЗА ШАГОМ</span><h1>Моя адаптация</h1><p>Понятный маршрут от первого дня до самостоятельной работы.</p></div>
    <section className="card onboarding-summary"><div><span className="section-kicker">ОБЩИЙ ПРОГРЕСС</span><h2>{done} из {onboarding.checklist.length} шагов завершено</h2></div><strong className="large-percent">{onboarding.overall_progress}%</strong><div className="progress-track"><span style={{ width: onboarding.overall_progress + '%' }} /></div></section>
    <ol className="stages" aria-label="Этапы адаптации">{onboarding.stages.map((stage, index) => <li key={stage.id} className={'stage ' + stage.status} aria-current={stage.status === 'current' ? 'step' : undefined}>
      <span className="stage-number">{stage.status === 'completed' ? <Check size={16} /> : index + 1}</span><div><small>{stage.period}</small><strong>{stage.title}</strong><span>{stage.progress_percent}% выполнено</span></div>
    </li>)}</ol>
    {onboarding.next_recommended_step ? <div className="recommendation"><Sparkles size={21} /><div><strong>Рекомендуем продолжить</strong><span>{onboarding.next_recommended_step.title}</span></div><button className="text-link" onClick={() => onSelectPrompt(onboarding.next_recommended_step!.suggested_prompt)}>Спросить AI-buddy<ArrowRight size={16} /></button></div> : <div className="recommendation"><Check size={22} /><div><strong>Все шаги завершены</strong><span>Вы прошли текущий чек-лист адаптации.</span></div></div>}
    <div className="section-heading"><h2>Ваш чек-лист</h2><span>{data?.current_user.team}</span></div>
    <div className="checklist">{onboarding.checklist.map((item, index) => <article className={'card task-card ' + (item.status === 'done' ? 'task-done' : '')} key={item.id}>
      <span className="task-index">{item.status === 'done' ? <Check size={18} /> : <Circle size={17} />}</span><div className="task-body">
        <div className="task-heading"><div><span className="task-kicker">ШАГ {String(index + 1).padStart(2, '0')}</span><h3>{item.title}</h3></div><label className="status-select"><span className="sr-only">Статус: {item.title}</span><select aria-label={'Статус: ' + item.title} disabled={isBusy} value={item.status} onChange={event => onToggleChecklist(item.id, event.target.value as ChecklistStatus)}>
          {Object.entries(checklistLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select></label></div>
        <p>{item.description}</p><div className="task-footer"><span className="muted-line"><Clock3 size={14} />{item.estimated_time}</span><button className="text-link" onClick={() => onSelectPrompt(item.suggested_prompt)}>Обсудить с помощником<ArrowRight size={15} /></button></div>
      </div>
    </article>)}</div>
  </div>
}
