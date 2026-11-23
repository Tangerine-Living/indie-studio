<script setup lang="ts">
import {
  Check,
  FileText,
  Info,
  RefreshCw,
} from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { DEFAULT_NEGATIVE_PROMPT, ENGINES, TILE_SIZES } from '../domain/types'
import type { Engine, UpdateSettings } from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { shortPath } from '../lib/utils'
import FieldBox from '../components/FieldBox.vue'
import SectionHeader from '../components/SectionHeader.vue'
import AISettingsPanel from '../components/AISettingsPanel.vue'
import PaletteManager from '../components/PaletteManager.vue'

const store = useAppStore()

const PERSPECTIVES = [
  ['3/4-topdown-orthographic', '3/4 俯视（正交）'],
  ['top-down', '正俯视'],
  ['side-view', '侧视'],
] as const

const OUTLINES = [
  ['1px-dark', '1px 深色描边'],
  ['1px-colored', '1px 同色系描边'],
  ['none', '无描边'],
] as const

const LIGHTS = [
  ['top-left', '左上'],
  ['top-right', '右上'],
  ['top', '顶部'],
] as const

const project = computed(() => store.session?.project ?? null)
const settings = computed(() => project.value?.settings ?? null)
const style = computed(() => settings.value?.styleGuide ?? null)

const notice = ref<string | null>(null)
let noticeTimer: ReturnType<typeof setTimeout> | null = null

/** Shows a short confirmation once the debounced project save lands. */
const savedHint = ref(false)
let savedHintTimer: ReturnType<typeof setTimeout> | null = null
watch(
  () => store.saveState,
  (next, previous) => {
    if (next !== 'saved' || previous !== 'saving') return
    savedHint.value = true
    if (savedHintTimer) clearTimeout(savedHintTimer)
    savedHintTimer = setTimeout(() => {
      savedHint.value = false
    }, 2000)
  },
)

const TABS = [
  { id: 'project', label: '项目设置' },
  { id: 'general', label: '通用设置' },
] as const
type SettingsTab = (typeof TABS)[number]['id']

/** Categories shown under each top-level tab. */
const SECTIONS = {
  project: [
    { id: 'basic', label: '基本信息' },
    { id: 'style', label: '风格指南' },
    { id: 'palettes', label: '调色板' },
  ],
  general: [
    { id: 'models', label: '模型配置' },
    { id: 'editor', label: '编辑器' },
    { id: 'updates', label: '应用与更新' },
  ],
} as const

const activeTab = ref<SettingsTab>('project')
const activeSection = ref<string>(SECTIONS.project[0].id)

function switchTab(tab: SettingsTab) {
  activeTab.value = tab
  activeSection.value = SECTIONS[tab][0].id
}

function showNotice(text: string) {
  notice.value = text
  if (noticeTimer) clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    if (notice.value === text) notice.value = null
  }, 4000)
}

onBeforeUnmount(() => {
  if (noticeTimer) clearTimeout(noticeTimer)
  if (savedHintTimer) clearTimeout(savedHintTimer)
})

function value(event: Event): string {
  return (event.target as HTMLInputElement | HTMLSelectElement).value
}

function setProjectName(event: Event) {
  const name = value(event)
  store.patchProject((current) => ({ ...current, name }))
}

function setEngine(event: Event) {
  const engine = value(event) as Engine
  store.patchProject((current) => ({ ...current, engine }))
}

function setTileSize(event: Event) {
  store.patchSettings({ tileSize: Number(value(event)) })
}



function setStyle(key: 'perspective' | 'outline' | 'light', event: Event) {
  const current = style.value
  if (!current) return
  store.patchSettings({ styleGuide: { ...current, [key]: value(event) } })
}

function setStylePrompt(event: Event) {
  const current = style.value
  if (!current) return
  store.patchSettings({
    styleGuide: {
      ...current,
      stylePrompt: (event.target as HTMLTextAreaElement).value,
    },
  })
}

