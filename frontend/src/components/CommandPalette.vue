<script setup lang="ts">
import { ArrowRight, Search } from 'lucide-vue-next'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Route } from '../domain/types'
import { useAppStore } from '../stores/app'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [boolean]; shortcuts: [] }>()

const store = useAppStore()
const { t } = useI18n()

interface Entry {
  id: string
  group: string
  label: string
  hint?: string
  run: () => void
}

const NAV: Array<[Route, string]> = [
  ['dashboard', 'nav.dashboard'],
  ['quests', 'nav.quests'],
  ['lore', 'nav.lore'],
  ['tables', 'nav.tables'],
  ['assets', 'nav.assets'],
  ['generate', 'nav.generate'],
  ['maps', 'nav.maps'],
  ['export', 'nav.export'],
  ['settings', 'nav.settings'],
]

const query = ref('')
const active = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)

function close() {
  emit('update:open', false)
}

const entries = computed<Entry[]>(() => {
  const needle = query.value.trim().toLowerCase()
  const hits = (...parts: Array<string | undefined>) =>
    !needle || parts.some((part) => (part ?? '').toLowerCase().includes(needle))
  const out: Entry[] = []
  const groupCount = (group: string) => out.filter((entry) => entry.group === group).length

  for (const [route, key] of NAV) {
    const label = String(t(key))
    if (!hits(label, route)) continue
    out.push({
      id: `nav:${route}`,
      group: '命令',
      label,
      hint: '前往',
      run: () => {
        store.setRoute(route)
        close()
      },
    })
  }
  if (hits('保存', 'save')) {
    out.push({
      id: 'cmd:save',
      group: '命令',
      label: '保存当前项目',
      hint: 'Ctrl S',
      run: () => {
        void store.saveNow()
        close()
      },
    })
  }
  if (hits('快捷键', 'shortcut', 'keyboard')) {
    out.push({
      id: 'cmd:shortcuts',
      group: '命令',
      label: '查看快捷键',
      hint: '?',
      run: () => {
        close()
        emit('shortcuts')
      },
    })
  }
  if (hits('引导', '教程', 'onboarding', 'guide')) {
    out.push({
      id: 'cmd:onboarding',
      group: '命令',
      label: '重新查看新手引导',
      hint: '3 步',
      run: () => {
        close()
        window.dispatchEvent(new Event('pixelbench:onboarding'))
      },
    })
  }

  for (const quest of store.quests) {
    const type = String(t(`questType.${quest.type}`))
    const status = String(t(`status.${quest.status}`))
    if (!hits(quest.title, quest.brief.prompt, type, status)) continue
    if (groupCount('任务') >= 6) break
    out.push({
      id: `quest:${quest.id}`,
      group: '任务',
      label: quest.title,
      hint: `${type} · ${status}`,
      run: () => {
        store.requestFocus('quest', quest.id)
        store.setRoute('quests')
        close()
      },
    })
  }

  for (const entity of store.entities) {
    const kind = String(t(`entityKind.${entity.kind}`))
    const status = String(t(`entityStatus.${entity.status}`))
    if (!hits(entity.name, entity.summary, entity.aliases.join(' '), entity.tags.join(' '), kind, status)) continue
    if (groupCount('设定') >= 6) break
    out.push({
      id: `entity:${entity.id}`,
      group: '设定',
      label: entity.name,
      hint: `${kind} · ${status}`,
      run: () => {
        store.requestFocus('entity', entity.id)
        store.setRoute('lore')
        close()
      },
    })
  }

  for (const table of store.tables) {
    const hint = `${table.columns.length} 列 · ${table.rows.length} 行`
    if (!hits(table.name, table.key, table.note, hint)) continue
    if (groupCount('数值表') >= 6) break
    out.push({
      id: `table:${table.id}`,
      group: '数值表',
      label: table.name,
      hint,
      run: () => {
        store.requestFocus('table', table.id)
        store.setRoute('tables')
        close()
      },
    })
  }

  for (const asset of store.assets) {
    const type = String(t(`assetType.${asset.type}`))
    const status = String(t(`assetStatus.${asset.status}`))
    if (!hits(asset.name, type, status, asset.tags.join(' '))) continue
    if (groupCount('素材') >= 6) break
    out.push({
      id: `asset:${asset.id}`,
      group: '素材',
      label: asset.name,
      hint: `${type} · ${status}`,
      run: () => {
        store.requestFocus('asset', asset.id)
        store.setRoute('assets')
        close()
      },
    })
  }

  for (const map of store.maps) {
    const hint = `${map.width} × ${map.height} · ${map.tileSize}px`
    if (!hits(map.name, hint)) continue
    if (groupCount('地图') >= 4) break
    out.push({
      id: `map:${map.id}`,
      group: '地图',
      label: map.name,
      hint,
      run: () => {
        store.requestFocus('map', map.id)
        store.setRoute('maps')
        close()
      },
    })
  }

  return out
})

