<template>
  <component :is="bare ? 'div' : 'el-card'" v-bind="bare ? {} : { shadow: 'never', class: 'kb-table-card' }">
    <!-- 活跃筛选条件栏 -->
    <div v-if="activeFilterTags.length > 0 || $slots.actions" class="kb-filter-tags">
      <template v-if="activeFilterTags.length > 0">
        <span class="kb-filter-tags-label">筛选条件：</span>
        <el-tag
          v-for="tag in activeFilterTags"
          :key="tag.prop"
          closable
          size="small"
          class="kb-filter-tag"
          @click.stop="openTagPopover(tag.prop, $event)"
          @close="removeFilter(tag.prop)"
        >
          {{ tag.label }}：{{ tag.display }}
        </el-tag>
        <el-button type="primary" link size="small" @click="clearAllFilters">清除全部</el-button>
      </template>
      <div v-if="$slots.actions" class="kb-filter-tags-actions">
        <slot name="actions" />
      </div>
    </div>

    <!-- 条件栏弹出的修改面板（虚拟定位） -->
    <el-popover
      v-if="tagEditCol"
      :visible="tagPopoverVisible"
      :virtual-ref="tagAnchorEl"
      virtual-triggering
      trigger="manual"
      placement="bottom-start"
      :width="getPopoverWidth(tagEditCol)"
      popper-class="kb-filter-popover"
    >
      <!-- 复用筛选面板 -->
      <div class="kb-filter-panel">
        <div class="kb-filter-panel-title">修改「{{ tagEditCol.label }}」</div>

        <!-- text -->
        <template v-if="tagEditCol.filter.type === 'text'">
          <el-input v-model="tempFilters[tagEditCol.prop]" placeholder="输入关键词..." clearable size="default" @keyup.enter="applyFilter(tagEditCol)" />
          <div v-if="tagEditCol.filter.shortcuts?.length" class="kb-filter-shortcuts">
            <span v-for="sc in tagEditCol.filter.shortcuts" :key="sc.label" class="kb-shortcut-btn" @click="applyShortcut(tagEditCol, sc)">{{ sc.label }}</span>
          </div>
        </template>

        <!-- search-select -->
        <template v-else-if="tagEditCol.filter.type === 'search-select'">
          <el-select v-model="tempFilters[tagEditCol.prop]" filterable allow-create clearable default-first-option placeholder="选择或输入..." size="default" style="width: 100%" @change="applyFilter(tagEditCol)">
            <el-option v-for="opt in getEnumOptions(tagEditCol)" :key="opt" :label="opt || '(空)'" :value="opt" />
          </el-select>
        </template>

        <!-- multi-select -->
        <template v-else-if="tagEditCol.filter.type === 'multi-select'">
          <el-select v-model="tempFilters[tagEditCol.prop]" multiple filterable clearable collapse-tags collapse-tags-tooltip placeholder="选择（可多选）..." size="default" style="width: 100%">
            <el-option v-for="opt in getEnumOptions(tagEditCol)" :key="opt" :label="opt || '(空)'" :value="opt" />
          </el-select>
        </template>

        <!-- number -->
        <template v-else-if="tagEditCol.filter.type === 'number'">
          <div class="kb-filter-range">
            <el-input-number v-model="tempFilters[tagEditCol.prop + '_min']" :controls="false" placeholder="最小" size="default" style="width: 100%" />
            <span class="kb-filter-range-sep">—</span>
            <el-input-number v-model="tempFilters[tagEditCol.prop + '_max']" :controls="false" placeholder="最大" size="default" style="width: 100%" />
          </div>
          <div v-if="tagEditCol.filter.shortcuts?.length" class="kb-filter-shortcuts">
            <span v-for="sc in tagEditCol.filter.shortcuts" :key="sc.label" class="kb-shortcut-btn" @click="applyShortcut(tagEditCol, sc)">{{ sc.label }}</span>
          </div>
        </template>

        <!-- enum -->
        <template v-else-if="tagEditCol.filter.type === 'enum'">
          <el-checkbox-group v-model="tempFilters[tagEditCol.prop]" class="kb-filter-enum-group">
            <el-checkbox v-for="opt in getEnumOptions(tagEditCol)" :key="opt" :label="opt" :value="opt" size="default">{{ opt || '(空)' }}</el-checkbox>
          </el-checkbox-group>
        </template>

        <!-- date -->
        <template v-else-if="tagEditCol.filter.type === 'date'">
          <DateRangePicker v-model="tempFilters[tagEditCol.prop]" />
        </template>

        <!-- cascade-org -->
        <template v-else-if="tagEditCol.filter.type === 'cascade-org'">
          <el-select v-model="cascadeOrg.org1.value" placeholder="一级组织" clearable @change="onCascadeOrg1Change" style="width: 100%">
            <el-option v-for="o in cascadeOrg.org1.options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-select v-model="cascadeOrg.org2.value" placeholder="二级组织" clearable @change="onCascadeOrg2Change" style="width: 100%" :disabled="!cascadeOrg.org1.value">
            <el-option v-for="o in cascadeOrg.org2.options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-select v-model="cascadeOrg.org3.value" placeholder="三级组织" clearable @change="onCascadeOrg3Change" style="width: 100%" :disabled="!cascadeOrg.org2.value">
            <el-option v-for="o in cascadeOrg.org3.options" :key="o" :label="o" :value="o" />
          </el-select>
          <el-select v-model="cascadeOrg.org4.value" placeholder="四级组织" clearable style="width: 100%" :disabled="!cascadeOrg.org3.value">
            <el-option v-for="o in cascadeOrg.org4.options" :key="o" :label="o" :value="o" />
          </el-select>
        </template>

        <div class="kb-filter-actions">
          <el-button size="small" @click="resetFilter(tagEditCol)">重置</el-button>
          <el-button type="primary" size="small" @click="applyFilter(tagEditCol)">应用</el-button>
        </div>
      </div>
    </el-popover>

    <!-- 表格 -->
    <el-table
      ref="tableRef"
      :data="filteredData"
      v-loading="loading"
      style="width: 100%"
      class="kb-table"
      :row-class-name="rowClassName"
      :highlight-current-row="highlightCurrentRow"
      :empty-text="emptyText"
      @row-click="(row, col, e) => $emit('row-click', row, col, e)"
      @selection-change="handleSelectionChange"
    >
      <el-table-column v-if="showSelection" type="selection" width="55" />
      <el-table-column
        v-for="col in columns"
        :key="col.prop"
        :prop="col.prop"
        :label="col.label"
        :width="col.width"
        :min-width="col.minWidth"
        :align="col.align"
        :sortable="col.sortable !== false"
        :sort-method="col.sortMethod"
        :show-overflow-tooltip="col.showOverflowTooltip || false"
        :formatter="col.formatter"
        :fixed="col.fixed"
      >
        <!-- 表头：带筛选图标 -->
        <template #header>
          <div class="kb-col-header">
            <span>{{ col.label }}</span>
            <el-popover
              v-if="col.filter"
              :visible="popoverVisible[col.prop]"
              trigger="manual"
              placement="bottom"
              :width="getPopoverWidth(col)"
              popper-class="kb-filter-popover"
              @show="onPopoverShow(col)"
            >
              <template #reference>
                <el-icon
                  class="kb-filter-icon"
                  :class="{ 'is-active': !!filters[col.prop] }"
                  @click.stop="togglePopover(col.prop)"
                >
                  <Filter />
                </el-icon>
              </template>

              <!-- text 筛选 -->
              <div v-if="col.filter.type === 'text'" class="kb-filter-panel">
                <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
                <el-input
                  v-model="tempFilters[col.prop]"
                  :placeholder="'输入关键词...'"
                  clearable
                  size="default"
                  @keyup.enter="applyFilter(col)"
                />
                <div v-if="col.filter.shortcuts && col.filter.shortcuts.length" class="kb-filter-shortcuts">
                  <span v-for="sc in col.filter.shortcuts" :key="sc.label" class="kb-shortcut-btn" @click="applyShortcut(col, sc)">{{ sc.label }}</span>
                </div>
                <div class="kb-filter-actions">
                  <el-button size="small" @click="resetFilter(col)">重置</el-button>
                  <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
                </div>
              </div>

        <!-- search-select 筛选（下拉选择 + 输入搜索） -->
        <div v-else-if="col.filter.type === 'search-select'" class="kb-filter-panel">
          <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
          <el-select
            v-model="tempFilters[col.prop]"
            filterable
            allow-create
            clearable
            default-first-option
            :placeholder="'选择或输入...'"
            size="default"
            style="width: 100%"
            @change="applyFilter(col)"
          >
            <el-option v-for="opt in getEnumOptions(col)" :key="opt" :label="opt || '(空)'" :value="opt" />
          </el-select>
          <div class="kb-filter-actions">
            <el-button size="small" @click="resetFilter(col)">重置</el-button>
            <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
          </div>
        </div>

        <!-- multi-select 筛选（多选下拉 + 搜索） -->
        <div v-else-if="col.filter.type === 'multi-select'" class="kb-filter-panel">
          <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
          <el-select
            v-model="tempFilters[col.prop]"
            multiple
            filterable
            clearable
            collapse-tags
            collapse-tags-tooltip
            :placeholder="'选择（可多选）...'"
            size="default"
            style="width: 100%"
          >
            <el-option v-for="opt in getEnumOptions(col)" :key="opt" :label="opt || '(空)'" :value="opt" />
          </el-select>
          <div class="kb-filter-actions">
            <el-button size="small" @click="resetFilter(col)">重置</el-button>
            <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
          </div>
        </div>

              <!-- number 范围筛选 -->
              <div v-else-if="col.filter.type === 'number'" class="kb-filter-panel">
                <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
                <div class="kb-filter-range">
                  <el-input-number v-model="tempFilters[col.prop + '_min']" :controls="false" placeholder="最小" size="default" style="width: 100%" />
                  <span class="kb-filter-range-sep">—</span>
                  <el-input-number v-model="tempFilters[col.prop + '_max']" :controls="false" placeholder="最大" size="default" style="width: 100%" />
                </div>
                <div v-if="col.filter.shortcuts && col.filter.shortcuts.length" class="kb-filter-shortcuts">
                  <span v-for="sc in col.filter.shortcuts" :key="sc.label" class="kb-shortcut-btn" @click="applyShortcut(col, sc)">{{ sc.label }}</span>
                </div>
                <div class="kb-filter-actions">
                  <el-button size="small" @click="resetFilter(col)">重置</el-button>
                  <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
                </div>
              </div>

              <!-- enum 筛选 -->
              <div v-else-if="col.filter.type === 'enum'" class="kb-filter-panel">
                <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
                <el-checkbox-group v-model="tempFilters[col.prop]" class="kb-filter-enum-group">
                  <el-checkbox v-for="opt in getEnumOptions(col)" :key="opt" :label="opt" :value="opt" size="default">{{ opt || '(空)' }}</el-checkbox>
                </el-checkbox-group>
                <div class="kb-filter-actions">
                  <el-button size="small" @click="resetFilter(col)">重置</el-button>
                  <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
                </div>
              </div>

              <!-- date 范围筛选 -->
              <div v-else-if="col.filter.type === 'date'" class="kb-filter-panel">
                <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
                <DateRangePicker v-model="tempFilters[col.prop]" />
                <div class="kb-filter-actions">
                  <el-button size="small" @click="resetFilter(col)">重置</el-button>
                  <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
                </div>
              </div>

              <!-- cascade-org 级联组织筛选 -->
              <div v-else-if="col.filter.type === 'cascade-org'" class="kb-filter-panel">
                <div class="kb-filter-panel-title">筛选「{{ col.label }}」</div>
                <el-select v-model="cascadeOrg.org1.value" placeholder="一级组织" clearable @change="onCascadeOrg1Change" style="width: 100%">
                  <el-option v-for="o in cascadeOrg.org1.options" :key="o" :label="o" :value="o" />
                </el-select>
                <el-select v-model="cascadeOrg.org2.value" placeholder="二级组织" clearable @change="onCascadeOrg2Change" style="width: 100%" :disabled="!cascadeOrg.org1.value">
                  <el-option v-for="o in cascadeOrg.org2.options" :key="o" :label="o" :value="o" />
                </el-select>
                <el-select v-model="cascadeOrg.org3.value" placeholder="三级组织" clearable @change="onCascadeOrg3Change" style="width: 100%" :disabled="!cascadeOrg.org2.value">
                  <el-option v-for="o in cascadeOrg.org3.options" :key="o" :label="o" :value="o" />
                </el-select>
                <el-select v-model="cascadeOrg.org4.value" placeholder="四级组织" clearable style="width: 100%" :disabled="!cascadeOrg.org3.value">
                  <el-option v-for="o in cascadeOrg.org4.options" :key="o" :label="o" :value="o" />
                </el-select>
                <div class="kb-filter-actions">
                  <el-button size="small" @click="resetFilter(col)">重置</el-button>
                  <el-button type="primary" size="small" @click="applyFilter(col)">应用</el-button>
                </div>
              </div>
            </el-popover>
          </div>
        </template>

        <!-- 单元格：支持自定义插槽 -->
        <template v-if="col.slotName" #default="scope">
          <slot :name="'cell-' + col.slotName" v-bind="scope" />
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="kb-pagination">
      <el-pagination
        v-model:current-page="localPage"
        v-model:page-size="localPageSize"
        :page-sizes="pageSizes"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="$emit('size-change')"
        @current-change="$emit('page-change')"
      />
    </div>
  </component>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { Filter } from '@element-plus/icons-vue'
