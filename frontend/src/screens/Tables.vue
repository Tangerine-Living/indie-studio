<script setup lang="ts">
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Columns3,
  Copy,
  Download,
  Pencil,
  Plus,
  Search,
  Table2,
  Trash2,
  X,
} from 'lucide-vue-next'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { COLUMN_TYPES } from '../domain/types'
import type {
  CellValue,
  ColumnType,
  DataTable,
  TableColumn,
  TableRow,
} from '../domain/types'
import {
  cellText,
  normalizeTableKey,
  rowReferenceLabel,
  rowTitle,
  tableIssues,
  zeroCell,
} from '../domain/tables'
import { useAppStore } from '../stores/app'
import { confirmDialog } from '../lib/dialogs'
import { newID, nowISO, scrollScreenTop, uniq } from '../lib/utils'
import EmptyState from '../components/EmptyState.vue'
import AppModal from '../components/AppModal.vue'

const store = useAppStore()

const TYPE_LABELS: Record<ColumnType, string> = {
  text: '文本',
  int: '整数',
  number: '小数',
  bool: '开关',
  enum: '枚举',
  ref: '引用',
}

const query = ref('')
const rowQuery = ref('')
const selectedRowIds = ref<string[]>([])
const rowEditorId = ref<string | null>(null)
const selectedId = ref<string | null>(null)
const draft = ref<DataTable | null>(null)
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const showColumns = ref(false)
const notice = ref('')
const nameRef = ref<HTMLInputElement | null>(null)

let loadingDraft = false
let saveTimer: ReturnType<typeof setTimeout> | null = null
let noticeTimer: ReturnType<typeof setTimeout> | null = null
let inFlightSave: Promise<void> | null = null
let saveSeq = 0

const filteredTables = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return store.tables
    .filter((table) => {
      if (!needle) return true
      return `${table.name} ${table.key} ${table.note}`.toLowerCase().includes(needle)
    })
    .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
})

const issueCounts = computed(() => {
  const map = new Map<string, number>()
  for (const table of store.tables) map.set(table.id, tableIssues(table, store.tables).length)
  return map
})

const allTables = computed(() => {
  const current = draft.value
  if (!current) return store.tables
  return store.tables.map((table) => (table.id === current.id ? current : table))
})

const issues = computed(() =>
  draft.value ? tableIssues(draft.value, allTables.value) : [],
)

const issueRows = computed(() => {
  const map = new Map<string, string[]>()
  for (const issue of issues.value) {
    if (!issue.rowId) continue
    const list = map.get(issue.rowId) ?? []
    list.push(issue.message)
    map.set(issue.rowId, list)
  }
  return map
})

const keyConflict = computed(() => {
  const current = draft.value
  if (!current) return null
  return store.tables.find((table) => table.id !== current.id && table.key === current.key) ?? null
})

const referencedBy = computed(() => {
  const current = draft.value
  if (!current) return []
  return store.tables.filter(
    (table) =>
      table.id !== current.id &&
      table.columns.some((column) => column.type === 'ref' && column.refTableId === current.id),
  )
})

const uiColumns = computed(() => draft.value?.columns ?? [])

const filteredRows = computed(() => {
  const table = draft.value
  if (!table) return []
  const needle = rowQuery.value.trim().toLowerCase()
  if (!needle) return table.rows
  return table.rows.filter((row) =>
    table.columns.some((column) => cellText(row.values[column.id]).toLowerCase().includes(needle)),
  )
})

const exportBlocked = computed(
  () =>
    !draft.value ||
    issues.value.length > 0 ||
    keyConflict.value !== null ||
    store.busy ||
    saveState.value === 'saving' ||
    saveState.value === 'error',
)

function snapshot(table: DataTable): DataTable {
  return JSON.parse(JSON.stringify(table)) as DataTable
}

function cancelTimer() {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
}

async function persist(): Promise<void> {
  const current = draft.value
  if (!current) return
  const seq = (saveSeq += 1)
  saveState.value = 'saving'
  const run = (async () => {
    const saved = await store.saveTable(snapshot(current))
    if (seq !== saveSeq) return
    saveState.value = saved ? 'saved' : 'error'
  })()
  inFlightSave = run
  try {
    await run
  } finally {
    if (inFlightSave === run) inFlightSave = null
  }
}

/**
 * Save now: a debounced write is cancelled and run, and an already-running
 * write is awaited, so export and table switches never read half-saved data.
 */
async function flushSave(): Promise<void> {
  if (saveTimer) {
    cancelTimer()
    await persist()
  }
  if (inFlightSave) await inFlightSave
}

function scheduleSave() {
  if (loadingDraft || !draft.value) return
  cancelTimer()
  saveState.value = 'idle'
  saveTimer = setTimeout(() => {
    saveTimer = null
    void persist()
  }, 700)
}

function loadDraft(table: DataTable | null) {
  saveSeq += 1
  loadingDraft = true
  selectedId.value = table?.id ?? null
  selectedRowIds.value = []
  rowEditorId.value = null
  draft.value = table ? snapshot(table) : null
  rowQuery.value = ''
  showColumns.value = false
  notice.value = ''
  saveState.value = 'idle'
  void nextTick(() => {
    loadingDraft = false
  })
}

/**
 * A failed save (blank name, duplicate key, disk error) leaves edits only in
 * the draft; switching away would drop them without a word.
 */
async function discardIfSaveFailed(): Promise<boolean> {
  if (saveState.value !== 'error') return true
  return confirmDialog({
    title: '这张表还有修改没能保存',
    message: '表名或表键不合法（名称为空、表键与其他表重复），或写入磁盘失败。',
    detail: '继续会丢弃这些修改。',
    confirmText: '丢弃修改',
    tone: 'danger',
  })
}

async function selectTable(table: DataTable | null) {
  if (table?.id === selectedId.value) return
  if (!(await discardIfSaveFailed())) return
  await flushSave()
  loadDraft(table)
  // After the new records render, so scroll anchoring cannot pull it back.
  await nextTick()
  scrollScreenTop()
}

