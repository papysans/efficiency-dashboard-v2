<template>
  <el-container class="app-container">
    <el-header class="app-header">
      <el-menu
        :default-active="activeMenu"
        mode="horizontal"
        :ellipsis="false"
        router
        class="app-menu"
      >
        <el-menu-item index="app-logo" class="app-logo" disabled>
          <el-icon><DataAnalysis /></el-icon>
          <span>AI Coding 指标看板</span>
        </el-menu-item>
        <el-menu-item index="/">首页</el-menu-item>
        <el-sub-menu index="group-org">
          <template #title>组织看板</template>
          <el-menu-item index="/org-v2">组织</el-menu-item>
          <el-menu-item index="/user-v2">用户</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="group-project">
          <template #title>项目看板</template>
          <el-menu-item index="/needs-v2">需求 Need</el-menu-item>
          <el-menu-item index="/project-v2">项目</el-menu-item>
          <el-menu-item index="/repo-v2">仓库</el-menu-item>
          <el-menu-item index="/commit-v2">提交</el-menu-item>
          <el-menu-item index="/task-v2">任务</el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-header>
    <el-main class="app-main">
      <router-view v-slot="{ Component }">
        <keep-alive :include="['TaskViewV2']">
          <component :is="Component" />
        </keep-alive>
      </router-view>
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const activeMenu = computed(() => {
  if (route.path.startsWith('/needs/')) return '/needs-v2'
  if (route.path.startsWith('/kanban/need')) return '/needs-v2'
  if (route.path === '/cloud/kanban') return '/needs-v2'
  return route.path
})
</script>

<style scoped>
.app-container {
  height: 100vh;
}

.app-header {
  padding: 0;
  height: auto;
}

.app-menu {
  display: flex;
  align-items: center;
}

.app-logo {
  font-size: 18px;
  font-weight: bold;
  color: #409eff !important;
  cursor: default !important;
}

.app-logo:hover {
  background-color: transparent !important;
}

.app-main {
  background-color: #f5f7fa;
  overflow-y: auto;
}
</style>