import DateRangePicker from './DateRangePicker.vue'
import { getOrgV2 } from '@/api/es'

const props = defineProps({
  columns: { type: Array, required: true },
  data: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  total: { type: Number, default: 0 },
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 250 },
  pageSizes: { type: Array, default: () => [250, 500, 1000] },
  rowClassName: { type: [String, Function], default: '' },
  highlightCurrentRow: { type: Boolean, default: true },
  emptyText: { type: String, default: '暂无数据' },
  showSelection: { type: Boolean, default: false },
  bare: { type: Boolean, default: false },
})

const emit = defineEmits(['row-click', 'page-change', 'size-change', 'update:page', 'update:pageSize', 'filter-change', 'selection-change'])

function handleSelectionChange(val) {
  emit('selection-change', val)
}

const tableRef = ref(null)

const localPage = computed({
  get: () => props.page,
  set: (val) => emit('update:page', val),
})
const localPageSize = computed({
  get: () => props.pageSize,
  set: (val) => emit('update:pageSize', val),
})

// ===== 筛选状态 =====

const filters = reactive({})
const tempFilters = reactive({})
const popoverVisible = reactive({})

// ===== 级联组织筛选 =====
const cascadeOrg = reactive({
  org1: { value: '', options: [] },
  org2: { value: '', options: [] },
  org3: { value: '', options: [] },
  org4: { value: '', options: [] },
})

