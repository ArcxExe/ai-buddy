import { useSyncExternalStore } from 'react'
import type { AppContextResponse } from '../types'

type DraftContext = Pick<AppContextResponse, 'current_user' | 'active_role'>

export function draftKey(context: DraftContext | undefined, kind: 'chat' | 'review', requestId?: string) {
  if (!context) return undefined
  return 'ai-buddy:draft:v1:' + JSON.stringify([context.current_user.id, context.active_role, kind, requestId ?? ''])
}

// Only unsent text lives here. Nothing is written by mounting/unmounting a view.
// Failed writes remain available in memory for navigation within this page session.
export function createDraftStorage() {
  const fallback = new Map<string, string>()
  const unsaved = new Set<string>()
  const listeners = new Set<() => void>()
  const read = (key: string | undefined): string => {
    if (!key) return ''
    if (unsaved.has(key)) return fallback.get(key)!
    try {
      const text = window.sessionStorage.getItem(key) ?? ''
      fallback.set(key, text)
      return text
    } catch { return fallback.get(key) ?? '' }
  }
  const write = (key: string | undefined, text: string) => {
    if (!key) return
    fallback.set(key, text)
    unsaved.add(key)
    try {
      if (text) window.sessionStorage.setItem(key, text)
      else window.sessionStorage.removeItem(key)
      unsaved.delete(key)
    } catch { /* Storage denial/quota errors must not interrupt editing. */ }
    listeners.forEach(listener => listener())
  }
  return {
    read,
    write,
    clearIfUnchanged(key: string | undefined, submitted: string) {
      if (read(key) === submitted) write(key, '')
    },
    subscribe(listener: () => void) {
      listeners.add(listener)
      return () => { listeners.delete(listener) }
    },
  }
}

export const draftStorage = createDraftStorage()

export function useDraft(key: string | undefined) {
  const text = useSyncExternalStore(draftStorage.subscribe, () => draftStorage.read(key), () => '')
  return [text, (value: string) => draftStorage.write(key, value)] as const
}
