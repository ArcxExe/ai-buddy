import React from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { context } from './fixtures'
import { queryKeys } from '../api/queries'

export function createTestClient(seed = true) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, gcTime: Infinity },
      mutations: { retry: false },
    },
  })
  if (seed) {
    client.setQueryData(queryKeys.context, structuredClone(context))
    client.setQueryData(queryKeys.messages, [])
  }
  return client
}
export function withClient(client: QueryClient, path = '/') {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}>{children}</MemoryRouter></QueryClientProvider>
  }
}
export function renderWithClient(ui: React.ReactNode, client: QueryClient, path = '/') {
  return render(ui, { wrapper: withClient(client, path) })
}
export function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
