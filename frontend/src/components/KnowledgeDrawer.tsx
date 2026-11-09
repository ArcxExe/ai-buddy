import React, { useState, useEffect } from 'react'
import { useAppStore } from '../store/useAppStore'
import { fetchKnowledge } from '../api/client'
import { KnowledgeArticle } from '../types'
import { X, Search, ShieldCheck, Lock, BookOpen } from 'lucide-react'

export const KnowledgeDrawer: React.FC = () => {
  const { isKnowledgeDrawerOpen, toggleKnowledgeDrawer, activeRole } = useAppStore()
  const [articles, setArticles] = useState<KnowledgeArticle[]>([])
  const [hiddenCount, setHiddenCount] = useState(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  useEffect(() => {
    if (isKnowledgeDrawerOpen) {
      loadArticles(searchQuery)
    }
  }, [isKnowledgeDrawerOpen, activeRole])

  const loadArticles = async (q?: string) => {
    setIsLoading(true)
    try {
      const data = await fetchKnowledge(activeRole, q)
      setArticles(data.visible_articles)
      setHiddenCount(data.hidden_by_rbac_count)
    } catch (e) {
      console.error(e)
    } finally {
      setIsLoading(false)
    }
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    loadArticles(searchQuery)
  }

  if (!isKnowledgeDrawerOpen) return null

  return (
    <div className="fixed inset-0 z-50 overflow-hidden bg-black/30 backdrop-blur-2xs flex justify-end">
      <div className="w-full max-w-md bg-white h-full shadow-2xl flex flex-col animate-in slide-in-from-right duration-200">
        {/* Header */}
        <div className="p-4 border-b border-avito-border flex items-center justify-between">
          <div className="flex items-center gap-2">
            <BookOpen className="w-4 h-4 text-avito-blue" />
            <h3 className="font-bold text-sm text-avito-dark">База знаний AvitoTech</h3>
          </div>
          <button
            onClick={toggleKnowledgeDrawer}
            className="p-1 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Search */}
        <form onSubmit={handleSearch} className="p-4 border-b border-gray-100 bg-gray-50/50">
          <div className="relative">
            <Search className="w-4 h-4 absolute left-3 top-3 text-gray-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Поиск по регламентам и Playbook..."
              className="w-full bg-white border border-gray-200 rounded-xl pl-9 pr-3 py-2 text-xs text-gray-900 focus:outline-hidden focus:border-avito-blue"
            />
          </div>
          {hiddenCount > 0 && (
            <div className="mt-2 text-[11px] text-amber-700 bg-amber-50 px-2.5 py-1.5 rounded-lg border border-amber-200 flex items-center gap-1.5">
              <Lock className="w-3 h-3 text-amber-600 flex-shrink-0" />
              <span>
                Скрыто политикой <b>RBAC</b> для роли <i>{activeRole}</i>: {hiddenCount} конфиденциальных документов
              </span>
            </div>
          )}
        </form>

        {/* Articles List */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {isLoading ? (
            <div className="text-center py-8 text-xs text-gray-400">Загрузка статей...</div>
          ) : articles.length === 0 ? (
            <div className="text-center py-8 text-xs text-gray-400">Ничего не найдено</div>
          ) : (
            articles.map((art) => (
              <div key={art.id} className="p-3.5 rounded-xl border border-gray-200/80 bg-white hover:border-blue-200 transition space-y-2">
                <div className="flex items-start justify-between gap-2">
                  <h4 className="text-xs font-bold text-avito-dark">{art.title}</h4>
                  <span className={`text-[10px] font-bold px-2 py-0.5 rounded-full flex-shrink-0 ${
                    art.sensitivity === 'intern'
                      ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                      : art.sensitivity === 'mentor_only'
                      ? 'bg-blue-50 text-blue-700 border border-blue-200'
                      : 'bg-purple-50 text-purple-700 border border-purple-200'
                  }`}>
                    {art.sensitivity}
                  </span>
                </div>
                <p className="text-xs text-gray-600 leading-relaxed">{art.summary}</p>
                <div className="text-[10px] text-gray-400 font-medium">Источник: {art.source}</div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
