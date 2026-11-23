<script setup lang="ts">
import { Loader2, Plus, Trash2, Wand2, X } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ASSET_TYPES,
  PRIORITIES,
  QUEST_STATUSES,
  QUEST_TYPES,
  TILE_SIZES,
} from '../domain/types'
import type { AssetType, Priority, Project, Quest, QuestStatus, QuestType } from '../domain/types'
import { useAppStore } from '../stores/app'
import {
  applyBriefTemplate,
  briefNeedsTemplate,
  templateAcceptanceFor,
} from '../lib/briefTemplates'
import { fromDateInput, newID, nowISO, toDateInput } from '../lib/utils'
import AppModal from '../components/AppModal.vue'
import FieldBox from '../components/FieldBox.vue'

const props = defineProps<{
  open: boolean
  quest: Quest | null
  /** Column a brand-new task starts in; ignored when editing. */
  initialStatus?: QuestStatus
}>()

const emit = defineEmits<{ 'update:open': [boolean] }>()

const store = useAppStore()
const { t } = useI18n()

const draft = ref<Quest | null>(null)
const formError = ref<string | null>(null)
const confirmDelete = ref<boolean>(false)
const commentText = ref('')
/** What the template wrote last, so type/title edits can refresh untouched copy. */
const templateSeed = ref<{
  type: AssetType | ''
  title: string
  style: string
  prompt: string
} | null>(null)

const project = computed<Project | null>(() => store.session?.project ?? null)
const isEdit = computed(() => Boolean(props.quest))
const member = computed(() => project.value?.members[0] ?? null)

function cloneQuest(quest: Quest): Quest {
  // JSON round-trip strips Vue reactive proxies and keeps the nested structure plain.
  return JSON.parse(JSON.stringify(quest)) as Quest
}

function emptyQuest(project: Project, status: QuestStatus = 'backlog'): Quest {
  const now = nowISO()
  const quest: Quest = {
    schemaVersion: 1,
    id: newID(),
    title: '',
    type: 'asset',
    status,
    priority: 'medium',
    assignee: project.members[0]?.id,
    description: '',
    brief: {
      assetType: 'sprite',
      tileSize: project.settings.tileSize,
      paletteId: project.settings.defaultPaletteId,
      style: '',
      references: [],
      prompt: '',
    },
    acceptance: [],
    links: { assetIds: [], mapIds: [], urls: [] },
    comments: [],
    createdAt: now,
    updatedAt: now,
  }
  // A fresh asset quest starts from the sprite scaffold instead of a blank brief.
  const seeded = applyBriefTemplate({
    brief: quest.brief,
    acceptance: quest.acceptance,
    title: quest.title,
  })
  return { ...quest, brief: seeded.brief, acceptance: seeded.acceptance }
}

watch(
  [() => props.open, () => props.quest?.id, () => store.session?.path, () => props.initialStatus],
  ([open]) => {
    if (!open) return
    const current = project.value
    if (!current) return
    const next = props.quest ? cloneQuest(props.quest) : emptyQuest(current, props.initialStatus)
    draft.value = next
    templateSeed.value = props.quest
      ? null
      : {
          type: next.brief.assetType,
          title: next.title,
          style: next.brief.style,
          prompt: next.brief.prompt,
        }
    formError.value = null
    confirmDelete.value = false
    commentText.value = ''
  },
  { immediate: true },
)

function close() {
  emit('update:open', false)
}

function patch<K extends keyof Quest>(key: K, value: Quest[K]) {
  if (!draft.value) return
  draft.value = { ...draft.value, [key]: value }
}

function patchBrief<K extends keyof Quest['brief']>(key: K, value: Quest['brief'][K]) {
  if (!draft.value) return
  draft.value = { ...draft.value, brief: { ...draft.value.brief, [key]: value } }
}

