<script setup lang="ts">
import { Download, FolderOpen, Gamepad2, Package, PanelsTopLeft } from 'lucide-vue-next'
import { ref } from 'vue'
import type { Asset, AssetExportResult, MapExportFormat, MapSummary } from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import SectionHeader from '../components/SectionHeader.vue'
import EmptyState from '../components/EmptyState.vue'

const store = useAppStore()
const busyId = ref<string | null>(null)
const busyAssetId = ref<string | null>(null)
const result = ref<AssetExportResult | null>(null)
const resultLabel = ref('')
/** For a combined map export the Godot project lands in a sibling folder. */
const resultGodotDir = ref('')

const FORMAT_LABEL: Record<MapExportFormat, string> = {
  tiled: 'Tiled',
  godot: 'Godot 4',
  all: '两者',
}
const FORMATS: MapExportFormat[] = ['tiled', 'godot', 'all']

async function run(map: MapSummary, format: MapExportFormat) {
  busyId.value = `${map.id}:${format}`
  try {
    const exported = await store.exportMap(map.id, format)
    if (exported) {
      result.value = exported
      resultLabel.value = `${map.name} · ${FORMAT_LABEL[format]}`
      resultGodotDir.value = format === 'all' ? `${exported.dir}-godot` : ''
    }
  } finally {
    busyId.value = null
  }
}

async function runAsset(asset: Asset) {
  busyAssetId.value = asset.id
  try {
    result.value = await platform.exportAsset(asset.id)
    resultLabel.value = `${asset.name} · PNG`
    resultGodotDir.value = ''
  } catch (raw) {
    store.setError(raw)
  } finally {
    busyAssetId.value = null
  }
}

async function openExports() {
  try {
    await platform.openExportsFolder()
  } catch (raw) {
    store.setError(raw)
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-lg font-semibold tracking-tight text-ink">导出</h2>
      <button type="button" class="pb-btn px-3 py-1.5 text-xs" @click="openExports()">
        <FolderOpen :size="13" />
        打开导出目录
      </button>
    </div>

    <div class="grid gap-3 sm:grid-cols-3">
      <div class="pb-card p-4">
        <div class="flex items-center gap-2 text-xs font-medium text-ink">
          <PanelsTopLeft :size="14" class="text-accent" />
          Tiled
        </div>
        <p class="mt-2 text-[11px] leading-relaxed text-muted">
          TMX 1.10 + TSX，含图层、对象属性与翻转标志。
        </p>
        <p class="mt-2 font-mono text-[10px] text-faint">exports/maps/&lt;名称&gt;/</p>
      </div>
      <div class="pb-card p-4">
        <div class="flex items-center gap-2 text-xs font-medium text-ink">
          <Gamepad2 :size="14" class="text-accent" />
          Godot 4.3+
        </div>
        <p class="mt-2 text-[11px] leading-relaxed text-muted">
          自包含项目：TileSet、TileMapLayer 场景与 Marker2D 对象。
        </p>
        <p class="mt-2 font-mono text-[10px] text-faint">exports/maps/&lt;名称&gt;-godot/</p>
      </div>
      <div class="pb-card p-4">
        <div class="flex items-center gap-2 text-xs font-medium text-ink">
          <Package :size="14" class="text-accent" />
          通用 JSON
        </div>
        <p class="mt-2 text-[11px] leading-relaxed text-muted">
          每次地图导出附带 <span class="font-mono">pixelbench-map</span> JSON：碰撞、对象深度与地图设置。
        </p>
        <p class="mt-2 font-mono text-[10px] text-faint">exports/maps/&lt;名称&gt;/&lt;名称&gt;.json</p>
      </div>
    </div>

    <section class="pb-card p-4">
      <SectionHeader title="地图" hint="导出前会先保存当前地图" />
      <EmptyState v-if="store.maps.length === 0" title="没有地图" hint="在地图编辑器里新建一张地图后再导出。" />
      <ul v-else class="mt-3 space-y-2">
        <li
          v-for="map in store.maps"
          :key="map.id"
          class="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-panel-2/40 px-3 py-2.5"
        >
          <div class="min-w-0 flex-1">
            <div class="truncate text-xs font-medium text-ink">{{ map.name }}</div>
            <div class="mt-0.5 font-mono text-[10px] text-faint">
              {{ map.width }} × {{ map.height }} 格 · {{ map.tileSize }} px
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-1.5">
            <button
              v-for="format in FORMATS"
              :key="format"
              type="button"
              class="pb-btn px-2.5 py-1 text-[11px]"
              :class="format === 'all' ? 'pb-btn-primary' : ''"
              :disabled="busyId !== null"
              @click="run(map, format)"
            >
              <Download v-if="busyId !== `${map.id}:${format}`" :size="12" />
              <span v-else class="size-3 animate-spin rounded-full border border-current border-t-transparent" />
              {{ FORMAT_LABEL[format] }}
            </button>
          </div>
        </li>
      </ul>
    </section>

    <section v-if="result" class="pb-card p-4">
      <SectionHeader title="最近一次导出" :hint="resultLabel" />
      <div class="mt-3 rounded-lg border border-line bg-canvas/70 px-3 py-2">
        <div class="text-[10px] text-faint">目录</div>
        <div class="mt-1 break-all font-mono text-[10px] text-ink">{{ result.dir }}</div>
        <template v-if="resultGodotDir">
          <div class="mt-2 text-[10px] text-faint">Godot 项目</div>
          <div class="mt-1 break-all font-mono text-[10px] text-ink">{{ resultGodotDir }}</div>
        </template>
      </div>
      <div class="mt-2 flex flex-wrap gap-1.5">
        <span
          v-for="file in result.files"
          :key="file"
          class="rounded border border-line bg-panel-2/60 px-1.5 py-0.5 font-mono text-[10px] text-muted"
        >
          {{ file }}
        </span>
      </div>
    </section>

    <section class="pb-card p-4">
      <SectionHeader title="素材" hint="导出当前版本的 PNG 与版本元数据；精灵表附带逐帧图与 JSON" />
      <EmptyState v-if="store.assets.length === 0" title="没有素材" hint="在素材库导入或采用 AI 草稿后再导出。" />
      <ul v-else class="mt-3 space-y-1.5">
        <li
          v-for="asset in store.assets"
          :key="asset.id"
          class="flex items-center gap-3 rounded-lg border border-line bg-panel-2/40 px-3 py-2"
        >
          <div class="min-w-0 flex-1">
            <div class="truncate text-xs font-medium text-ink">{{ asset.name }}</div>
            <div class="mt-0.5 font-mono text-[10px] text-faint">
              {{ asset.dimensions.w }}×{{ asset.dimensions.h }} · {{ asset.currentVersion ?? '无版本' }}
            </div>
          </div>
          <button
            type="button"
            class="pb-btn shrink-0 px-2.5 py-1 text-[11px] whitespace-nowrap"
            :disabled="busyAssetId !== null || !asset.currentVersion"
            :title="asset.currentVersion ? '导出当前版本到 exports/' : '这个素材还没有保存过版本'"
            @click="runAsset(asset)"
          >
            <Download v-if="busyAssetId !== asset.id" :size="12" />
            <span v-else class="size-3 animate-spin rounded-full border border-current border-t-transparent" />
            {{ busyAssetId === asset.id ? '导出中' : '导出' }}
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>