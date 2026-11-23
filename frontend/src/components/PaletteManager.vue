<script setup lang="ts">
import { Download, Palette as PaletteIcon, Plus, Scissors, Trash2, Upload } from 'lucide-vue-next'
import { computed, onBeforeUnmount, ref } from 'vue'
import type { Palette } from '../domain/types'
import { useAppStore } from '../stores/app'
import { newID } from '../lib/utils'
import { confirmDialog } from '../lib/dialogs'
import {
  PALETTE_FORMATS,
  paletteFromText,
  parsePaletteText,
  serializePalette,
  trimPalette,
} from '../lib/palette'
import type { PaletteFormat } from '../lib/palette'
import SectionHeader from './SectionHeader.vue'

const store = useAppStore()

const editingId = ref<string | null>(null)
const importInput = ref<HTMLInputElement | null>(null)
const exportFormat = ref<PaletteFormat>('gpl')
const newColor = ref('#ffffff')
const notice = ref<string | null>(null)
const busy = ref(false)
let noticeTimer: ReturnType<typeof setTimeout> | null = null

const editing = computed(
  () => store.palettes.find((palette) => palette.id === editingId.value) ?? null,
)
const defaultPaletteId = computed(
  () => store.session?.project.settings.defaultPaletteId ?? '',
)
const colorBudget = computed(
  () => store.session?.project.settings.styleGuide.maxColors ?? 16,
)

function showNotice(text: string) {
  notice.value = text
  if (noticeTimer) clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    if (notice.value === text) notice.value = null
  }, 4000)
}

onBeforeUnmount(() => {
  if (noticeTimer) clearTimeout(noticeTimer)
})

async function persist(next: Palette, message = '调色板已保存'): Promise<Palette | null> {
  busy.value = true
  try {
    const saved = await store.savePalette(next)
    if (!saved) {
      showNotice(store.error?.message ?? '保存调色板失败')
      return null
    }
    showNotice(message)
    return saved
  } finally {
    busy.value = false
  }
}

function toggleEdit(palette: Palette) {
  editingId.value = editingId.value === palette.id ? null : palette.id
}

function setAsDefault(palette: Palette) {
  store.patchSettings({ defaultPaletteId: palette.id })
  showNotice(`默认调色板已切换为「${palette.name}」`)
}

async function createPalette() {
  const base = editing.value ?? store.palettes[0]
  const next: Palette = {
    schemaVersion: 1,
    id: `palette-${newID()}`,
    name: `${base?.name ?? '新调色板'} 副本`,
    colors: base ? [...base.colors] : ['#111821', '#e8c07a', '#4f8f4a', '#f2f6fa'],
    source: 'custom',
    locked: false,
  }
  const saved = await persist(next, `已新建「${next.name}」`)
  if (saved) editingId.value = saved.id
}

function pickImport() {
  importInput.value?.click()
}

async function onImportFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const parsed = parsePaletteText(await file.text(), file.name.replace(/\.[^.]+$/, ''))
    if (!parsed) {
      showNotice('没有读到颜色：支持 GPL、JASC PAL 或十六进制列表')
      return
    }
    const palette = paletteFromText(`palette-${newID()}`, parsed.name, parsed.colors)
    const saved = await persist(palette, `已导入「${palette.name}」（${palette.colors.length} 色）`)
    if (saved) editingId.value = saved.id
  } catch {
    showNotice('读取文件失败')
  }
}

