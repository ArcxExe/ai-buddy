import { create } from 'zustand'

// UI only. All server data and request status are owned by TanStack Query.
interface AppStore {
  isMobileNavOpen: boolean
  toggleMobileNav: () => void
  closeMobileNav: () => void
}
export const useAppStore = create<AppStore>(set => ({
  isMobileNavOpen: false,
  toggleMobileNav: () => set(state => ({ isMobileNavOpen: !state.isMobileNavOpen })),
  closeMobileNav: () => set({ isMobileNavOpen: false }),
}))