watch(query, () => {
  active.value = 0
})

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    query.value = ''
    active.value = 0
    await nextTick()
    inputRef.value?.focus()
    inputRef.value?.select()
  },
)

watch(entries, (list) => {
  if (active.value >= list.length) active.value = Math.max(0, list.length - 1)
})

function move(delta: number) {
  const count = entries.value.length
  if (count === 0) return
  active.value = (active.value + delta + count) % count
}

function run(entry: Entry) {
  entry.run()
}

function onEnter() {
  const entry = entries.value[active.value]
  if (entry) run(entry)
}
</script>

<template>
  <Teleport to="body">
    <template v-if="props.open">
      <div class="fixed inset-0 z-[60] bg-black/60 backdrop-blur-[2px]" @click="close" />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="命令面板"
        class="fixed top-[12vh] left-1/2 z-[61] flex w-[min(94vw,640px)] -translate-x-1/2 flex-col overflow-hidden rounded-2xl border border-line bg-panel shadow-[0_24px_80px_rgba(0,0,0,0.55)]"
      >
        <div class="flex items-center gap-2.5 border-b border-line px-4 py-3">
          <Search :size="15" class="shrink-0 text-faint" />
          <input
            ref="inputRef"
            v-model="query"
            aria-label="命令面板搜索"
            class="min-w-0 flex-1 bg-transparent text-sm text-ink outline-none placeholder:text-faint"
            placeholder="搜索任务、设定、数值表、素材、地图或命令…"
            @keydown.down.prevent="move(1)"
            @keydown.up.prevent="move(-1)"
            @keydown.enter.prevent="onEnter"
            @keydown.esc.prevent="close"
          />
          <span class="pb-kbd shrink-0">Esc</span>
        </div>
        <div class="max-h-[62vh] min-h-24 overflow-y-auto py-1.5">
          <template v-for="(entry, index) in entries" :key="entry.id">
            <div
              v-if="index === 0 || entries[index - 1]?.group !== entry.group"
              class="px-4 pt-2 pb-1 text-[10px] font-medium tracking-wide text-faint"
            >
              {{ entry.group }}
            </div>
            <button
              type="button"
              class="flex w-full items-center gap-3 px-4 py-2 text-left"
              :class="index === active ? 'bg-accent/10' : 'hover:bg-panel-2'"
              @mouseenter="active = index"
              @click="run(entry)"
            >
              <span class="min-w-0 flex-1 truncate text-xs" :class="index === active ? 'text-ink' : 'text-muted'">
                {{ entry.label }}
              </span>
              <span v-if="entry.hint" class="shrink-0 text-[10px] text-faint">{{ entry.hint }}</span>
              <ArrowRight v-if="index === active" :size="12" class="shrink-0 text-accent" />
            </button>
          </template>
          <p v-if="entries.length === 0" class="px-4 py-6 text-center text-xs text-faint">
            没有匹配的任务、设定、数值表、素材、地图或命令
          </p>
        </div>
        <div class="flex shrink-0 items-center gap-3 border-t border-line bg-panel-2/50 px-4 py-2 text-[10px] text-faint">
          <span><span class="pb-kbd">↑</span> <span class="pb-kbd">↓</span> 选择</span>
          <span><span class="pb-kbd">Enter</span> 打开</span>
          <span class="ml-auto">Ctrl K 随时呼出</span>
        </div>
      </div>
    </template>
  </Teleport>
</template>