import type { CellValue, ColumnType, DataTable, TableIssue, TableRow } from './types'

/**
 * Mirrors project.NormalizeTableKey. Kept in one place so the editor, the mock
 * platform and the exported files agree on field names.
 */
export function normalizeTableKey(raw: string): string {
  let key = raw.trim().toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '')
  if (!key) key = 'table'
  if (/^[0-9]/.test(key)) key = `t_${key}`
  if (key === 'id') key = 'id_value'
  if (key.length > 64) {
    key = key.slice(0, 64).replace(/_+$/, '')
    if (!key) key = 'table'
  }
  return key
}

export function zeroCell(type: ColumnType): CellValue {
  switch (type) {
    case 'int':
    case 'number':
      return 0
    case 'bool':
      return false
    default:
      return ''
  }
}

/** A required cell counts as filled when a false boolean or a zero is stored. */
export function cellEmpty(value: CellValue | undefined | null): boolean {
  if (value === undefined || value === null) return true
  return typeof value === 'string' && value.trim() === ''
}

export function cellText(value: CellValue | undefined | null): string {
  if (value === undefined || value === null) return ''
  if (typeof value === 'boolean') return value ? '是' : '否'
  return String(value)
}

/** Best human label for a row: its first filled text cell, else a position. */
export function rowTitle(table: DataTable, row: TableRow, index: number): string {
  for (const col of table.columns) {
    if (col.type !== 'text') continue
    const value = row.values[col.id]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return `第 ${index + 1} 行`
}

export function rowReferenceLabel(table: DataTable, row: TableRow, index: number): string {
  const title = rowTitle(table, row, index)
  return title.startsWith('第 ') ? `${title} · ${row.id.slice(0, 6)}` : title
}

/**
 * Mirrors project.TableIssues. The editor uses it for instant feedback; the Go
 * implementation stays authoritative and runs again before export.
 */
export function tableIssues(table: DataTable, tables: DataTable[]): TableIssue[] {
  const byId = new Map(tables.map((item) => [item.id, item]))
  const issues: TableIssue[] = []
  const add = (rowId: string, columnId: string, message: string) => {
    if (issues.length < 200) issues.push({ rowId, columnId, message })
  }

  for (const col of table.columns) {
    if (col.type === 'enum' && col.options.length === 0) {
      add('', col.id, `列「${col.name}」是枚举，但还没有可选值`)
    }
    if (col.type === 'ref') {
      if (!col.refTableId) {
        add('', col.id, `列「${col.name}」还没有选择要引用的表`)
      } else if (!byId.has(col.refTableId)) {
        add('', col.id, `列「${col.name}」引用的表已被删除`)
      }
    }
  }

  const refRows = new Map<string, Set<string>>()
  for (const col of table.columns) {
    if (col.type !== 'ref' || !col.refTableId) continue
    const target = byId.get(col.refTableId)
    if (!target) continue
    if (!refRows.has(col.refTableId)) {
      refRows.set(col.refTableId, new Set(target.rows.map((row) => row.id)))
    }
  }

  table.rows.forEach((row) => {
    for (const col of table.columns) {
      const value = row.values[col.id]
      const present = value !== undefined && value !== null
      if (col.required && cellEmpty(value)) {
        add(row.id, col.id, `「${col.name}」是必填项`)
        continue
      }
      if (!present) continue
      if (col.type === 'int' || col.type === 'number') {
        if (typeof value !== 'number' || !Number.isFinite(value)) {
          add(row.id, col.id, `「${col.name}」不是数字`)
          continue
        }
        if (col.min !== undefined && value < col.min) {
          add(row.id, col.id, `「${col.name}」不能小于 ${col.min}`)
        }
        if (col.max !== undefined && value > col.max) {
          add(row.id, col.id, `「${col.name}」不能大于 ${col.max}`)
        }
      } else if (col.type === 'enum') {
        if (typeof value !== 'string' || !col.options.includes(value)) {
          add(row.id, col.id, `「${col.name}」的值不在可选范围内`)
        }
      } else if (col.type === 'ref') {
        if (typeof value !== 'string' || !value) continue
        const rows = refRows.get(col.refTableId ?? '')
        if (rows && !rows.has(value)) {
          add(row.id, col.id, `「${col.name}」引用的行已被删除`)
        }
      }
    }
  })
  return issues
}