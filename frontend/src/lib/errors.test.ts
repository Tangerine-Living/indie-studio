import { describe, expect, it } from 'vitest'
import { AppError, normalizeError } from './errors'

describe('normalizeError', () => {
  it('splits a backend error string into code, message and detail', () => {
    const error = normalizeError(
      new Error('PROJECT_LOCKED: 项目已被另一个 Indie Studio 实例打开 (C:\\work\\game)'),
    )
    expect(error).toBeInstanceOf(AppError)
    expect(error.code).toBe('PROJECT_LOCKED')
    expect(error.message).toBe('项目已被另一个 Indie Studio 实例打开')
    expect(error.detail).toBe('C:\\work\\game')
  })

  it('keeps messages without a code', () => {
    const error = normalizeError(new Error('网络连接失败'))
    expect(error.code).toBe('UNEXPECTED')
    expect(error.message).toBe('网络连接失败')
  })

  it('passes through AppError unchanged', () => {
    const original = new AppError('IO', '写入失败')
    expect(normalizeError(original)).toBe(original)
  })

  it('handles strings and unknown values', () => {
    expect(normalizeError('INVALID_NAME: 项目名称不能为空').code).toBe('INVALID_NAME')
    expect(normalizeError(null).code).toBe('UNEXPECTED')
  })
})