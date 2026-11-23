<script setup lang="ts">
import {
  Download,
  Eraser,
  Eye,
  EyeOff,
  Grid3x3,
  Layers,
  Map as MapIcon,
  Maximize2,
  MousePointer2,
  Paintbrush,
  Pipette,
  Play,
  Plus,
  Redo2,
  Save,
  Shield,
  Sparkles,
  Square,
  Trash2,
  Undo2,
  X,
  ZoomIn,
  ZoomOut,
} from 'lucide-vue-next'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import type {
  AssetExportResult,
  GameMap,
  MapExportFormat,
  MapLayer,
  MapLayerType,
  MapObject,
  MapObjectKind,
} from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { confirmDialog, promptDialog } from '../lib/dialogs'
import MapCanvas from '../maps/MapCanvas.vue'
import TilePicker from '../maps/TilePicker.vue'
import {
  addObject,
  autotileCount,
  createMap,
  createObject,
  eraseCells,
  findLayer,
  LAYER_PRESETS,
  OBJECT_KIND_LABELS,
  objectDepth,
  paintCells,
  removeObject,
  repaintAutotile,
  setCollision,
  tileAt,
  tilesetFromAsset,
  TILE_SIZE_OPTIONS,
  updateObject,
  type Cell,
  type MapTool,
} from '../maps/model'

const store = useAppStore()
const canvasRef = ref<InstanceType<typeof MapCanvas> | null>(null)

const map = ref<GameMap | null>(null)
const loading = ref(false)
const mapError = ref('')
const activeLayerId = ref('ground')
const tool = ref<MapTool>('brush')
const tileSel = ref({ set: 0, tile: 0 })
const tilesetIndex = ref(0)
const objectKind = ref<MapObjectKind>('npc')
const selectedObjectId = ref<string | null>(null)
const showGrid = ref(true)
const previewing = ref(false)
const zoom = ref(1)
const cursorCell = ref<Cell | null>(null)
const images = ref<ReadonlyMap<string, CanvasImageSource>>(new Map())
const saving = ref<'idle' | 'pending' | 'saved' | 'error'>('idle')
const lastSaved = ref('')
const exportResult = ref<AssetExportResult | null>(null)
const exportFormat = ref<MapExportFormat>('tiled')

const undoCount = ref(0)
const redoCount = ref(0)
const undoStack: string[] = []
const redoStack: string[] = []

const newMapOpen = ref(false)
const newName = ref('新地图')
const newWidth = ref(40)
const newHeight = ref(30)
const newTileSize = ref(32)

const newLayerType = ref<MapLayerType>('decoration')
const autotileMode = ref<'16' | '47'>('16')

const expandEdges = reactive({ top: 0, right: 0, bottom: 0, left: 0 })
const extendTerrain = ref(true)
const expanding = ref(false)
const expandNotice = ref('')
const EXPAND_SIDES = [
  { key: 'top', label: '上' },
  { key: 'right', label: '右' },
  { key: 'bottom', label: '下' },
  { key: 'left', label: '左' },
] as const

const activeLayer = computed(() => (map.value ? findLayer(map.value, activeLayerId.value) ?? null : null))
const selectedObject = computed(() =>
  map.value && selectedObjectId.value
    ? map.value.objects.find((object) => object.id === selectedObjectId.value) ?? null
    : null,
)
const tileSet = computed(() => map.value?.tilesets[tilesetIndex.value] ?? null)
const objectProperties = computed(() =>
  selectedObject.value ? Object.entries(selectedObject.value.properties ?? {}) : [],
)
const selectedDepth = computed(() =>
  selectedObject.value && map.value ? objectDepth(selectedObject.value, map.value) : 0,
)
const OBJECT_KINDS = Object.entries(OBJECT_KIND_LABELS) as Array<[MapObjectKind, string]>
const newMapTilesets = computed(() => {
  return (store.assets ?? [])
    .map((asset) => tilesetFromAsset(asset, newTileSize.value))
    .filter((set): set is NonNullable<typeof set> => Boolean(set))
})

const TOOLS: Array<{ id: MapTool; label: string; icon: typeof Paintbrush }> = [
  { id: 'brush', label: '画笔', icon: Paintbrush },
  { id: 'eraser', label: '橡皮', icon: Eraser },
  { id: 'rect', label: '矩形', icon: Square },
  { id: 'eyedropper', label: '吸管', icon: Pipette },
  { id: 'collision', label: '碰撞', icon: Shield },
  { id: 'object', label: '对象', icon: MousePointer2 },
]

let saveTimer: ReturnType<typeof setTimeout> | null = null

function snapshot(): string | null {
  const current = map.value
  if (!current) return null
  return JSON.stringify({ layers: current.layers, objects: current.objects })
}

function pushUndo() {
  const snap = snapshot()
  if (!snap) return
  undoStack.push(snap)
  if (undoStack.length > 60) undoStack.shift()
  redoStack.length = 0
  undoCount.value = undoStack.length
  redoCount.value = redoStack.length
}

function undo() {
  const current = map.value
  if (!current || undoStack.length === 0) return
  const snap = snapshot()
  if (snap) redoStack.push(snap)
  const state = JSON.parse(undoStack.pop() as string) as Pick<GameMap, 'layers' | 'objects'>
  map.value = { ...current, layers: state.layers, objects: state.objects }
  undoCount.value = undoStack.length
  redoCount.value = redoStack.length
  scheduleSave()
}

function redo() {
  const current = map.value
  if (!current || redoStack.length === 0) return
  const snap = snapshot()
  if (snap) undoStack.push(snap)
  const state = JSON.parse(redoStack.pop() as string) as Pick<GameMap, 'layers' | 'objects'>
  map.value = { ...current, layers: state.layers, objects: state.objects }
  undoCount.value = undoStack.length
  redoCount.value = redoStack.length
  scheduleSave()
}

function scheduleSave() {
  if (!map.value) return
  saving.value = 'pending'
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveTimer = null
    void saveNow()
  }, 700)
}

