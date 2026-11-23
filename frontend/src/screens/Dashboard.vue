<script setup lang="ts">
import {
  ArrowRight,
  Check,
  Flag,
  FolderOpen,
  Images,
  ListChecks,
  Loader2,
  Palette,
  Plus,
  X,
} from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ASSET_TYPES, QUEST_STATUSES } from '../domain/types'
import type { Route } from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { ASSET_STATUS_CLASS, QUEST_STATUS_CLASS, QUEST_STATUS_DOT } from '../lib/badges'
import { formatDate, fromDateInput, newID, pct, relativeTime, shortPath } from '../lib/utils'
import AssetThumb from '../components/AssetThumb.vue'
import EmptyState from '../components/EmptyState.vue'
import ProgressBar from '../components/ProgressBar.vue'
import SectionHeader from '../components/SectionHeader.vue'
import StatCard from '../components/StatCard.vue'

const store = useAppStore()
const { t } = useI18n()

const project = computed(() => store.session?.project ?? null)
const approved = computed(() => store.assets.filter((asset) => asset.status === 'approved').length)
const doneQuests = computed(() => store.quests.filter((quest) => quest.status === 'done').length)
const doneMilestones = computed(
  () => project.value?.milestones.filter((item) => item.done).length ?? 0,
)
const defaultPalette = computed(() =>
  store.palettes.find((item) => item.id === project.value?.settings.defaultPaletteId),
)
const recentAssets = computed(() =>
  [...store.assets]
    .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
    .slice(0, 6),
)
const nextMilestone = computed(() => project.value?.milestones.find((item) => !item.done))

interface ActivityRow {
  key: string
  kind: 'quest' | 'asset'
  title: string
  sub: string
  statusText: string
  statusClass: string
  at: string
  route: Route
  focusId: string
}

/** Quests and assets merged into one timeline, newest first. */
const recentActivity = computed<ActivityRow[]>(() => {
  const rows: ActivityRow[] = [
    ...store.quests.map((quest) => ({
      key: `quest-${quest.id}`,
      kind: 'quest' as const,
      title: quest.title,
      sub: t(`questType.${quest.type}`),
      statusText: t(`status.${quest.status}`),
      statusClass: QUEST_STATUS_CLASS[quest.status],
      at: quest.updatedAt,
      route: 'quests' as Route,
      focusId: quest.id,
    })),
    ...store.assets.map((asset) => ({
      key: `asset-${asset.id}`,
      kind: 'asset' as const,
      title: asset.name,
      sub: `${asset.dimensions.w}×${asset.dimensions.h}`,
      statusText: t(`assetStatus.${asset.status}`),
      statusClass: ASSET_STATUS_CLASS[asset.status],
      at: asset.updatedAt,
      route: 'assets' as Route,
      focusId: asset.id,
    })),
  ]
  return rows
    .filter((row) => Boolean(row.at))
    .sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime())
    .slice(0, 7)
})

const activeAIJobs = computed(() =>
  store.aiJobs.filter((job) => job.state === 'queued' || job.state === 'running'),
)
const latestAIJobs = computed(() => store.aiJobs.slice(0, 3))

const JOB_STATE_LABEL: Record<string, string> = {
  queued: '排队中',
  running: '生成中',
  done: '已完成',
  failed: '失败',
  canceled: '已取消',
}

const milestoneTitle = ref('')
const milestoneDue = ref('')

const statusRows = computed(() =>
  QUEST_STATUSES.map((status) => {
    const count = store.quests.filter((quest) => quest.status === status).length
    return {
      status,
      label: t(`status.${status}`),
      count,
      percent: pct(count, store.quests.length),
      bar: QUEST_STATUS_DOT[status],
    }
  }),
)

const typeRows = computed(() =>
  ASSET_TYPES.map((type) => ({
    type,
    label: t(`assetType.${type}`),
    count: store.assets.filter((asset) => asset.type === type).length,
  })).filter((row) => row.count > 0),
)


