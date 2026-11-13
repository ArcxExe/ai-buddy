import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAppContext } from '../api/queries'
import { ChatWorkspace } from '../components/ChatWorkspace'
import type { ComponentProps } from 'react'

type ChatPageProps = Omit<ComponentProps<typeof ChatWorkspace>, 'initialDraft' | 'initialHint'>
export function ChatPage(props: ChatPageProps) {
  const { state, pathname, search } = useLocation()
  const navigate = useNavigate()
  const { data } = useAppContext()
  const value: unknown = state
  const prompt = typeof value === 'object' && value !== null && 'prompt' in value && typeof value.prompt === 'string' ? value.prompt : ''
  const hint = typeof value === 'object' && value !== null && 'hint' in value && typeof value.hint === 'string' ? value.hint : undefined
  // Consume the navigation suggestion after the child has a user context to save
  // it. Back/reload must not resurrect a question that was successfully sent.
  useEffect(() => {
    if (data && prompt) navigate(pathname + search, { replace: true, state: null })
  }, [data, prompt, pathname, search, navigate])
  return <ChatWorkspace {...props} initialDraft={prompt} initialHint={hint} />
}
