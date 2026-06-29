import { defineConfig, type Connect, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// dev/preview 下精确的 /kanban（不带尾斜杠）302 到 /kanban/（保留 query），
// 对齐生产 nginx 行为；vite 自带的 base 回落只覆盖 /，不覆盖不带斜杠的 base。
function kanbanSlashRedirect(): Plugin {
  const middleware: Connect.NextHandleFunction = (req, res, next) => {
    const [pathname, query] = (req.url ?? '').split('?')
    if (pathname === '/kanban') {
      res.statusCode = 302
      res.setHeader('Location', '/kanban/' + (query ? `?${query}` : ''))
      res.end()
      return
    }
    next()
  }
  return {
    name: 'kanban-slash-redirect',
    configureServer(server) {
      server.middlewares.use(middleware)
    },
    configurePreviewServer(server) {
      server.middlewares.use(middleware)
    },
  }
}

// dev 时 /kanban/api 代理到后端 Go 服务(9990)，并剥掉 /kanban 前缀（后端注册的是 /api/v2/*）。
// 对齐生产：外层 ingress 透传 /kanban → portal nginx location /kanban/api/ 反代并 strip。
// 产物 dist/ 由 compose/portal/Dockerfile 塞进 nginx，nginx 已有 /kanban/api 代理 + SPA fallback。
export default defineConfig({
  // 整站挂在 /kanban 子路径下：assets 引用前缀 /kanban/，与 router basename=/kanban 配套。
  base: '/kanban/',
  plugins: [react(), tailwindcss(), kanbanSlashRedirect()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 8881,
    proxy: {
      // 默认代理到本地后端(9990，剥 /kanban 前缀，后端注册 /api/v2/*)。
      // 设 KANBAN_API_TARGET 时改指远端看板(如内网生产)：保留 /kanban 走其网关，
      // 并可经 KANBAN_API_AUTH / KANBAN_API_COOKIE 注入登录凭据（仅本机调试，凭据走 env、勿提交）。
      '/kanban/api': (() => {
        const remote = process.env.KANBAN_API_TARGET
        const auth = process.env.KANBAN_API_AUTH
        const cookie = process.env.KANBAN_API_COOKIE
        return {
          target: remote || 'http://localhost:9990',
          changeOrigin: true,
          secure: false,
          rewrite: remote ? undefined : (p: string) => p.replace(/^\/kanban/, ''),
          headers: {
            ...(auth ? { Authorization: auth } : {}),
            ...(cookie ? { Cookie: cookie } : {}),
          },
        }
      })(),
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        // 代码分割：把体量大的依赖拆到独立 chunk，降低主入口 chunk 体积、提升首屏与缓存命中。
        // echarts 独立成块（最大头）；react 运行时 + 路由一块；数据/状态/HTTP 一块。
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('echarts') || id.includes('zrender')) return 'echarts'
          if (
            id.includes('react-dom') ||
            id.includes('/react/') ||
            id.includes('react-router') ||
            id.includes('scheduler')
          ) {
            return 'react-vendor'
          }
          if (id.includes('@tanstack') || id.includes('zustand') || id.includes('axios')) {
            return 'data-vendor'
          }
          return undefined
        },
      },
    },
  },
})
