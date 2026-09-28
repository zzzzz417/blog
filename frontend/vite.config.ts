import { fileURLToPath, URL } from 'node:url'

import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, fileURLToPath(new URL('..', import.meta.url)), 'BLOG_')
  const backend = env.BLOG_DEV_API_TARGET || 'http://127.0.0.1:8080'

  return {
    plugins: [vue()],
    server: {
      proxy: {
        // Keep the browser's Host so the backend can verify same-origin writes.
        '/api': { target: backend, changeOrigin: false },
        '/media': { target: backend, changeOrigin: false },
      },
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
  }
})
