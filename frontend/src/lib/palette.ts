import type { Palette } from '../domain/types'

export type PaletteFormat = 'gpl' | 'pal' | 'hex'

export const PALETTE_FORMATS: ReadonlyArray<{ id: PaletteFormat; label: string; ext: string }> = [
  { id: 'gpl', label: 'GIMP 调色板（.gpl）', ext: 'gpl' },
  { id: 'pal', label: 'JASC 调色板（.pal）', ext: 'pal' },
  { id: 'hex', label: '十六进制列表（.hex）', ext: 'hex' },
]

const HEX_TOKEN = /#?([0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b/g

/** Expands #abc to #aabbcc and lowercases; returns null for non-hex input. */
export function normalizeColor(raw: string): string | null {
  const value = raw.trim()
  const match = /^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.exec(value)
  if (!match) return null
  let hex = match[1].toLowerCase()
  if (hex.length === 3) hex = hex.split('').map((char) => char + char).join('')
  return `#${hex}`
}

export function rgbToHex(r: number, g: number, b: number): string {
  const clamp = (value: number) => Math.max(0, Math.min(255, Math.round(value)))
  return `#${[r, g, b].map((value) => clamp(value).toString(16).padStart(2, '0')).join('')}`
}

export function hexToRgb(hex: string): { r: number; g: number; b: number } {
  const color = normalizeColor(hex) ?? '#000000'
  return {
    r: Number.parseInt(color.slice(1, 3), 16),
    g: Number.parseInt(color.slice(3, 5), 16),
    b: Number.parseInt(color.slice(5, 7), 16),
  }
}

export function dedupeColors(colors: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const raw of colors) {
    const color = normalizeColor(raw)
    if (!color || seen.has(color)) continue
    seen.add(color)
    result.push(color)
  }
  return result
}

function triples(text: string): number[][] {
  const result: number[][] = []
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#') || trimmed.startsWith(';') || trimmed.startsWith('//')) {
      continue
    }
    const parts = trimmed.split(/\s+/)
    if (parts.length < 3) continue
    const [r, g, b] = parts.slice(0, 3).map((part) => Number(part))
    if (![r, g, b].every((value) => Number.isFinite(value) && value >= 0 && value <= 255)) continue
    result.push([r, g, b])
  }
  return result
}

/** Reads GPL, JASC-PAL and plain hex/rgb lists into one palette payload. */
export function parsePaletteText(
  text: string,
  fallbackName = '导入调色板',
): { name: string; colors: string[] } | null {
  if (!text.trim()) return null

  if (/^GIMP Palette/i.test(text.trim())) {
    let name = fallbackName
    const colors: string[] = []
    for (const line of text.split(/\r?\n/)) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#') || /^GIMP Palette/i.test(trimmed)) continue
      const nameMatch = /^Name:\s*(.+)$/i.exec(trimmed)
      if (nameMatch) {
        name = nameMatch[1].trim() || fallbackName
        continue
      }
      if (/^Columns:/i.test(trimmed) || /^Channels:/i.test(trimmed)) continue
      const parts = trimmed.split(/\s+/)
      const [r, g, b] = parts.slice(0, 3).map((part) => Number(part))
      if ([r, g, b].every((value) => Number.isFinite(value))) {
        colors.push(rgbToHex(r, g, b))
      }
    }
    const unique = dedupeColors(colors)
    return unique.length > 0 ? { name, colors: unique } : null
  }

  if (/^JASC-PAL/i.test(text.trim())) {
    const colors: string[] = []
    const lines = text
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
    // Header, version and color count are the first three lines.
    for (const line of lines.slice(3)) {
      if (line.startsWith(';') || line.startsWith('#')) continue
      const [r, g, b] = line.split(/\s+/).slice(0, 3).map((part) => Number(part))
      if ([r, g, b].every((value) => Number.isFinite(value))) colors.push(rgbToHex(r, g, b))
    }
    const unique = dedupeColors(colors)
    return unique.length > 0 ? { name: fallbackName, colors: unique } : null
  }

  const hexes = [...text.matchAll(HEX_TOKEN)].map((match) => match[0])
  if (hexes.length > 0) {
    const unique = dedupeColors(hexes)
    if (unique.length > 0) return { name: fallbackName, colors: unique }
  }

  const rgb = triples(text)
  if (rgb.length > 0) {
    return { name: fallbackName, colors: dedupeColors(rgb.map(([r, g, b]) => rgbToHex(r, g, b))) }
  }
  return null
}

/** Serializes a palette to the requested interchange format. */
export function serializePalette(palette: Palette, format: PaletteFormat): string {
  const colors = dedupeColors(palette.colors)
  if (format === 'pal') {
    const rows = colors.map((color) => {
      const { r, g, b } = hexToRgb(color)
      return `${r} ${g} ${b}`
    })
    return ['JASC-PAL', '0100', String(colors.length), ...rows].join('\r\n')
  }
  if (format === 'hex') {
    return colors.join('\n')
  }
  const rows = colors.map((color) => {
    const { r, g, b } = hexToRgb(color)
    return `${String(r).padStart(3, ' ')} ${String(g).padStart(3, ' ')} ${String(b).padStart(3, ' ')}\t${color}`
  })
  return ['GIMP Palette', `Name: ${palette.name}`, 'Columns: 0', '#', ...rows].join('\n')
}

export function paletteFromText(id: string, name: string, colors: string[]): Palette {
  return {
    schemaVersion: 1,
    id,
    name,
    colors: dedupeColors(colors),
    source: 'imported',
    locked: false,
  }
}

/** Keeps the leading colors so a palette can satisfy a project-wide color budget. */
export function trimPalette(colors: string[], max: number): string[] {
  return dedupeColors(colors).slice(0, Math.max(1, Math.trunc(max)))
}