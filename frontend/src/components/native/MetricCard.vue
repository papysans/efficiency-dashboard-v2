<template>
  <article class="kn-metric" :style="{ '--metric-accent': accent }">
    <p class="kn-metric-label">{{ label }}<el-tooltip v-if="tip" :content="tip" placement="top" :show-after="60" popper-class="kn-tip"><sup class="kn-th-mark">?</sup></el-tooltip></p>
    <p class="kn-metric-value" :class="{ 'is-pos': tone === 'pos', 'is-neg': tone === 'neg' }">
      <slot>{{ value }}</slot>
    </p>
    <p v-if="hint" class="kn-metric-hint" :title="hint">{{ hint }}</p>
  </article>
</template>

<script setup>
defineProps({
  label: { type: String, default: '' },
  value: { type: [String, Number], default: '-' },
  hint: { type: String, default: '' },
  // tip: 可选的 label 旁「?」悬浮说明（口径文案）。不传则不渲染，向后兼容。
  tip: { type: String, default: '' },
  accent: { type: String, default: 'var(--native-primary)' },
  // tone: 'pos' | 'neg' | '' —— 用于给数值上色（提效正/负）
  tone: { type: String, default: '' },
})
</script>

<style scoped>
.kn-metric {
  min-width: 0;
  border-radius: var(--native-radius-lg);
  border: 1px solid color-mix(in oklab, var(--native-border) 24%, transparent);
  background: color-mix(in oklab, var(--native-panel) 88%, var(--native-bg-subtle));
  padding: 1rem;
  box-shadow: var(--native-shadow-sm);
}
.kn-metric-label {
  margin: 0;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: color-mix(in oklab, var(--metric-accent) 72%, var(--native-dim));
}
.kn-metric-value {
  margin: 0.5rem 0 0;
  font-size: 1.4rem;
  line-height: 1.05;
  font-weight: 600;
  letter-spacing: -0.04em;
  color: var(--native-foreground);
}
.kn-metric-value.is-pos { color: var(--native-success); }
.kn-metric-value.is-neg { color: var(--native-error); }
.kn-metric-hint {
  margin: 0.5rem 0 0;
  font-size: 0.8125rem;
  color: var(--native-muted);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