async function loadCascadeOrgOptions(level, parent) {
  try {
    const result = await getOrgV2({ level, parent: parent || '' })
    const data = result.data || result
    return (data.data || []).map(d => d.org_name)
  } catch {
    return []
  }
}

async function onCascadeOrg1Change(val) {
  cascadeOrg.org2.value = ''
  cascadeOrg.org3.value = ''
  cascadeOrg.org4.value = ''
  cascadeOrg.org2.options = []
  cascadeOrg.org3.options = []
  cascadeOrg.org4.options = []
  if (val) {
    cascadeOrg.org2.options = await loadCascadeOrgOptions('org2', val)
  }
}

async function onCascadeOrg2Change(val) {
  cascadeOrg.org3.value = ''
  cascadeOrg.org4.value = ''
  cascadeOrg.org3.options = []
  cascadeOrg.org4.options = []
  if (val) {
    cascadeOrg.org3.options = await loadCascadeOrgOptions('org3', cascadeOrg.org1.value + '/' + val)
  }
}

async function onCascadeOrg3Change(val) {
  cascadeOrg.org4.value = ''
  cascadeOrg.org4.options = []
  if (val) {
    cascadeOrg.org4.options = await loadCascadeOrgOptions('org4', cascadeOrg.org1.value + '/' + cascadeOrg.org2.value + '/' + val)
  }
}

