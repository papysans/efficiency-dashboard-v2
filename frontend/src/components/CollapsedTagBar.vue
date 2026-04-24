<template>
  <Teleport to="body">
    <Transition name="kb-panel-slide">
      <div v-if="tags.length > 0" class="kb-collapsed-panel">
        <div class="kb-collapsed-panel__title">已折叠</div>
        <button
          v-for="tag in tags"
          :key="tag.key"
          class="kb-collapsed-panel__item"
          :title="'展开：' + tag.label"
          @click="$emit('expand', tag.key)"
        >
          <svg viewBox="0 0 16 16" fill="none" class="kb-collapsed-panel__icon">
            <path d="M6 4l4 4-4 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span>{{ tag.label }}</span>
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
defineProps({
  tags: { type: Array, default: () => [] },
})
defineEmits(['expand'])
</script>

<style scoped>
.kb-collapsed-panel {
  position: fixed;
  top: 72px;
  right: 0;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 0 6px 6px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-right: none;
  border-radius: 8px 0 0 8px;
  box-shadow: -2px 4px 12px rgba(0, 0, 0, 0.08);
  min-width: 80px;
}

.kb-collapsed-panel__title {
  font-size: 10px;
  color: #c0c4cc;
  padding: 0 10px 4px 4px;
  letter-spacing: 0.5px;
  white-space: nowrap;
}

.kb-collapsed-panel__item {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px 5px 6px;
  border-radius: 6px 0 0 6px;
  border: none;
  background: transparent;
  color: #606266;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  text-align: left;
  transition: background 0.15s, color 0.15s;
  line-height: 1.4;
}

.kb-collapsed-panel__item:hover {
  background: #ecf5ff;
  color: #409eff;
}

.kb-collapsed-panel__icon {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
  opacity: 0.6;
}

.kb-collapsed-panel__item:hover .kb-collapsed-panel__icon {
  opacity: 1;
}

.kb-panel-slide-enter-active,
.kb-panel-slide-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.kb-panel-slide-enter-from,
.kb-panel-slide-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
