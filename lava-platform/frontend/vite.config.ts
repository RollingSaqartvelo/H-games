import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.VITE_API_TARGET ?? 'http://localhost:8080'
  const wsTarget  = env.VITE_WS_TARGET  ?? 'ws://localhost:8080'

  return {
    plugins: [react()],

    // In production the frontend is served by the operator's nginx/Node.js,
    // which also handles HMAC signing before proxying to the game provider.
    // In development, Vite proxies requests directly to the Go backend.
    server: {
      port: 3000,
      allowedHosts: true,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
        '/ws': {
          target: wsTarget,
          ws: true,
          changeOrigin: true,
        },
        '/tma': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },

    build: {
      outDir: 'dist',
      sourcemap: false,
      rollupOptions: {
        output: {
          manualChunks: {
            react: ['react', 'react-dom'],
            zustand: ['zustand'],
          },
        },
      },
    },
  }
})
