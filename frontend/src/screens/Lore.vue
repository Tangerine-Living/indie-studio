<script setup lang="ts">
import {
  ArrowUpRight,
  BookOpen,
  CalendarClock,
  Copy,
  FileText,
  Flag,
  Image as ImageIcon,
  Map as MapIcon,
  MapPin,
  Loader2,
  Package,
  Plus,
  Save,
  Search,
  Sparkles,
  Trash2,
  Type,
  User,
  X,
} from 'lucide-vue-next'
import type { Component } from 'vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ENTITY_KINDS,
  ENTITY_STATUSES,
} from '../domain/types'
import type { AssetType, Entity, EntityKind, EntityStatus } from '../domain/types'
import { useAppStore } from '../stores/app'
import { useDraftsStore } from '../stores/drafts'
import { confirmDialog } from '../lib/dialogs'
import { newID, nowISO, scrollScreenTop, uniq } from '../lib/utils'
import { COPYWRITER_SYSTEM, PROMPT_ENGINEER_SYSTEM } from '../lib/aiPrompts'
import AppModal from '../components/AppModal.vue'
import AssetThumb from '../components/AssetThumb.vue'
import EmptyState from '../components/EmptyState.vue'

const store = useAppStore()
const drafts = useDraftsStore()
const projectPath = computed(() => store.session?.path ?? '')

const KIND_META: Record<EntityKind, { label: string; icon: Component }> = {
  character: { label: '角色', icon: User },
  place: { label: '地点', icon: MapPin },
  faction: { label: '势力', icon: Flag },
  item: { label: '物品', icon: Package },
  event: { label: '事件', icon: CalendarClock },
  term: { label: '术语', icon: Type },
}

const STATUS_META: Record<EntityStatus, { label: string; class: string }> = {
  idea: { label: '构想', class: 'text-faint' },
  draft: { label: '草稿', class: 'text-accent' },
  stable: { label: '定稿', class: 'text-green' },
}

const query = ref('')
const kindFilter = ref<EntityKind | 'all'>('all')
const selectedId = ref<string | null>(null)
const draft = ref<Entity | null>(null)
const aliasInput = ref('')
const tagInput = ref('')
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const nameRef = ref<HTMLInputElement | null>(null)
const pickedAsset = ref('')
const pickedMap = ref('')

let loadingDraft = false
let saveSeq = 0
const dirty = ref(false)

const counts = computed(() => {
  const out: Record<EntityKind, number> = {
    character: 0,
    place: 0,
    faction: 0,
    item: 0,
    event: 0,
    term: 0,
  }
  for (const entity of store.entities) out[entity.kind] += 1
  return out
})

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return store.entities
    .filter((entity) => {
      if (kindFilter.value !== 'all' && entity.kind !== kindFilter.value) return false
      if (!needle) return true
      const haystack = [
        entity.name,
        entity.summary,
        entity.body,
        entity.aliases.join(' '),
        entity.tags.join(' '),
        entity.fields.map((field) => `${field.key} ${field.value}`).join(' '),
      ]
        .join(' ')
        .toLowerCase()
      return haystack.includes(needle)
    })
    // Alphabetical order keeps the index still while autosave rewrites an entry.
    .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
})

const kindOptions = computed(() =>
  ENTITY_KINDS.map((kind) => ({ kind, ...KIND_META[kind], count: counts.value[kind] })),
)

function kindLabel(kind: EntityKind): string {
  return KIND_META[kind]?.label ?? kind
}

function splitList(text: string): string[] {
  return text
    .split(/[,，、\n]/)
    .map((part) => part.trim())
    .filter(Boolean)
}

function snapshot(entity: Entity): Entity {
  return JSON.parse(JSON.stringify(entity)) as Entity
}

/** Reads the draft plus the raw alias/tag text into a clean payload. */
function buildPayload(): Entity | null {
  if (!draft.value) return null
  return {
    ...snapshot(draft.value),
    aliases: uniq(splitList(aliasInput.value)),
    tags: uniq(splitList(tagInput.value)),
    fields: draft.value.fields
      .map((field) => ({ key: field.key.trim(), value: field.value.trim() }))
      .filter((field) => field.key),
    relations: draft.value.relations
      .filter((relation) => relation.entityId)
      .map((relation) => ({ entityId: relation.entityId, label: relation.label.trim() || '相关' })),
  }
}

async function persist(): Promise<boolean> {
  const payload = buildPayload()
  if (!payload) return false
  const seq = (saveSeq += 1)
  saveState.value = 'saving'
  const saved = await store.saveEntity(payload)
  if (seq !== saveSeq) return false
  if (!saved) {
    saveState.value = 'error'
    return false
  }
  saveState.value = 'saved'
  dirty.value = false
  return true
}

async function saveNow(): Promise<void> {
  if (saveState.value === 'saving') return
  await persist()
}

/* ------------------------------------------------------ unsaved changes */

const unsavedOpen = ref(false)
const unsavedBusy = ref(false)
type UnsavedChoice = 'save' | 'discard' | 'cancel'
let unsavedResolve: ((choice: UnsavedChoice) => void) | null = null
let unsavedPromise: Promise<UnsavedChoice> | null = null

