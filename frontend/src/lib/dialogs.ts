import { reactive } from 'vue'

/**
 * App-owned confirm and prompt dialogs. Native `window.confirm` renders a white
 * WebView2 sheet that ignores the app's theme, so every destructive action goes
 * through this queue instead.
 */
export interface ConfirmOptions {
  title: string
  message?: string
  /** Monospace block for values the user is about to send or lose. */
  detail?: string
  confirmText?: string
  cancelText?: string
  tone?: 'default' | 'danger'
}

export interface PromptOptions {
  title: string
  message?: string
  placeholder?: string
  value?: string
  confirmText?: string
}

type Resolver = (value: boolean | string | null) => void

export const dialogState = reactive({
  open: false,
  mode: 'confirm' as 'confirm' | 'prompt',
  title: '',
  message: '',
  detail: '',
  confirmText: '确定',
  cancelText: '取消',
  tone: 'default' as 'default' | 'danger',
  promptValue: '',
  promptPlaceholder: '',
  resolve: null as Resolver | null,
})

function open(): Promise<boolean | string | null> {
  return new Promise((resolve) => {
    dialogState.resolve = resolve
    dialogState.open = true
  })
}

function reset() {
  dialogState.open = false
  dialogState.message = ''
  dialogState.detail = ''
  dialogState.promptValue = ''
  dialogState.promptPlaceholder = ''
}

export async function confirmDialog(options: ConfirmOptions): Promise<boolean> {
  dialogState.mode = 'confirm'
  dialogState.title = options.title
  dialogState.message = options.message ?? ''
  dialogState.detail = options.detail ?? ''
  dialogState.confirmText = options.confirmText ?? '确定'
  dialogState.cancelText = options.cancelText ?? '取消'
  dialogState.tone = options.tone ?? 'default'
  const result = await open()
  return result === true
}

export async function promptDialog(options: PromptOptions): Promise<string | null> {
  dialogState.mode = 'prompt'
  dialogState.title = options.title
  dialogState.message = options.message ?? ''
  dialogState.detail = ''
  dialogState.confirmText = options.confirmText ?? '添加'
  dialogState.cancelText = '取消'
  dialogState.tone = 'default'
  dialogState.promptValue = options.value ?? ''
  dialogState.promptPlaceholder = options.placeholder ?? ''
  const result = await open()
  if (typeof result !== 'string') return null
  return result.trim() || null
}

/** Called by the dialog host; resolves the pending promise exactly once. */
export function settleDialog(confirmed: boolean) {
  const resolve = dialogState.resolve
  if (!resolve) return
  const value =
    dialogState.mode === 'prompt' ? (confirmed ? dialogState.promptValue : null) : confirmed
  reset()
  dialogState.resolve = null
  resolve(value)
}