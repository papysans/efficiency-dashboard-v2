<template>
  <span class="kn-ratio-pill" :class="`is-${toneName}`">{{ label }}</span>
</template>

<script setup>
import { computed } from 'vue'
import { formatV2Ratio } from '@/utils/formatters'

const props = defineProps({
  // v2 提效比，小数口径：0.25 => 25%
  value: { type: [Number, String], default: null },
  digits: { type: Number, default: 1 },
})

const label = computed(() => formatV2Ratio(props.value, props.digits))

// 色调阈值对齐 opencode RatioPill（基于百分比值）：<0 红 / >=300 绿 / >=150 蓝 / 其余中性
const toneName = computed(() => {
  const num = Number(props.value)
  if (props.value == null || props.value === '' || !Number.isFinite(num)) return 'neutral'
  const pct = num * 100
  if (pct < 0) return 'neg'
  if (pct >= 300) return 'pos'
  if (pct >= 150) return 'info'
  return 'neutral'
})
</script>

<style scoped>
.kn-ratio-pill {
  display: inline-flex;
  min-width: 4.5rem;
  align-items: center;
  justify-content: center;
  border: 1px solid;
  border-radius: var(--native-radius-full);
  padding: 0.2rem 0.6rem;
  font-size: 0.75rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
.is-neutral { border-color: var(--native-border); background: var(--native-surface); color: var(--native-muted); }
.is-neg { border-color: color-mix(in oklab, var(--native-error) 30%, transparent); background: color-mix(in oklab, var(--native-error) 12%, transparent); color: var(--native-error); }
.is-info { border-color: color-mix(in oklab, var(--native-info) 30%, transparent); background: color-mix(in oklab, var(--native-info) 12%, transparent); color: var(--native-info); }
.is-pos { border-color: color-mix(in oklab, var(--native-success) 30%, transparent); background: color-mix(in oklab, var(--native-success) 12%, transparent); color: var(--native-success); }
</style>