async function saveNow() {
  if (!map.value) return
  const id = map.value.id
  const saved = await store.saveMap(map.value)
  if (!saved || !map.value || map.value.id !== id) {
    saving.value = 'error'
    return
  }
  map.value = { ...map.value, updatedAt: saved.updatedAt }
  saving.value = 'saved'
  lastSaved.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function mutate(updater: (current: GameMap) => GameMap) {
  const current = map.value
  if (!current) return
  const next = updater(current)
  if (next === current) return
  map.value = next
  scheduleSave()
}

async function loadImages() {
  const current = map.value
  if (!current) return
  for (const set of current.tilesets) {
    if (images.value.has(set.assetId)) continue
    try {
      const url = await platform.assetURL(set.assetId, 'source')
      const image = new Image()
      image.onload = () => {
        images.value = new Map(images.value).set(set.assetId, image)
      }
      image.src = url
    } catch {
      /* a missing atlas leaves empty cells; the rest of the map still opens */
    }
  }
}

function adoptMap(next: GameMap) {
  map.value = next
  mapError.value = ''
  previewing.value = false
  selectedObjectId.value = null
  undoStack.length = 0
  redoStack.length = 0
  undoCount.value = 0
  redoCount.value = 0
  exportResult.value = null
  saving.value = 'idle'
  activeLayerId.value = next.layers[0]?.id ?? 'ground'
  tilesetIndex.value = 0
  tileSel.value = { set: 0, tile: 0 }
  autotileMode.value = next.layers[0]?.autotile?.mode ?? '16'
  void loadImages()
  void nextTick(() => canvasRef.value?.fit())
}

async function openMap(id: string) {
  if (map.value?.id === id) return
  loading.value = true
  mapError.value = ''
  const loaded = await store.openMap(id)
  loading.value = false
  if (!loaded) {
    mapError.value = store.error?.message ?? '地图读取失败'
    return
  }
  adoptMap(loaded)
}

async function createNewMap() {
  const tilesets = newMapTilesets.value
  const next = createMap({
    name: newName.value.trim() || '未命名地图',
    width: newWidth.value,
    height: newHeight.value,
    tileSize: newTileSize.value,
    tilesets,
  })
  const saved = await store.saveMap(next)
  if (!saved) {
    mapError.value = store.error?.message ?? '地图创建失败'
    return
  }
  newMapOpen.value = false
  adoptMap(saved)
}

/**
 * Hands the planned map settings to AI generation: a new map usually starts
 * from an AI tileset, so this is offered right in the new-map dialog.
 */
function generateMapTileset() {
  newMapOpen.value = false
  const name = newName.value.trim() || '未命名地图'
  const size = newTileSize.value
  store.openMapGenerate({
    mapId: '',
    mapName: name,
    width: newWidth.value,
    height: newHeight.value,
    tileSize: size,
    tilesets: [],
    assetType: 'tile',
    prompt:
      `像素风 2.5D 俯视地形图集，${size}px 瓦片，草地、泥路、水面、石地、树篱，` +
      '按等距网格排列的 16 格自动拼接瓦片，纯色背景，格线对齐，无文字',
  })
}

async function deleteMap(id: string) {
  const name = store.maps.find((summary) => summary.id === id)?.name ?? '这张地图'
  const confirmed = await confirmDialog({
    title: '删除地图',
    message: `「${name}」会移入 .workbench/trash，可在文件系统中找回。`,
    confirmText: '删除',
    tone: 'danger',
  })
  if (!confirmed) return
  const ok = await store.deleteMap(id)
  if (!ok) return
  if (map.value?.id === id) {
    map.value = null
    const next = store.maps[0]
    if (next) void openMap(next.id)
  }
}

/* ------------------------------------------------------- expand & AI handoff */

function edgeValue(value: unknown): number {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return 0
  return Math.max(0, Math.min(256, Math.trunc(parsed)))
}

/**
 * Extends every autotile layer into the new border cells using its base tile,
 * then repaints borders so the old edge blends into the grown canvas.
 */
function fillNewTerrain(base: GameMap, edges: { top: number; right: number; bottom: number; left: number }): GameMap {
  const oldWidth = base.width - edges.left - edges.right
  const oldHeight = base.height - edges.top - edges.bottom
  const cells: Cell[] = []
  for (let y = 0; y < base.height; y += 1) {
    for (let x = 0; x < base.width; x += 1) {
      const insideOld =
        x >= edges.left && x < edges.left + oldWidth && y >= edges.top && y < edges.top + oldHeight
      if (!insideOld) cells.push({ x, y })
    }
  }
  if (cells.length === 0) return base
  let next = base
  for (const layer of base.layers) {
    if (!layer.autotile || layer.type === 'collision' || layer.type === 'object') continue
    next = paintCells(next, layer.id, cells, layer.autotile.set, layer.autotile.base, true)
  }
  return next
}

async function applyExpand() {
  const current = map.value
  if (!current) return
  const edges = {
    top: edgeValue(expandEdges.top),
    right: edgeValue(expandEdges.right),
    bottom: edgeValue(expandEdges.bottom),
    left: edgeValue(expandEdges.left),
  }
  if (edges.top + edges.right + edges.bottom + edges.left === 0) {
    mapError.value = '请至少在一个方向扩展'
    return
  }
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  expanding.value = true
  expandNotice.value = ''
  const expanded = await store.expandMap(current.id, edges)
  let result = expanded
  if (expanded && extendTerrain.value) {
    const filled = fillNewTerrain(expanded, edges)
    if (filled !== expanded) result = await store.saveMap(filled)
  }
  expanding.value = false
  if (!result) {
    mapError.value = store.error?.message ?? '扩展失败'
    return
  }
  mapError.value = ''
  expandNotice.value = `已扩展至 ${result.width}×${result.height} 格${
    extendTerrain.value ? '，新区域沿用边界地形' : ''
  }`
  expandEdges.top = 0
  expandEdges.right = 0
  expandEdges.bottom = 0
  expandEdges.left = 0
  adoptMap(result)
}

/** Builds a style-matched prompt from the map and hands it to the AI workbench. */
function openMapAI(kind: 'tile' | 'prop' | 'fx') {
  const current = map.value
  if (!current) return
  const guide = store.session?.project.settings.styleGuide
  const sets = current.tilesets.map((set, index) => {
    const asset = store.assets.find((item) => item.id === set.assetId)
    return asset?.name ?? `图集 ${index + 1}`
  })
  const style = [
    guide?.perspective === 'top-down' ? '正俯视' : '3/4 俯视（正交）',
    `${guide?.maxColors ?? 16} 色以内`,
    guide?.outline === 'none' ? '无描边' : '1 像素深色描边',
    '与现有地图风格统一',
  ].join('，')
  const info = `2.5D 俯视像素地图「${current.name}」，${current.tileSize}px 瓦片，${current.width}×${current.height} 格`
  const existing = sets.length ? `现有图集：${sets.join('、')}` : '当前地图没有图集'
  const prompt =
    kind === 'tile'
      ? `${info}，为扩展后的新区域补全地形。请生成按 ${current.tileSize}px 等距网格排列、可自动拼接的 16 格地形瓦片（草地、泥路、水面、石地等），${existing}。${style}，纯色背景，格线对齐，无文字`
      : kind === 'prop'
        ? `${info}，生成可放在对象层的装饰与道具（树木、路灯、摊位、招牌等），${existing}。${style}，透明背景，单个素材主体居中，底边对齐`
        : `${info}，生成地图内可循环播放的像素特效（水面波纹、火把光、蒸汽、落叶），${existing}。${style}，纯色背景，横向 4 帧等距排列`
  store.openGenerateFromMap(current, prompt, kind === 'tile' ? 'tile' : kind === 'prop' ? 'prop' : 'fx')
}

/* ------------------------------------------------------------- canvas events */

function onBeginStroke() {
  pushUndo()
}

function onPaint(cells: Cell[]) {
  const layer = activeLayer.value
  if (!layer || !map.value) return
  if (layer.type === 'collision') {
    mutate((current) => setCollision(current, cells, true))
    return
  }
  if (layer.type === 'object') return
  if (layer.autotile) {
    mutate((current) =>
      paintCells(current, layer.id, cells, layer.autotile!.set, layer.autotile!.base, true),
    )
    return
  }
  mutate((current) => paintCells(current, layer.id, cells, tileSel.value.set, tileSel.value.tile, false))
}

function onErase(cells: Cell[]) {
  const layer = activeLayer.value
  if (!layer || !map.value) return
  if (layer.type === 'collision') {
    mutate((current) => setCollision(current, cells, false))
    return
  }
  if (layer.type === 'object') {
    mutate((current) => {
      let next = current
      for (const cell of cells) {
        const hit = current.objects.find(
          (object) =>
            cell.x >= object.x &&
            cell.x < object.x + object.w &&
            cell.y >= object.y &&
            cell.y < object.y + object.h,
        )
        if (hit) next = removeObject(next, hit.id)
      }
      return next
    })
    return
  }
  mutate((current) => eraseCells(current, layer.id, cells))
}

function onCollision(cells: Cell[], solid: boolean) {
  mutate((current) => setCollision(current, cells, solid))
}

function onPick(cell: Cell) {
  const layer = activeLayer.value
  if (!layer) return
  const tile = tileAt(layer, cell.x, cell.y)
  if (!tile) return
  if (tileSel.value.set !== tile.set && map.value?.tilesets[tile.set]) {
    tilesetIndex.value = tile.set
  }
  tileSel.value = { set: tile.set, tile: tile.tile }
  if (layer.autotile) {
    mutate((current) => ({
      ...current,
      layers: current.layers.map((item) =>
        item.id === layer.id && item.autotile
          ? { ...item, autotile: { ...item.autotile, set: tile.set, base: tile.tile } }
          : item,
      ),
    }))
  }
}

function onSelectObject(id: string | null) {
  selectedObjectId.value = id
}

function onAddObject(cell: Cell) {
  const count = map.value?.objects.filter((object) => object.kind === objectKind.value).length ?? 0
  const object = createObject(objectKind.value, cell.x, cell.y, count)
  mutate((current) => addObject(current, object))
  selectedObjectId.value = object.id
  tool.value = 'object'
}

function onMoveObject(id: string, cell: { x: number; y: number }) {
  mutate((current) => {
    const object = current.objects.find((item) => item.id === id)
    if (!object) return current
    return updateObject(current, { ...object, x: cell.x, y: cell.y })
  })
}

function patchObject(patch: Partial<MapObject>) {
  const object = selectedObject.value
  if (!object) return
  mutate((current) => updateObject(current, { ...object, ...patch }))
}

function patchNumberField(field: 'x' | 'y' | 'w' | 'h' | 'elevation', value: number) {
  const object = selectedObject.value
  if (!object || !Number.isFinite(value)) return
  patchObject({ ...object, [field]: value })
}

function deleteSelectedObject() {
  const object = selectedObject.value
  if (!object) return
  pushUndo()
  mutate((current) => removeObject(current, object.id))
  selectedObjectId.value = null
}

/* ------------------------------------------------------------- panel events */

function selectLayer(id: string) {
  activeLayerId.value = id
  const layer = map.value ? findLayer(map.value, id) : null
  autotileMode.value = layer?.autotile?.mode ?? '16'
}

function toggleLayerVisible(layer: MapLayer) {
  pushUndo()
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) =>
      item.id === layer.id ? { ...item, visible: !item.visible } : item,
    ),
  }))
}