function setNegativePrompt(event: Event) {
  const current = style.value
  if (!current) return
  store.patchSettings({
    styleGuide: {
      ...current,
      negativePrompt: (event.target as HTMLTextAreaElement).value,
    },
  })
}

function setMaxColors(event: Event) {
  const current = style.value
  if (!current) return
  store.patchSettings({
    styleGuide: {
      ...current,
      maxColors: Math.min(64, Math.max(2, Number(value(event)) || 16)),
    },
  })
}

async function openLogs() {
  if (store.info.mode === 'browser') {
    showNotice('浏览器预览模式没有本地日志目录')
    return
  }
  try {
    await platform.openLogsFolder()
    showNotice('已打开日志目录')
  } catch (raw) {
    store.setError(raw)
  }
}

const updateSettings = computed(() => store.updateSettings)
const updateStatus = computed(() => store.updateStatus)
const updateBusy = computed(() => store.updateBusy)
const downloadedPath = computed(() => store.updateDownloadPath)

onMounted(() => {
  if (!store.updateSettings) void store.loadUpdateSettings()
  if (!store.editorSettings) void store.loadEditorSettings()
})

const editorPrefs = computed(
  () => store.editorSettings ?? { showGrid: true, openMode: 'fit' as const },
)

async function patchEditor(patch: Partial<{ showGrid: boolean; openMode: 'fit' | 'actual' }>) {
  const saved = await store.saveEditorSettings({ ...editorPrefs.value, ...patch })
  if (saved) showNotice('编辑器设置已保存')
}

function setShowGrid(event: Event) {
  void patchEditor({ showGrid: (event.target as HTMLInputElement).checked })
}

function setOpenMode(event: Event) {
  void patchEditor({ openMode: value(event) === 'actual' ? 'actual' : 'fit' })
}

async function saveUpdate(patch: Partial<UpdateSettings>) {
  await store.saveUpdateSettings(patch)
}

function setUpdateChannel(event: Event) {
  void saveUpdate({ channel: value(event) === 'dev' ? 'dev' : 'stable' })
}

function setUpdateAuto(event: Event) {
  void saveUpdate({ autoCheck: (event.target as HTMLInputElement).checked })
}

async function checkUpdates() {
  const status = await store.checkUpdates()
  if (status?.message) showNotice(status.message)
}

async function downloadUpdate() {
  const path = await store.downloadUpdate()
  if (path) showNotice('已下载并校验更新文件')
}

</script>

