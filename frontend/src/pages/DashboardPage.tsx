import { Link } from 'react-router-dom'
import { ArrowRight, ArrowUpRight, MessagesSquare, ListChecks, BookOpen, Files, Check, Sparkles, Clock3 } from 'lucide-react'
import { useAppContext } from '../api/queries'
import { LoadingState } from '../components/ui/LoadingState'

const quickActions = [
  { to: '/chat', title: 'Спросить AI-buddy', text: 'Помощь в нужный момент', icon: MessagesSquare, tone: 'blue' },
  { to: '/onboarding', title: 'Продолжить адаптацию', text: 'Шаг за шагом к цели', icon: ListChecks, tone: 'green' },
  { to: '/knowledge', title: 'Найти инструкцию', text: 'Знания вашей команды', icon: BookOpen, tone: 'purple' },
  { to: '/requests', title: 'Посмотреть заявки', text: 'Все действия в одном месте', icon: Files, tone: 'sand' },
]
export function DashboardPage() {
  const { data, isError } = useAppContext()
  if (!data) return <div className="page"><LoadingState label="Загрузка главной страницы" failed={isError} /></div>
  const { current_user: user, onboarding, metrics } = data
  const stage = onboarding.stages.find(item => item.id === onboarding.current_stage_id)
  const done = onboarding.checklist.filter(item => item.status === 'done').length
  const next = onboarding.next_recommended_step
  return <div className="page dashboard">
    <div className="page-intro"><span className="eyebrow">ВАШ СТАРТ В АВИТО</span><h1>Привет, {user.name.split(' ')[0]}<span className="greeting-dot">.</span></h1><p>Осваивайтесь в команде в своём темпе. Мы рядом на каждом шаге.</p></div>
    <div className="dashboard-primary">
      <section className="progress-card" aria-label="Прогресс адаптации">
        <div className="card-topline"><span className="section-kicker"><span className="small-dot" />Моя адаптация</span><Link to="/onboarding" className="icon-link" aria-label="Открыть мою адаптацию"><ArrowUpRight size={20} /></Link></div>
        <div className="progress-card-body"><div><h2>Хорошее начало.<br />Дальше — больше.</h2><p>{stage?.title || 'Ваш путь адаптации'}</p><span className="completion-count"><Check size={15} />{done} из {onboarding.checklist.length} шагов завершено</span></div><div className="progress-ring" style={{ background: 'conic-gradient(#008a50 ' + onboarding.overall_progress + '%, #dceee4 0)' }}><div><strong>{onboarding.overall_progress}<span>%</span></strong><small>вашего пути</small></div></div></div>
        <Link className="text-link" to="/onboarding">Продолжить адаптацию<ArrowRight size={16} /></Link>
      </section>
      <section className="next-card card"><div className="card-topline"><span className="section-kicker">СЛЕДУЮЩИЙ ШАГ</span><span className="icon-tile purple"><Sparkles size={19} /></span></div>
        <h2>{next?.title || (onboarding.checklist.length ? 'Все шаги завершены' : 'Чек-лист пока не назначен')}</h2><p>{next?.description || (onboarding.checklist.length ? 'Отличная работа! Все задачи текущего чек-листа выполнены. Помощник по-прежнему рядом.' : 'Шаги адаптации появятся здесь после назначения. Пока можно обратиться к помощнику.')}</p>
        {next && <span className="muted-line"><Clock3 size={14} />{next.estimated_time}</span>}
        <Link className="button button-dark" to={next ? '/onboarding' : '/chat'}>{next ? 'Перейти к шагу' : 'Открыть помощника'}<ArrowRight size={16} /></Link>
      </section>
    </div>
    <section aria-labelledby="quick-title"><div className="section-heading"><h2 id="quick-title">С чего начнём?</h2><span>Выберите, что нужно сейчас</span></div>
      <div className="quick-grid">{quickActions.map(({ to, title, text, icon: Icon, tone }) => <Link className="quick-card card" key={to} to={to}><div className="card-topline"><span className={'icon-tile ' + tone}><Icon size={21} /></span><ArrowUpRight size={17} className="muted-icon" /></div><h3>{title}</h3><p>{text}</p></Link>)}</div>
    </section>
    <div className="dashboard-secondary">
      <section className="card impact-card"><div className="section-heading"><h2>Помощь, которую видно</h2><span className="neutral-badge">Расчётные показатели</span></div><div className="metric-grid"><div><strong>{metrics.routine_questions_resolved}</strong><span>вопросов разобрано</span></div><div><strong>{metrics.mcp_actions_executed}</strong><span>действий выполнено</span></div><div><strong>{metrics.mentor_hours_saved.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}<small> ч</small></strong><span>оценка экономии времени</span></div></div><p className="fine-print">Экономия времени — расчётная оценка, а не измеренный бизнес-результат.</p></section>
      <section className="card team-card"><span className="section-kicker">ВАША КОМАНДА</span><h2>{user.team}</h2>{user.mentor_name && <div className="team-person"><span className="avatar small">{user.mentor_name.split(' ').map(part => part[0]).slice(0, 2).join('')}</span><div><strong>{user.mentor_name}</strong><span>Наставник · ваш buddy</span></div></div>}<Link className="text-link" to="/chat" state={{ prompt: 'Помоги подготовиться к встрече 1:1 с наставником', hint: 'scenario_03_first_task_and_mentor_1on1' }}>Подготовиться к 1:1<ArrowRight size={15} /></Link></section>
    </div>
  </div>
}