async function createTable() {
  if (!(await discardIfSaveFailed())) return
  await flushSave()
  let suffix = 1
  while (store.tables.some((table) => table.key === `table_${suffix}`)) suffix += 1
  const table: DataTable = {
    schemaVersion: 1,
    id: newID(),
    name: '未命名数值表',
    key: `table_${suffix}`,
    note: '',
    columns: [
      { id: newID(), name: '名称', key: 'name', type: 'text', required: true, options: [] },
    ],
    rows: [],
    createdAt: nowISO(),
    updatedAt: nowISO(),
  }
  const saved = await store.saveTable(table)
  if (!saved) return
  loadDraft(saved)
  await nextTick()
  scrollScreenTop()
  nameRef.value?.focus()
  nameRef.value?.select()
}

async function removeTable() {
  const current = draft.value
  if (!current) return
  if (referencedBy.value.length > 0) {
    await confirmDialog({
      title: `不能删除「${current.name}」`,
      message: '这些表把它当作引用目标，先把对应列改成别的表，再删除。',
      detail: referencedBy.value.map((table) => table.name).join('、'),
      confirmText: '知道了',
      cancelText: '关闭',
    })
    return
  }
  const ok = await confirmDialog({
    title: `删除「${current.name}」`,
    message: '整张表和所有行会进入项目回收站。',
    detail: `${current.rows.length} 行 · 表键 ${current.key}`,
    confirmText: '删除表',
    tone: 'danger',
  })
  if (!ok) return
  cancelTimer()
  saveSeq += 1
  // Let a running write land before deleting, or it could recreate the file.
  if (inFlightSave) await inFlightSave
  const deleted = await store.deleteTable(current.id)
  if (!deleted) return
  loadDraft(store.tables[0] ?? null)
}

function setTableKey(raw: string) {
  if (!draft.value) return
  draft.value.key = normalizeTableKey(raw)
}

function setColumnKey(column: TableColumn, raw: string) {
  const table = draft.value
  if (!table) return
  const base = normalizeTableKey(raw)
  let key = base
  for (let suffix = 2; table.columns.some((item) => item.id !== column.id && item.key === key); suffix += 1) {
    key = `${base}_${suffix}`
  }
  column.key = key
}

function addColumn(type: ColumnType = 'text') {
  const table = draft.value
  if (!table) return
  let suffix = table.columns.length + 1
  while (table.columns.some((column) => column.key === `col_${suffix}`)) suffix += 1
  const column: TableColumn = {
    id: newID(),
    name: `列 ${suffix}`,
    key: `col_${suffix}`,
    type,
    required: false,
    options: type === 'enum' ? ['选项一', '选项二'] : [],
  }
  if (type === 'ref') {
    // Never point silently at the table itself; the user picks a target.
    const target = store.tables.find((item) => item.id !== table.id)
    if (target) column.refTableId = target.id
  }
  for (const row of table.rows) row.values[column.id] = zeroCell(type)
  table.columns.push(column)
  showColumns.value = true
}

async function removeColumn(column: TableColumn) {
  const table = draft.value
  if (!table) return
  const filled = table.rows.filter((row) => !cellEmptyValue(row.values[column.id])).length
  const ok = await confirmDialog({
    title: `删除列「${column.name}」`,
    message: filled > 0 ? '这一列和已有的值会一并删除。' : '这一列还没有填过值。',
    detail: filled > 0 ? `${filled} 行有值` : '',
    confirmText: '删除列',
    tone: 'danger',
  })
  if (!ok) return
  table.columns = table.columns.filter((item) => item.id !== column.id)
  for (const row of table.rows) delete row.values[column.id]
}

function coerceValue(value: CellValue | undefined, type: ColumnType): CellValue {
  if (value === undefined) return zeroCell(type)
  switch (type) {
    case 'int':
    case 'number': {
      const number = typeof value === 'number' ? value : Number(value)
      if (!Number.isFinite(number)) return 0
      return type === 'int' ? Math.trunc(number) : number
    }
    case 'bool':
      return typeof value === 'boolean' ? value : value === 'true'
    default:
      return typeof value === 'string' ? value : cellText(value)
  }
}

function changeColumnType(column: TableColumn) {
  const table = draft.value
  if (!table) return
  if (column.type === 'enum' && column.options.length === 0) {
    column.options = ['选项一', '选项二']
  }
  if (column.type === 'ref') {
    const target = store.tables.find((item) => item.id !== table.id)
    if (!column.refTableId && target) column.refTableId = target.id
  }
  if (column.type !== 'int' && column.type !== 'number') {
    delete column.min
    delete column.max
  }
  for (const row of table.rows) {
    if (row.values[column.id] === undefined) continue
    row.values[column.id] = coerceValue(row.values[column.id], column.type)
  }
}

function setOptions(column: TableColumn, raw: string) {
  column.options = uniq(
    raw
      .split(/[,，、\n]/)
      .map((part) => part.trim())
      .filter(Boolean),
  )
}

function setLimit(column: TableColumn, key: 'min' | 'max', raw: string) {
  const text = raw.trim()
  if (!text) {
    delete column[key]
    return
  }
  const number = Number(text)
  if (Number.isFinite(number)) column[key] = number
}

const tableScroll = ref<HTMLElement | null>(null)
const dragRowId = ref<string | null>(null)
const dragOverRowId = ref<string | null>(null)

/** Focuses the editable control of one cell, scrolling it into view. */
function focusCell(rowId: string, columnId: string) {
  void nextTick(() => {
    const cell = tableScroll.value?.querySelector<HTMLElement>(`[data-cell="${rowId}:${columnId}"]`)
    const control = cell?.querySelector<HTMLElement>('input:not([type="hidden"]), select, textarea')
    control?.focus()
    if (typeof control?.scrollIntoView === 'function') {
      control.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    }
  })
}