// ===== 条件栏 tag 弹出编辑 =====
const tagPopoverVisible = ref(false)
const tagAnchorEl = ref(null)
const tagEditProp = ref(null)

const tagEditCol = computed(() => {
  if (!tagEditProp.value) return null
  return props.columns.find(c => c.prop === tagEditProp.value) || null
})

function openTagPopover(prop, event) {
  // 关闭所有表头 popover
  Object.keys(popoverVisible).forEach(k => { popoverVisible[k] = false })
  // 获取被点击的 tag 元素
  const el = event.currentTarget
  const col = props.columns.find(c => c.prop === prop)
  if (!col || !col.filter) return
  // 初始化临时值
  onPopoverShow(col)
  tagAnchorEl.value = el
  tagEditProp.value = prop
  tagPopoverVisible.value = true
}

function closeTagPopover() {
  tagPopoverVisible.value = false
  tagEditProp.value = null
}

// 点击筛选图标：打开当前、关闭其他
function togglePopover(prop) {
  const opening = !popoverVisible[prop]
  Object.keys(popoverVisible).forEach(k => { popoverVisible[k] = false })
  closeTagPopover()
  if (opening) popoverVisible[prop] = true
}

// 点击页面其他区域时关闭所有 popover
function onDocumentClick(e) {
  const target = e.target
  if (target.closest('.kb-filter-popover') || target.closest('.kb-filter-icon') || target.closest('.kb-filter-tag')) return
  if (target.closest('.el-picker-panel') || target.closest('.el-popper')) return
  Object.keys(popoverVisible).forEach(k => { popoverVisible[k] = false })
  closeTagPopover()
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick, true)
})

