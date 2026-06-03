<template>
  <button
    type="button"
    class="kn-sort-th"
    :class="{ 'is-numeric': numeric, 'is-active': active }"
    :aria-sort="active ? (desc ? 'descending' : 'ascending') : 'none'"
    @click="$emit('sort')"
  >
    <span class="kn-sort-label">{{ label }}</span>
    <span class="kn-sort-carets" aria-hidden="true">
      <svg viewBox="0 0 16 16" width="11" height="11" class="kn-sort-caret">
        <path
          class="kn-caret-up"
          :class="{ 'is-on': active && !desc }"
          d="M8 3.5 12 8H4z"
          fill="currentColor"
        />
        <path
          class="kn-caret-down"
          :class="{ 'is-on': active && desc }"
          d="M8 12.5 4 8h8z"
          fill="currentColor"
        />
      </svg>
    </span>
  </button>
</template>

<script setup>
defineProps({
  field: { type: String, required: true },
  label: { type: String, required: true },
  active: { type: Boolean, default: false },
  desc: { type: Boolean, default: false },
  numeric: { type: Boolean, default: false },
})

defineEmits(['sort'])
</script>

<style scoped>
.kn-sort-th {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  width: 100%;
  padding: 0;
  margin: 0;
  border: 0;
  background: none;
  font: inherit;
  font-weight: 600;
  color: var(--native-muted);
  white-space: nowrap;
  cursor: pointer;
  user-select: none;
  transition: color 0.15s;
}
.kn-sort-th:hover {
  color: var(--native-foreground);
}
.kn-sort-th.is-active {
  color: var(--native-primary);
}
.kn-sort-th.is-numeric {
  justify-content: flex-end;
}
.kn-sort-label {
  overflow: hidden;
  text-overflow: ellipsis;
}
.kn-sort-carets {
  display: inline-flex;
  flex-shrink: 0;
  line-height: 0;
}
.kn-sort-caret {
  display: block;
}
.kn-caret-up,
.kn-caret-down {
  opacity: 0.28;
  transition: opacity 0.15s;
}
.kn-caret-up.is-on,
.kn-caret-down.is-on {
  opacity: 1;
  color: var(--native-primary);
}
</style>