function addRow(focus = true): TableRow | null {
  const table = draft.value
  if (!table) return null
  const row: TableRow = { id: newID(), values: {} }
  for (const column of table.columns) row.values[column.id] = zeroCell(column.type)
  table.rows.push(row)
  rowQuery.value = ''
  const first = table.columns[0]
  if (focus && first) focusCell(row.id, first.id)
  return row
}

/** Enter moves down the same column, adding a row when at the bottom. */
function moveCell(row: TableRow, column: TableColumn) {
  const table = draft.value
  if (!table) return
  const visible = filteredRows.value
  const index = visible.findIndex((item) => item.id === row.id)
  if (index >= 0 && index < visible.length - 1) {
    focusCell(visible[index + 1].id, column.id)
    return
  }
  const added = addRow(false)
  if (added) focusCell(added.id, column.id)
}

function startRowDrag(event: DragEvent, row: TableRow) {
  dragRowId.value = row.id
  event.dataTransfer?.setData('text/plain', row.id)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

function dropRow(target: TableRow) {
  const table = draft.value
  const sourceId = dragRowId.value
  dragRowId.value = null
  dragOverRowId.value = null
  if (!table || !sourceId || sourceId === target.id) return
  const from = table.rows.findIndex((item) => item.id === sourceId)
  const to = table.rows.findIndex((item) => item.id === target.id)
  if (from < 0 || to < 0) return
  const [moved] = table.rows.splice(from, 1)
  table.rows.splice(from < to ? to - 1 : to, 0, moved)
  const first = table.columns[0]
  if (first) focusCell(moved.id, first.id)
}

function duplicateRow(row: TableRow) {
  const table = draft.value
  if (!table) return
  const index = table.rows.findIndex((item) => item.id === row.id)
  if (index < 0) return
  const copy: TableRow = {
    id: newID(),
    values: JSON.parse(JSON.stringify(row.values)) as Record<string, CellValue>,
  }
  table.rows.splice(index + 1, 0, copy)
  const first = table.columns[0]
  if (first) focusCell(copy.id, first.id)
}

/** True once a cell holds something beyond the column's zero value. */
function hasStoredValue(value: CellValue | undefined): boolean {
  if (value === undefined || value === null) return false
  if (typeof value === 'string') return value.trim() !== ''
  if (typeof value === 'number') return value !== 0
  return value
}

async function removeRow(row: TableRow) {
  const table = draft.value
  if (!table) return
  const index = table.rows.findIndex((item) => item.id === row.id)
  if (index < 0) return
  const filled = table.columns.some((column) => hasStoredValue(row.values[column.id]))
  if (filled) {
    let refs = 0
    for (const other of store.tables) {
      if (other.id === table.id) continue
      for (const column of other.columns) {
        if (column.type !== 'ref' || column.refTableId !== table.id) continue
        refs += other.rows.filter((item) => item.values[column.id] === row.id).length
      }
    }
    const ok = await confirmDialog({
      title: `删除第 ${index + 1} 行`,
      message: `「${rowTitle(table, row, index)}」会从表中删除。`,
      detail: refs > 0
        ? `另有 ${refs} 处引用这一行，删除后会在对应的表里标为「引用的行已被删除」。删除不能撤销。`
        : '删除后不能撤销；需要保留时先用「复制这一行」。',
      confirmText: '删除行',
      tone: 'danger',
    })
    if (!ok) return
  }
  table.rows = table.rows.filter((item) => item.id !== row.id)
  selectedRowIds.value = selectedRowIds.value.filter((id) => id !== row.id)
  if (rowEditorId.value === row.id) rowEditorId.value = null
}

const editingRow = computed(
  () => draft.value?.rows.find((row) => row.id === rowEditorId.value) ?? null,
)
const editingIndex = computed(() => {
  const table = draft.value
  const row = editingRow.value
  return table && row ? table.rows.indexOf(row) : -1
})

function openRowEditor(row: TableRow) {
  rowEditorId.value = row.id
}

function closeRowEditor() {
  rowEditorId.value = null
}

function stepRow(delta: number) {
  const table = draft.value
  const index = editingIndex.value
  if (!table || index < 0) return
  const next = table.rows[index + delta]
  if (next) rowEditorId.value = next.id
}

async function deleteEditingRow() {
  const row = editingRow.value
  if (!row) return
  await removeRow(row)
}

function isRowSelected(row: TableRow): boolean {
  return selectedRowIds.value.includes(row.id)
}

function toggleRow(row: TableRow, checked: boolean) {
  if (checked) {
    if (!selectedRowIds.value.includes(row.id)) selectedRowIds.value.push(row.id)
  } else {
    selectedRowIds.value = selectedRowIds.value.filter((id) => id !== row.id)
  }
}

const allVisibleSelected = computed(
  () => filteredRows.value.length > 0 && filteredRows.value.every(isRowSelected),
)

function toggleAllRows(checked: boolean) {
  const visible = filteredRows.value.map((row) => row.id)
  if (checked) {
    selectedRowIds.value = [...new Set([...selectedRowIds.value, ...visible])]
  } else {
    const remove = new Set(visible)
    selectedRowIds.value = selectedRowIds.value.filter((id) => !remove.has(id))
  }
}

function clearSelection() {
  selectedRowIds.value = []
}

/** Deletes every selected row, warning once about references and non-empty rows. */
async function removeSelectedRows() {
  const table = draft.value
  if (!table) return
  const ids = new Set(selectedRowIds.value)
  const rows = table.rows.filter((row) => ids.has(row.id))
  if (rows.length === 0) return
  const filled = rows.filter((row) =>
    table.columns.some((column) => hasStoredValue(row.values[column.id])),
  )
  let refs = 0
  for (const other of store.tables) {
    if (other.id === table.id) continue
    for (const column of other.columns) {
      if (column.type !== 'ref' || column.refTableId !== table.id) continue
      refs += other.rows.filter((item) => {
        const value = item.values[column.id]
        return typeof value === 'string' && ids.has(value)
      }).length
    }
  }
  if (filled.length > 0) {
    const ok = await confirmDialog({
      title: `删除选中的 ${rows.length} 行`,
      message: `这些行会从「${table.name}」删除。`,
      detail: refs > 0
        ? `另有 ${refs} 处引用这些行，删除后会在对应的表里标为「引用的行已被删除」。删除不能撤销。`
        : '删除后不能撤销；需要保留时先用「复制这一行」。',
      confirmText: `删除 ${rows.length} 行`,
      tone: 'danger',
    })
    if (!ok) return
  }
  table.rows = table.rows.filter((row) => !ids.has(row.id))
  selectedRowIds.value = []
  if (rowEditorId.value && !table.rows.some((row) => row.id === rowEditorId.value)) {
    rowEditorId.value = null
  }
}

function setCell(row: TableRow, column: TableColumn, raw: string | number | boolean) {
  if (column.type === 'bool') {
    row.values[column.id] = Boolean(raw)
    return
  }
  if (column.type === 'int' || column.type === 'number') {
    const text = String(raw).trim()
    if (!text) {
      delete row.values[column.id]
      return
    }
    const number = Number(text)
    if (!Number.isFinite(number)) {
      delete row.values[column.id]
      return
    }
    row.values[column.id] = column.type === 'int' ? Math.trunc(number) : number
    return
  }
  const text = String(raw)
  if (!text) {
    delete row.values[column.id]
    return
  }
  row.values[column.id] = text
}

function numberCell(value: CellValue | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : ''
}

function stringCell(row: TableRow, column: TableColumn): string {
  const value = row.values[column.id]
  return typeof value === 'string' ? value : ''
}

function boolCell(row: TableRow, column: TableColumn): boolean {
  return row.values[column.id] === true
}

function invalidEnumValue(row: TableRow, column: TableColumn): string | null {
  const value = row.values[column.id]
  if (typeof value === 'string' && value && !column.options.includes(value)) return value
  return null
}

function refTarget(column: TableColumn): DataTable | undefined {
  if (!column.refTableId) return undefined
  if (draft.value?.id === column.refTableId) return draft.value
  return store.tables.find((table) => table.id === column.refTableId)
}

function refOptions(column: TableColumn): Array<{ id: string; label: string }> {
  const target = refTarget(column)
  if (!target) return []
  return target.rows.map((row, index) => ({ id: row.id, label: rowReferenceLabel(target, row, index) }))
}

function missingRef(row: TableRow, column: TableColumn): string | null {
  const value = row.values[column.id]
  if (typeof value !== 'string' || !value) return null
  const target = refTarget(column)
  if (!target) return value
  return target.rows.some((item) => item.id === value) ? null : value
}

function rowIssues(row: TableRow): string[] {
  return issueRows.value.get(row.id) ?? []
}

function rowNumber(rowId: string): string {
  const table = draft.value
  if (!table) return '?'
  const index = table.rows.findIndex((row) => row.id === rowId)
  return index < 0 ? '?' : String(index + 1)
}

function rowDisplayIndex(row: TableRow): number {
  const table = draft.value
  if (!table) return 0
  return table.rows.findIndex((item) => item.id === row.id) + 1
}

function cellEmptyValue(value: CellValue | undefined): boolean {
  if (value === undefined || value === null) return true
  return typeof value === 'string' && value.trim() === ''
}

async function exportCurrent() {
  const current = draft.value
  if (!current || exportBlocked.value) return
  await flushSave()
  const result = await store.exportTable(current.id)
  if (!result) return
  notice.value = result.dir
  if (noticeTimer) clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    notice.value = ''
  }, 6000)
}

