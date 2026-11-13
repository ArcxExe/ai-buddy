import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { BookOpen, Search, ArrowUpRight, LockKeyhole, X } from 'lucide-react'
import { useAppContext, useKnowledge } from '../api/queries'
import { getErrorMessage } from '../api/client'
import { Markdown } from '../components/ui/Markdown'
import { Notice } from '../components/ui/Notice'
import { LoadingState } from '../components/ui/LoadingState'

export function KnowledgePage() {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') || ''
  const articleId = params.get('article')
  const [input, setInput] = useState(query)
  const context = useAppContext()
  const knowledge = useKnowledge(query, true)
  const heading = useRef<HTMLHeadingElement>(null)
  const items = knowledge.data?.items ?? []
  const article = items.find(item => item.id === articleId)
  const error = knowledge.error
  useEffect(() => { setInput(query) }, [query])
  useEffect(() => { if (article) heading.current?.focus({ preventScroll: true }) }, [article?.id])
  return <div className="page knowledge-page"><div className="page-intro"><span className="eyebrow">ОТВЕТЫ УЖЕ ЕСТЬ</span><h1>База знаний</h1><p>Регламенты и инструкции вашей команды. Только доступное вашей роли.</p></div>
    <form className="knowledge-search" onSubmit={event => { event.preventDefault(); if (input === query) void knowledge.refetch(); else setParams(input ? { q: input } : {}) }}>
      <Search size={21} aria-hidden="true" /><label className="sr-only" htmlFor="knowledge-query">Поиск по базе знаний</label><input id="knowledge-query" value={input} onChange={event => setInput(event.target.value)} placeholder="Найти инструкцию, процесс или ответ..." /><button className="button button-dark" type="submit">Найти</button>
    </form>
    <div className="knowledge-meta"><span>{knowledge.data ? 'Доступно материалов: ' + items.length : knowledge.isIdentityUncertain || context.isError ? 'Материалы скрыты до обновления контекста' : error ? 'Материалы временно недоступны' : 'Загружаем материалы'}</span>{!!knowledge.data?.hidden_by_rbac_count && <span><LockKeyhole size={14} />Недоступно вашей роли: {knowledge.data.hidden_by_rbac_count}</span>}</div>
    {error && <Notice tone="error" action={<button className="button button-secondary" disabled={knowledge.isFetching} onClick={() => { void knowledge.refetch() }}>Повторить</button>}>{getErrorMessage(error)}</Notice>}
    {knowledge.data && knowledge.isFetching && <Notice tone="info">Обновляем материалы. Последний результат остаётся доступен.</Notice>}
    {!knowledge.data ? <LoadingState label="Загрузка базы знаний" failed={!!error || context.isError || knowledge.isIdentityUncertain} /> : <>
      {articleId && !article && <Notice tone="warning">Статья не найдена среди доступных материалов. Проверьте роль или поисковый запрос.</Notice>}
      <div className={'knowledge-layout' + (article ? ' has-article' : '')}>
        <div className="knowledge-list">{items.length === 0 ? <div className="card empty-state"><BookOpen size={30} /><h2>Ничего не найдено</h2><p>Попробуйте другие слова или очистите поисковый запрос.</p>{query && <button className="button button-secondary" onClick={() => setParams({})}>Сбросить поиск</button>}</div> : items.map(item => <article className={'card knowledge-card' + (item.id === articleId ? ' selected' : '')} key={item.id}><div className="card-topline"><span className="neutral-badge">{item.category}</span><BookOpen size={19} className="muted-icon" /></div><h2>{item.title}</h2><p>{item.summary}</p><div className="article-source">{item.source}</div><button className="text-link" onClick={() => setParams({ ...(query ? { q: query } : {}), article: item.id })} aria-label={'Читать: ' + item.title}>Читать инструкцию<ArrowUpRight size={15} /></button></article>)}</div>
        {article && <article className="card article-reader"><div className="card-topline"><span className="section-kicker">ИНСТРУКЦИЯ</span><button className="icon-button" aria-label="Закрыть статью" onClick={() => setParams(query ? { q: query } : {})}><X size={19} /></button></div><h2 tabIndex={-1} ref={heading}>{article.title}</h2><p className="article-source">{article.source}</p><Markdown>{article.content}</Markdown>{!!article.tags?.length && <div className="article-tags">{article.tags.map(tag => <span className="neutral-badge" key={tag}>{tag}</span>)}</div>}</article>}
      </div>
    </>}
  </div>
}