function getPopoverWidth(col) {
  if (col.filter.type === 'cascade-org') return 400
  if (col.filter.type === 'date') return 580
  if (col.filter.type === 'enum') return 200
  if (col.filter.type === 'multi-select') return 260
  return 240
}

// 获取 enum / search-select 选项
function getEnumOptions(col) {
  if (col.filter.options && col.filter.options.length > 0) return col.filter.options
  const vals = new Set()
  props.data.forEach(row => {
    const v = row[col.prop]
    if (v != null && String(v).trim() !== '') vals.add(String(v))
  })
  return Array.from(vals).sort()
}

// 日期快捷条件（内置 + 自定义）
function getDateShortcuts(col) {
  const builtIn = [
    { label: '今天', value: () => makeDateRange(0) },
    { label: '最近一周', value: () => makeDateRange(7) },
    { label: '最近一个月', value: () => makeDateRange(30) },
    { label: '最近三个月', value: () => makeDateRange(90) },
  ]
  return [...builtIn, ...(col.filter.shortcuts || [])]
}

function makeDateRange(days) {
  const end = new Date()
  const start = new Date()
  if (days > 0) start.setDate(start.getDate() - days)
  else start.setHours(0, 0, 0, 0)
  return [fmtDate(start), fmtDate(end)]
}

