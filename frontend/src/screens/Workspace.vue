<script setup lang="ts">
import {
  BookOpen,
  Download,
  FolderOpen,
  Images,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Map as MapIcon,
  Package,
  Search,
  Settings,
  Sparkles,
  Table2,
} from 'lucide-vue-next'
import type { Component } from 'vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Route } from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { confirmDialog } from '../lib/dialogs'
import { shortPath } from '../lib/utils'
import PixelMark from '../components/PixelMark.vue'
import SaveIndicator from '../components/SaveIndicator.vue'
import CommandPalette from '../components/CommandPalette.vue'
import ShortcutsModal from '../components/ShortcutsModal.vue'
import OnboardingOverlay from '../components/OnboardingOverlay.vue'
import Dashboard from './Dashboard.vue'
import Quests from './Quests.vue'
import Lore from './Lore.vue'
import Tables from './Tables.vue'
import Assets from './Assets.vue'
import SettingsScreen from './SettingsScreen.vue'
import Editor from './Editor.vue'
import Generate from './Generate.vue'
import Maps from './Maps.vue'
import ExportScreen from './ExportScreen.vue'

const store = useAppStore()
const { t } = useI18n()
const shortcutsOpen = ref(false)
const paletteOpen = ref(false)

const NAV_ITEMS: Array<{ route: Route; icon: Component; labelKey: string; stage?: string }> = [
  { route: 'dashboard', icon: LayoutDashboard, labelKey: 'nav.dashboard' },
  { route: 'quests', icon: ListChecks, labelKey: 'nav.quests' },
  { route: 'lore', icon: BookOpen, labelKey: 'nav.lore' },
  { route: 'tables', icon: Table2, labelKey: 'nav.tables' },
  { route: 'assets', icon: Images, labelKey: 'nav.assets' },
  { route: 'generate', icon: Sparkles, labelKey: 'nav.generate' },
  { route: 'maps', icon: MapIcon, labelKey: 'nav.maps' },
  { route: 'export', icon: Package, labelKey: 'nav.export' },
]

const project = computed(() => store.session?.project ?? null)

function isActive(item: Route): boolean {
  return store.route === item || (item === 'assets' && store.route === 'editor')
}

/** Sidebar badge: how many records the module holds, when it holds any. */
function navCount(item: Route): number | null {
  if (item === 'quests') return store.quests.length || null
  if (item === 'lore') return store.entities.length || null
  if (item === 'tables') return store.tables.length || null
  if (item === 'assets') return store.assets.length || null
  return null
}

function onKeyDown(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    paletteOpen.value = true
    return
  }
  const target = event.target as HTMLElement | null
  const typing = target !== null && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
  if (!typing && event.key === '?') {
    event.preventDefault()
    shortcutsOpen.value = true
    return
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    // The editor owns its own save (immutable asset version).
    if (store.route === 'editor') return
    event.preventDefault()
    void store.saveNow()
  }
}

onMounted(() => window.addEventListener('keydown', onKeyDown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeyDown))

async function reveal() {
  try {
    await platform.openProjectFolder()
  } catch (raw) {
    store.setError(raw)
  }
}

async function closeWithConfirm() {
  const ok = await confirmDialog({
    title: '关闭项目',
    message: '未保存的修改会先自动保存，然后回到项目列表。',
    confirmText: '关闭项目',
  })
  if (ok) void store.closeProject()
}
</script>

