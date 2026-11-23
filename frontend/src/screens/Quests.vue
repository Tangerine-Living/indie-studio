<script setup lang="ts">
import {
  CalendarClock,
  GripVertical,
  LayoutGrid,
  List,
  ListChecks,
  MessageSquare,
  Paperclip,
  Plus,
  Search,
} from 'lucide-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { QUEST_STATUSES, QUEST_TYPES } from '../domain/types'
import type { Quest, QuestStatus, QuestType } from '../domain/types'
import { useAppStore } from '../stores/app'
import { PRIORITY_DOT_CLASS, QUEST_STATUS_CLASS } from '../lib/badges'
import { cx, daysUntil, formatDate, relativeTime } from '../lib/utils'
import EmptyState from '../components/EmptyState.vue'
import QuestDialog from './QuestDialog.vue'

const store = useAppStore()
const { t } = useI18n()

const view = ref<'board' | 'list'>('board')
const search = ref('')
const typeFilter = ref<QuestType | 'all'>('all')
const dialogOpen = ref(false)
const selected = ref<Quest | null>(null)
const newStatus = ref<QuestStatus>('backlog')
const dragOver = ref<QuestStatus | null>(null)

const COLUMN_ACCENT: Record<QuestStatus, string> = {
  backlog: 'bg-faint',
  todo: 'bg-blue',
  doing: 'bg-accent',
  review: 'bg-[#c99cff]',
  done: 'bg-green',
}

const project = computed(() => store.session?.project ?? null)
const done = computed(() => store.quests.filter((quest) => quest.status === 'done').length)

const filtered = computed(() => {
  const needle = search.value.trim().toLowerCase()
  return store.quests.filter((quest) => {
    if (typeFilter.value !== 'all' && quest.type !== typeFilter.value) return false
    if (!needle) return true
    return (
      quest.title.toLowerCase().includes(needle) ||
      (quest.description ?? '').toLowerCase().includes(needle) ||
      quest.brief.prompt.toLowerCase().includes(needle)
    )
  })
})

const columns = computed(() =>
  QUEST_STATUSES.map((status) => ({
    status,
    items: filtered.value.filter((quest) => quest.status === status),
  })),
)

function openNew(status: QuestStatus = 'backlog') {
  newStatus.value = status
  selected.value = null
  dialogOpen.value = true
}

function openEdit(quest: Quest) {
  selected.value = quest
  dialogOpen.value = true
}

onMounted(() => {
  const id = store.consumeFocus('quest')
  const quest = id ? store.quests.find((item) => item.id === id) : null
  if (quest) openEdit(quest)
})

// The command palette can name a quest while this screen is already open.
watch(
  () => store.focusRequest,
  () => {
    const id = store.consumeFocus('quest')
    if (!id) return
    const quest = store.quests.find((item) => item.id === id)
    if (quest) openEdit(quest)
  },
)

function onDialogOpen(next: boolean) {
  dialogOpen.value = next
  if (!next) selected.value = null
}

function assigneeName(quest: Quest): string {
  if (!quest.assignee) return '未指派'
  return project.value?.members.find((member) => member.id === quest.assignee)?.name ?? quest.assignee
}

function dueClass(quest: Quest): string {
  const due = daysUntil(quest.dueAt)
  if (due !== null && due < 0) return 'text-red'
  if (due !== null && due <= 7) return 'text-accent'
  return ''
}

function onDragOver(status: QuestStatus, event: DragEvent) {
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
  dragOver.value = status
}

function onDragLeave(status: QuestStatus) {
  if (dragOver.value === status) dragOver.value = null
}

function onDrop(status: QuestStatus, event: DragEvent) {
  event.preventDefault()
  const id = event.dataTransfer?.getData('text/plain') ?? ''
  dragOver.value = null
  if (id) void store.moveQuest(id, status)
}

function onDragStart(quest: Quest, event: DragEvent) {
  event.dataTransfer?.setData('text/plain', quest.id)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}
</script>