function setLayerElevation(layer: MapLayer, value: number) {
  const elevation = Math.max(0, Math.min(16, Math.round(value)))
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) => (item.id === layer.id ? { ...item, elevation } : item)),
  }))
}

function setLayerOpacity(layer: MapLayer, value: number) {
  const opacity = Math.max(0.05, Math.min(1, Math.round(value * 100) / 100))
  pushUndo()
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) => (item.id === layer.id ? { ...item, opacity } : item)),
  }))
}

function addLayer() {
  const preset = LAYER_PRESETS.find((item) => item.type === newLayerType.value)
  if (!preset) return
  const used = new Set(map.value?.layers.map((layer) => layer.id) ?? [])
  let index = 1
  let id = `${preset.type}-${index}`
  while (used.has(id)) {
    index += 1
    id = `${preset.type}-${index}`
  }
  pushUndo()
  mutate((current) => ({
    ...current,
    layers: [
      ...current.layers,
      {
        id,
        name: `${preset.name} ${index}`,
        type: preset.type,
        elevation: 0,
        visible: true,
        opacity: preset.type === 'collision' ? 0.5 : 1,
        tiles: [],
      },
    ],
  }))
  activeLayerId.value = id
}

async function deleteLayer(layer: MapLayer) {
  if (!map.value || map.value.layers.length <= 1) return
  const confirmed = await confirmDialog({
    title: '删除图层',
    message: `「${layer.name}」中的地块会一起删除。可以用 Ctrl+Z 撤销。`,
    confirmText: '删除图层',
    tone: 'danger',
  })
  if (!confirmed) return
  pushUndo()
  mutate((current) => ({
    ...current,
    layers: current.layers.filter((item) => item.id !== layer.id),
  }))
  if (activeLayerId.value === layer.id) {
    activeLayerId.value = map.value.layers.find((item) => item.id !== layer.id)?.id ?? 'ground'
  }
}