function fmtDate(d) {
  const pad = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// popover 显示时，拷贝当前值到临时变量
function onPopoverShow(col) {
  const f = col.filter
  if (f.type === 'text' || f.type === 'search-select') {
    tempFilters[col.prop] = filters[col.prop] || ''
  } else if (f.type === 'multi-select') {
    tempFilters[col.prop] = filters[col.prop] ? [...filters[col.prop]] : []
  } else if (f.type === 'number') {
    tempFilters[col.prop + '_min'] = filters[col.prop + '_min'] ?? null
    tempFilters[col.prop + '_max'] = filters[col.prop + '_max'] ?? null
  } else if (f.type === 'enum') {
    tempFilters[col.prop] = filters[col.prop] ? [...filters[col.prop]] : []
  } else if (f.type === 'date') {
    tempFilters[col.prop] = filters[col.prop] ? [...filters[col.prop]] : null
  } else if (f.type === 'cascade-org') {
    const stored = filters[col.prop] || {}
    cascadeOrg.org1.value = stored.org1 || ''
    cascadeOrg.org2.value = stored.org2 || ''
    cascadeOrg.org3.value = stored.org3 || ''
    cascadeOrg.org4.value = stored.org4 || ''
    // 加载各级 options
    loadCascadeOrgOptions('org1', '').then(opts => { cascadeOrg.org1.options = opts })
    if (stored.org1) loadCascadeOrgOptions('org2', stored.org1).then(opts => { cascadeOrg.org2.options = opts })
    if (stored.org2) loadCascadeOrgOptions('org3', stored.org1 + '/' + stored.org2).then(opts => { cascadeOrg.org3.options = opts })
    if (stored.org3) loadCascadeOrgOptions('org4', stored.org1 + '/' + stored.org2 + '/' + stored.org3).then(opts => { cascadeOrg.org4.options = opts })
  }
}

// 快捷条件点击 —— 填值并直接应用
function applyShortcut(col, sc) {
  const f = col.filter
  const val = typeof sc.value === 'function' ? sc.value() : sc.value

  if (f.type === 'date') {
    filters[col.prop] = val
    tempFilters[col.prop] = val
  } else if (f.type === 'number') {
    const min = val.min ?? null
    const max = val.max ?? null
    filters[col.prop] = { min, max }
    if (min != null) filters[col.prop + '_min'] = min; else delete filters[col.prop + '_min']
    if (max != null) filters[col.prop + '_max'] = max; else delete filters[col.prop + '_max']
    tempFilters[col.prop + '_min'] = min
    tempFilters[col.prop + '_max'] = max
  } else if (f.type === 'text' || f.type === 'search-select') {
    filters[col.prop] = val
    tempFilters[col.prop] = val
  } else if (f.type === 'multi-select') {
    const arr = Array.isArray(val) ? val : [val]
    if (arr.length > 0) { filters[col.prop] = arr; tempFilters[col.prop] = [...arr] }
  }

  popoverVisible[col.prop] = false
  closeTagPopover()
  emit('filter-change', { ...filters })
}

// 确定
function applyFilter(col) {
  const f = col.filter
  if (f.type === 'text' || f.type === 'search-select') {
    const val = (tempFilters[col.prop] || '').trim()
    if (val) filters[col.prop] = val; else delete filters[col.prop]
  } else if (f.type === 'number') {
    const min = tempFilters[col.prop + '_min']
    const max = tempFilters[col.prop + '_max']
    if (min != null || max != null) {
      filters[col.prop] = { min, max }
      if (min != null) filters[col.prop + '_min'] = min; else delete filters[col.prop + '_min']
      if (max != null) filters[col.prop + '_max'] = max; else delete filters[col.prop + '_max']
    } else {
      delete filters[col.prop]; delete filters[col.prop + '_min']; delete filters[col.prop + '_max']
    }
  } else if (f.type === 'enum' || f.type === 'multi-select') {
    const val = tempFilters[col.prop] || []
    if (val.length > 0) filters[col.prop] = [...val]; else delete filters[col.prop]
  } else if (f.type === 'date') {
    const val = tempFilters[col.prop]
    if (val && val.length === 2 && (val[0] || val[1])) {
      filters[col.prop] = [val[0] || '2020-01-01', val[1] || fmtDate(new Date())]
    } else {
      delete filters[col.prop]
    }
  } else if (f.type === 'cascade-org') {
    const val = {
      org1: cascadeOrg.org1.value || '',
      org2: cascadeOrg.org2.value || '',
      org3: cascadeOrg.org3.value || '',
      org4: cascadeOrg.org4.value || '',
    }
    if (val.org1 || val.org2 || val.org3 || val.org4) {
      filters[col.prop] = val
    } else {
      delete filters[col.prop]
    }
  }
  popoverVisible[col.prop] = false
  closeTagPopover()
  emit('filter-change', { ...filters })
}

// 重置单个
function resetFilter(col) {
  const f = col.filter
  if (f.type === 'text' || f.type === 'search-select') {
    tempFilters[col.prop] = ''; delete filters[col.prop]
  } else if (f.type === 'number') {
    tempFilters[col.prop + '_min'] = null; tempFilters[col.prop + '_max'] = null
    delete filters[col.prop]; delete filters[col.prop + '_min']; delete filters[col.prop + '_max']
  } else if (f.type === 'enum' || f.type === 'multi-select') {
    tempFilters[col.prop] = []; delete filters[col.prop]
  } else if (f.type === 'date') {
    tempFilters[col.prop] = null
    delete filters[col.prop]
  } else if (f.type === 'cascade-org') {
    cascadeOrg.org1.value = ''
    cascadeOrg.org2.value = ''
    cascadeOrg.org3.value = ''
    cascadeOrg.org4.value = ''
    cascadeOrg.org1.options = []
    cascadeOrg.org2.options = []
    cascadeOrg.org3.options = []
    cascadeOrg.org4.options = []
    delete filters[col.prop]
  }
  popoverVisible[col.prop] = false
  closeTagPopover()
  emit('filter-change', { ...filters })
}

// 移除（条件栏关闭按钮）
function removeFilter(prop) {
  const col = props.columns.find(c => c.prop === prop)
  if (!col) return
  if (col.filter.type === 'number') {
    delete filters[prop]; delete filters[prop + '_min']; delete filters[prop + '_max']
  } else {
    delete filters[prop]
  }
  emit('filter-change', { ...filters })
}

// 清除全部
function clearAllFilters() {
  Object.keys(filters).forEach(k => delete filters[k])
  emit('filter-change', {})
}

// ===== 外部 API：设置初始筛选值 =====

/** 父组件调用，设置某列的筛选值（不触发 emit）；value 为空/null 时清除该筛选 */
function setFilter(prop, value) {
  const col = props.columns.find(c => c.prop === prop)
  if (!col || !col.filter) return
  const f = col.filter
  if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) {
    delete filters[prop]
    if (f.type === 'number') { delete filters[prop + '_min']; delete filters[prop + '_max'] }
    return
  }
  if (f.type === 'date' && Array.isArray(value)) {
    filters[prop] = value
  } else if (f.type === 'number' && typeof value === 'object') {
    filters[prop] = value
    if (value.min != null) filters[prop + '_min'] = value.min
    if (value.max != null) filters[prop + '_max'] = value.max
  } else if ((f.type === 'enum' || f.type === 'multi-select') && Array.isArray(value)) {
    filters[prop] = value
  } else if (f.type === 'cascade-org' && typeof value === 'object') {
    filters[prop] = value
  } else if (typeof value === 'string') {
    filters[prop] = value
  }
}