function askUnsaved(): Promise<UnsavedChoice> {
  if (unsavedPromise) return unsavedPromise
  unsavedPromise = new Promise((resolve) => {
    unsavedResolve = resolve
    unsavedOpen.value = true
  })
  return unsavedPromise
}

function onUnsavedModal(open: boolean) {
  if (!open) answerUnsaved('cancel')
}

function answerUnsaved(choice: UnsavedChoice) {
  unsavedOpen.value = false
  const resolve = unsavedResolve
  unsavedResolve = null
  unsavedPromise = null
  resolve?.(choice)
}

/** Returns false only when the user chooses to stay on the unsaved entry. */
async function continueAfterUnsaved(): Promise<boolean> {
  if (!dirty.value) return true
  const choice = await askUnsaved()
  if (choice === 'cancel') return false
  if (choice === 'save') {
    unsavedBusy.value = true
    const ok = await persist()
    unsavedBusy.value = false
    if (!ok) return false
  }
  return true
}

function loadDraft(entity: Entity | null) {
  saveSeq += 1
  loadingDraft = true
  selectedId.value = entity?.id ?? null
  draft.value = entity ? snapshot(entity) : null
  aliasInput.value = entity?.aliases.join('、') ?? ''
  tagInput.value = entity?.tags.join('、') ?? ''
  pickedAsset.value = ''
  pickedMap.value = ''
  saveState.value = 'idle'
  dirty.value = false
  void nextTick(() => {
    loadingDraft = false
  })
}

async function selectEntity(entity: Entity | null) {
  if (entity?.id === selectedId.value) return
  if (!(await continueAfterUnsaved())) return
  loadDraft(entity)
  // After the new records render, so scroll anchoring cannot pull it back.
  await nextTick()
  scrollScreenTop()
}

/**
 * Kind chips are a view filter, so a selection that the filter hides moves to
 * the first entry still visible. The search box keeps the selection instead.
 */
function setKind(kind: EntityKind | 'all') {
  kindFilter.value = kind
  const current = draft.value
  if (current && filtered.value.some((entity) => entity.id === current.id)) return
  void selectEntity(filtered.value[0] ?? null)
}

async function createEntity() {
  if (!(await continueAfterUnsaved())) return
  const now = nowISO()
  const entity: Entity = {
    schemaVersion: 1,
    id: newID(),
    kind: kindFilter.value === 'all' ? 'character' : kindFilter.value,
    name: '未命名设定',
    status: 'idea',
    aliases: [],
    summary: '',
    body: '',
    tags: [],
    fields: [],
    relations: [],
    assetIds: [],
    mapIds: [],
    artPrompt: '',
    createdAt: now,
    updatedAt: now,
  }
  const saved = await store.saveEntity(entity)
  if (!saved) return
  if (kindFilter.value !== 'all' && kindFilter.value !== saved.kind) kindFilter.value = 'all'
  loadDraft(saved)
  await nextTick()
  scrollScreenTop()
  nameRef.value?.focus()
  nameRef.value?.select()
}

async function removeEntity() {
  const current = draft.value
  if (!current) return
  const ok = await confirmDialog({
    title: `删除「${current.name}」`,
    message: '其他设定里指向它的关系会一并移除，文件会进入项目回收站。',
    confirmText: '删除设定',
    tone: 'danger',
  })
  if (!ok) return
  saveSeq += 1
  dirty.value = false
  const deleted = await store.deleteEntity(current.id)
  if (!deleted) return
  drafts.clear(projectPath.value, current.id)
  loadDraft(null)
  const next = store.entities[0]
  if (next) loadDraft(next)
}

/** Structured description shared by the AI request and the local fallback. */
function entityBriefLines(entity: Entity): string[] {
  const clean = (text: string) => text.trim().replace(/[。；，、！？\s]+$/u, '')
  const lines: string[] = [`名称：${entity.name}`, `类型：${kindLabel(entity.kind)}`]
  if (entity.aliases.length) lines.push(`别名：${entity.aliases.join('、')}`)
  if (entity.summary.trim()) lines.push(`一句话：${clean(entity.summary)}`)
  if (entity.body.trim()) lines.push(`正文：${entity.body.trim()}`)
  const fields = entity.fields
    .filter((field) => field.key.trim() && field.value.trim())
    .map((field) => `${clean(field.key)}：${clean(field.value)}`)
  if (fields.length) lines.push(`档案：${fields.join('；')}`)
  const relations = entity.relations
    .map((relation) => {
      const target = store.entities.find((item) => item.id === relation.entityId)
      return target ? `${relation.label}「${target.name}」` : ''
    })
    .filter(Boolean)
  if (relations.length) lines.push(`关系：${relations.join('；')}`)
  const assets = entity.assetIds
    .map((id) => store.assets.find((asset) => asset.id === id)?.name)
    .filter((name): name is string => Boolean(name))
  if (assets.length) lines.push(`关联素材：${assets.join('、')}`)
  const maps = entity.mapIds
    .map((id) => store.maps.find((map) => map.id === id)?.name)
    .filter((name): name is string => Boolean(name))
  if (maps.length) lines.push(`关联地图：${maps.join('、')}`)
  if (entity.tags.length) lines.push(`标签：${entity.tags.join('、')}`)
  return lines
}

const promptOpen = ref(false)
const promptText = ref('')
const promptNotice = ref('')
const promptLoading = ref(false)
const promptError = ref('')
const promptFixed = computed(() => Boolean((draft.value?.artPrompt ?? '').trim()))