<template>
  <div v-if="store.session && project" class="flex h-full bg-canvas">
    <aside class="flex w-[236px] shrink-0 flex-col border-r border-line bg-panel/85">
      <div class="flex items-center gap-2.5 px-4 pt-4 pb-3">
        <PixelMark :size="22" />
        <div class="min-w-0">
          <div class="text-[13px] leading-none font-semibold tracking-tight text-ink">Indie Studio</div>
          <div class="mt-1 text-[10px] text-faint">像素工坊</div>
        </div>
      </div>

      <div class="mx-3 mb-3 rounded-lg border border-line bg-panel-2/70 px-3 py-2.5">
        <div class="flex items-center gap-2">
          <span class="size-2 shrink-0 rounded-[2px] bg-accent" />
          <span class="truncate text-xs font-medium text-ink">{{ project.name }}</span>
        </div>
        <div class="mt-1 truncate font-mono text-[10px] text-faint" :title="store.session.path">
          {{ shortPath(store.session.path, 30) }}
        </div>
      </div>

      <nav class="space-y-0.5 px-3">
        <button
          v-for="item in NAV_ITEMS"
          :key="item.route"
          type="button"
          class="pb-nav"
          :class="isActive(item.route) ? 'pb-nav-active' : ''"
          @click="store.setRoute(item.route)"
        >
          <component
            :is="item.icon"
            :size="16"
            :class="isActive(item.route) ? 'text-accent' : ''"
          />
          <span class="flex-1 text-left">{{ t(item.labelKey) }}</span>
          <span v-if="navCount(item.route)" class="font-mono text-[10px] text-faint">
            {{ navCount(item.route) }}
          </span>
          <span
            v-if="item.stage"
            class="rounded border border-line px-1 font-mono text-[9px] text-faint"
          >
            {{ item.stage }}
          </span>
        </button>
      </nav>

      <div class="mt-auto space-y-0.5 border-t border-line-soft px-3 py-3">
        <button type="button" class="pb-nav" @click="reveal()">
          <FolderOpen :size="16" />
          <span class="flex-1 text-left">打开项目文件夹</span>
        </button>
        <button type="button" class="pb-nav" @click="closeWithConfirm()">
          <LogOut :size="16" />
          <span class="flex-1 text-left">关闭项目</span>
        </button>
        <button
          type="button"
          class="pb-nav"
          :class="store.route === 'settings' ? 'pb-nav-active' : ''"
          @click="store.setRoute('settings')"
        >
          <Settings :size="16" :class="store.route === 'settings' ? 'text-accent' : ''" />
          <span class="flex-1 text-left">{{ t('nav.settings') }}</span>
        </button>
      </div>
    </aside>

    <div class="flex min-w-0 flex-1 flex-col">
      <header class="flex h-12 shrink-0 items-center gap-3 border-b border-line bg-panel/60 px-4">
        <button
          type="button"
          class="relative w-full max-w-[300px]"
          title="搜索任务、设定、数值表、素材、地图与命令（Ctrl K）"
          @click="paletteOpen = true"
        >
          <Search :size="14" class="absolute top-1/2 left-3 z-10 -translate-y-1/2 text-faint" />
          <span class="pb-input flex h-8 items-center py-0 pr-16 pl-8 text-xs text-faint">
            搜索或运行命令
          </span>
          <span class="pb-kbd absolute top-1/2 right-2 -translate-y-1/2">Ctrl K</span>
        </button>

        <div class="ml-auto flex items-center gap-3">
          <button
            v-if="store.updateStatus?.available"
            type="button"
            class="pb-btn h-8 px-2.5 py-1 text-[11px]"
            :title="store.updateStatus.notes || '发现新版本，可在设置里查看并下载'"
            @click="store.setRoute('settings')"
          >
            <Download :size="13" class="text-accent" />
            新版本 {{ store.updateStatus.latestVersion ?? '' }}
          </button>
          <SaveIndicator />
        </div>
      </header>

      <main
        class="min-h-0 flex-1"
        :class="
          store.route === 'editor' || store.route === 'maps'
            ? 'overflow-hidden'
            : 'overflow-y-auto pb-grid-bg'
        "
      >
        <Editor v-if="store.route === 'editor'" />
        <Maps v-else-if="store.route === 'maps'" />
        <div v-else class="mx-auto max-w-[1240px] px-6 py-5">
          <Dashboard v-if="store.route === 'dashboard'" />
          <Quests v-else-if="store.route === 'quests'" />
          <Lore v-else-if="store.route === 'lore'" />
          <Tables v-else-if="store.route === 'tables'" />
          <Assets v-else-if="store.route === 'assets'" />
          <Generate v-else-if="store.route === 'generate'" />
          <ExportScreen v-else-if="store.route === 'export'" />
          <SettingsScreen v-else-if="store.route === 'settings'" />
        </div>
      </main>

      <CommandPalette v-model:open="paletteOpen" @shortcuts="shortcutsOpen = true" />
      <ShortcutsModal v-model:open="shortcutsOpen" />
      <OnboardingOverlay />

      <footer
        class="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-panel/70 px-5 font-mono text-[10px] text-faint"
      >
        <span class="truncate" :title="store.session.path">{{ shortPath(store.session.path, 64) }}</span>
        <span class="ml-auto">schema v{{ project.schemaVersion }}</span>
        <span>{{ project.engine }}</span>
        <span class="text-green">本地 · 离线</span>
      </footer>
    </div>
  </div>
</template>