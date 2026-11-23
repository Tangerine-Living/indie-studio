import { defineStore } from 'pinia'
import { ref } from 'vue'

/**
 * Lore drafts survive route changes and app restarts. They are keyed by the
 * project path so two projects never share a character's copy or prompt.
 * Released final content lives in the entity itself (`artPrompt`) or its body.
 */
export interface DraftEntry {
  copy: string
  prompt: string
  updatedAt: string
}

type DraftMap = Record<string, Record<string, DraftEntry>>

const STORAGE_KEY = 'pixelbench:lore-drafts'

function read(): DraftMap {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as DraftMap
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

export const useDraftsStore = defineStore('loreDrafts', () => {
  const projects = ref<DraftMap>(read())

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(projects.value))
    } catch {
      // Storage full or disabled: drafts still work for this session.
    }
  }

  function get(projectPath: string, entityId: string): DraftEntry | undefined {
    return projects.value[projectPath]?.[entityId]
  }

  function set(projectPath: string, entityId: string, patch: Partial<DraftEntry>) {
    if (!projectPath || !entityId) return
    const byEntity = (projects.value[projectPath] ??= {})
    const entry = (byEntity[entityId] ??= { copy: '', prompt: '', updatedAt: '' })
    Object.assign(entry, patch, { updatedAt: new Date().toISOString() })
    persist()
  }

  function clear(projectPath: string, entityId: string) {
    const byEntity = projects.value[projectPath]
    if (!byEntity) return
    delete byEntity[entityId]
    persist()
  }

  /** Test helper: forget every draft without touching project files. */
  function reset() {
    projects.value = {}
    persist()
  }

  return { projects, get, set, clear, reset }
})