function assetTypeFor(entity: Entity): AssetType {
  return entity.kind === 'character'
    ? 'sprite'
    : entity.kind === 'place'
      ? 'building'
      : entity.kind === 'item'
        ? 'prop'
        : 'other'
}

function localAssetPrompt(entity: Entity): string {
  return [...entityBriefLines(entity), store.styleGuideText()].filter(Boolean).join('，')
}

/** Asks the text model for the prompt, keeping the local build as fallback. */
async function regenerateAssetPrompt() {
  const entity = draft.value
  if (!entity) return
  promptLoading.value = true
  promptNotice.value = ''
  promptError.value = ''
  const brief = entityBriefLines(entity)
  const settings = store.session?.project.settings
  const palette = store.palettes.find(
    (item) => item.id === settings?.defaultPaletteId,
  )
  const style = store.styleGuideText()
  const { result, error: aiError } = await store.generateText(
    {
      system: PROMPT_ENGINEER_SYSTEM,
      user: [
        `素材类型：${kindLabel(entity.kind)}`,
        settings ? `项目瓦片尺寸：${settings.tileSize}px` : '',
        `项目风格指南：${style}`,
        palette && palette.colors.length ? `项目调色板：${palette.colors.join(' ')}` : '',
        '设定档案：',
        ...brief,
        '请据此写一条像素美术提示词。',
      ]
        .filter(Boolean)
        .join('\n'),
      maxTokens: 1600,
      temperature: 0.6,
    },
    { silent: true },
  )
  promptLoading.value = false
  const text = result?.text.trim() ?? ''
  if (text) {
    promptText.value = text
    drafts.set(projectPath.value, entity.id, { prompt: text })
  } else {
    promptText.value = localAssetPrompt(entity)
    drafts.set(projectPath.value, entity.id, { prompt: promptText.value })
    const reason = aiError
      ? [aiError.message, aiError.detail].filter(Boolean).join(' · ')
      : '模型没有返回提示词'
    promptError.value = `AI 起稿失败：${reason}。已填入本地拼装的提示词，可以编辑后再用。`
  }
}

/** Opens the confirmation layer; a fixed or cached prompt is never re-rolled. */
function openAssetPrompt() {
  const entity = draft.value
  if (!entity) return
  promptOpen.value = true
  promptNotice.value = ''
  promptError.value = ''
  const fixed = (entity.artPrompt ?? '').trim()
  if (fixed) {
    promptText.value = fixed
    promptLoading.value = false
    return
  }
  const cached = drafts.get(projectPath.value, entity.id)?.prompt
  if (cached) {
    promptText.value = cached
    promptLoading.value = false
    return
  }
  void regenerateAssetPrompt()
}

function cacheAssetPrompt() {
  const entity = draft.value
  if (entity) drafts.set(projectPath.value, entity.id, { prompt: promptText.value })
}

/** Writes the confirmed prompt into the entity and saves the entry. */
async function fixAssetPrompt() {
  const entity = draft.value
  if (!entity) return
  const text = promptText.value.trim()
  if (!text) return
  entity.artPrompt = text
  const saved = await persist()
  if (!saved) {
    promptError.value = store.error?.message ?? '保存角色提示词失败'
    return
  }
  promptNotice.value = '已设为角色固定提示词，之后生成素材会直接复用这条。'
}

function openWorkbenchFromPrompt() {
  const entity = draft.value
  if (!entity) return
  const text = promptText.value.trim() || localAssetPrompt(entity)
  promptOpen.value = false
  store.openGenerateFromRecord({
    name: entity.name,
    prompt: text,
    assetType: assetTypeFor(entity),
    sourceKind: 'entity',
    sourceLabel: entity.name,
    sourceId: entity.id,
    style: '',
    promptStyled: true,
  })
}

/* -------------------------------------------------------- copy drafting */

const copyOpen = ref(false)
const copyText = ref('')
const copyNotice = ref('')
const copyLoading = ref(false)
const copyError = ref('')

/** Deterministic fallback when no text model is configured. */
function localCopyText(entity: Entity): string {
  const lines: string[] = []
  const aliases = entity.aliases.length ? `（${entity.aliases.join('、')}）` : ''
  lines.push(`${entity.name}${aliases}，${kindLabel(entity.kind)}。`)
  if (entity.summary.trim()) lines.push(entity.summary.trim())
  if (entity.body.trim()) lines.push(entity.body.trim())
  const fields = entity.fields.filter((field) => field.key.trim() && field.value.trim())
  if (fields.length) {
    lines.push(`档案：${fields.map((field) => `${field.key}：${field.value}`).join('；')}。`)
  }
  const assets = entity.assetIds
    .map((id) => store.assets.find((asset) => asset.id === id)?.name)
    .filter((name): name is string => Boolean(name))
  if (assets.length) lines.push(`素材：${assets.join('、')}。`)
  const maps = entity.mapIds
    .map((id) => store.maps.find((map) => map.id === id)?.name)
    .filter((name): name is string => Boolean(name))
  if (maps.length) lines.push(`地图：${maps.join('、')}。`)
  const relations = entity.relations
    .map((relation) => {
      const target = store.entities.find((item) => item.id === relation.entityId)
      return target ? `${relation.label}「${target.name}」` : ''
    })
    .filter(Boolean)
  if (relations.length) lines.push(`关系：${relations.join('；')}。`)
  if (entity.tags.length) lines.push(`关键词：${entity.tags.join('、')}。`)
  return lines.join('\n')
}