<template>
  <div v-if="project && settings && style && store.session" class="space-y-4">
    <div class="flex flex-wrap items-center gap-1 rounded-lg border border-line bg-panel-2/40 p-1">
      <button
        v-for="tab in TABS"
        :key="tab.id"
        type="button"
        class="rounded-md px-3 py-1.5 text-[12px] transition-colors"
        :class="activeTab === tab.id ? 'bg-accent text-canvas' : 'text-muted hover:bg-panel-3 hover:text-ink'"
        :aria-pressed="activeTab === tab.id"
        @click="switchTab(tab.id)"
      >
        {{ tab.label }}
      </button>
      <span class="ml-auto mr-1 flex items-center gap-2">
        <span
          v-if="savedHint"
          class="pb-fade-in flex items-center gap-1 rounded-md border border-green/30 bg-green/10 px-2.5 py-1 text-[11px] text-green"
        >
          <Check :size="11" />
          设置已保存
        </span>
        <span
          v-if="notice"
          class="pb-fade-in rounded-md border border-line bg-panel-2 px-2.5 py-1 text-[11px] text-ink"
        >
          {{ notice }}
        </span>
      </span>
    </div>

    <div class="flex flex-wrap items-center gap-1 px-1">
      <button
        v-for="section in SECTIONS[activeTab]"
        :key="section.id"
        type="button"
        class="rounded-md px-2.5 py-1 text-[11px] transition-colors"
        :class="activeSection === section.id ? 'bg-panel-3 text-ink' : 'text-faint hover:text-ink'"
        :aria-pressed="activeSection === section.id"
        @click="activeSection = section.id"
      >
        {{ section.label }}
      </button>
    </div>

    <section v-if="activeTab === 'project' && activeSection === 'basic'" class="pb-card p-4">
      <SectionHeader title="项目" hint="作为导出与生成的默认值" />
      <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <FieldBox label="项目名称">
          <input class="pb-input" :value="project.name" maxlength="80" @input="setProjectName" />
        </FieldBox>
        <FieldBox label="目标引擎">
          <select class="pb-input" :value="project.engine" @change="setEngine">
            <option v-for="engine in ENGINES" :key="engine" :value="engine">
              {{ engine === 'godot' ? 'Godot 4' : engine === 'tiled' ? 'Tiled' : '通用格式' }}
            </option>
          </select>
        </FieldBox>
        <FieldBox label="瓦片尺寸">
          <select class="pb-input" :value="settings.tileSize" @change="setTileSize">
            <option v-for="size in TILE_SIZES" :key="size" :value="size">{{ size }} × {{ size }} px</option>
          </select>
        </FieldBox>
        </div>
    </section>

    <section v-if="activeTab === 'project' && activeSection === 'style'" class="pb-card p-4">
      <SectionHeader title="风格指南" hint="生成与后处理会按它约束结果" />
      <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <FieldBox label="视角">
          <select class="pb-input" :value="style.perspective" @change="setStyle('perspective', $event)">
            <option v-for="[key, label] in PERSPECTIVES" :key="key" :value="key">{{ label }}</option>
          </select>
        </FieldBox>
        <FieldBox label="描边">
          <select class="pb-input" :value="style.outline" @change="setStyle('outline', $event)">
            <option v-for="[key, label] in OUTLINES" :key="key" :value="key">{{ label }}</option>
          </select>
        </FieldBox>
        <FieldBox label="受光方向">
          <select class="pb-input" :value="style.light" @change="setStyle('light', $event)">
            <option v-for="[key, label] in LIGHTS" :key="key" :value="key">{{ label }}</option>
          </select>
        </FieldBox>
        <FieldBox label="最大颜色数">
          <input
            type="number"
            min="2"
            max="64"
            class="pb-input"
            :value="style.maxColors"
            @change="setMaxColors"
          />
        </FieldBox>
      </div>
      <div class="mt-3">
        <FieldBox
          label="风格锚点"
          hint="每次 AI 起稿与生成素材都会带上的固定画风词；写清画风、质感、配色倾向或参考作品"
        >
          <textarea
            class="pb-input h-16 resize-y font-mono text-[11px] leading-relaxed"
            :value="style.stylePrompt"
            placeholder="例如：高清像素风，新世代幻想，纯色背景，高饱和但克制的配色"
            @change="setStylePrompt"
          />
        </FieldBox>
      </div>
      <div class="mt-3">
        <FieldBox label="默认负面提示词" hint="生成台打开时预填；留空会回到内置默认">
          <textarea
            class="pb-input h-20 resize-y font-mono text-[11px] leading-relaxed"
            :value="style.negativePrompt"
            :placeholder="DEFAULT_NEGATIVE_PROMPT"
            @change="setNegativePrompt"
          />
        </FieldBox>
      </div>
    </section>

    <PaletteManager v-if="activeTab === 'project' && activeSection === 'palettes'" />

    <AISettingsPanel v-if="activeTab === 'general' && activeSection === 'models'" />

    <section v-if="activeTab === 'general' && activeSection === 'editor'" class="pb-card p-4">
      <SectionHeader title="编辑器" hint="只影响像素编辑器的打开方式与画布显示" />
      <div class="mt-4 space-y-3">
        <label class="flex items-center justify-between gap-3 rounded-lg border border-line bg-panel-2/40 px-3 py-2.5 text-[11px] text-muted">
          显示像素网格
          <input
            type="checkbox"
            class="size-3.5 accent-[var(--color-accent)]"
            :checked="editorPrefs.showGrid"
            @change="setShowGrid"
          />
        </label>
        <FieldBox label="打开素材时" hint="手动点「适应」仍可随时重新适应窗口">
          <select class="pb-input h-7 px-2 text-[11px]" :value="editorPrefs.openMode" @change="setOpenMode">
            <option value="fit">适应窗口</option>
            <option value="actual">100%</option>
          </select>
        </FieldBox>
      </div>
    </section>

    <section v-if="activeTab === 'general' && activeSection === 'updates'" class="pb-card p-4">
        <SectionHeader title="应用" />
        <dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-2 text-[11px]">
          <div class="flex items-baseline justify-between gap-2 border-b border-line-soft pb-1.5">
            <dt class="text-faint">版本</dt>
            <dd class="truncate text-right font-mono text-[10px] text-ink">{{ store.info.version }}</dd>
          </div>
          <div class="flex items-baseline justify-between gap-2 border-b border-line-soft pb-1.5">
            <dt class="text-faint">平台</dt>
            <dd class="truncate text-right font-mono text-[10px] text-ink">{{ store.info.platform }}</dd>
          </div>
        </dl>
        <div v-if="updateSettings" class="mt-4 space-y-2.5 rounded-lg border border-line bg-panel-2/40 px-3 py-2.5">
          <div class="flex items-center justify-between gap-2">
            <span class="text-[11px] text-muted">更新通道</span>
            <select class="pb-input h-7 w-28 px-2 text-[11px]" :value="updateSettings.channel" @change="setUpdateChannel">
              <option value="stable">稳定版</option>
              <option value="dev">开发版</option>
            </select>
          </div>
          <label class="flex items-center justify-between gap-2 text-[11px] text-muted">
            启动时检查更新
            <input
              type="checkbox"
              class="size-3.5 accent-[var(--color-accent)]"
              :checked="updateSettings.autoCheck"
              @change="setUpdateAuto"
            />
          </label>
          <div class="flex flex-wrap items-center gap-2 pt-0.5">
            <button type="button" class="pb-btn px-2.5 py-1 text-[11px]" :disabled="updateBusy" @click="checkUpdates()">
              <RefreshCw :size="12" :class="updateBusy ? 'animate-spin' : ''" />
              检查更新
            </button>
            <button
              v-if="updateStatus?.available && updateStatus.downloadUrl"
              type="button"
              class="pb-btn pb-btn-primary px-2.5 py-1 text-[11px]"
              :disabled="updateBusy"
              @click="downloadUpdate()"
            >
              下载并校验
            </button>
            <span v-if="updateStatus" class="text-[10px]" :class="updateStatus.available ? 'text-accent' : 'text-faint'">
              {{ updateStatus.message }}
            </span>
          </div>
          <p v-if="updateStatus?.available && updateStatus.notes" class="text-[10px] leading-relaxed text-muted">
            {{ updateStatus.notes }}
          </p>
          <p v-if="downloadedPath" class="break-all font-mono text-[10px] text-green" :title="downloadedPath">
            已校验：{{ shortPath(downloadedPath, 46) }}
          </p>
        </div>
        <div class="mt-4 flex flex-wrap gap-2">
          <button type="button" class="pb-btn px-3 py-1.5 text-xs" @click="openLogs()">
            <FileText :size="13" />
            打开日志目录
          </button>
        </div>
        <p class="mt-3 flex items-start gap-1.5 text-[11px] leading-relaxed text-faint">
          <Info :size="12" class="mt-0.5 shrink-0" />
          更新包经签名校验后才会下载，安装由你手动执行；应用不发送遥测。
        </p>
      </section>
  </div>
</template>