watch(draft, scheduleSave, { deep: true })

onMounted(() => {
  const id = store.consumeFocus('table')
  const target = id ? store.tables.find((table) => table.id === id) : store.tables[0]
  if (target) loadDraft(target)
})

// The command palette can name a table while this screen is already open;
// then no route change remounts it, so react to the focus request directly.
watch(
  () => store.focusRequest,
  () => {
    const id = store.consumeFocus('table')
    if (!id) return
    const target = store.tables.find((table) => table.id === id)
    if (target) void selectTable(target)
  },
)

onBeforeUnmount(() => {
  if (saveTimer) {
    cancelTimer()
    void persist()
  }
  if (noticeTimer) clearTimeout(noticeTimer)
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-baseline gap-3">
        <h2 class="text-lg font-semibold tracking-tight text-ink">数值表</h2>
        <span class="font-mono text-[11px] text-faint">{{ store.tables.length }} 张</span>
      </div>
      <button type="button" class="pb-btn pb-btn-primary h-9 py-1" @click="createTable()">
        <Plus :size="15" />
        新建表
      </button>
    </div>

    <div class="grid grid-cols-1 items-start gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
      <aside class="pb-card overflow-hidden">
        <div class="border-b border-line-soft p-3">
          <div class="relative">
            <Search :size="13" class="absolute top-1/2 left-2.5 -translate-y-1/2 text-faint" />
            <input
              v-model="query"
              class="pb-input h-8 py-1 pl-8 text-xs"
              placeholder="搜索表名或表键"
            />
          </div>
        </div>
        <div class="max-h-[calc(100vh-19rem)] min-h-[240px] overflow-y-auto p-1.5">
          <button
            v-for="table in filteredTables"
            :key="table.id"
            type="button"
            class="mb-0.5 flex w-full flex-col rounded-lg px-2.5 py-2 text-left transition-colors"
            :class="selectedId === table.id ? 'bg-accent-soft' : 'hover:bg-panel-2'"
            @click="selectTable(table)"
          >
            <div class="flex w-full items-center gap-1.5">
              <Table2
                :size="12"
                class="shrink-0"
                :class="selectedId === table.id ? 'text-accent' : 'text-faint'"
              />
              <span class="min-w-0 flex-1 truncate text-xs font-medium text-ink">
                {{ table.name }}
              </span>
              <span
                v-if="issueCounts.get(table.id)"
                class="shrink-0 rounded bg-red/10 px-1 font-mono text-[10px] text-red"
                :title="`${issueCounts.get(table.id)} 个问题会阻止导出`"
              >
                {{ issueCounts.get(table.id) }}
              </span>
            </div>
            <div class="mt-1 flex items-center gap-2 pl-[18px] font-mono text-[10px] text-faint">
              <span class="truncate">{{ table.key }}</span>
              <span class="shrink-0">{{ table.columns.length }} 列 · {{ table.rows.length }} 行</span>
            </div>
          </button>
          <p v-if="filteredTables.length === 0" class="px-3 py-10 text-center text-[11px] text-faint">
            {{ store.tables.length ? '没有匹配的表' : '还没有数值表' }}
          </p>
        </div>
      </aside>

      <section v-if="draft" class="pb-card min-h-[520px] overflow-hidden">
        <header class="border-b border-line-soft bg-panel px-5 py-3.5">
          <div class="flex items-start gap-3">
            <div class="min-w-0 flex-1">
              <input
                ref="nameRef"
                v-model="draft.name"
                class="w-full rounded-md bg-transparent text-lg font-semibold tracking-tight text-ink outline-none placeholder:text-faint focus:bg-panel-2/60"
                placeholder="表名"
                aria-label="数值表名称"
              />
              <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1.5">
                <span class="text-[10px] text-faint">表键</span>
                <input
                  :value="draft.key"
                  class="pb-input h-7 w-36 py-0 font-mono text-[11px]"
                  aria-label="表键"
                  @change="setTableKey(($event.target as HTMLInputElement).value)"
                />
                <span class="font-mono text-[10px] text-faint">
                  {{ draft.columns.length }} 列 · {{ draft.rows.length }} 行
                </span>
                <span
                  class="text-[10px]"
                  :class="saveState === 'error' ? 'text-red' : 'text-faint'"
                  title="改动会自动保存"
                >
                  {{
                    saveState === 'saving'
                      ? '保存中…'
                      : saveState === 'error'
                        ? '保存失败'
                        : saveState === 'saved'
                          ? '已保存'
                          : ''
                  }}
                </span>
              </div>
              <p v-if="keyConflict" class="mt-1 text-[10px] text-red">
                表键「{{ draft.key }}」已被「{{ keyConflict.name }}」使用。
              </p>
              <p v-else-if="referencedBy.length" class="mt-1 text-[10px] text-faint">
                被 {{ referencedBy.map((table) => table.name).join('、') }} 引用，不能删除。
              </p>
            </div>
            <button
              type="button"
              class="pb-btn h-8 shrink-0 px-2.5 py-1 text-[11px]"
              :disabled="exportBlocked"
              :title="
                issues.length
                  ? `${issues.length} 个问题会阻止导出`
                  : keyConflict
                    ? `表键「${draft.key}」已被「${keyConflict.name}」使用`
                    : saveState === 'error'
                      ? '改动还没有保存成功，先修正表名或表键'
                      : saveState === 'saving'
                        ? '正在保存，稍等'
                        : '导出到 exports/tables/<表键>/，同时生成 JSON 与 CSV'
              "
              @click="exportCurrent()"
            >
              <Download :size="13" />
              导出 JSON / CSV
            </button>
            <button
              type="button"
              class="flex size-8 shrink-0 items-center justify-center rounded-md text-faint transition-colors hover:bg-red/10 hover:text-red disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-faint"
              :disabled="referencedBy.length > 0"
              :title="referencedBy.length ? '被其他表引用，先把引用列改掉才能删除' : '删除数值表'"
              @click="removeTable()"
            >
              <Trash2 :size="14" />
            </button>
          </div>
          <input
            v-model="draft.note"
            class="pb-input mt-2.5 h-8 py-1 text-xs"
            placeholder="一句话说明这张表的用途（会写进导出的 JSON）"
            aria-label="数值表说明"
          />
          <p v-if="notice" class="mt-1.5 truncate font-mono text-[10px] text-green" :title="notice">
            已导出到 {{ notice }}
          </p>
        </header>

        <div v-if="issues.length" class="border-b border-line-soft bg-red/5 px-5 py-3">
          <div class="flex items-center gap-2 text-xs text-red">
            <AlertTriangle :size="13" />
            {{ issues.length }} 个问题会阻止导出
          </div>
          <ul class="mt-1.5 space-y-0.5 text-[11px] text-muted">
            <li v-for="(issue, index) in issues.slice(0, 4)" :key="`${issue.rowId}-${issue.columnId}-${index}`">
              <span v-if="issue.rowId" class="font-mono text-faint">第 {{ rowNumber(issue.rowId) }} 行 · </span>
              {{ issue.message }}
            </li>
          </ul>
          <p v-if="issues.length > 4" class="mt-1 text-[11px] text-faint">
            还有 {{ issues.length - 4 }} 个问题，表格里的红色数字标出了出问题的行。
          </p>
        </div>

        <section>
          <div class="flex flex-wrap items-center justify-between gap-2 px-5 py-3">
            <div class="flex items-baseline gap-2">
              <span class="pb-label">行</span>
              <span class="font-mono text-[11px] text-faint">{{ draft.rows.length }}</span>
            </div>
            <div class="flex items-center gap-2">
              <input
                v-model="rowQuery"
                class="pb-input h-8 w-44 py-1 text-xs"
                placeholder="筛选行"
                aria-label="筛选行"
              />
              <button type="button" class="pb-btn h-8 px-2.5 py-1 text-[11px]" @click="showColumns = true">
                <Columns3 :size="13" />
                列定义
                <span class="font-mono text-[10px] text-faint">{{ draft.columns.length }}</span>
              </button>
              <button type="button" class="pb-btn pb-btn-ghost h-8 px-2.5 py-1 text-[11px]" @click="addRow()">
                <Plus :size="13" />
                添加行
              </button>
            </div>
          </div>

          <div class="px-5 pb-5">
            <div
              v-if="selectedRowIds.length > 0"
              class="mb-2 flex flex-wrap items-center gap-2 rounded-lg border border-accent/30 bg-accent-soft px-3 py-1.5 text-[11px]"
            >
              <span class="text-ink">已选 {{ selectedRowIds.length }} 行</span>
              <button
                type="button"
                class="pb-btn h-6 border-red/30 bg-red/10 px-2 py-0 text-[10px] text-red hover:bg-red/20"
                @click="removeSelectedRows()"
              >
                <Trash2 :size="11" />
                删除选中
              </button>
              <button type="button" class="text-faint hover:text-ink" @click="clearSelection()">
                取消选择
              </button>
            </div>
            <div v-if="draft.rows.length === 0" class="rounded-lg border border-dashed border-line px-4 py-10 text-center">
              <p class="text-xs text-faint">这张表还没有行。</p>
              <button type="button" class="pb-btn mt-3 h-8 px-3 py-1 text-[11px]" @click="addRow()">
                <Plus :size="13" />
                添加第一行
              </button>
            </div>
            <div
              ref="tableScroll"
              v-else
              class="max-h-[56vh] overflow-auto rounded-lg border border-line-soft"
            >
              <table class="min-w-full text-xs">
                <thead class="sticky top-0 z-10 bg-panel-2">
                  <tr>
                    <th
                      class="sticky left-0 z-20 w-16 bg-panel-2 px-2 py-2 text-center font-normal"
                    >
                      <label class="flex items-center justify-center gap-1.5" title="选择当前显示的所有行">
                        <input
                          type="checkbox"
                          class="size-3.5 accent-accent"
                          :checked="allVisibleSelected"
                          :disabled="filteredRows.length === 0"
                          aria-label="全选当前行"
                          @change="toggleAllRows(($event.target as HTMLInputElement).checked)"
                        />
                        <span class="font-mono text-[10px] text-faint">#</span>
                      </label>
                    </th>
                    <th
                      v-for="column in draft.columns"
                      :key="column.id"
                      class="min-w-[150px] border-l border-line-soft px-2 py-2 text-left align-bottom font-normal"
                    >
                      <div class="flex items-center gap-1">
                        <span class="text-[11px] font-medium text-ink">{{ column.name }}</span>
                        <span v-if="column.required" class="text-accent" title="必填">*</span>
                      </div>
                      <div class="mt-0.5 flex items-center gap-1.5 font-mono text-[10px] text-faint">
                        <span>{{ column.key }}</span>
                        <span class="rounded bg-panel-3 px-1">{{ TYPE_LABELS[column.type] }}</span>
                      </div>
                    </th>
                    <th class="sticky right-0 z-20 w-20 border-l border-line-soft bg-panel-2 px-2 py-2"></th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="row in filteredRows"
                    :key="row.id"
                    class="border-t border-line-soft/70 transition-colors hover:bg-panel-2/40"
                    :class="
                      dragOverRowId === row.id
                        ? 'bg-accent/5'
                        : isRowSelected(row)
                          ? 'bg-accent/[0.035]'
                          : ''
                    "
                    @dragover.prevent="dragRowId && (dragOverRowId = row.id)"
                    @dragleave="dragOverRowId === row.id && (dragOverRowId = null)"
                    @drop.prevent="dropRow(row)"
                  >
                    <td class="sticky left-0 z-10 bg-panel px-2 py-1 align-middle">
                      <div class="flex items-center justify-center gap-1.5">
                        <input
                          type="checkbox"
                          class="size-3.5 shrink-0 accent-accent"
                          :checked="isRowSelected(row)"
                          :aria-label="`选择第 ${rowDisplayIndex(row)} 行`"
                          @change="toggleRow(row, ($event.target as HTMLInputElement).checked)"
                        />
                        <span
                          class="cursor-grab font-mono text-[10px] active:cursor-grabbing"
                          :class="rowIssues(row).length ? 'text-red' : 'text-faint'"
                          :title="rowIssues(row).join('\n') || `${row.id} · 拖动可调整顺序`"
                          draggable="true"
                          @dragstart="startRowDrag($event, row)"
                          @dragend="dragRowId = null; dragOverRowId = null"
                        >
                          {{ rowIssues(row).length ? rowIssues(row).length : rowDisplayIndex(row) }}
                        </span>
                      </div>
                    </td>
                    <td
                      v-for="column in draft.columns"
                      :key="column.id"
                      class="border-l border-line-soft/70 px-1 py-0.5"
                      :data-cell="`${row.id}:${column.id}`"
                    >
                      <input
                        v-if="column.type === 'text'"
                        :value="stringCell(row, column)"
                        class="h-7 w-full min-w-[120px] rounded bg-transparent px-1.5 text-xs text-ink outline-none focus:bg-panel-2"
                        :aria-label="`${column.name} 第 ${rowDisplayIndex(row)} 行`"
                        @input="setCell(row, column, ($event.target as HTMLInputElement).value)"
                        @keydown.enter.prevent="moveCell(row, column)"
                      />
                      <input
                        v-else-if="column.type === 'int' || column.type === 'number'"
                        :value="numberCell(row.values[column.id])"
                        type="number"
                        :step="column.type === 'int' ? 1 : 'any'"
                        class="h-7 w-full min-w-[90px] rounded bg-transparent px-1.5 font-mono text-xs text-ink outline-none focus:bg-panel-2"
                        :aria-label="`${column.name} 第 ${rowDisplayIndex(row)} 行`"
                        @change="setCell(row, column, ($event.target as HTMLInputElement).value)"
                        @keydown.enter.prevent="moveCell(row, column)"
                      />
                      <label
                        v-else-if="column.type === 'bool'"
                        class="flex h-7 items-center justify-center"
                        :aria-label="`${column.name} 第 ${rowDisplayIndex(row)} 行`"
                      >
                        <input
                          type="checkbox"
                          class="accent-accent"
                          :checked="boolCell(row, column)"
                          @change="setCell(row, column, ($event.target as HTMLInputElement).checked)"
                        />
                      </label>
                      <select
                        v-else-if="column.type === 'enum'"
                        :value="stringCell(row, column)"
                        class="h-7 w-full min-w-[110px] rounded bg-transparent px-1 text-xs text-ink outline-none focus:bg-panel-2"
                        :aria-label="`${column.name} 第 ${rowDisplayIndex(row)} 行`"
                        @change="setCell(row, column, ($event.target as HTMLSelectElement).value)"
                      >
                        <option value="">—</option>
                        <option
                          v-if="invalidEnumValue(row, column)"
                          :value="invalidEnumValue(row, column) ?? ''"
                        >
                          {{ invalidEnumValue(row, column) }}（不在可选范围）
                        </option>
                        <option v-for="option in column.options" :key="option" :value="option">
                          {{ option }}
                        </option>
                      </select>
                      <select
                        v-else-if="column.type === 'ref'"
                        :value="stringCell(row, column)"
                        class="h-7 w-full min-w-[140px] rounded bg-transparent px-1 text-xs text-ink outline-none focus:bg-panel-2"
                        :aria-label="`${column.name} 第 ${rowDisplayIndex(row)} 行`"
                        @change="setCell(row, column, ($event.target as HTMLSelectElement).value)"
                      >
                        <option value="">未设置</option>
                        <option
                          v-if="missingRef(row, column)"
                          :value="missingRef(row, column) ?? ''"
                        >
                          {{ missingRef(row, column)?.slice(0, 6) }}…（引用的行已删除）
                        </option>
                        <option v-for="option in refOptions(column)" :key="option.id" :value="option.id">
                          {{ option.label }}
                        </option>
                      </select>
                    </td>
                    <td class="sticky right-0 z-10 w-20 border-l border-line-soft/70 bg-panel px-1.5 py-1 text-right align-middle">
                      <div class="flex items-center justify-end gap-0.5">
                        <button
                          type="button"
                          class="flex size-6 items-center justify-center rounded text-faint hover:bg-panel-3 hover:text-ink"
                          title="编辑这一行（适合列很多的表）"
                          @click="openRowEditor(row)"
                        >
                          <Pencil :size="12" />
                        </button>
                        <button
                          type="button"
                          class="flex size-6 items-center justify-center rounded text-faint hover:bg-panel-3 hover:text-ink"
                          title="复制这一行"
                          @click="duplicateRow(row)"
                        >
                          <Copy :size="12" />
                        </button>
                        <button
                          type="button"
                          class="flex size-6 items-center justify-center rounded text-faint hover:bg-red/10 hover:text-red"
                          title="删除这一行"
                          @click="removeRow(row)"
                        >
                          <X :size="12" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-if="draft.rows.length > 0 && filteredRows.length === 0" class="mt-2 text-center text-[11px] text-faint">
              没有匹配的行。清空筛选可以看到全部 {{ draft.rows.length }} 行。
            </p>
            <p v-else-if="!showColumns" class="mt-2 text-[10px] text-faint">
              列名和字段键在「列定义」里改；表头显示每列的导出字段。
            </p>
          </div>
        </section>
      </section>

      <EmptyState
        v-else
        title="还没有数值表"
        hint="物品、技能、敌人、掉落……用带类型的表统一管理，导出成 JSON 和 CSV 交给引擎。"
      >
        <template #icon><Table2 :size="24" /></template>
        <template #action>
          <button type="button" class="pb-btn pb-btn-primary" @click="createTable()">
            <Plus :size="15" />
            新建第一张表
          </button>
        </template>
      </EmptyState>
    </div>

    <AppModal
      v-model:open="showColumns"
      title="列定义"
      description="列名、字段键与类型；表头显示每列的导出字段。"
      wide
    >
      <div class="flex flex-wrap items-center gap-1.5 pb-2">
        <button
          v-for="type in COLUMN_TYPES"
          :key="type"
          type="button"
          class="rounded-md px-1.5 py-1 text-[11px] text-faint transition-colors hover:bg-panel-2 hover:text-ink"
          :title="`添加一列 ${TYPE_LABELS[type]}`"
          @click="addColumn(type)"
        >
          + {{ TYPE_LABELS[type] }}
        </button>
      </div>
      <div class="space-y-2">
            <div
              v-for="column in uiColumns"
              :key="column.id"
              class="rounded-lg border border-line-soft bg-panel-2/40 p-2.5"
            >
              <div class="flex flex-wrap items-center gap-2">
                <input
                  v-model="column.name"
                  class="pb-input h-8 min-w-0 flex-1 py-1 text-xs"
                  placeholder="列名"
                  :aria-label="`列名：${column.name || '未命名列'}`"
                />
                <input
                  :value="column.key"
                  class="pb-input h-8 w-28 py-1 font-mono text-[11px]"
                  placeholder="字段键"
                  :aria-label="`字段键：${column.name || '未命名列'}`"
                  @change="setColumnKey(column, ($event.target as HTMLInputElement).value)"
                />
                <select
                  v-model="column.type"
                  class="pb-input h-8 w-24 py-0 text-xs"
                  :aria-label="`列类型：${column.name || '未命名列'}`"
                  @change="changeColumnType(column)"
                >
                  <option v-for="type in COLUMN_TYPES" :key="type" :value="type">
                    {{ TYPE_LABELS[type] }}
                  </option>
                </select>
                <label class="flex items-center gap-1 text-[11px] text-muted">
                  <input v-model="column.required" type="checkbox" class="accent-accent" />
                  必填
                </label>
                <button
                  type="button"
                  class="flex size-7 shrink-0 items-center justify-center rounded-md text-faint hover:bg-red/10 hover:text-red"
                  title="删除列"
                  @click="removeColumn(column)"
                >
                  <X :size="13" />
                </button>
              </div>

              <div v-if="column.type === 'enum'" class="mt-1.5 flex items-center gap-2">
                <span class="shrink-0 text-[10px] text-faint">可选值</span>
                <input
                  :value="column.options.join('、')"
                  class="pb-input h-7 flex-1 py-0 text-[11px]"
                  placeholder="用顿号或逗号分隔"
                  @change="setOptions(column, ($event.target as HTMLInputElement).value)"
                />
              </div>
              <div v-else-if="column.type === 'ref'" class="mt-1.5 flex items-center gap-2">
                <span class="shrink-0 text-[10px] text-faint">引用表</span>
                <select v-model="column.refTableId" class="pb-input h-7 max-w-56 py-0 text-[11px]">
                  <option value="">选择表…</option>
                  <option v-for="table in store.tables" :key="table.id" :value="table.id">
                    {{ table.name }}
                  </option>
                </select>
              </div>
              <div
                v-else-if="column.type === 'int' || column.type === 'number'"
                class="mt-1.5 flex items-center gap-2"
              >
                <span class="shrink-0 text-[10px] text-faint">范围</span>
                <input
                  :value="column.min ?? ''"
                  type="number"
                  class="pb-input h-7 w-24 py-0 text-[11px]"
                  placeholder="最小"
                  @change="setLimit(column, 'min', ($event.target as HTMLInputElement).value)"
                />
                <span class="text-faint">—</span>
                <input
                  :value="column.max ?? ''"
                  type="number"
                  class="pb-input h-7 w-24 py-0 text-[11px]"
                  placeholder="最大"
                  @change="setLimit(column, 'max', ($event.target as HTMLInputElement).value)"
                />
                <span class="text-[10px] text-faint">留空表示不限</span>
              </div>
            </div>
            <p v-if="uiColumns.length === 0" class="px-1 text-[11px] text-faint">
              还没有列。先加一列，再往表格里填行。
            </p>
          </div>
    </AppModal>

    <AppModal
      :open="editingRow !== null"
      title="编辑行"
      :description="draft ? `${draft.name} · 第 ${editingIndex + 1} 行` : ''"
      wide
      @update:open="(open: boolean) => { if (!open) closeRowEditor() }"
    >
      <div v-if="editingRow && draft" class="grid gap-3.5 sm:grid-cols-2">
        <label v-for="column in draft.columns" :key="column.id" class="block">
          <span class="pb-label">
            {{ column.name || column.key }}
            <span v-if="column.required" class="text-accent">*</span>
          </span>
          <span class="mt-1.5 block">
            <input
              v-if="column.type === 'text'"
              :value="stringCell(editingRow, column)"
              class="pb-input"
              @input="setCell(editingRow, column, ($event.target as HTMLInputElement).value)"
            />
            <input
              v-else-if="column.type === 'int' || column.type === 'number'"
              :value="numberCell(editingRow.values[column.id])"
              type="number"
              :step="column.type === 'int' ? 1 : 'any'"
              class="pb-input font-mono"
              @change="setCell(editingRow, column, ($event.target as HTMLInputElement).value)"
            />
            <span v-else-if="column.type === 'bool'" class="flex h-9 items-center gap-2">
              <input
                type="checkbox"
                class="size-4 accent-accent"
                :checked="boolCell(editingRow, column)"
                @change="setCell(editingRow, column, ($event.target as HTMLInputElement).checked)"
              />
              <span class="text-xs text-muted">{{ boolCell(editingRow, column) ? '是' : '否' }}</span>
            </span>
            <select
              v-else-if="column.type === 'enum'"
              :value="stringCell(editingRow, column)"
              class="pb-input"
              @change="setCell(editingRow, column, ($event.target as HTMLSelectElement).value)"
            >
              <option value="">—</option>
              <option v-if="invalidEnumValue(editingRow, column)" :value="invalidEnumValue(editingRow, column) ?? ''">
                {{ invalidEnumValue(editingRow, column) }}（不在可选范围）
              </option>
              <option v-for="option in column.options" :key="option" :value="option">{{ option }}</option>
            </select>
            <select
              v-else-if="column.type === 'ref'"
              :value="stringCell(editingRow, column)"
              class="pb-input"
              @change="setCell(editingRow, column, ($event.target as HTMLSelectElement).value)"
            >
              <option value="">未设置</option>
              <option v-if="missingRef(editingRow, column)" :value="missingRef(editingRow, column) ?? ''">
                {{ missingRef(editingRow, column)?.slice(0, 6) }}…（引用的行已删除）
              </option>
              <option v-for="option in refOptions(column)" :key="option.id" :value="option.id">
                {{ option.label }}
              </option>
            </select>
          </span>
          <span class="mt-1 block font-mono text-[10px] text-faint">
            {{ column.key }} · {{ TYPE_LABELS[column.type] }}
          </span>
        </label>
        <p v-if="draft.columns.length === 0" class="text-xs text-faint">
          这张表还没有列，先在「列定义」里加一列。
        </p>
      </div>
      <template #footer>
        <button
          type="button"
          class="pb-btn"
          :disabled="editingIndex <= 0"
          @click="stepRow(-1)"
        >
          <ChevronLeft :size="13" />
          上一行
        </button>
        <button
          type="button"
          class="pb-btn"
          :disabled="!draft || editingIndex < 0 || editingIndex >= draft.rows.length - 1"
          @click="stepRow(1)"
        >
          下一行
          <ChevronRight :size="13" />
        </button>
        <span class="flex-1"></span>
        <button
          type="button"
          class="pb-btn border-red/30 bg-red/10 text-red hover:bg-red/20"
          :disabled="!editingRow"
          @click="deleteEditingRow()"
        >
          <Trash2 :size="13" />
          删除这一行
        </button>
        <button type="button" class="pb-btn pb-btn-primary" @click="closeRowEditor()">完成</button>
      </template>
    </AppModal>
  </div>
</template>