/** 获取某列的当前筛选值 */
function getFilter(prop) {
  return filters[prop] ?? null
}

// ===== 筛选数据（仅处理非 serverSide 的列） =====

/** 获取行中用于筛选的值：优先使用 filter.valueGetter，否则取 row[prop] */
function getFilterValue(row, col) {
  if (col.filter.valueGetter) return col.filter.valueGetter(row)
  return row[col.prop]
}

const filteredData = computed(() => {
  let data = props.data
  if (!data) return []

  for (const col of props.columns) {
    if (!col.filter || !filters[col.prop]) continue
    if (col.filter.serverSide) continue

    const f = col.filter

    if ((f.type === 'text' || f.type === 'search-select') && filters[col.prop]) {
      const kw = filters[col.prop].toLowerCase()
      const isExact = f.type === 'search-select' && getEnumOptions(col).some(opt => opt.toLowerCase() === kw)
      data = data.filter(row => {
        const val = getFilterValue(row, col)
        if (val == null) return false
        const str = String(val).toLowerCase()
        return isExact ? str === kw : str.includes(kw)
      })
    } else if (f.type === 'multi-select' && filters[col.prop]?.length > 0) {
      const allowed = new Set(filters[col.prop].map(v => String(v)))
      data = data.filter(row => {
        const val = getFilterValue(row, col)
        return allowed.has(val != null ? String(val) : '')
      })
    } else if (f.type === 'number' && filters[col.prop]) {
      const { min, max } = filters[col.prop]
      data = data.filter(row => {
        const val = getFilterValue(row, col)
        if (val == null) return false
        const num = Number(val)
        if (isNaN(num)) return false
        if (min != null && num < min) return false
        if (max != null && num > max) return false
        return true
      })
    } else if (f.type === 'enum' && filters[col.prop]?.length > 0) {
      const allowed = new Set(filters[col.prop])
      data = data.filter(row => {
        const val = getFilterValue(row, col)
        return allowed.has(val != null ? String(val) : '')
      })
    } else if (f.type === 'date' && filters[col.prop]?.length === 2) {
      const [startStr, endStr] = filters[col.prop]
      const startD = new Date(startStr + ' 00:00:00')
      const endD = new Date(endStr + ' 23:59:59')
      data = data.filter(row => {
        const val = getFilterValue(row, col)
        if (!val) return false
        const d = new Date(val)
        return d >= startD && d <= endD
      })
    } else if (f.type === 'cascade-org' && filters[col.prop]) {
      const orgFilter = filters[col.prop]
      data = data.filter(row => {
        if (orgFilter.org1 && row.org1 !== orgFilter.org1) return false
        if (orgFilter.org2 && row.org2 !== orgFilter.org2) return false
        if (orgFilter.org3 && row.org3 !== orgFilter.org3) return false
        if (orgFilter.org4 && row.org4 !== orgFilter.org4) return false
        return true
      })
    }
  }

  return data
})