function useLocalCopy() {
  const entity = draft.value
  if (!entity) return
  copyText.value = localCopyText(entity)
  drafts.set(projectPath.value, entity.id, { copy: copyText.value })
  copyError.value = ''
}

function cacheCopyDraft() {
  const entity = draft.value
  if (entity) drafts.set(projectPath.value, entity.id, { copy: copyText.value })
}

/** Opens the draft; an existing draft is shown as-is until 重新生成. */
function openCopyDraft() {
  const entity = draft.value
  if (!entity) return
  copyOpen.value = true
  copyNotice.value = ''
  copyError.value = ''
  const cached = drafts.get(projectPath.value, entity.id)?.copy
  if (cached !== undefined) {
    copyText.value = cached
    copyLoading.value = false
    return
  }
  void regenerateCopy()
}

/** Drafts the copy with the text model, keeping the previous draft on failure. */
async function regenerateCopy() {
  const entity = draft.value
  if (!entity) return
  const previous = copyText.value
  copyNotice.value = ''
  copyError.value = ''
  copyLoading.value = true
  const { result, error: aiError } = await store.generateText(
    {
      system: COPYWRITER_SYSTEM,
      user: ['请为下面这条设定写一段文案：', ...entityBriefLines(entity)].join('\n'),
      maxTokens: 2000,
      temperature: 0.8,
    },
    { silent: true },
  )
  copyLoading.value = false
  const text = result?.text.trim() ?? ''
  if (text) {
    copyText.value = text
    drafts.set(projectPath.value, entity.id, { copy: text })
  } else {
    copyText.value = previous
    const reason = aiError
      ? [aiError.message, aiError.detail].filter(Boolean).join(' · ')
      : '模型没有返回正文'
    copyError.value = `AI 文案生成失败：${reason}。可以先用本地档案草稿。`
  }
}

async function copyDraftText() {
  try {
    await navigator.clipboard.writeText(copyText.value)
    copyNotice.value = '已复制'
  } catch {
    copyNotice.value = '无法写入剪贴板'
  }
}

function insertCopyIntoBody() {
  const entity = draft.value
  if (!entity) return
  const text = copyText.value.trim()
  if (!text) return
  // The draft replaces the body; it is not appended to the previous text.
  entity.body = text
  copyOpen.value = false
}

function addField() {
  draft.value?.fields.push({ key: '', value: '' })
}

function removeField(index: number) {
  draft.value?.fields.splice(index, 1)
}

function addRelation() {
  draft.value?.relations.push({ entityId: '', label: '' })
}

function removeRelation(index: number) {
  draft.value?.relations.splice(index, 1)
}

function addAsset(id: string) {
  if (!draft.value || !id || draft.value.assetIds.includes(id)) return
  draft.value.assetIds.push(id)
  pickedAsset.value = ''
}

function removeAsset(id: string) {
  if (!draft.value) return
  draft.value.assetIds = draft.value.assetIds.filter((item) => item !== id)
}

function addMap(id: string) {
  if (!draft.value || !id || draft.value.mapIds.includes(id)) return
  draft.value.mapIds.push(id)
  pickedMap.value = ''
}

function removeMap(id: string) {
  if (!draft.value) return
  draft.value.mapIds = draft.value.mapIds.filter((item) => item !== id)
}

const relationOptions = computed(() => {
  if (!draft.value) return []
  return ENTITY_KINDS.map((kind) => ({
    kind,
    label: KIND_META[kind].label,
    options: store.entities.filter((entity) => entity.id !== draft.value?.id && entity.kind === kind),
  })).filter((group) => group.options.length > 0)
})

const missingRelationTargets = computed(() => {
  if (!draft.value) return new Set<string>()
  const known = new Set(store.entities.map((entity) => entity.id))
  return new Set(
    draft.value.relations.map((relation) => relation.entityId).filter((id) => id && !known.has(id)),
  )
})

function relationTargetName(id: string): string {
  return store.entities.find((entity) => entity.id === id)?.name ?? '已删除的设定'
}

const linkedAssets = computed(() => {
  if (!draft.value) return []
  return draft.value.assetIds.map((id) => ({
    id,
    asset: store.assets.find((asset) => asset.id === id) ?? null,
  }))
})

const availableAssets = computed(() => {
  const linked = new Set(draft.value?.assetIds ?? [])
  return store.assets.filter((asset) => !linked.has(asset.id))
})

const linkedMaps = computed(() => {
  if (!draft.value) return []
  return draft.value.mapIds.map((id) => ({
    id,
    map: store.maps.find((map) => map.id === id) ?? null,
  }))
})

const availableMaps = computed(() => {
  const linked = new Set(draft.value?.mapIds ?? [])
  return store.maps.filter((map) => !linked.has(map.id))
})

const backlinks = computed(() => {
  const current = draft.value
  if (!current) return []
  return store.entities
    .filter((entity) => entity.id !== current.id)
    .map((entity) => ({
      entity,
      labels: entity.relations
        .filter((relation) => relation.entityId === current.id)
        .map((relation) => relation.label),
    }))
    .filter((link) => link.labels.length > 0)
})

