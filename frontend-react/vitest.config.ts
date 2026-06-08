import { defineConfig } from 'vitest/config'
import path from 'node:path'

// 纯逻辑单测配置：只解析 @ 别名 + node 环境，不加载 react/tailwind 插件（更快、更稳）。
// 组件/DOM 测试如需 jsdom，可在对应测试文件用 // @vitest-environment jsdom 覆盖。
export default defineConfig({
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.{test,spec}.ts'],
  },
})