function exportPalette(palette: Palette) {
  const format = PALETTE_FORMATS.find((item) => item.id === exportFormat.value)
  if (!format) return
  try {
    const blob = new Blob([serializePalette(palette, format.id)], {
      type: 'text/plain;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    const safeName = palette.name.replace(/[\\/:*?"<>|]/g, '_') || 'palette'
    anchor.href = url
    anchor.download = `${safeName}.${format.ext}`
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
    showNotice(`已导出「${palette.name}」为 .${format.ext}`)
  } catch {
    showNotice('当前环境无法下载文件，可在项目 palettes/ 目录找到 JSON')
  }
}

async function rename(palette: Palette, event: Event) {
  const name = (event.target as HTMLInputElement).value.trim()
  if (!name || name === palette.name) return
  await persist({ ...palette, name })
}

async function addColor(palette: Palette) {
  const color = newColor.value.toLowerCase()
  if (palette.colors.includes(color)) {
    showNotice('该颜色已在调色板中')
    return
  }
  if (palette.colors.length >= 256) {
    showNotice('单个调色板最多 256 色')
    return
  }
  await persist({ ...palette, colors: [...palette.colors, color] }, '已添加颜色')
}

async function removeColor(palette: Palette, color: string) {
  if (palette.colors.length <= 2) {
    showNotice('调色板至少保留两种颜色')
    return
  }
  await persist(
    { ...palette, colors: palette.colors.filter((item) => item !== color) },
    '已移除颜色',
  )
}

async function removePalette(palette: Palette) {
  if (palette.id === defaultPaletteId.value) {
    showNotice('项目默认调色板不能删除，先把默认切换成其他调色板')
    return
  }
  const ok = await confirmDialog({
    title: `删除调色板「${palette.name}」`,
    message: '会移动到项目的回收目录，需要时可以从 .workbench/trash 找回。',
    confirmText: '删除调色板',
    tone: 'danger',
  })
  if (!ok) return
  const done = await store.deletePalette(palette.id)
  if (!done) {
    showNotice(store.error?.message ?? '删除调色板失败')
    return
  }
  if (editingId.value === palette.id) editingId.value = null
  showNotice(`已删除「${palette.name}」`)
}

async function trimToBudget(palette: Palette) {
  const colors = trimPalette(palette.colors, colorBudget.value)
  if (colors.length === palette.colors.length) {
    showNotice(`当前已不超过 ${colorBudget.value} 色`)
    return
  }
  await persist({ ...palette, colors }, `已裁剪到 ${colors.length} 色`)
}
</script>

<template>
  <section class="pb-card p-4">
    <SectionHeader
      title="调色板"
      hint="用于生成与后处理，支持 GPL / PAL / HEX 导入导出"
    />
    <div class="mt-4 flex flex-wrap items-center gap-2">
      <button type="button" class="pb-btn h-8 px-2.5 py-1 text-xs" :disabled="busy" @click="createPalette()">
        <Plus :size="13" />
        新建副本
      </button>
      <button type="button" class="pb-btn h-8 px-2.5 py-1 text-xs" :disabled="busy" @click="pickImport()">
        <Upload :size="13" />
        导入
      </button>
      <span class="mx-1 h-5 w-px bg-line" />
      <span class="text-[10px] text-faint">导出格式</span>
      <select v-model="exportFormat" class="pb-input h-8 w-44 py-1 text-xs" aria-label="导出格式">
        <option v-for="format in PALETTE_FORMATS" :key="format.id" :value="format.id">
          {{ format.label }}
        </option>
      </select>
      <input
        ref="importInput"
        type="file"
        class="hidden"
        accept=".gpl,.pal,.hex,.txt,text/plain"
        @change="onImportFile"
      />
      <span class="text-[10px] text-faint">项目色数上限 {{ colorBudget }} 色</span>
    </div>

    <div
      v-if="notice"
      class="mt-3 rounded-lg border border-line bg-panel-2 px-3 py-1.5 text-[11px] text-ink"
    >
      {{ notice }}
    </div>

    <ul class="mt-3 space-y-2">
      <li
        v-for="palette in store.palettes"
        :key="palette.id"
        class="rounded-lg border border-line bg-panel-2/30 p-2.5"
        :class="editingId === palette.id ? 'border-accent/40' : ''"
      >
        <div class="flex flex-wrap items-center gap-2">
          <PaletteIcon :size="13" class="text-accent" />
          <button
            type="button"
            class="min-w-0 truncate text-xs font-medium text-ink hover:text-accent"
            :title="editingId === palette.id ? '收起编辑器' : '编辑调色板'"
            @click="toggleEdit(palette)"
          >
            {{ palette.name }}
          </button>
          <span class="font-mono text-[10px] text-faint">{{ palette.colors.length }} 色</span>
          <span
            v-if="palette.id === defaultPaletteId"
            class="rounded bg-accent/15 px-1.5 py-0.5 text-[9px] text-accent"
          >
            项目默认
          </span>
          <span v-if="palette.locked" class="rounded bg-panel-3 px-1.5 py-0.5 text-[9px] text-faint">
            已锁定
          </span>
          <span class="ml-auto flex items-center gap-1">
            <button
              type="button"
              class="pb-btn h-6 px-1.5 py-0 text-[10px]"
              :disabled="palette.id === defaultPaletteId"
              title="设为项目默认调色板"
              @click="setAsDefault(palette)"
            >
              设为默认
            </button>
            <button
              type="button"
              class="pb-icon-btn size-6"
              title="按所选格式导出"
              @click="exportPalette(palette)"
            >
              <Download :size="12" />
            </button>
            <button
              type="button"
              class="pb-icon-btn size-6 hover:text-red disabled:pointer-events-none disabled:opacity-30"
              :disabled="busy || palette.locked || palette.id === defaultPaletteId"
              :title="palette.id === defaultPaletteId
                ? '项目默认调色板不能删除'
                : '删除调色板（移到回收目录）'"
              @click="removePalette(palette)"
            >
              <Trash2 :size="12" />
            </button>
          </span>
        </div>

        <div class="mt-1.5 flex flex-wrap gap-1">
          <span
            v-for="color in palette.colors"
            :key="color"
            :title="color"
            class="size-4 rounded border border-black/30"
            :style="{ background: color }"
          />
        </div>

        <div v-if="editingId === palette.id" class="mt-2.5 space-y-2 border-t border-line-soft pt-2.5">
          <div class="flex flex-wrap items-center gap-2">
            <label class="flex items-center gap-1 text-[10px] text-faint">
              名称
              <input
                class="pb-input h-7 w-40 py-0 text-[11px]"
                :value="palette.name"
                maxlength="48"
                :disabled="busy || palette.locked"
                @change="rename(palette, $event)"
              />
            </label>
            <label class="flex items-center gap-1 text-[10px] text-faint">
              新增
              <input v-model="newColor" type="color" class="h-7 w-9 rounded border border-line bg-canvas" />
              <button
                type="button"
                class="pb-btn h-7 px-2 py-0 text-[10px]"
                :disabled="busy || palette.locked"
                @click="addColor(palette)"
              >
                <Plus :size="11" />
              </button>
            </label>
            <button
              type="button"
              class="pb-btn h-7 px-2 py-0 text-[10px]"
              :disabled="busy || palette.locked || palette.colors.length <= colorBudget"
              :title="`保留前 ${colorBudget} 色，其余移除`"
              @click="trimToBudget(palette)"
            >
              <Scissors :size="11" />
              裁剪到 {{ colorBudget }} 色
            </button>
          </div>
          <div class="flex flex-wrap gap-1">
            <button
              v-for="color in palette.colors"
              :key="color"
              type="button"
              class="group relative size-5 rounded border border-black/30"
              :style="{ background: color }"
              :title="`${color} · 点击移除`"
              :disabled="busy || palette.locked"
              @click="removeColor(palette, color)"
            >
              <Trash2
                :size="9"
                class="absolute inset-0 m-auto text-white opacity-0 drop-shadow transition-opacity group-hover:opacity-100"
              />
            </button>
          </div>
          <p class="text-[10px] text-faint">
            颜色会写入 palettes/{{ palette.id }}.json；锁定或内置调色板请先新建副本再改。
          </p>
        </div>
      </li>
    </ul>
  </section>
</template>