async function reveal() {
  try {
    await platform.openProjectFolder()
  } catch (raw) {
    store.setError(raw)
  }
}

function toggleMilestone(id: string) {
  store.patchProject((current) => ({
    ...current,
    milestones: current.milestones.map((item) =>
      item.id === id ? { ...item, done: !item.done } : item,
    ),
  }))
}

function addMilestone() {
  const title = milestoneTitle.value.trim()
  if (!title) return
  const dueAt = fromDateInput(milestoneDue.value) || undefined
  store.patchProject((current) => ({
    ...current,
    milestones: [...current.milestones, { id: newID(), title, dueAt, done: false }],
  }))
  milestoneTitle.value = ''
  milestoneDue.value = ''
}

function removeMilestone(id: string) {
  store.patchProject((current) => ({
    ...current,
    milestones: current.milestones.filter((item) => item.id !== id),
  }))
}
</script>

<template>
  <div v-if="project" class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <div class="flex items-center gap-3">
          <h2 class="text-2xl leading-none font-semibold tracking-tight text-ink">
            {{ project.name }}
          </h2>
          <span class="pb-chip border-accent/25 bg-accent-soft font-mono text-accent">
            {{ project.engine }}
          </span>
        </div>
        <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-faint">
          <span>更新于 {{ relativeTime(project.updatedAt) }}</span>
          <span>{{ project.settings.tileSize }}px 瓦片</span>
          <span>{{ defaultPalette?.name ?? '未设置调色板' }}</span>
          <span class="font-mono" :title="store.session?.path">
            {{ shortPath(store.session?.path ?? '', 40) }}
          </span>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button type="button" class="pb-btn" @click="reveal()">
          <FolderOpen :size="15" />
          打开文件夹
        </button>
        <button type="button" class="pb-btn pb-btn-primary" @click="store.setRoute('quests')">
          任务板
          <ArrowRight :size="15" />
        </button>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard label="素材总数" :value="store.assets.length" :sub="`${approved} 个已批准`" tone="blue">
        <template #icon><Images :size="17" /></template>
      </StatCard>
      <StatCard
        label="任务"
        :value="store.quests.length"
        :sub="`${doneQuests} 个已完成，${pct(doneQuests, store.quests.length)}%`"
        tone="accent"
      >
        <template #icon><ListChecks :size="17" /></template>
      </StatCard>
      <StatCard
        label="里程碑"
        :value="`${doneMilestones}/${project.milestones.length}`"
        :sub="nextMilestone ? `下一个：${nextMilestone.title}` : '全部完成'"
        tone="green"
      >
        <template #icon><Flag :size="17" /></template>
      </StatCard>
      <StatCard
        label="调色板"
        :value="store.palettes.length"
        :sub="`默认 ${defaultPalette?.colors.length ?? 0} 色，上限 ${project.settings.styleGuide.maxColors}`"
        tone="neutral"
      >
        <template #icon><Palette :size="17" /></template>
      </StatCard>
    </div>

    <div class="grid gap-4 lg:grid-cols-[1.35fr_1fr]">
      <div class="space-y-4">
        <section class="pb-card p-4">
          <SectionHeader title="任务进展" hint="拖拽任务可推进状态">
            <template #action>
              <span class="font-mono text-[11px] text-faint">
                {{ doneQuests }}/{{ store.quests.length }}
              </span>
            </template>
          </SectionHeader>
          <div class="mt-4 space-y-2.5">
            <div v-for="row in statusRows" :key="row.status" class="flex items-center gap-3">
              <span class="w-14 shrink-0 text-[11px] text-muted">{{ row.label }}</span>
              <div class="h-2 flex-1 overflow-hidden rounded-full bg-line-soft">
                <div
                  class="h-full rounded-full transition-[width] duration-300"
                  :class="row.bar"
                  :style="{ width: `${row.percent}%` }"
                />
              </div>
              <span class="w-6 shrink-0 text-right font-mono text-[11px] text-faint">
                {{ row.count }}
              </span>
            </div>
          </div>

          <div class="mt-5 border-t border-line-soft pt-4">
            <div class="flex items-center justify-between text-[11px] text-muted">
              <span>素材构成</span>
              <span class="font-mono">{{ store.assets.length }}</span>
            </div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span v-for="row in typeRows" :key="row.type" class="pb-chip">
                {{ row.label }}
                <span class="font-mono text-ink">{{ row.count }}</span>
              </span>
              <span v-if="store.assets.length === 0" class="text-[11px] text-faint">暂无素材</span>
            </div>
          </div>
        </section>

        <section class="pb-card p-4">
          <SectionHeader title="最近活动">
            <template #action>
              <button type="button" class="text-[11px] text-accent hover:underline" @click="store.setRoute('quests')">
                任务板
              </button>
            </template>
          </SectionHeader>
          <EmptyState
            v-if="recentActivity.length === 0"
            class="mt-3"
            title="还没有活动"
            hint="新建任务或采用一份生成草稿后显示动态。"
          >
            <template #icon><ListChecks :size="20" /></template>
          </EmptyState>
          <ul v-else class="mt-2 divide-y divide-line-soft">
            <li v-for="row in recentActivity" :key="row.key" class="flex items-center gap-3 py-2.5">
              <span
                class="w-9 shrink-0 rounded px-1 py-0.5 text-center text-[9px]"
                :class="row.kind === 'quest' ? 'bg-accent-soft text-accent' : 'bg-panel-3 text-muted'"
              >
                {{ row.kind === 'quest' ? '任务' : '素材' }}
              </span>
              <div class="min-w-0 flex-1">
                <button
                  type="button"
                  class="block max-w-full truncate text-left text-xs font-medium text-ink hover:underline"
                  @click="store.requestFocus(row.kind, row.focusId); store.setRoute(row.route)"
                >
                  {{ row.title }}
                </button>
                <div class="mt-0.5 flex items-center gap-2 text-[10px] text-faint">
                  <span>{{ row.sub }}</span>
                  <span class="pb-chip" :class="row.statusClass">{{ row.statusText }}</span>
                </div>
              </div>
              <div class="shrink-0 text-right text-[10px] text-faint">{{ relativeTime(row.at) }}</div>
            </li>
          </ul>
        </section>
      </div>

      <div class="space-y-4">
        <section class="pb-card p-4">
          <SectionHeader title="里程碑" hint="点击勾选完成" />
          <ul class="mt-3 space-y-1">
            <li
              v-for="milestone in project.milestones"
              :key="milestone.id"
              class="flex items-center gap-1"
            >
              <button
                type="button"
                class="flex min-w-0 flex-1 items-center gap-2.5 rounded-lg px-2 py-2 text-left transition-colors hover:bg-panel-2"
                @click="toggleMilestone(milestone.id)"
              >
                <span
                  class="flex size-4 shrink-0 items-center justify-center rounded border"
                  :class="
                    milestone.done
                      ? 'border-green/50 bg-green/15 text-green'
                      : 'border-line bg-canvas text-transparent'
                  "
                >
                  <Check :size="11" />
                </span>
                <span class="min-w-0 flex-1">
                  <span
                    class="block truncate text-xs"
                    :class="milestone.done ? 'text-muted line-through' : 'text-ink'"
                  >
                    {{ milestone.title }}
                  </span>
                  <span v-if="milestone.dueAt" class="mt-0.5 block text-[10px] text-faint">
                    {{ formatDate(milestone.dueAt) }}
                  </span>
                </span>
              </button>
              <button
                type="button"
                class="pb-icon-btn size-7 shrink-0 text-faint hover:text-red"
                title="删除里程碑"
                @click="removeMilestone(milestone.id)"
              >
                <X :size="12" />
              </button>
            </li>
            <li v-if="project.milestones.length === 0" class="py-3 text-center text-[11px] text-faint">
              暂无里程碑
            </li>
          </ul>
          <form class="mt-2 flex items-center gap-1.5" @submit.prevent="addMilestone()">
            <input
              v-model="milestoneTitle"
              class="pb-input h-8 min-w-0 flex-1 py-1 text-xs"
              maxlength="60"
              placeholder="新增里程碑，例如：可玩 Demo"
            />
            <input
              v-model="milestoneDue"
              type="date"
              class="pb-input h-8 w-32 shrink-0 py-1 text-[11px]"
            />
            <button
              type="submit"
              class="pb-icon-btn size-8 shrink-0"
              :disabled="!milestoneTitle.trim()"
              title="添加里程碑"
            >
              <Plus :size="13" />
            </button>
          </form>
          <ProgressBar class="mt-3" :value="pct(doneMilestones, project.milestones.length)" tone="green" />
        </section>

        <section v-if="store.aiJobs.length > 0" class="pb-card p-4">
          <SectionHeader title="生成任务">
            <template #action>
              <button type="button" class="text-[11px] text-accent hover:underline" @click="store.setRoute('generate')">
                生成台
              </button>
            </template>
          </SectionHeader>
          <ul class="mt-3 space-y-2">
            <li
              v-for="job in latestAIJobs"
              :key="job.id"
              class="rounded-lg border border-line-soft bg-canvas/50 px-2.5 py-2"
            >
              <div class="flex items-center gap-2 text-[11px]">
                <span
                  class="font-medium"
                  :class="
                    job.state === 'running'
                      ? 'text-accent'
                      : job.state === 'done'
                        ? 'text-green'
                        : job.state === 'failed'
                          ? 'text-[#e07a7a]'
                          : 'text-faint'
                  "
                >
                  {{ JOB_STATE_LABEL[job.state] ?? job.state }}
                </span>
                <span class="ml-auto font-mono text-[9px] text-faint">
                  {{ job.width }}×{{ job.height }}
                </span>
              </div>
              <p class="mt-1 truncate text-[10px] text-muted" :title="job.prompt">{{ job.prompt }}</p>
              <div v-if="job.state === 'running'" class="mt-1.5 flex items-center gap-2">
                <ProgressBar :value="job.progress" class="flex-1" />
                <span class="font-mono text-[9px] text-faint">{{ Math.round(job.progress) }}%</span>
              </div>
            </li>
          </ul>
          <p v-if="activeAIJobs.length > 0" class="mt-2 flex items-center gap-1.5 text-[10px] text-faint">
            <Loader2 :size="11" class="animate-spin" />
            {{ activeAIJobs.length }} 个任务进行中
          </p>
        </section>

              </div>
    </div>

    <section class="pb-card p-4">
      <SectionHeader title="最近素材">
        <template #action>
          <button type="button" class="text-[11px] text-accent hover:underline" @click="store.setRoute('assets')">
            素材库
          </button>
        </template>
      </SectionHeader>
      <EmptyState
        v-if="recentAssets.length === 0"
        class="mt-3"
        title="还没有素材"
        hint="导入图片或采用一份生成草稿。"
      >
        <template #icon><Images :size="20" /></template>
      </EmptyState>
      <div v-else class="mt-3 flex flex-wrap gap-3">
        <button
          v-for="asset in recentAssets"
          :key="asset.id"
          type="button"
          class="flex w-[150px] cursor-pointer flex-col gap-2 rounded-lg border border-line bg-panel-2/50 p-2 text-left transition-colors hover:border-accent/40 hover:bg-accent-soft/40"
          @click="store.requestFocus('asset', asset.id); store.setRoute('assets')"
        >
          <AssetThumb :asset="asset" :size="134" />
          <div class="min-w-0">
            <div class="truncate text-[11px] font-medium text-ink">{{ asset.name }}</div>
            <div class="mt-1 flex items-center justify-between gap-1">
              <span class="text-[10px] text-faint">
                {{ asset.dimensions.w }}×{{ asset.dimensions.h }}
              </span>
              <span class="pb-chip" :class="ASSET_STATUS_CLASS[asset.status]">
                {{ t(`assetStatus.${asset.status}`) }}
              </span>
            </div>
          </div>
        </button>
      </div>
    </section>
  </div>
</template>