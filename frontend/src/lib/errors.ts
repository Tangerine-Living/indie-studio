/**
 * Backend errors arrive as `"CODE: message (detail)"` through the Wails
 * bridge, or as a plain Error from the mock platform. Normalising them keeps
 * the UI able to branch on a stable code.
 */
export class AppError extends Error {
  readonly code: string
  readonly detail?: string

  constructor(code: string, message: string, detail?: string) {
    super(message)
    this.name = 'AppError'
    this.code = code
    this.detail = detail
  }
}

const CODE_PATTERN = /^([A-Z][A-Z0-9_]{1,63}):\s*([\s\S]*)$/

export function normalizeError(raw: unknown): AppError {
  if (raw instanceof AppError) return raw
  if (raw instanceof Error) {
    const match = CODE_PATTERN.exec(raw.message)
    if (match) {
      const body = match[2]
      const detailMatch = /\s\(([^()]*)\)$/.exec(body)
      return new AppError(
        match[1],
        detailMatch ? body.slice(0, detailMatch.index).trim() : body.trim(),
        detailMatch ? detailMatch[1] : undefined,
      )
    }
    return new AppError('UNEXPECTED', raw.message || '发生未知错误')
  }
  if (typeof raw === 'string') {
    const match = CODE_PATTERN.exec(raw)
    if (match) {
      const detailMatch = /\s\(([^()]*)\)$/.exec(match[2])
      return new AppError(
        match[1],
        detailMatch ? match[2].slice(0, detailMatch.index).trim() : match[2].trim(),
        detailMatch ? detailMatch[1] : undefined,
      )
    }
    return new AppError('UNEXPECTED', raw)
  }
  if (raw && typeof raw === 'object' && 'message' in raw) {
    return normalizeError(new Error(String((raw as { message: unknown }).message)))
  }
  return new AppError('UNEXPECTED', '发生未知错误')
}

