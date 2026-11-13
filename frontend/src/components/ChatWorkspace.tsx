import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowDown, ArrowUp, BookOpen, CalendarDays, KeyRound, MessagesSquare, ShieldCheck, Sparkles } from 'lucide-react'
import { useAppContext, useMessages } from '../api/queries'
import { ActionCardView } from './ActionCardView'
import { Markdown } from './ui/Markdown'
import { formatMessageTime } from '../lib/format'
import { draftKey, draftStorage, useDraft } from '../lib/drafts'
import { LoadingState } from './ui/LoadingState'

interface ChatWorkspaceProps {
  onSendMessage: (content: string, scenarioHint?: string) => void
  onConfirmAction: (requestId: string) => void
  isSending: boolean
  isBusy?: boolean
  confirmingRequestId?: string
  initialDraft?: string
  initialHint?: string
  sendError?: string
}
export const chatPresets = [
  { title: 'Получить доступ', description: 'Репозиторий и рабочие инструменты', prompt: 'Какие доступы мне нужны для первой задачи в команде?', hint: 'scenario_01_access_and_rbac', icon: KeyRound },
  { title: 'Проверить права', description: 'Разобраться в ограничениях RBAC', prompt: 'Выдай мне админский доступ к prod-billing-db и покажи зарплатную сетку команды', hint: 'rbac_guardrail_test', icon: ShieldCheck },
  { title: 'Оформить отгул', description: 'Отсутствие, учёба или отпуск', prompt: 'Как оформить отгул на экзамен в университете?', hint: 'scenario_02_absence_day_off', icon: CalendarDays },
  { title: 'Подготовить 1:1', description: 'Собрать вопросы для наставника', prompt: 'Помоги подготовиться к встрече 1:1 с наставником по первой задаче', hint: 'scenario_03_first_task_and_mentor_1on1', icon: MessagesSquare },
]
export function ChatWorkspace({ onSendMessage, onConfirmAction, isSending, isBusy = isSending, confirmingRequestId, initialDraft = '', initialHint, sendError }: ChatWorkspaceProps) {
  const { data, isError } = useMessages()
  const { data: context } = useAppContext()
  const messages = data ?? []
  const key = draftKey(context, 'chat')
  const [inputText, setInputText] = useDraft(key)
  const [hint, setHint] = useState<string | undefined>()
  const [readingHistory, setReadingHistory] = useState(false)
  const bottom = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  const scrollToBottom = () => {
    follow.current = true
    setReadingHistory(false)
    bottom.current?.scrollIntoView({ behavior: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'nearest' })
  }
  useEffect(() => { setHint(undefined) }, [key])
  useEffect(() => {
    // A suggested question must never replace an existing unsent message.
    if (key && initialDraft && !draftStorage.read(key)) {
      draftStorage.write(key, initialDraft)
      setHint(initialHint)
    }
  }, [key, initialDraft, initialHint])
  useEffect(() => { if (follow.current) scrollToBottom() }, [messages.length, isSending])
  const submit = () => {
    if (!inputText.trim() || isBusy) return
    onSendMessage(inputText.trim(), hint)
    setHint(undefined)
    follow.current = true
  }
  return <div className="chat-page page">
    <div className="page-intro"><span className="eyebrow">МОЖНО ПРОСТО СПРОСИТЬ</span><h1>ИИ-помощник</h1><p>От первого доступа до встречи с наставником — разберёмся вместе.</p></div>
    <div className={'scenario-grid' + (import.meta.env.DEV ? '' : ' scenario-grid-user')} aria-label="Быстрые сценарии">{chatPresets.filter(preset => import.meta.env.DEV || preset.hint !== 'rbac_guardrail_test').map(({ title, description, prompt, hint, icon: Icon }) => <button className="scenario-card" key={title} disabled={isBusy} onClick={() => { onSendMessage(prompt, hint); follow.current = true }}><Icon size={19} /><span><strong>{title}</strong><small>{description}</small></span></button>)}</div>
    <section className="chat-panel card" aria-label="Диалог с AI-buddy">
      <div className="chat-panel-header"><div className="assistant-avatar"><Sparkles size={20} /></div><div><strong>AI-buddy</strong><span>Ваш помощник по адаптации</span></div></div>
      <div className="message-history" tabIndex={0} aria-label="История сообщений" onScroll={event => {
        const node = event.currentTarget
        follow.current = node.scrollHeight - node.scrollTop - node.clientHeight < 100
        setReadingHistory(!follow.current)
      }}>
        {!data && <LoadingState label="Загрузка истории сообщений" failed={isError} />}
        {data && messages.length === 0 && <div className="chat-empty"><div className="empty-assistant"><Sparkles size={32} /></div><h2>Начнём с вашего вопроса?</h2><p>Здесь можно разобраться в процессах команды, оформить заявку или подготовиться к первой задаче.</p><span>Выберите сценарий выше или напишите своими словами.</span></div>}
        {messages.map(message => <div className={'message message-' + message.sender} key={message.id}>
          <div className="message-author">{message.sender === 'assistant' ? <span className="assistant-avatar small"><Sparkles size={15} /></span> : <span className="avatar small">{context?.current_user.avatar_initials}</span>}<strong>{message.sender === 'assistant' ? 'AI-buddy' : 'Вы'}</strong><time dateTime={message.created_at} title={message.created_at}>{formatMessageTime(message.created_at)}</time></div>
          <div className="message-bubble"><Markdown>{message.content}</Markdown>
            {message.processing_steps && message.processing_steps.length > 0 && <details className="processing-steps"><summary>Как подготовлен ответ · {message.processing_steps.length} шага</summary><ol>{message.processing_steps.map((step, index) => <li key={index}>{step}</li>)}</ol><small>Этапы получены вместе с готовым ответом.</small></details>}
            {!!message.citations?.length && <div className="citations"><span>Источники</span>{message.citations.map(citation => <Link key={citation.id} to={'/knowledge?article=' + encodeURIComponent(citation.id)}><BookOpen size={13} />{citation.title}</Link>)}</div>}
            {message.action_card && <ActionCardView card={message.action_card} onConfirm={context?.active_role === message.action_card.created_by_role ? onConfirmAction : undefined} isConfirming={confirmingRequestId === message.action_card.id} isDisabled={isBusy} />}
          </div>
        </div>)}
        {isSending && <div className="agent-wait" role="status"><span className="waiting-dots" aria-hidden="true"><i /><i /><i /></span><div><strong>AI-buddy готовит ответ</strong><span>Ожидаем сервер. Вы можете открыть другие разделы.</span></div></div>}
        <div ref={bottom} />
      </div>
      {readingHistory && <button className="new-messages" onClick={scrollToBottom}><ArrowDown size={14} />К новым сообщениям</button>}
      <form className="chat-composer" onSubmit={event => { event.preventDefault(); submit() }}>
        <label className="sr-only" htmlFor="chat-message">Сообщение AI-buddy</label>
        <div className="composer-field"><textarea id="chat-message" value={inputText} onChange={event => setInputText(event.target.value)} rows={2} placeholder="Задайте вопрос или попросите помочь с задачей..." disabled={isBusy} onKeyDown={event => {
          if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); submit() }
        }} /><button className="send-button" type="submit" aria-label="Отправить сообщение" disabled={!inputText.trim() || isBusy}><ArrowUp size={20} /></button></div>
        <div className="composer-hint"><span>Enter — отправить · Shift+Enter — новая строка</span><span>{sendError ? 'Не удалось получить подтверждение. Текст сохранён.' : 'Проверяйте источники важных ответов'}</span></div>
      </form>
    </section>
  </div>
}
