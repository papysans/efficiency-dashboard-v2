import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// 与现有 Vue 前端一致：dev 时 /api 代理到后端 Go 服务(9990)。
// 产物 dist/ 由 compose/portal/Dockerfile 塞进 nginx，nginx 已有 /api 代理 + SPA fallback。
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 8881,
    proxy: {
      '/api': { target: 'http://localhost:9990', changeOrigin: true },
    },
  },
  build: { outDir: 'dist', sourcemap: false },
})
