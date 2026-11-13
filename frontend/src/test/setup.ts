import '@testing-library/jest-dom'
import { beforeEach } from 'vitest'

// Each test starts a separate browser session. Navigation within a test keeps drafts.
beforeEach(() => window.sessionStorage.clear())

// Mock scrollTo and scrollIntoView
window.HTMLElement.prototype.scrollIntoView = () => {}