function value(event: Event): string {
  return (event.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value
}

function setType(event: Event) {
  patch('type', value(event) as QuestType)
}

/** Keeps template copy in sync with the title until the user rewrites it. */
function setTitle(event: Event) {
  if (!draft.value) return
  patch('title', value(event))
  const current = draft.value
  const seed = templateSeed.value
  if (!current || !seed) return
  if (
    seed.type !== current.brief.assetType ||
    seed.style.trim() !== current.brief.style.trim() ||
    seed.prompt.trim() !== current.brief.prompt.trim()
  ) {
    return
  }
  const stale = new Set(templateAcceptanceFor(seed.type, seed.title, current.brief.tileSize))
  const acceptance = current.acceptance.filter((item) => !stale.has(item.trim()))
  const result = applyBriefTemplate({
    brief: current.brief,
    acceptance,
    title: current.title,
    overwrite: true,
  })
  patch('brief', result.brief)
  patch('acceptance', result.acceptance)
  templateSeed.value = {
    type: current.brief.assetType,
    title: current.title,
    style: result.brief.style,
    prompt: result.brief.prompt,
  }
}

function setStatus(event: Event) {
  patch('status', value(event) as QuestStatus)
}

function setPriority(event: Event) {
  patch('priority', value(event) as Priority)
}

function setAssignee(event: Event) {
  patch('assignee', value(event) || undefined)
}

function setStartAt(event: Event) {
  patch('startAt', fromDateInput(value(event)) || undefined)
}

function setDueAt(event: Event) {
  patch('dueAt', fromDateInput(value(event)) || undefined)
}

function setAssetType(event: Event) {
  const current = draft.value
  const nextType = value(event) as AssetType | ''
  if (!current) {
    patchBrief('assetType', nextType)
    return
  }
  const seed = templateSeed.value
  // Untouched template copy follows the type; anything the user rewrote stays.
  const untouched =
    seed !== null &&
    seed.type === current.brief.assetType &&
    seed.style.trim() === current.brief.style.trim() &&
    seed.prompt.trim() === current.brief.prompt.trim()

  patchBrief('assetType', nextType)
  if (untouched) {
    const stale = new Set(
      templateAcceptanceFor(current.brief.assetType, current.title, current.brief.tileSize),
    )
    const acceptance = current.acceptance.filter((item) => !stale.has(item.trim()))
    const result = applyBriefTemplate({
      brief: { ...current.brief, assetType: nextType },
      acceptance,
      title: current.title,
      overwrite: true,
    })
    patch('brief', result.brief)
    patch('acceptance', result.acceptance)
    templateSeed.value = {
      type: nextType,
      title: current.title,
      style: result.brief.style,
      prompt: result.brief.prompt,
    }
    return
  }
  autoFillTemplate()
}

/** Fills style/prompt/acceptance only when the brief is still blank. */
function autoFillTemplate() {
  const current = draft.value
  if (!current || !briefNeedsTemplate(current.brief)) return
  const result = applyBriefTemplate({
    brief: current.brief,
    acceptance: current.acceptance,
    title: current.title,
  })
  patch('brief', result.brief)
  patch('acceptance', result.acceptance)
  templateSeed.value = {
    type: current.brief.assetType,
    title: current.title,
    style: result.brief.style,
    prompt: result.brief.prompt,
  }
}

/** Explicit button: replace style/prompt with the template for this type. */
function applyTemplate() {
  const current = draft.value
  if (!current) return
  const result = applyBriefTemplate({
    brief: current.brief,
    acceptance: current.acceptance,
    title: current.title,
    overwrite: true,
  })
  patch('brief', result.brief)
  patch('acceptance', result.acceptance)
  templateSeed.value = {
    type: current.brief.assetType,
    title: current.title,
    style: result.brief.style,
    prompt: result.brief.prompt,
  }
}

function setBriefTile(event: Event) {
  patchBrief('tileSize', Number(value(event)))
}

function addAcceptance() {
  if (!draft.value) return
  patch('acceptance', [...draft.value.acceptance, ''])
}

function setAcceptance(index: number, text: string) {
  if (!draft.value) return
  patch(
    'acceptance',
    draft.value.acceptance.map((item, i) => (i === index ? text : item)),
  )
}

function removeAcceptance(index: number) {
  if (!draft.value) return
  patch(
    'acceptance',
    draft.value.acceptance.filter((_, i) => i !== index),
  )
}

function toggleAsset(assetId: string) {
  if (!draft.value) return
  const has = draft.value.links.assetIds.includes(assetId)
  patch('links', {
    ...draft.value.links,
    assetIds: has
      ? draft.value.links.assetIds.filter((id) => id !== assetId)
      : [...draft.value.links.assetIds, assetId],
  })
}

function addComment() {
  const text = commentText.value.trim()
  if (!draft.value || !text || !member.value) return
  patch('comments', [
    ...draft.value.comments,
    { at: nowISO(), author: member.value.id, text },
  ])
  commentText.value = ''
}

async function finalize(): Promise<Quest | null> {
  const current = draft.value
  if (!current) return null
  if (!current.title.trim()) {
    formError.value = '任务标题不能为空'
    return null
  }
  return store.saveQuest({
    ...current,
    title: current.title.trim(),
    acceptance: current.acceptance.map((item) => item.trim()).filter(Boolean),
  })
}

async function submit() {
  if (await finalize()) close()
}

/** Saves the quest, then opens the generate screen with its brief prefilled. */
async function generateFromQuest() {
  const current = draft.value
  if (!current) return
  if (!current.brief.assetType) {
    formError.value = '请先在素材 brief 里选择素材类型'
    return
  }
  const saved = await finalize()
  if (!saved) return
  close()
  store.openGenerateFromQuest(saved)
}

async function remove() {
  if (!props.quest) return
  const ok = await store.deleteQuest(props.quest.id)
  if (ok) close()
}
</script>

<template>
  <AppModal
    :open="props.open"
    wide
    :title="isEdit ? '编辑任务' : '新建任务'"
    description="任务可以关联素材草稿；保存后会写入 quests/ 目录，可直接提交到 git。"
    @update:open="emit('update:open', $event)"
  >
    <div v-if="draft && project" class="space-y-4">
      <FieldBox label="标题">
        <input
          class="pb-input"
          :value="draft.title"
          maxlength="200"
          autofocus
          placeholder="例如：杂货铺外观"
          @input="setTitle"
        />
      </FieldBox>

      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <FieldBox label="类型">
          <select class="pb-input" :value="draft.type" @change="setType">
            <option v-for="type in QUEST_TYPES" :key="type" :value="type">
              {{ t(`questType.${type}`) }}
            </option>
          </select>
        </FieldBox>
        <FieldBox label="状态">
          <select class="pb-input" :value="draft.status" @change="setStatus">
            <option v-for="status in QUEST_STATUSES" :key="status" :value="status">
              {{ t(`status.${status}`) }}
            </option>
          </select>
        </FieldBox>
        <FieldBox label="优先级">
          <select class="pb-input" :value="draft.priority" @change="setPriority">
            <option v-for="priority in PRIORITIES" :key="priority" :value="priority">
              {{ t(`priority.${priority}`) }}
            </option>
          </select>
        </FieldBox>
        <FieldBox label="负责人">
          <select class="pb-input" :value="draft.assignee ?? ''" @change="setAssignee">
            <option value="">未指派</option>
            <option v-for="item in project.members" :key="item.id" :value="item.id">
              {{ item.name }}
            </option>
          </select>
        </FieldBox>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <FieldBox label="开始日期">
          <input
            type="date"
            class="pb-input"
            :value="toDateInput(draft.startAt)"
            @input="setStartAt"
          />
        </FieldBox>
        <FieldBox label="截止日期">
          <input type="date" class="pb-input" :value="toDateInput(draft.dueAt)" @input="setDueAt" />
        </FieldBox>
      </div>

      <FieldBox label="描述">
        <textarea
          class="pb-input min-h-20 resize-y"
          :value="draft.description ?? ''"
          placeholder="这条任务要解决什么问题？"
          @input="patch('description', value($event))"
        />
      </FieldBox>

      <fieldset class="rounded-xl border border-line bg-panel-2/40 p-3">
        <legend class="px-1">
          <span class="flex items-center gap-2">
            <span class="text-xs font-medium text-accent">素材 brief</span>
            <button
              type="button"
              class="inline-flex items-center gap-1 text-[10px] text-muted hover:text-accent hover:underline"
              title="按素材类型填入风格、提示词与验收项（会覆盖已填的风格与提示词）"
              @click="applyTemplate()"
            >
              <Wand2 :size="11" />
              套用模板
            </button>
          </span>
        </legend>
        <div class="mt-2 grid grid-cols-2 gap-3 sm:grid-cols-3">
          <FieldBox label="素材类型">
            <select class="pb-input" :value="draft.brief.assetType" @change="setAssetType">
              <option value="">未指定</option>
              <option v-for="type in ASSET_TYPES" :key="type" :value="type">
                {{ t(`assetType.${type}`) }}
              </option>
            </select>
          </FieldBox>
          <FieldBox label="瓦片尺寸">
            <select class="pb-input" :value="draft.brief.tileSize" @change="setBriefTile">
              <option v-for="size in TILE_SIZES" :key="size" :value="size">{{ size }} px</option>
            </select>
          </FieldBox>
          <FieldBox label="调色板">
            <select
              class="pb-input"
              :value="draft.brief.paletteId"
              @change="patchBrief('paletteId', value($event))"
            >
              <option v-for="palette in store.palettes" :key="palette.id" :value="palette.id">
                {{ palette.name }}
              </option>
            </select>
          </FieldBox>
        </div>
        <div class="mt-3 space-y-3">
          <FieldBox label="风格要求">
            <input
              class="pb-input"
              :value="draft.brief.style"
              placeholder="3/4 俯视、木结构、暖色招牌、1px 深色描边"
              @input="patchBrief('style', value($event))"
            />
          </FieldBox>
          <FieldBox label="提示词" hint="生成时会自动叠加项目风格指南">
            <textarea
              class="pb-input min-h-20 resize-y"
              :value="draft.brief.prompt"
              placeholder="描述要生成的素材内容…"
              @input="patchBrief('prompt', value($event))"
            />
          </FieldBox>
        </div>
      </fieldset>

      <div>
        <div class="flex items-center justify-between">
          <span class="pb-label">验收清单</span>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-[11px] text-accent hover:underline"
            @click="addAcceptance()"
          >
            <Plus :size="12" />
            添加一项
          </button>
        </div>
        <div class="mt-2 space-y-2">
          <div v-for="(item, index) in draft.acceptance" :key="index" class="flex items-center gap-2">
            <span class="font-mono text-[10px] text-faint">{{ index + 1 }}</span>
            <input
              class="pb-input py-1.5"
              :value="item"
              placeholder="可验证的完成标准"
              @input="setAcceptance(index, value($event))"
            />
            <button
              type="button"
              class="pb-icon-btn size-8 shrink-0"
              aria-label="删除验收项"
              @click="removeAcceptance(index)"
            >
              <X :size="13" />
            </button>
          </div>
          <p v-if="draft.acceptance.length === 0" class="text-[11px] text-faint">
            还没有验收项，建议至少写一条可检查的标准。
          </p>
        </div>
      </div>

      <div>
        <span class="pb-label">关联素材</span>
        <div
          class="mt-2 grid max-h-40 grid-cols-1 gap-1.5 overflow-y-auto rounded-lg border border-line bg-canvas/60 p-2 sm:grid-cols-2"
        >
          <label
            v-for="asset in store.assets"
            :key="asset.id"
            class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs text-muted transition-colors hover:bg-panel-2"
          >
            <input
              type="checkbox"
              class="accent-[#f2b84b]"
              :checked="draft.links.assetIds.includes(asset.id)"
              @change="toggleAsset(asset.id)"
            />
            <span class="truncate text-ink">{{ asset.name }}</span>
            <span class="ml-auto shrink-0 text-[10px] text-faint">
              {{ t(`assetType.${asset.type}`) }}
            </span>
          </label>
          <p
            v-if="store.assets.length === 0"
            class="col-span-full px-2 py-3 text-center text-[11px] text-faint"
          >
            项目里还没有素材
          </p>
        </div>
      </div>

      <div>
        <span class="pb-label">评论</span>
        <div class="mt-2 space-y-2">
          <div
            v-for="(comment, index) in draft.comments"
            :key="`${comment.at}-${index}`"
            class="rounded-lg border border-line bg-panel-2/50 px-3 py-2"
          >
            <div class="flex items-center gap-2 text-[10px] text-faint">
              <span class="font-medium text-muted">
                {{ project.members.find((item) => item.id === comment.author)?.name ?? comment.author }}
              </span>
              <span>{{ comment.at.slice(0, 16).replace('T', ' ') }}</span>
            </div>
            <p class="mt-1 text-xs leading-relaxed text-ink">{{ comment.text }}</p>
          </div>
          <div class="flex items-end gap-2">
            <textarea
              v-model="commentText"
              class="pb-input min-h-9 resize-y py-1.5 text-xs"
              rows="1"
              placeholder="写一条评论…"
            />
            <button
              type="button"
              class="pb-btn shrink-0 px-3 py-1.5 text-xs"
              :disabled="!commentText.trim() || !member"
              @click="addComment()"
            >
              添加
            </button>
          </div>
        </div>
      </div>

      <p v-if="formError" class="rounded-lg border border-red/25 bg-red/10 px-3 py-2 text-xs text-red">
        {{ formError }}
      </p>
    </div>

    <template #footer>
      <div v-if="isEdit" class="mr-auto flex items-center gap-2">
        <template v-if="confirmDelete">
          <span class="text-[11px] text-red">删除后移入项目回收站？</span>
          <button
            type="button"
            class="pb-btn pb-btn-danger px-2.5 py-1 text-xs"
            :disabled="store.busy"
            @click="remove()"
          >
            确认删除
          </button>
          <button
            type="button"
            class="pb-btn pb-btn-ghost px-2.5 py-1 text-xs"
            @click="confirmDelete = false"
          >
            取消
          </button>
        </template>
        <button
          v-else
          type="button"
          class="pb-btn pb-btn-ghost px-2.5 py-1 text-xs text-red"
          @click="confirmDelete = true"
        >
          <Trash2 :size="13" />
          删除任务
        </button>
      </div>
      <button type="button" class="pb-btn" @click="close()">取消</button>
      <button
        type="button"
        class="pb-btn"
        :disabled="store.busy || !draft?.brief.assetType"
        :title="
          draft?.brief.assetType
            ? '保存任务并带着 brief 打开生成台'
            : '先选择素材 brief 的素材类型'
        "
        @click="generateFromQuest()"
      >
        <Wand2 :size="14" />
        从任务生成草稿
      </button>
      <button type="button" class="pb-btn pb-btn-primary" :disabled="store.busy" @click="submit()">
        <Loader2 v-if="store.busy" :size="15" class="animate-spin" />
        保存任务
      </button>
    </template>
  </AppModal>
</template>