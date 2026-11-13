/// <reference types="vitest" />
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'

export default defineConfig(({ mode }) => {
  // Resolve env files relative to this config, including when npm uses --prefix.
  const env = loadEnv(mode, fileURLToPath(new URL('.', import.meta.url)), '')
  const port = Number(env.DEV_PORT || 5173)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('DEV_PORT must be an integer between 1 and 65535')
  }

  return {
    plugins: [react()],
    build: {
      manifest: true,
      rollupOptions: {
        output: {
          // Keep Markdown and its parser dependencies in the lazy route graph.
          // These three shared groups are required by the application shell/API.
          manualChunks(id) {
            const module = id.replaceAll('\\', '/')
            if (/\/node_modules\/(react|react-dom|scheduler|react-router|react-router-dom)\//.test(module)) return 'framework'
            if (module.includes('/node_modules/@tanstack/')) return 'query'
            if (module.includes('/node_modules/zod/')) return 'validation'
            if (module.includes('/node_modules/lucide-react/')) return 'icons'
          },
        },
      },
    },
    server: {
      host: env.DEV_HOST || '127.0.0.1',
      port,
      strictPort: true,
      // Generated HTML reports must not trigger dev-page reloads during tests.
      watch: { ignored: ['**/coverage/**'] },
      proxy: {
        '/api': {
          target: env.API_PROXY_TARGET || 'http://127.0.0.1:18080',
          changeOrigin: true,
        },
      },
    },
    test: {
      globals: true,
      environment: 'jsdom',
      setupFiles: './src/test/setup.ts',
      coverage: {
        provider: 'v8',
        reporter: ['text', 'json', 'html'],
        exclude: [
          'node_modules/**',
          'dist/**',
          '*.config.*',
          'src/main.tsx',
          'src/types/**',
          'src/test/**',
        ],
      },
    },
  }
})
