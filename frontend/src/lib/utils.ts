import clsx, { type ClassValue } from 'clsx'

export { clsx }

export function cx(...inputs: ClassValue[]): string {
  return clsx(inputs)
}

export function newID(): string {
  const bytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && 'getRandomValues' in crypto) {
    crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256)
  }
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

export function nowISO(): string {
  return new Date().toISOString()
}

/** Format an ISO date (or date-only string) for Chinese UI. */
export function formatDate(value?: string | null): string {
  if (!value) return '未设置'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`
}

export function formatDateShort(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${date.getMonth() + 1}月${date.getDate()}日`
}

export function relativeTime(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const delta = Date.now() - date.getTime()
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  if (delta < minute) return '刚刚'
  if (delta < hour) return `${Math.floor(delta / minute)} 分钟前`
  if (delta < day) return `${Math.floor(delta / hour)} 小时前`
  if (delta < 30 * day) return `${Math.floor(delta / day)} 天前`
  return formatDateShort(value)
}

export function daysUntil(value?: string | null): number | null {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return Math.ceil((date.getTime() - Date.now()) / 86_400_000)
}

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const value = n / 1024 ** i
  return `${value >= 10 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`
}

/** Shorten a long Windows path from the middle, keeping the tail readable. */
export function shortPath(path: string, max = 46): string {
  if (path.length <= max) return path
  const parts = path.split(/[\\/]/)
  let tail = parts.pop() ?? ''
  while (parts.length > 0) {
    const candidate = parts[parts.length - 1] + '\\' + tail
    if (candidate.length + 4 > max) break
    tail = candidate
    parts.pop()
  }
  return `…\\${tail}`
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}

export function pct(part: number, total: number): number {
  if (total <= 0) return 0
  return Math.round((part / total) * 100)
}

export function toDateInput(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.slice(0, 10)
  return date.toISOString().slice(0, 10)
}

export function fromDateInput(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  return `${trimmed.slice(0, 10)}T00:00:00Z`
}

/** Remove duplicate ids while keeping order. */
export function uniq<T>(items: T[]): T[] {
  return Array.from(new Set(items))
}

/**
 * Put the scrolling screen pane back at the top, so switching to another
 * record starts at its title instead of wherever the last one was scrolled to.
 */
export function scrollScreenTop() {
  const main = document.querySelector('main')
  if (main) main.scrollTop = 0
}