function toggleAutotile(enabled: boolean) {
  const layer = activeLayer.value
  if (!layer || layer.type === 'collision') return
  if (!enabled) {
    pushUndo()
    mutate((current) => ({
      ...current,
      layers: current.layers.map((item) =>
        item.id === layer.id ? { ...item, autotile: undefined } : item,
      ),
    }))
    return
  }
  const set = map.value?.tilesets[tilesetIndex.value]
  if (!set) {
    mapError.value = '当前地图没有可用图集，先在素材库准备一张 tile 图集'
    return
  }
  const count = autotileCount(autotileMode.value)
  if (tileSel.value.tile + count > set.tileCount) {
    mapError.value = `该图集在瓦片 ${tileSel.value.tile} 之后不足 ${count} 个变体`
    return
  }
  mapError.value = ''
  pushUndo()
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) =>
      item.id === layer.id
        ? {
            ...item,
            autotile: { set: tileSel.value.set, base: tileSel.value.tile, mode: autotileMode.value },
          }
        : item,
    ),
  }))
}

function changeAutotileMode(mode: '16' | '47') {
  autotileMode.value = mode
  const layer = activeLayer.value
  if (!layer?.autotile) return
  const set = map.value?.tilesets[layer.autotile.set]
  if (set && layer.autotile.base + autotileCount(mode) > set.tileCount) {
    mapError.value = '图集不足以容纳 47 个变体，保持 16 模式'
    return
  }
  mapError.value = ''
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) =>
      item.id === layer.id && item.autotile
        ? { ...item, autotile: { ...item.autotile, mode } }
        : item,
    ),
  }))
}

function repaintAllAutotile() {
  const layer = activeLayer.value
  if (!layer?.autotile) return
  pushUndo()
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) =>
      item.id === layer.id ? repaintAutotile(item, current) : item,
    ),
  }))
}

function onTileSelect(set: number, tile: number) {
  tileSel.value = { set, tile }
  const layer = activeLayer.value
  if (!layer?.autotile) return
  if (layer.autotile.set !== set) {
    pushUndo()
    mutate((current) => ({
      ...current,
      layers: current.layers.map((item) =>
        item.id === layer.id && item.autotile
          ? { ...item, autotile: { ...item.autotile, set } }
          : item,
      ),
    }))
    return
  }
  const tileset = map.value?.tilesets[set]
  if (!tileset || tile + autotileCount(layer.autotile.mode) > tileset.tileCount) return
  mutate((current) => ({
    ...current,
    layers: current.layers.map((item) =>
      item.id === layer.id && item.autotile
        ? { ...item, autotile: { ...item.autotile, base: tile } }
        : item,
    ),
  }))
}

function setSetting<K extends keyof GameMap['settings']>(key: K, value: GameMap['settings'][K]) {
  mutate((current) => ({ ...current, settings: { ...current.settings, [key]: value } }))
}

function setProp(key: string, value: string) {
  const object = selectedObject.value
  if (!object) return
  patchObject({ properties: { ...(object.properties ?? {}), [key]: value } })
}

function removeProp(key: string) {
  const object = selectedObject.value
  if (!object) return
  const next = { ...(object.properties ?? {}) }
  delete next[key]
  patchObject({ properties: next })
}

async function addProp() {
  const key = await promptDialog({
    title: '添加自定义属性',
    message: '属性名会写入对象数据，并原样导出到 Tiled 与 Godot。',
    placeholder: '例如 item、event',
  })
  if (!key) return
  setProp(key, '')
}

async function exportNow() {
  if (!map.value) return
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  await saveNow()
  const result = await store.exportMap(map.value.id, exportFormat.value)
  if (result) exportResult.value = result
}

function onKeyDown(event: KeyboardEvent) {
  const target = event.target as HTMLElement | null
  if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return
  // Modal dialogs own the keyboard; editor shortcuts must not fire behind them.
  if (newMapOpen.value) {
    if (event.key === 'Escape') newMapOpen.value = false
    return
  }
  const mod = event.ctrlKey || event.metaKey
  if (mod && event.key.toLowerCase() === 'z') {
    event.preventDefault()
    if (event.shiftKey) redo()
    else undo()
    return
  }
  if (mod && event.key.toLowerCase() === 'y') {
    event.preventDefault()
    redo()
    return
  }
  if (mod && event.key.toLowerCase() === 's') {
    event.preventDefault()
    void saveNow()
    return
  }
  if ((event.key === 'Delete' || event.key === 'Backspace') && selectedObject.value) {
    event.preventDefault()
    deleteSelectedObject()
    return
  }
  if (previewing.value) return
  const shortcuts: Record<string, MapTool> = {
    b: 'brush',
    e: 'eraser',
    r: 'rect',
    i: 'eyedropper',
    c: 'collision',
    v: 'object',
  }
  const next = shortcuts[event.key.toLowerCase()]
  if (next) {
    event.preventDefault()
    tool.value = next
  }
  if (event.key === 'g') showGrid.value = !showGrid.value
}

watch(
  () => store.maps,
  (list) => {
    if (!map.value && !loading.value && list.length > 0) void openMap(list[0].id)
  },
)

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
  const projectTile = store.session?.project.settings.tileSize
  if (projectTile && (TILE_SIZE_OPTIONS as readonly number[]).includes(projectTile)) {
    newTileSize.value = projectTile
  }
  const focused = store.consumeFocus('map')
  const first =
    (focused ? store.maps.find((summary) => summary.id === focused) : undefined) ?? store.maps[0]
  // With no maps, show the empty state; creating one stays the user's choice.
  if (first) void openMap(first.id)
})

// The command palette can name a map while this screen is already open.
watch(
  () => store.focusRequest,
  async () => {
    const id = store.consumeFocus('map')
    if (!id || map.value?.id === id) return
    if (saveTimer) {
      clearTimeout(saveTimer)
      saveTimer = null
      await saveNow()
    }
    void openMap(id)
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown)
  if (saveTimer) clearTimeout(saveTimer)
  if (map.value) void store.saveMap(map.value)
})