// ===== 活跃筛选标签 =====

const activeFilterTags = computed(() => {
  const tags = []
  for (const col of props.columns) {
    if (!col.filter || !filters[col.prop]) continue
    const f = col.filter

    if ((f.type === 'text' || f.type === 'search-select') && filters[col.prop]) {
      const isExact = f.type === 'search-select' && getEnumOptions(col).some(opt => opt === filters[col.prop])
      tags.push({ prop: col.prop, label: col.label, display: isExact ? filters[col.prop] : `包含 "${filters[col.prop]}"` })
    } else if (f.type === 'multi-select' && filters[col.prop]?.length > 0) {
      tags.push({ prop: col.prop, label: col.label, display: filters[col.prop].join(', ') })
    } else if (f.type === 'number' && filters[col.prop]) {
      const { min, max } = filters[col.prop]
      let d = ''
      if (min != null && max != null) d = `${min} ~ ${max}`
      else if (min != null) d = `>= ${min}`
      else if (max != null) d = `<= ${max}`
      tags.push({ prop: col.prop, label: col.label, display: d })
    } else if (f.type === 'enum' && filters[col.prop]?.length > 0) {
      tags.push({ prop: col.prop, label: col.label, display: filters[col.prop].join(', ') })
    } else if (f.type === 'date' && filters[col.prop]?.length === 2) {
      const [s, e] = filters[col.prop]
      tags.push({ prop: col.prop, label: col.label, display: `${s} ~ ${e}` })
    } else if (f.type === 'cascade-org' && filters[col.prop]) {
      const orgVal = filters[col.prop]
      const parts = [orgVal.org1, orgVal.org2, orgVal.org3, orgVal.org4].filter(Boolean)
      if (parts.length > 0) {
        tags.push({ prop: col.prop, label: col.label, display: parts.join('/') })
      }
    }
  }
  return tags
})

defineExpose({ filteredData, clearAllFilters, setFilter, getFilter, tableRef })
</script>

<style scoped>
.kb-filter-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 0;
  margin-bottom: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5);
}
.kb-filter-tags-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.kb-filter-tags-label {
  font-size: 13px;
  color: #909399;
  white-space: nowrap;
}
.kb-filter-tag {
  max-width: 300px;
  cursor: pointer;
}

.kb-col-header {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  white-space: nowrap;
}

/* 隐藏排序箭头，保留点击表头文字排序能力 */
:deep(.caret-wrapper) {
  display: none;
}
.kb-filter-icon {
  cursor: pointer;
  font-size: 14px;
  color: #c0c4cc;
  transition: color 0.2s;
  vertical-align: middle;
}
.kb-filter-icon:hover {
  color: #409eff;
}
.kb-filter-icon.is-active {
  color: #409eff;
}

.kb-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.kb-filter-range {
  display: flex;
  align-items: center;
  gap: 6px;
}
.kb-filter-range-sep {
  color: #909399;
  flex-shrink: 0;
}
.kb-filter-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.kb-filter-enum-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 200px;
  overflow-y: auto;
}

/* 快捷条件 */
.kb-filter-shortcuts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.kb-shortcut-btn {
  display: inline-block;
  padding: 2px 8px;
  font-size: 12px;
  color: #409eff;
  background: #ecf5ff;
  border: 1px solid #d9ecff;
  border-radius: 3px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s;
  line-height: 1.6;
}
.kb-shortcut-btn:hover {
  background: #409eff;
  color: #fff;
  border-color: #409eff;
}


</style>
