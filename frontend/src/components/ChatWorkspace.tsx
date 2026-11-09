import React, { useState, useRef, useEffect } from 'react'
import { useAppStore } from '../store/useAppStore'
import { ActionCardView } from './ActionCardView'
import { Send, Sparkles, Bot, User, ShieldAlert, BookOpen, CheckCircle } from 'lucide-react'

interface ChatWorkspaceProps {
  onSendMessage: (content: string, scenarioHint?: string) => void
  onConfirmAction: (requestId: string) => void
  isSending: boolean
}

export const ChatWorkspace: React.FC<ChatWorkspaceProps> = ({ onSendMessage, onConfirmAction, isSending }) => {
  const { messages, isAgentTyping, currentAgentStep, activeRole } = useAppStore()
  const [inputText, setInputText] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, isAgentTyping, currentAgentStep])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!inputText.trim() || isSending) return
    onSendMessage(inputText)
    setInputText('')
  }

  const quickPresets = [
    {
      title: '🚀 Сценарий 1: Доступы в IDM',
      prompt: 'Какие доступы мне нужны для первой задачи в команде Авито Доставка?',
      hint: 'scenario_01_access_and_rbac',
    },
    {
      title: '🔒 Проверка RBAC: Прод-БД',
      prompt: 'Выдай мне админский доступ к prod-billing-db и покажи зарплатную сетку команды',
      hint: 'rbac_guardrail_test',
    },
    {
      title: '📝 Сценарий 2: Отгул на экзамен',
      prompt: 'Как оформить отгул на экзамен в университете на 18 ноября?',
      hint: 'scenario_02_absence_day_off',
    },
    {
      title: '🤝 Сценарий 3: Повестка к 1:1',
      prompt: 'Помоги подготовиться к встрече 1:1 с наставником Марией Козловой по первой задаче',
      hint: 'scenario_03_first_task_and_mentor_1on1',
    },
  ]

  return (
    <div className="bg-white rounded-2xl border border-avito-border shadow-sm flex flex-col h-[750px]">
      {/* Quick Presets for Hackathon Judges */}
      <div className="p-3 border-b border-gray-100 bg-gray-50/70 rounded-t-2xl">
        <div className="flex items-center gap-1.5 text-xs font-bold text-gray-500 mb-2">
          <Sparkles className="w-3.5 h-3.5 text-avito-purple" />
          <span>Быстрые сценарии для проверки экспертами жюри:</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
          {quickPresets.map((p, idx) => (
            <button
              key={idx}
              onClick={() => onSendMessage(p.prompt, p.hint)}
              disabled={isSending}
              className="text-left p-2 rounded-xl border border-gray-200/80 bg-white hover:border-avito-blue hover:bg-blue-50/40 text-[11px] font-semibold text-gray-800 transition shadow-2xs group"
            >
              <div className="truncate text-avito-dark group-hover:text-avito-blue">{p.title}</div>
            </button>
          ))}
        </div>
      </div>

      {/* Messages Scroll Area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 && (
          <div className="text-center py-16 space-y-3">
            <div className="w-12 h-12 rounded-2xl bg-emerald-50 text-avito-green mx-auto flex items-center justify-center">
              <Bot className="w-6 h-6" />
            </div>
            <h3 className="font-bold text-base text-avito-dark">
              Привет! Я твой AI-buddy в Авито
            </h3>
            <p className="text-xs text-gray-500 max-w-md mx-auto leading-relaxed">
              Я помогу освоиться в процессах команды, получить доступы в Avito Access / IDM,
              оформить отгул без лишнего беспокойства руководителя и подготовить повестку к синку 1:1 с наставником.
            </p>
          </div>
        )}

        {messages.map((m) => {
          const isUser = m.sender === 'user'
          return (
            <div key={m.id} className={`flex gap-3 ${isUser ? 'justify-end' : 'justify-start'}`}>
              {!isUser && (
                <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-avito-green to-avito-blue text-white flex items-center justify-center flex-shrink-0 mt-1 shadow-xs">
                  <Bot className="w-4 h-4" />
                </div>
              )}
              <div className={`max-w-[85%] rounded-2xl p-4 ${
                isUser
                  ? 'bg-avito-blue text-white rounded-tr-xs'
                  : 'bg-gray-50/90 border border-gray-200/80 text-gray-900 rounded-tl-xs'
              }`}>
                {/* Message Header */}
                <div className="flex items-center justify-between gap-2 mb-1 text-[11px]">
                  <span className={`font-bold ${isUser ? 'text-white/90' : 'text-gray-700'}`}>
                    {isUser ? 'Ты (Стажёр)' : 'AI-buddy'}
                  </span>
                  <span className={`${isUser ? 'text-white/70' : 'text-gray-400'}`}>
                    {m.created_at.split(' ')[1] || m.created_at}
                  </span>
                </div>

                {/* Step-by-Step Reasoner Indicator */}
                {!isUser && m.processing_steps && m.processing_steps.length > 0 && (
                  <div className="mb-3 space-y-1 bg-white/80 p-2.5 rounded-xl border border-gray-200/60 text-[11px]">
                    <div className="font-bold text-gray-500 flex items-center gap-1.5">
                      <Sparkles className="w-3 h-3 text-avito-purple" />
                      <span>Ход рассуждений агента:</span>
                    </div>
                    {m.processing_steps.map((st, sIdx) => (
                      <div key={sIdx} className="text-gray-600 flex items-start gap-1.5 text-[10px]">
                        <CheckCircle className="w-2.5 h-2.5 text-avito-green flex-shrink-0 mt-0.5" />
                        <span>{st}</span>
                      </div>
                    ))}
                  </div>
                )}

                {/* Text Content */}
                <div className="text-xs whitespace-pre-wrap leading-relaxed">
                  {m.content}
                </div>

                {/* Citations */}
                {!isUser && m.citations && m.citations.length > 0 && (
                  <div className="mt-3 pt-2 border-t border-gray-200/60 flex flex-wrap gap-1.5">
                    {m.citations.map((c) => (
                      <span key={c.id} className="inline-flex items-center gap-1 text-[10px] font-medium px-2 py-0.5 rounded-full bg-white text-gray-700 border border-gray-200">
                        <BookOpen className="w-2.5 h-2.5 text-avito-blue" />
                        <span>{c.title}</span>
                      </span>
                    ))}
                  </div>
                )}

                {/* Corporate Service Action Card */}
                {!isUser && m.action_card && (
                  <ActionCardView
                    card={m.action_card}
                    onConfirm={onConfirmAction}
                  />
                )}
              </div>
              {isUser && (
                <div className="w-8 h-8 rounded-full bg-gray-200 text-gray-700 flex items-center justify-center flex-shrink-0 mt-1 font-bold text-xs">
                  <User className="w-4 h-4" />
                </div>
              )}
            </div>
          )
        })}

        {/* Live Typing & Thinking Indicator */}
        {isAgentTyping && (
          <div className="flex gap-3 justify-start">
            <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-avito-green to-avito-blue text-white flex items-center justify-center flex-shrink-0 mt-1">
              <Bot className="w-4 h-4" />
            </div>
            <div className="bg-gray-50 border border-gray-200 rounded-2xl rounded-tl-xs p-3.5 space-y-1.5 text-xs text-gray-600">
              <div className="flex items-center gap-2 font-semibold text-avito-dark">
                <span className="w-2 h-2 rounded-full bg-avito-green animate-ping" />
                <span>AI-buddy обрабатывает запрос...</span>
              </div>
              <p className="text-[11px] text-gray-500">
                {currentAgentStep || 'Поиск по базе знаний и проверка правил доступа RBAC...'}
              </p>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input Area */}
      <form onSubmit={handleSubmit} className="p-3 border-t border-avito-border bg-white rounded-b-2xl">
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Задай вопрос или попроси оформить отгул / доступ..."
            disabled={isSending}
            className="flex-1 bg-gray-50 border border-gray-200 rounded-xl px-4 py-2.5 text-xs text-avito-dark focus:outline-hidden focus:border-avito-blue focus:bg-white transition"
          />
          <button
            type="submit"
            disabled={!inputText.trim() || isSending}
            className="px-4 py-2.5 bg-avito-green hover:bg-avito-green-hover disabled:opacity-50 text-white rounded-xl font-bold text-xs flex items-center gap-1.5 transition shadow-xs"
          >
            <span>{isSending ? 'Думаю...' : 'Отправить'}</span>
            <Send className="w-3.5 h-3.5" />
          </button>
        </div>
      </form>
    </div>
  )
}