</script>

<template>
  <div class="flex h-full min-h-0 bg-canvas">
    <aside class="flex w-[196px] shrink-0 flex-col border-r border-line bg-panel/70">
      <div class="flex items-center justify-between border-b border-line-soft px-3 py-2.5">
        <div class="flex items-center gap-2 text-xs font-semibold text-ink">
          <MapIcon :size="14" class="text-accent" />
          地图
        </div>
        <button type="button" class="pb-icon-btn size-7" title="新建地图" @click="newMapOpen = true">
          <Plus :size="14" />
        </button>
      </div>
      <div class="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
        <button
          v-for="summary in store.maps"
          :key="summary.id"
          type="button"
          class="w-full rounded-lg border px-2.5 py-2 text-left transition-colors"
          :class="
            map?.id === summary.id
              ? 'border-accent/50 bg-accent/10'
              : 'border-transparent hover:bg-panel-2'
          "
          @click="openMap(summary.id)"
        >
          <div class="truncate text-xs font-medium text-ink">{{ summary.name }}</div>
          <div class="mt-0.5 font-mono text-[10px] text-faint">
            {{ summary.width }}×{{ summary.height }} · {{ summary.tileSize }}px
          </div>
        </button>
        <div v-if="loading" class="px-2 py-3 text-[11px] text-faint">读取地图…</div>
        <div v-if="store.maps.length === 0 && !loading" class="px-2 py-3 text-[11px] text-faint">
          还没有地图，点击右上角新建。
        </div>
      </div>
      <div v-if="map" class="border-t border-line-soft p-2">
        <button
          type="button"
          class="pb-btn pb-btn-ghost w-full justify-start px-2 py-1.5 text-[11px] text-muted hover:text-danger"
          @click="deleteMap(map.id)"
        >
          <Trash2 :size="13" />
          删除当前地图
        </button>
      </div>
    </aside>

    <div class="flex min-w-0 flex-1 flex-col">
      <div v-if="map" class="flex h-11 shrink-0 items-center gap-1 border-b border-line bg-panel/60 px-2.5">
        <div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
        <button
          v-for="item in TOOLS"
          :key="item.id"
          type="button"
          class="pb-icon-btn size-8"
          :class="tool === item.id ? '!border-accent/60 !bg-accent/15 !text-accent' : ''"
          :title="item.label"
          @click="tool = item.id"
        >
          <component :is="item.icon" :size="15" />
        </button>
        <span class="mx-1 h-5 w-px bg-line" />
        <button
          type="button"
          class="pb-icon-btn size-8"
          :disabled="undoCount === 0"
          title="撤销 Ctrl+Z"
          @click="undo"
        >
          <Undo2 :size="15" />
        </button>
        <button
          type="button"
          class="pb-icon-btn size-8"
          :disabled="redoCount === 0"
          title="重做 Ctrl+Y"
          @click="redo"
        >
          <Redo2 :size="15" />
        </button>
        <span class="mx-1 h-5 w-px bg-line" />
        <button type="button" class="pb-icon-btn size-8" title="缩小" @click="canvasRef?.zoomOut()">
          <ZoomOut :size="15" />
        </button>
        <span class="w-11 text-center font-mono text-[10px] text-muted">{{ Math.round(zoom * 100) }}%</span>
        <button type="button" class="pb-icon-btn size-8" title="放大" @click="canvasRef?.zoomIn()">
          <ZoomIn :size="15" />
        </button>
        <button
          type="button"
          class="pb-icon-btn size-8"
          :class="showGrid ? '!text-accent' : ''"
          title="网格 (G)"
          @click="showGrid = !showGrid"
        >
          <Grid3x3 :size="15" />
        </button>
        <button
          type="button"
          class="pb-btn ml-1 h-8 shrink-0 px-2.5 py-0 text-xs whitespace-nowrap"
          :class="previewing ? '!border-accent/60 !bg-accent/15 !text-accent' : ''"
          @click="previewing = !previewing"
        >
          <X v-if="previewing" :size="13" />
          <Play v-else :size="13" />
          {{ previewing ? '退出预览' : '行走预览' }}
        </button>
        </div>
        <div class="ml-2 flex shrink-0 items-center gap-1">
          <button type="button" class="pb-btn h-8 shrink-0 px-2.5 py-0 text-xs whitespace-nowrap" @click="saveNow()">
            <Save :size="13" />
            保存
          </button>
          <select v-model="exportFormat" class="pb-input h-8 w-[92px] shrink-0 px-2 text-xs" title="导出格式">
            <option value="tiled">Tiled</option>
            <option value="godot">Godot</option>
            <option value="all">两者</option>
          </select>
          <button type="button" class="pb-btn pb-btn-primary h-8 shrink-0 px-2.5 py-0 text-xs whitespace-nowrap" @click="exportNow()">
            <Download :size="13" />
            导出
          </button>
        </div>
      </div>

      <div class="relative flex min-h-0 flex-1">
        <div class="relative min-w-0 flex-1">
          <MapCanvas
            v-if="map"
            ref="canvasRef"
            :map="map"
            :layer-id="activeLayerId"
            :tool="tool"
            :tile="tileSel"
            :object-kind="objectKind"
            :images="images"
            :show-grid="showGrid"
            :selected-object-id="selectedObjectId"
            :preview="previewing"
            @begin-stroke="onBeginStroke"
            @paint="onPaint"
            @erase="onErase"
            @collision="onCollision"
            @pick="onPick"
            @select-object="onSelectObject"
            @add-object="onAddObject"
            @move-object="onMoveObject"
            @zoom="zoom = $event"
            @cursor="cursorCell = $event"
            @exit-preview="previewing = false"
          />
          <div v-else class="flex h-full items-center justify-center">
            <div class="pb-panel max-w-sm p-5 text-center">
              <MapIcon :size="22" class="mx-auto text-accent" />
              <p class="mt-2 text-sm font-medium text-ink">还没有打开地图</p>
              <p class="mt-1 text-xs leading-relaxed text-faint">
                先让 AI 生成配套地形图集，或新建空白地图直接开画。
              </p>
              <div class="mt-3 flex flex-wrap justify-center gap-2">
                <button
                  type="button"
                  class="pb-btn pb-btn-primary text-xs"
                  @click="generateMapTileset()"
                >
                  <Sparkles :size="13" />
                  AI 生成地形图集
                </button>
                <button type="button" class="pb-btn text-xs" @click="newMapOpen = true">
                  <Plus :size="13" />
                  新建空白地图
                </button>
              </div>
            </div>
          </div>
          <p v-if="mapError" class="absolute top-2 left-2 rounded bg-danger/15 px-2 py-1 text-[11px] text-danger">
            {{ mapError }}
          </p>
        </div>

        <aside v-if="map" class="flex w-[288px] shrink-0 flex-col border-l border-line bg-panel/70">
          <div class="min-h-0 flex-1 overflow-y-auto">
            <section class="border-b border-line-soft p-3">
              <div class="flex items-center justify-between">
                <h3 class="flex items-center gap-1.5 text-xs font-semibold text-ink">
                  <Layers :size="13" class="text-accent" />
                  图层
                </h3>
                <span class="font-mono text-[10px] text-faint">{{ map?.layers.length ?? 0 }}</span>
              </div>
              <div class="mt-2 space-y-1">
                <div
                  v-for="layer in map?.layers ?? []"
                  :key="layer.id"
                  class="group flex items-center gap-1 rounded-md border px-1.5 py-1"
                  :class="
                    layer.id === activeLayerId
                      ? 'border-accent/50 bg-accent/10'
                      : 'border-transparent hover:bg-panel-2'
                  "
                >
                  <button
                    type="button"
                    class="text-faint hover:text-ink"
                    :title="layer.visible ? '隐藏图层' : '显示图层'"
                    :aria-label="`${layer.visible ? '隐藏' : '显示'}图层「${layer.name}」`"
                    @click="toggleLayerVisible(layer)"
                  >
                    <Eye v-if="layer.visible" :size="13" />
                    <EyeOff v-else :size="13" />
                  </button>
                  <button
                    type="button"
                    class="min-w-0 flex-1 truncate text-left text-xs text-ink"
                    @click="selectLayer(layer.id)"
                  >
                    {{ layer.name }}
                  </button>
                  <span class="font-mono text-[9px] text-faint">{{ layer.type }}</span>
                  <button
                    type="button"
                    class="text-faint opacity-0 transition-opacity group-hover:opacity-100 hover:text-danger"
                    title="删除图层"
                    @click="deleteLayer(layer)"
                  >
                    <X :size="12" />
                  </button>
                </div>
              </div>
              <div class="mt-2 flex items-center gap-1">
                <select v-model="newLayerType" class="pb-input h-7 flex-1 py-0 text-[11px]">
                  <option v-for="preset in LAYER_PRESETS" :key="preset.type" :value="preset.type">
                    {{ preset.name }}
                  </option>
                </select>
                <button type="button" class="pb-icon-btn size-7" title="添加图层" @click="addLayer">
                  <Plus :size="13" />
                </button>
              </div>
              <div v-if="activeLayer" class="mt-2 flex items-center gap-2">
                <span class="text-[10px] text-faint">高程</span>
                <input
                  :value="activeLayer.elevation"
                  type="number"
                  min="0"
                  max="16"
                  class="pb-input h-7 w-16 py-0 font-mono text-[11px]"
                  @change="setLayerElevation(activeLayer, Number(($event.target as HTMLInputElement).value))"
                />
                <span class="text-[10px] text-faint">× {{ map?.settings.heightStep ?? 16 }}px</span>
              </div>
              <div v-if="activeLayer" class="mt-2 flex items-center gap-2">
                <span class="text-[10px] text-faint">不透明度</span>
                <input
                  :value="activeLayer.opacity"
                  type="range"
                  min="0.05"
                  max="1"
                  step="0.05"
                  class="flex-1 accent-[#f2b84b]"
                  @change="setLayerOpacity(activeLayer, Number(($event.target as HTMLInputElement).value))"
                />
                <span class="w-8 text-right font-mono text-[10px] text-faint">
                  {{ Math.round(activeLayer.opacity * 100) }}%
                </span>
              </div>
              <p
                v-if="activeLayer?.type === 'light'"
                class="mt-2 rounded-md border border-line-soft bg-canvas/50 px-2 py-1.5 text-[10px] leading-relaxed text-faint"
              >
                用柔光瓦片绘制，按 screen 叠加发光。隐藏本图层即可关闭光效。
              </p>
            </section>

            <section v-if="map && activeLayer && activeLayer.type !== 'collision'" class="border-b border-line-soft p-3">
              <div class="flex items-center justify-between">
                <h3 class="text-xs font-semibold text-ink">图集与瓦片</h3>
                <span class="font-mono text-[10px] text-faint">{{ tileSel.set }} · {{ tileSel.tile }}</span>
              </div>
              <select
                v-if="map.tilesets.length > 1"
                :value="tilesetIndex"
                class="pb-input mt-2 h-7 py-0 text-[11px]"
                @change="tilesetIndex = Number(($event.target as HTMLSelectElement).value)"
              >
                <option v-for="(set, index) in map.tilesets" :key="set.id" :value="index">
                  #{{ index }} · {{ set.columns }} 列 · {{ set.tileCount }} 格
                </option>
              </select>
              <div v-if="map.tilesets.length === 0" class="mt-2 rounded-md border border-danger/30 bg-danger/10 p-2 text-[10px] leading-4 text-danger">
                这张地图没有图集。先导入或生成一张 tile 素材，再新建地图。
              </div>
              <div v-else class="mt-2">
                <TilePicker
                  :map="map"
                  :tileset-index="tilesetIndex"
                  :images="images"
                  :selected="tileSel"
                  @select="onTileSelect"
                />
              </div>
              <div class="mt-2 rounded-md border border-line bg-panel-2/60 p-2">
                <label class="flex items-center gap-2 text-[11px] text-muted">
                  <input
                    type="checkbox"
                    :checked="Boolean(activeLayer.autotile)"
                    @change="toggleAutotile(($event.target as HTMLInputElement).checked)"
                  />
                  自动拼接
                </label>
                <div v-if="activeLayer.autotile" class="mt-1.5 flex items-center gap-1.5">
                  <select
                    :value="activeLayer.autotile.mode"
                    class="pb-input h-6 py-0 text-[10px]"
                    @change="changeAutotileMode(($event.target as HTMLSelectElement).value as '16' | '47')"
                  >
                    <option value="16">16 格（四邻）</option>
                    <option value="47">47 格（含内角）</option>
                  </select>
                  <button type="button" class="pb-btn h-6 px-2 py-0 text-[10px]" @click="repaintAllAutotile()">
                    重算边界
                  </button>
                </div>
                <p class="mt-1 text-[10px] leading-4 text-faint">
                  按相邻同类格自动换图。47 格图集需按 Indie Studio 顺序排列。
                </p>
              </div>
            </section>

            <section v-if="map && tool === 'object'" class="border-b border-line-soft p-3">
              <h3 class="text-xs font-semibold text-ink">放置对象</h3>
              <select v-model="objectKind" class="pb-input mt-2 h-7 py-0 text-[11px]">
                <option v-for="[kind, label] in OBJECT_KINDS" :key="kind" :value="kind">
                  {{ label }}
                </option>
              </select>
              <p class="mt-1 text-[10px] leading-4 text-faint">
                单击空白处新增，拖动调整位置，Delete 删除选中对象。
              </p>
            </section>

            <section v-if="selectedObject" class="border-b border-line-soft p-3">
              <div class="flex items-center justify-between">
                <h3 class="text-xs font-semibold text-ink">对象属性</h3>
                <button
                  type="button"
                  class="text-faint hover:text-danger"
                  title="删除对象"
                  @click="deleteSelectedObject()"
                >
                  <Trash2 :size="13" />
                </button>
              </div>
              <div class="mt-2 space-y-1.5">
                <label class="block text-[10px] text-faint">名称</label>
                <input
                  :value="selectedObject.name"
                  class="pb-input h-7 py-0 text-[11px]"
                  @input="patchObject({ name: ($event.target as HTMLInputElement).value })"
                />
                <label class="block text-[10px] text-faint">类型</label>
                <select
                  :value="selectedObject.kind"
                  class="pb-input h-7 py-0 text-[11px]"
                  @change="patchObject({ kind: ($event.target as HTMLSelectElement).value as MapObjectKind })"
                >
                  <option v-for="[kind, label] in OBJECT_KINDS" :key="kind" :value="kind">
                    {{ label }}
                  </option>
                </select>
                <div class="grid grid-cols-4 gap-1">
                  <div v-for="field in ['x', 'y', 'w', 'h'] as const" :key="field">
                    <label class="block text-[10px] text-faint">{{ field }}</label>
                    <input
                      :value="selectedObject[field]"
                      type="number"
                      step="0.5"
                      class="pb-input h-7 py-0 font-mono text-[11px]"
                      @change="patchNumberField(field, Number(($event.target as HTMLInputElement).value))"
                    />
                  </div>
                </div>
                <div class="grid grid-cols-2 gap-1">
                  <label class="flex items-center gap-1.5 text-[11px] text-muted">
                    <input
                      type="checkbox"
                      :checked="selectedObject.ySort"
                      @change="patchObject({ ySort: ($event.target as HTMLInputElement).checked })"
                    />
                    Y 排序
                  </label>
                  <label class="flex items-center gap-1.5 text-[11px] text-muted">
                    <input
                      type="checkbox"
                      :checked="selectedObject.solid"
                      @change="patchObject({ solid: ($event.target as HTMLInputElement).checked })"
                    />
                    碰撞
                  </label>
                  <label class="flex items-center gap-1.5 text-[11px] text-muted">
                    <input
                      type="checkbox"
                      :checked="selectedObject.roofFade"
                      @change="patchObject({ roofFade: ($event.target as HTMLInputElement).checked })"
                    />
                    屋檐淡出
                  </label>
                  <div class="flex items-center gap-1">
                    <span class="text-[10px] text-faint">高程</span>
                    <input
                      :value="selectedObject.elevation"
                      type="number"
                      min="0"
                      max="16"
                      class="pb-input h-6 w-14 py-0 font-mono text-[10px]"
                      @change="patchNumberField('elevation', Number(($event.target as HTMLInputElement).value))"
                    />
                  </div>
                </div>
                <div class="flex items-center justify-between pt-1">
                  <span class="text-[10px] font-medium text-muted">自定义属性</span>
                  <button type="button" class="text-[10px] text-accent" @click="addProp()">+ 添加</button>
                </div>
                <div
                  v-for="[key, value] in objectProperties"
                  :key="key"
                  class="flex items-center gap-1"
                >
                  <input :value="key" readonly class="pb-input h-6 w-20 py-0 font-mono text-[10px] text-muted" />
                  <input
                    :value="value"
                    class="pb-input h-6 flex-1 py-0 text-[10px]"
                    @change="setProp(key, ($event.target as HTMLInputElement).value)"
                  />
                  <button type="button" class="text-faint hover:text-danger" @click="removeProp(key)">
                    <X :size="11" />
                  </button>
                </div>
                <div class="pt-1 font-mono text-[10px] text-faint">
                  深度 {{ selectedDepth.toFixed(0) }} px
                </div>
              </div>
            </section>

            <section v-if="map" class="border-b border-line-soft p-3">
              <h3 class="flex items-center gap-1.5 text-xs font-semibold text-ink">
                <Maximize2 :size="13" class="text-accent" />
                扩展画布
              </h3>
              <p class="mt-1 text-[10px] leading-4 text-faint">
                向任意方向增加格数，现有瓦片与对象整体平移，且不能撤销。
              </p>
              <div class="mt-2 grid grid-cols-4 gap-1">
                <label v-for="side in EXPAND_SIDES" :key="side.key" class="block">
                  <span class="block text-[10px] text-faint">{{ side.label }}</span>
                  <input
                    v-model.number="expandEdges[side.key]"
                    type="number"
                    min="0"
                    max="256"
                    class="pb-input h-7 py-0 font-mono text-[11px]"
                  />
                </label>
              </div>
              <label class="mt-2 flex items-center gap-2 text-[11px] text-muted">
                <input v-model="extendTerrain" type="checkbox" class="size-3.5 accent-[#f2b84b]" />
                沿用边界地形填充新区域
              </label>
              <button
                type="button"
                class="pb-btn mt-2 h-7 w-full py-0 text-[11px]"
                :disabled="expanding"
                @click="applyExpand()"
              >
                <Maximize2 :size="12" />
                {{ expanding ? '扩展中…' : '扩展画布' }}
              </button>
              <p v-if="expandNotice" class="mt-1 text-[10px] text-green">{{ expandNotice }}</p>

              <div class="mt-3 border-t border-line-soft pt-2">
                <div class="flex items-center gap-1.5 text-[11px] font-medium text-ink">
                  <Sparkles :size="12" class="text-accent" />
                  AI 补全新区域
                </div>
                <p class="mt-1 text-[10px] leading-4 text-faint">
                  按 {{ map.tileSize }}px 瓦片与当前风格，到生成台产出配套素材。
                </p>
                <div class="mt-1.5 grid grid-cols-3 gap-1">
                  <button type="button" class="pb-btn h-7 px-1 py-0 text-[10px]" @click="openMapAI('tile')">
                    地形瓦片
                  </button>
                  <button type="button" class="pb-btn h-7 px-1 py-0 text-[10px]" @click="openMapAI('prop')">
                    装饰道具
                  </button>
                  <button type="button" class="pb-btn h-7 px-1 py-0 text-[10px]" @click="openMapAI('fx')">
                    特效
                  </button>
                </div>
              </div>
            </section>

            <section v-if="map" class="p-3">
              <h3 class="text-xs font-semibold text-ink">地图设置</h3>
              <div class="mt-2 space-y-1.5 text-[11px] text-muted">
                <label class="flex items-center gap-2">
                  <input
                    type="checkbox"
                    :checked="map.settings.ySort"
                    @change="setSetting('ySort', ($event.target as HTMLInputElement).checked)"
                  />
                  对象 Y 排序
                </label>
                <label class="flex items-center gap-2">
                  <input
                    type="checkbox"
                    :checked="map.settings.roofFade"
                    @change="setSetting('roofFade', ($event.target as HTMLInputElement).checked)"
                  />
                  行走时屋檐淡出
                </label>
                <div class="flex items-center gap-2">
                  <span class="text-[10px] text-faint">每级高差</span>
                  <input
                    :value="map.settings.heightStep"
                    type="number"
                    min="1"
                    max="128"
                    class="pb-input h-7 w-16 py-0 font-mono text-[11px]"
                    @change="setSetting('heightStep', Number(($event.target as HTMLInputElement).value))"
                  />
                  <span class="text-[10px] text-faint">px</span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-[10px] text-faint">环境色</span>
                  <input
                    :value="map.settings.ambient"
                    class="pb-input h-7 w-24 py-0 font-mono text-[11px]"
                    @change="setSetting('ambient', ($event.target as HTMLInputElement).value)"
                  />
                </div>
              </div>
            </section>
          </div>
        </aside>
      </div>

      <footer
        class="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-panel/70 px-3 font-mono text-[10px] text-faint"
      >
        <span v-if="cursorCell">x {{ cursorCell.x }} · y {{ cursorCell.y }}</span>
        <span v-else>—</span>
        <span v-if="map">{{ map.width }}×{{ map.height }} · {{ map.tileSize }}px</span>
        <span v-if="activeLayer">图层 {{ activeLayer.name }}</span>
        <span class="ml-auto">
          <template v-if="saving === 'pending'">正在保存…</template>
          <template v-else-if="saving === 'saved'">已保存 {{ lastSaved }}</template>
          <template v-else-if="saving === 'error'">保存失败</template>
        </span>
        <span>{{ tool === 'brush' ? 'B' : tool === 'eraser' ? 'E' : tool === 'rect' ? 'R' : tool === 'eyedropper' ? 'I' : tool === 'collision' ? 'C' : 'V' }}</span>
      </footer>
    </div>

    <div
      v-if="exportResult"
      class="fixed inset-x-0 bottom-4 mx-auto w-[min(560px,90vw)] rounded-xl border border-line bg-panel p-3 shadow-2xl"
    >
      <div class="flex items-start gap-3">
        <Download :size="16" class="mt-0.5 shrink-0 text-green" />
        <div class="min-w-0 flex-1">
          <div class="text-xs font-medium text-ink">已导出 {{ exportResult.files.length }} 个文件</div>
          <div class="mt-1 truncate font-mono text-[10px] text-faint" :title="exportResult.dir">
            {{ exportResult.dir }}
          </div>
          <div class="mt-1 text-[10px] text-faint">{{ exportResult.files.join(' · ') }}</div>
        </div>
        <button type="button" class="text-faint hover:text-ink" @click="exportResult = null">
          <X :size="14" />
        </button>
      </div>
    </div>

    <div
      v-if="newMapOpen"
      class="fixed inset-0 z-40 flex items-center justify-center bg-canvas/85 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="new-map-title"
      @click.self="newMapOpen = false"
    >
      <div class="pb-panel w-full max-w-sm p-5">
        <h2 id="new-map-title" class="text-sm font-semibold text-ink">新建地图</h2>
        <p class="mt-1 text-[11px] text-faint">正交网格 + 3/4 视角贴图，尺寸以格为单位。</p>
        <div class="mt-3 space-y-2">
          <label class="block text-[10px] text-faint">名称</label>
          <input v-model="newName" class="pb-input h-8 text-sm" />
          <div class="grid grid-cols-3 gap-2">
            <div>
              <label class="block text-[10px] text-faint">宽</label>
              <input v-model.number="newWidth" type="number" min="1" max="1024" class="pb-input h-8 font-mono text-xs" />
            </div>
            <div>
              <label class="block text-[10px] text-faint">高</label>
              <input v-model.number="newHeight" type="number" min="1" max="1024" class="pb-input h-8 font-mono text-xs" />
            </div>
            <div>
              <label class="block text-[10px] text-faint">瓦片</label>
              <select v-model.number="newTileSize" class="pb-input h-8 text-xs">
                <option v-for="size in TILE_SIZE_OPTIONS" :key="size" :value="size">{{ size }}px</option>
              </select>
            </div>
          </div>
          </div>
        <div class="mt-4 flex flex-wrap items-center justify-end gap-2">
          <button type="button" class="pb-btn text-xs" @click="newMapOpen = false">取消</button>
          <button type="button" class="pb-btn text-xs" @click="createNewMap()">
            创建空白地图
          </button>
          <button
            type="button"
            class="pb-btn pb-btn-primary text-xs"
            @click="generateMapTileset()"
          >
            <Sparkles :size="13" />
            AI 生成图集
          </button>
        </div>
      </div>
    </div>
  </div>
</template>