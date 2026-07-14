import path from 'node:path'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:3008',
      '/open': 'http://127.0.0.1:3008',
      '/dav': 'http://127.0.0.1:3008',
    },
  },
  build: {
    target: 'es2022',
    sourcemap: true,
  },
})