<template>
  <div v-if="project" class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-baseline gap-3">
        <h2 class="text-lg font-semibold tracking-tight text-ink">任务</h2>
        <span class="font-mono text-[11px] text-faint">完成 {{ done }}/{{ store.quests.length }}</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="relative">
          <Search :size="13" class="absolute top-1/2 left-2.5 -translate-y-1/2 text-faint" />
          <input
            v-model="search"
            class="pb-input h-9 w-52 py-1 pl-8 text-xs"
            placeholder="搜索标题或提示词"
            aria-label="搜索任务"
          />
        </div>
        <select v-model="typeFilter" class="pb-input h-9 w-28 py-1 text-xs" aria-label="按类型筛选任务">
          <option value="all">全部类型</option>
          <option v-for="type in QUEST_TYPES" :key="type" :value="type">
            {{ t(`questType.${type}`) }}
          </option>
        </select>
        <div class="flex rounded-lg border border-line bg-panel-2 p-0.5">
          <button
            type="button"
            class="flex size-8 items-center justify-center rounded-md transition-colors"
            :class="view === 'board' ? 'bg-panel-3 text-accent' : 'text-faint hover:text-ink'"
            title="看板"
            @click="view = 'board'"
          >
            <LayoutGrid :size="14" />
          </button>
          <button
            type="button"
            class="flex size-8 items-center justify-center rounded-md transition-colors"
            :class="view === 'list' ? 'bg-panel-3 text-accent' : 'text-faint hover:text-ink'"
            title="列表"
            @click="view = 'list'"
          >
            <List :size="14" />
          </button>
        </div>
        <button type="button" class="pb-btn pb-btn-primary h-9 py-1" @click="openNew()">
          <Plus :size="15" />
          新建任务
        </button>
      </div>
    </div>

    <EmptyState
      v-if="store.quests.length === 0"
      title="还没有任务"
      hint="写清 brief 与验收标准，之后可以直接生成草稿。"
    >
      <template #icon><ListChecks :size="24" /></template>
      <template #action>
        <button type="button" class="pb-btn pb-btn-primary" @click="openNew()">
          <Plus :size="15" />
          创建第一条任务
        </button>
      </template>
    </EmptyState>

    <div v-else-if="view === 'board'" class="flex gap-2.5 overflow-x-auto pb-2">
      <section
        v-for="column in columns"
        :key="column.status"
        class="flex min-w-[176px] flex-1 basis-0 flex-col rounded-xl border bg-panel/50 transition-colors"
        :class="dragOver === column.status ? 'border-accent/60 bg-accent/[0.04]' : 'border-line'"
        @dragover="onDragOver(column.status, $event)"
        @dragleave="onDragLeave(column.status)"
        @drop="onDrop(column.status, $event)"
      >
        <header class="flex items-center gap-1.5 border-b border-line-soft px-2.5 py-2">
          <span class="size-2 shrink-0 rounded-[2px]" :class="COLUMN_ACCENT[column.status]" />
          <span class="truncate text-xs font-medium text-ink">{{ t(`status.${column.status}`) }}</span>
          <span class="ml-auto font-mono text-[10px] text-faint">{{ column.items.length }}</span>
          <button
            type="button"
            class="pb-icon-btn size-5 shrink-0 text-faint hover:text-ink"
            :title="`在「${t(`status.${column.status}`)}」列新建任务`"
            @click="openNew(column.status)"
          >
            <Plus :size="12" />
          </button>
        </header>
        <div class="flex min-h-[280px] flex-1 flex-col gap-2 p-2">
          <article
            v-for="quest in column.items"
            :key="quest.id"
            draggable="true"
            role="button"
            tabindex="0"
            class="group cursor-pointer rounded-lg border border-line bg-panel-2 p-2.5 transition-colors hover:border-accent/40 hover:bg-panel-3"
            @click="openEdit(quest)"
            @keydown.enter.prevent="openEdit(quest)"
            @keydown.space.prevent="openEdit(quest)"
            @dragstart="onDragStart(quest, $event)"
          >
            <div class="flex items-center gap-1.5">
              <GripVertical :size="11" class="text-faint opacity-0 group-hover:opacity-100" />
              <span class="pb-chip">{{ t(`questType.${quest.type}`) }}</span>
              <span class="ml-auto inline-flex items-center gap-1.5 text-[10px] text-muted">
                <span class="size-1.5 rounded-full" :class="PRIORITY_DOT_CLASS[quest.priority]" />
                {{ t(`priority.${quest.priority}`) }}
              </span>
            </div>
            <h3 class="mt-1.5 text-xs leading-snug font-medium text-ink">{{ quest.title }}</h3>
            <p
              v-if="quest.description"
              class="mt-1 line-clamp-2 text-[10px] leading-relaxed text-faint"
            >
              {{ quest.description }}
            </p>
            <div class="mt-2 flex items-center gap-2 text-[10px] text-faint">
              <span
                v-if="quest.dueAt"
                class="inline-flex items-center gap-1"
                :class="dueClass(quest)"
              >
                <CalendarClock :size="10" />
                {{ formatDate(quest.dueAt) }}
              </span>
              <span class="ml-auto inline-flex items-center gap-1">
                <Paperclip :size="10" />
                {{ quest.links.assetIds.length }}
              </span>
              <span class="inline-flex items-center gap-1">
                <MessageSquare :size="10" />
                {{ quest.comments.length }}
              </span>
            </div>
            <div class="mt-2 flex items-center justify-between border-t border-line-soft pt-1.5">
              <span class="truncate text-[10px] text-muted">{{ assigneeName(quest) }}</span>
              <span class="text-[10px] text-faint">{{ relativeTime(quest.updatedAt) }}</span>
            </div>
          </article>
          <div
            v-if="column.items.length === 0"
            class="flex flex-1 items-center justify-center rounded-lg border border-dashed border-line-soft py-8 text-[11px] text-faint"
          >
            拖到这里
          </div>
        </div>
      </section>
    </div>

    <div v-else class="pb-card overflow-hidden">
      <table class="w-full text-left text-xs">
        <thead class="border-b border-line bg-panel-2/60 text-[10px] tracking-wide text-muted">
          <tr class="whitespace-nowrap">
            <th class="px-4 py-2.5 font-medium">任务</th>
            <th class="px-3 py-2.5 font-medium">状态</th>
            <th class="px-3 py-2.5 font-medium">类型</th>
            <th class="px-3 py-2.5 font-medium">优先级</th>
            <th class="px-3 py-2.5 font-medium">截止</th>
            <th class="px-3 py-2.5 font-medium">负责人</th>
            <th class="px-4 py-2.5 text-right font-medium">更新</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-line-soft">
          <tr
            v-for="quest in filtered"
            :key="quest.id"
            class="cursor-pointer transition-colors hover:bg-panel-2/60"
            @click="openEdit(quest)"
          >
            <td class="max-w-[320px] px-4 py-2.5">
              <div class="truncate font-medium text-ink">{{ quest.title }}</div>
              <div v-if="quest.description" class="mt-0.5 truncate text-[10px] text-faint">
                {{ quest.description }}
              </div>
            </td>
            <td class="px-3 py-2.5">
              <span class="pb-chip" :class="QUEST_STATUS_CLASS[quest.status]">
                {{ t(`status.${quest.status}`) }}
              </span>
            </td>
            <td class="px-3 py-2.5">
              <span class="pb-chip">{{ t(`questType.${quest.type}`) }}</span>
            </td>
            <td class="px-3 py-2.5">
              <span class="inline-flex items-center gap-1.5 text-[11px] text-muted">
                <span class="size-1.5 rounded-full" :class="PRIORITY_DOT_CLASS[quest.priority]" />
                {{ t(`priority.${quest.priority}`) }}
              </span>
            </td>
            <td class="px-3 py-2.5">
              <span
                v-if="quest.dueAt"
                class="inline-flex items-center gap-1 text-[11px] whitespace-nowrap"
                :class="cx(dueClass(quest) || 'text-muted')"
              >
                <CalendarClock :size="11" />
                {{ formatDate(quest.dueAt) }}
              </span>
              <span v-else class="text-faint">—</span>
            </td>
            <td class="px-3 py-2.5 text-muted">{{ assigneeName(quest) }}</td>
            <td class="px-4 py-2.5 text-right text-[10px] text-faint">
              {{ relativeTime(quest.updatedAt) }}
            </td>
          </tr>
          <tr v-if="filtered.length === 0">
            <td colspan="7" class="px-4 py-10 text-center text-[11px] text-faint">没有匹配的任务</td>
          </tr>
        </tbody>
      </table>
    </div>

    <QuestDialog
      :open="dialogOpen"
      :quest="selected"
      :initial-status="newStatus"
      @update:open="onDialogOpen"
    />
  </div>
</template>