watch(
  draft,
  () => {
    if (!loadingDraft && draft.value) dirty.value = true
  },
  { deep: true },
)
watch([aliasInput, tagInput], () => {
  if (!loadingDraft && draft.value) dirty.value = true
})

onMounted(() => {
  store.setLeaveGuard(continueAfterUnsaved)
  const id = store.consumeFocus('entity')
  const target = id ? store.entities.find((entity) => entity.id === id) : store.entities[0]
  if (target) loadDraft(target)
})

// The command palette can name an entry while this screen is already open.
watch(
  () => store.focusRequest,
  () => {
    const id = store.consumeFocus('entity')
    if (!id) return
    const target = store.entities.find((entity) => entity.id === id)
    if (target) void selectEntity(target)
  },
)

onBeforeUnmount(() => {
  store.setLeaveGuard(null)
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-baseline gap-3">
        <h2 class="text-lg font-semibold tracking-tight text-ink">设定</h2>
        <span class="font-mono text-[11px] text-faint">{{ store.entities.length }} 条</span>
      </div>
      <button type="button" class="pb-btn pb-btn-primary h-9 py-1" @click="createEntity()">
        <Plus :size="15" />
        新建设定
      </button>
    </div>

    <div class="grid grid-cols-1 items-start gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
      <aside class="pb-card overflow-hidden">
        <div class="space-y-2.5 border-b border-line-soft p-3">
          <div class="relative">
            <Search :size="13" class="absolute top-1/2 left-2.5 -translate-y-1/2 text-faint" />
            <input
              v-model="query"
              class="pb-input h-8 py-1 pl-8 text-xs"
              placeholder="搜索名称、别名或正文"
            />
          </div>
          <div class="flex flex-wrap gap-1">
            <button
              type="button"
              class="flex items-center gap-1 rounded-md px-1.5 py-1 text-[11px] transition-colors"
              :class="kindFilter === 'all' ? 'bg-panel-3 text-ink' : 'text-faint hover:text-ink'"
              @click="setKind('all')"
            >
              全部
              <span class="font-mono text-[10px] text-faint">{{ store.entities.length }}</span>
            </button>
            <button
              v-for="option in kindOptions"
              :key="option.kind"
              type="button"
              class="flex items-center gap-1 rounded-md px-1.5 py-1 text-[11px] transition-colors"
              :class="kindFilter === option.kind ? 'bg-panel-3 text-ink' : 'text-faint hover:text-ink'"
              @click="setKind(option.kind)"
            >
              <component :is="option.icon" :size="11" />
              {{ option.label }}
              <span class="font-mono text-[10px] text-faint">{{ option.count }}</span>
            </button>
          </div>
        </div>

        <div class="max-h-[calc(100vh-19rem)] min-h-[240px] overflow-y-auto p-1.5">
          <button
            v-for="entity in filtered"
            :key="entity.id"
            type="button"
            class="mb-0.5 flex w-full flex-col rounded-lg px-2.5 py-2 text-left transition-colors"
            :class="selectedId === entity.id ? 'bg-accent-soft' : 'hover:bg-panel-2'"
            @click="selectEntity(entity)"
          >
            <div class="flex w-full items-center gap-1.5">
              <component
                :is="KIND_META[entity.kind].icon"
                :size="12"
                class="shrink-0"
                :class="selectedId === entity.id ? 'text-accent' : 'text-faint'"
              />
              <span class="min-w-0 flex-1 truncate text-xs font-medium text-ink">
                {{ entity.name }}
              </span>
              <span class="shrink-0 text-[10px]" :class="STATUS_META[entity.status].class">
                {{ STATUS_META[entity.status].label }}
              </span>
            </div>
            <p v-if="entity.summary" class="mt-1 line-clamp-2 pl-[18px] text-[10px] leading-relaxed text-faint">
              {{ entity.summary }}
            </p>
            <div
              v-if="entity.aliases.length || entity.tags.length"
              class="mt-1 flex flex-wrap gap-x-2 pl-[18px] text-[10px] text-faint"
            >
              <span v-for="alias in entity.aliases.slice(0, 2)" :key="alias">{{ alias }}</span>
              <span v-for="tag in entity.tags.slice(0, 2)" :key="tag">#{{ tag }}</span>
            </div>
          </button>
          <p v-if="filtered.length === 0" class="px-3 py-10 text-center text-[11px] text-faint">
            {{ store.entities.length ? '没有匹配的设定' : '还没有设定' }}
          </p>
        </div>
      </aside>

      <section v-if="draft" class="pb-card min-h-[520px] overflow-hidden">
        <header class="sticky top-0 z-10 flex items-start gap-3 border-b border-line-soft bg-panel px-5 py-3.5">
          <div class="min-w-0 flex-1">
            <input
              ref="nameRef"
              v-model="draft.name"
              class="w-full rounded-md bg-transparent text-lg font-semibold tracking-tight text-ink outline-none placeholder:text-faint focus:bg-panel-2/60"
              placeholder="设定名称"
              aria-label="设定名称"
            />
            <div class="mt-1.5 flex flex-wrap items-center gap-2">
              <select v-model="draft.kind" class="pb-input h-7 w-24 py-0 text-[11px]">
                <option v-for="kind in ENTITY_KINDS" :key="kind" :value="kind">
                  {{ kindLabel(kind) }}
                </option>
              </select>
              <select v-model="draft.status" class="pb-input h-7 w-24 py-0 text-[11px]">
                <option v-for="status in ENTITY_STATUSES" :key="status" :value="status">
                  {{ STATUS_META[status].label }}
                </option>
              </select>
              <span
                class="text-[10px]"
                :class="saveState === 'error' ? 'text-red' : dirty ? 'text-amber' : 'text-faint'"
                :title="dirty ? '改动尚未保存' : '已写入 entities 目录'"
              >
                {{
                  saveState === 'saving'
                    ? '保存中…'
                    : saveState === 'error'
                      ? '保存失败'
                      : dirty
                        ? '未保存'
                        : saveState === 'saved'
                          ? '已保存'
                          : ''
                }}
              </span>
            </div>
          </div>
          <button
            type="button"
            class="pb-btn h-7 shrink-0 px-2 py-0 text-[11px] whitespace-nowrap"
            :class="dirty ? 'pb-btn-primary' : ''"
            :disabled="!dirty || saveState === 'saving'"
            title="保存这条设定"
            @click="saveNow()"
          >
            <Loader2 v-if="saveState === 'saving'" :size="13" class="animate-spin" />
            <Save v-else :size="13" />
            保存
          </button>
          <button
            type="button"
            class="pb-btn h-7 shrink-0 px-2 py-0 text-[11px] whitespace-nowrap"
            title="先确认或固定提示词，再打开生成台"
            @click="openAssetPrompt()"
          >
            <Sparkles :size="13" />
            生成素材
          </button>
          <button
            type="button"
            class="pb-btn h-7 shrink-0 px-2 py-0 text-[11px] whitespace-nowrap"
            title="让 AI 依据设定档案写一段文案，可编辑后再插入正文"
            @click="openCopyDraft()"
          >
            <FileText :size="13" />
            AI 文案
          </button>
          <button
            type="button"
            class="flex size-7 shrink-0 items-center justify-center rounded-md text-faint transition-colors hover:bg-red/10 hover:text-red"
            title="删除设定"
            @click="removeEntity()"
          >
            <Trash2 :size="14" />
          </button>
        </header>

        <div class="px-5 py-4">
          <label class="pb-label" for="lore-summary">一句话</label>
          <input
            id="lore-summary"
            v-model="draft.summary"
            class="pb-input mt-1.5"
            placeholder="他是谁，或这里是什么地方"
          />
          <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label class="block">
              <span class="pb-label">别名</span>
              <input v-model="aliasInput" class="pb-input mt-1.5" placeholder="用顿号或逗号分隔" />
            </label>
            <label class="block">
              <span class="pb-label">标签</span>
              <input v-model="tagInput" class="pb-input mt-1.5" placeholder="主角、城镇、主线" />
            </label>
          </div>
        </div>

        <section class="border-t border-line-soft px-5 py-4">
          <div class="flex items-center justify-between">
            <span class="pb-label">正文</span>
            <span class="text-[10px] text-faint">空行分段，改完点「保存」</span>
          </div>
          <textarea
            v-model="draft.body"
            rows="12"
            aria-label="设定正文"
            class="pb-input mt-1.5 resize-y leading-relaxed"
            placeholder="写下背景、动机、外貌、口癖，或任何之后要用的细节。"
          />
        </section>

        <section class="border-t border-line-soft px-5 py-4">
          <div class="flex items-center justify-between">
            <span class="pb-label">档案</span>
            <button type="button" class="pb-btn pb-btn-ghost h-7 px-2 py-0 text-[11px]" @click="addField()">
              <Plus :size="12" />
              添加字段
            </button>
          </div>
          <div v-if="draft.fields.length" class="mt-2 space-y-1.5">
            <div v-for="(field, index) in draft.fields" :key="index" class="flex items-center gap-2">
              <input v-model="field.key" class="pb-input h-8 w-32 py-1 text-xs" placeholder="字段名" />
              <input v-model="field.value" class="pb-input h-8 flex-1 py-1 text-xs" placeholder="值" />
              <button
                type="button"
                class="flex size-7 shrink-0 items-center justify-center rounded-md text-faint hover:text-ink"
                title="移除字段"
                @click="removeField(index)"
              >
                <X :size="13" />
              </button>
            </div>
          </div>
          <p v-else class="mt-2 text-[11px] text-faint">
            年龄、所属、口头禅。表格化的信息放这里，别塞进正文。
          </p>
        </section>

        <section class="border-t border-line-soft px-5 py-4">
          <div class="flex items-center justify-between">
            <span class="pb-label">关系</span>
            <button type="button" class="pb-btn pb-btn-ghost h-7 px-2 py-0 text-[11px]" @click="addRelation()">
              <Plus :size="12" />
              添加关系
            </button>
          </div>
          <div v-if="draft.relations.length" class="mt-2 space-y-1.5">
            <div v-for="(relation, index) in draft.relations" :key="index" class="flex items-center gap-2">
              <input
                v-model="relation.label"
                class="pb-input h-8 w-28 py-1 text-xs"
                placeholder="关系，如 父亲"
              />
              <select v-model="relation.entityId" class="pb-input h-8 flex-1 py-1 text-xs">
                <option value="">选择设定…</option>
                <optgroup v-for="group in relationOptions" :key="group.kind" :label="group.label">
                  <option v-for="option in group.options" :key="option.id" :value="option.id">
                    {{ option.name }}
                  </option>
                </optgroup>
                <option
                  v-if="
                    relation.entityId &&
                    missingRelationTargets.has(relation.entityId)
                  "
                  :value="relation.entityId"
                >
                  已删除的设定
                </option>
              </select>
              <button
                type="button"
                class="flex size-7 shrink-0 items-center justify-center rounded-md text-faint hover:text-ink"
                title="移除关系"
                @click="removeRelation(index)"
              >
                <X :size="13" />
              </button>
            </div>
          </div>
          <p v-else class="mt-2 text-[11px] text-faint">
            关系是有方向的：在这里写「效忠于」，对方的名字旁边会显示被引用。
          </p>

          <div v-if="backlinks.length" class="mt-3 rounded-lg border border-line-soft bg-panel-2/40 p-2">
            <div class="px-1 pb-1 text-[10px] text-faint">被这些设定引用</div>
            <button
              v-for="link in backlinks"
              :key="link.entity.id"
              type="button"
              class="flex w-full items-center gap-2 rounded-md px-1.5 py-1.5 text-left transition-colors hover:bg-panel-3"
              @click="selectEntity(link.entity)"
            >
              <component :is="KIND_META[link.entity.kind].icon" :size="11" class="shrink-0 text-faint" />
              <span class="truncate text-[11px] text-ink">{{ link.entity.name }}</span>
              <span class="truncate text-[10px] text-faint">{{ link.labels.join('、') }}</span>
              <ArrowUpRight :size="11" class="ml-auto shrink-0 text-faint" />
            </button>
          </div>
        </section>

        <section class="border-t border-line-soft px-5 py-4">
          <div class="grid grid-cols-1 gap-5 xl:grid-cols-2">
            <div>
              <span class="pb-label">素材</span>
              <div v-if="linkedAssets.length" class="mt-2 space-y-1.5">
                <div
                  v-for="link in linkedAssets"
                  :key="link.id"
                  class="flex items-center gap-2 rounded-lg border border-line-soft bg-panel-2/40 p-1.5"
                >
                  <AssetThumb v-if="link.asset" :asset="link.asset" :size="28" />
                  <span v-else class="flex size-7 items-center justify-center rounded-md border border-line text-faint">
                    <ImageIcon :size="12" />
                  </span>
                  <span class="min-w-0 flex-1 truncate text-[11px]" :class="link.asset ? 'text-ink' : 'text-faint'">
                    {{ link.asset?.name ?? '素材已删除' }}
                  </span>
                  <button
                    type="button"
                    class="flex size-6 shrink-0 items-center justify-center rounded-md text-faint hover:text-ink"
                    title="移除素材"
                    @click="removeAsset(link.id)"
                  >
                    <X :size="12" />
                  </button>
                </div>
              </div>
              <select
                :value="pickedAsset"
                class="pb-input mt-2 h-8 py-1 text-xs"
                aria-label="添加素材"
                @change="addAsset(($event.target as HTMLSelectElement).value)"
              >
                <option value="">{{ availableAssets.length ? '添加素材…' : '素材都已关联' }}</option>
                <option v-for="asset in availableAssets" :key="asset.id" :value="asset.id">
                  {{ asset.name }}
                </option>
              </select>
            </div>

            <div>
              <span class="pb-label">地图</span>
              <div v-if="linkedMaps.length" class="mt-2 space-y-1.5">
                <div
                  v-for="link in linkedMaps"
                  :key="link.id"
                  class="flex items-center gap-2 rounded-lg border border-line-soft bg-panel-2/40 px-2 py-1.5"
                >
                  <MapIcon :size="12" class="shrink-0 text-faint" />
                  <span class="min-w-0 flex-1 truncate text-[11px]" :class="link.map ? 'text-ink' : 'text-faint'">
                    {{ link.map?.name ?? '地图已删除' }}
                  </span>
                  <span v-if="link.map" class="font-mono text-[10px] text-faint">
                    {{ link.map.width }}×{{ link.map.height }}
                  </span>
                  <button
                    type="button"
                    class="flex size-6 shrink-0 items-center justify-center rounded-md text-faint hover:text-ink"
                    title="移除地图"
                    @click="removeMap(link.id)"
                  >
                    <X :size="12" />
                  </button>
                </div>
              </div>
              <select
                :value="pickedMap"
                class="pb-input mt-2 h-8 py-1 text-xs"
                aria-label="添加地图"
                @change="addMap(($event.target as HTMLSelectElement).value)"
              >
                <option value="">{{ availableMaps.length ? '添加地图…' : '地图都已关联' }}</option>
                <option v-for="map in availableMaps" :key="map.id" :value="map.id">
                  {{ map.name }}
                </option>
              </select>
            </div>
          </div>
        </section>
      </section>

      <EmptyState
        v-else
        title="设定库是空的"
        hint="角色、地点、势力、物品、事件和术语都放这里，再用关系把它们串起来。"
      >
        <template #icon><BookOpen :size="24" /></template>
        <template #action>
          <button type="button" class="pb-btn pb-btn-primary" @click="createEntity()">
            <Plus :size="15" />
            新建第一条设定
          </button>
        </template>
      </EmptyState>
    </div>
  </div>

  <AppModal
    v-model:open="unsavedOpen"
    title="这条设定还没有保存"
    description="离开前要保存修改吗？"
    @update:open="onUnsavedModal"
  >
    <p class="text-xs leading-relaxed text-muted">
      「{{ draft?.name || '这条设定' }}」有未保存的改动。保存会写入 entities/ 目录，丢弃会恢复上次保存的内容。
    </p>
    <template #footer>
      <button type="button" class="pb-btn" :disabled="unsavedBusy" @click="answerUnsaved('cancel')">
        取消
      </button>
      <button type="button" class="pb-btn" :disabled="unsavedBusy" @click="answerUnsaved('discard')">
        丢弃修改
      </button>
      <button
        type="button"
        class="pb-btn pb-btn-primary"
        :disabled="unsavedBusy"
        @click="answerUnsaved('save')"
      >
        <Loader2 v-if="unsavedBusy" :size="13" class="animate-spin" />
        保存并继续
      </button>
    </template>
  </AppModal>

  <AppModal
    v-model:open="promptOpen"
    title="生成素材 · 提示词确认"
    description="先确认这条提示词，再打开生成台；固定后每次生成都复用它。"
  >
    <div v-if="promptLoading" class="flex items-center gap-2 py-12 text-xs text-muted">
      <Loader2 :size="15" class="animate-spin" />
      AI 正在起稿提示词…
    </div>
    <div v-else class="space-y-3">
      <div class="flex flex-wrap items-center gap-1.5">
        <span v-if="promptFixed" class="pb-chip border-accent/40 text-accent">角色固定提示词</span>
        <span v-if="draft" class="pb-chip font-mono text-[10px]">{{ kindLabel(draft.kind) }}</span>
        <span v-if="store.session" class="pb-chip font-mono text-[10px]">
          {{ store.session.project.settings.tileSize }}px
        </span>
      </div>
      <textarea
        v-model="promptText"
        class="pb-input min-h-44 py-2 text-xs leading-relaxed"
        aria-label="素材提示词"
        placeholder="描述主体、动作、构图、材质、配色、光影…"
        @input="cacheAssetPrompt"
      ></textarea>
      <p class="truncate text-[10px] text-faint" :title="store.styleGuideText()">
        项目风格：{{ store.styleGuideText() || '未设置' }}
      </p>
      <div v-if="promptError" class="rounded-lg border border-amber/30 bg-amber/10 px-3 py-2 text-[11px] text-amber">
        {{ promptError }}
      </div>
      <div v-if="promptNotice" class="rounded-lg border border-green/25 bg-green/10 px-3 py-2 text-[11px] text-green">
        {{ promptNotice }}
      </div>
    </div>
    <template #footer>
      <button type="button" class="pb-btn" @click="promptOpen = false">取消</button>
      <button
        type="button"
        class="pb-btn"
        :disabled="promptLoading"
        @click="regenerateAssetPrompt()"
      >
        重新生成
      </button>
      <button
        type="button"
        class="pb-btn"
        :disabled="promptLoading || !promptText.trim()"
        title="把这条提示词写进角色，之后不再随机起稿"
        @click="fixAssetPrompt()"
      >
        设为固定提示词
      </button>
      <button
        type="button"
        class="pb-btn pb-btn-primary"
        :disabled="promptLoading || !promptText.trim()"
        @click="openWorkbenchFromPrompt()"
      >
        打开生成台
      </button>
    </template>
  </AppModal>

  <AppModal
    v-model:open="copyOpen"
    title="AI 文案草稿"
    description="由文本模型根据设定档案起草；可编辑、重新生成，或改用本地档案草稿。"
  >
    <div v-if="copyLoading" class="flex items-center gap-2 py-12 text-xs text-muted">
      <Loader2 :size="15" class="animate-spin" />
      正在生成 AI 文案…
    </div>
    <div v-else class="space-y-2">
      <textarea
        v-model="copyText"
        class="pb-input min-h-52 py-2 text-xs leading-relaxed"
        aria-label="设定文案草稿"
        @input="cacheCopyDraft"
      ></textarea>
      <div
        v-if="copyError"
        class="flex flex-wrap items-center gap-2 rounded-lg border border-amber/30 bg-amber/10 px-3 py-2 text-[11px] text-amber"
      >
        <span class="min-w-0 flex-1">{{ copyError }}</span>
        <button type="button" class="pb-btn h-7 px-2 py-0 text-[11px]" @click="useLocalCopy()">
          使用本地草稿
        </button>
      </div>
    </div>
    <template #footer>
      <span v-if="copyNotice" class="mr-auto text-[11px] text-faint">{{ copyNotice }}</span>
      <button type="button" class="pb-btn" @click="copyOpen = false">取消</button>
      <template v-if="!copyLoading">
        <button type="button" class="pb-btn" :disabled="!copyText.trim()" @click="copyDraftText()">
          <Copy :size="13" />
          复制
        </button>
        <button type="button" class="pb-btn" @click="regenerateCopy()">重新生成</button>
        <button
          type="button"
          class="pb-btn pb-btn-primary"
          :disabled="!copyText.trim()"
          @click="insertCopyIntoBody()"
        >
          覆盖正文
        </button>
      </template>
    </template>
  </AppModal>
</template>