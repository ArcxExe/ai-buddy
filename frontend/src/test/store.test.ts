import { describe, it, expect, beforeEach } from 'vitest'
import { useAppStore } from '../store/useAppStore'
describe('UI store', () => {
  beforeEach(() => useAppStore.setState(useAppStore.getInitialState(), true))
  it('only stores mobile navigation UI state and actions', () => {
    expect(Object.keys(useAppStore.getState()).sort()).toEqual(['closeMobileNav', 'isMobileNavOpen', 'toggleMobileNav'])
  })
  it('opens, toggles and explicitly closes navigation', () => {
    useAppStore.getState().toggleMobileNav()
    expect(useAppStore.getState().isMobileNavOpen).toBe(true)
    useAppStore.getState().toggleMobileNav()
    expect(useAppStore.getState().isMobileNavOpen).toBe(false)
    useAppStore.getState().toggleMobileNav()
    useAppStore.getState().closeMobileNav()
    expect(useAppStore.getState().isMobileNavOpen).toBe(false)
  })
})
