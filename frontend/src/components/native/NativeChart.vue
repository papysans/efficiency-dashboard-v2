<template>
  <section class="kn-chart-card">
    <div v-show="hasOption" ref="chartRef" class="kn-chart" :style="{ height }"></div>
    <div v-if="!hasOption" class="kn-chart-empty" :style="{ height }">{{ empty }}</div>
  </section>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useChart } from '@/composables/useChart'

const props = defineProps({
  option: { type: Object, default: null },
  height: { type: String, default: '280px' },
  empty: { type: String, default: '暂无图表数据' },
})

const chartRef = ref(null)
const { setOption } = useChart(chartRef)
const hasOption = computed(() => !!props.option)

watch(
  () => props.option,
  async opt => {
    if (!opt) return
    await nextTick()
    setOption(opt)
  },
  { immediate: true },
)
</script>

<style scoped>
.kn-chart-card {
  border-radius: var(--native-radius-lg);
  border: 1px solid color-mix(in oklab, var(--native-border) 24%, transparent);
  background: var(--native-panel);
  padding: 0.75rem;
  box-shadow: var(--native-shadow-sm);
}
.kn-chart { width: 100%; }
.kn-chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.875rem;
  color: var(--native-muted);
}
</style>
