<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import type { GameMap } from '../domain/types'

const props = defineProps<{
  map: GameMap
  tilesetIndex: number
  images: ReadonlyMap<string, CanvasImageSource>
  selected: { set: number; tile: number }
}>()

const emit = defineEmits<{
  (e: 'select', set: number, tile: number): void
}>()

const canvas = ref<HTMLCanvasElement | null>(null)

function paint() {
  const el = canvas.value
  const set = props.map.tilesets[props.tilesetIndex]
  if (!el || !set) return
  const image = props.images.get(set.assetId)
  const ctx = el.getContext('2d')
  if (!ctx) return
  const cols = set.columns
  const rows = Math.max(1, Math.ceil(set.tileCount / cols))
  el.width = cols * set.tileSize
  el.height = rows * set.tileSize
  ctx.clearRect(0, 0, el.width, el.height)
  ctx.fillStyle = '#111a24'
  ctx.fillRect(0, 0, el.width, el.height)
  if (image) {
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(image, 0, 0, el.width, el.height)
  }
  ctx.strokeStyle = 'rgba(160, 190, 220, 0.25)'
  ctx.lineWidth = 1
  for (let x = 1; x < cols; x += 1) {
    ctx.beginPath()
    ctx.moveTo(x * set.tileSize + 0.5, 0)
    ctx.lineTo(x * set.tileSize + 0.5, el.height)
    ctx.stroke()
  }
  for (let y = 1; y < rows; y += 1) {
    ctx.beginPath()
    ctx.moveTo(0, y * set.tileSize + 0.5)
    ctx.lineTo(el.width, y * set.tileSize + 0.5)
    ctx.stroke()
  }
  if (props.selected.set === props.tilesetIndex && props.selected.tile < set.tileCount) {
    const col = props.selected.tile % cols
    const row = Math.floor(props.selected.tile / cols)
    ctx.strokeStyle = '#6ec6e8'
    ctx.lineWidth = 2
    ctx.strokeRect(col * set.tileSize + 1, row * set.tileSize + 1, set.tileSize - 2, set.tileSize - 2)
  } else if (props.selected.set === props.tilesetIndex) {
    ctx.strokeStyle = 'rgba(110, 198, 232, 0.4)'
    ctx.lineWidth = 2
    ctx.strokeRect(1, 1, el.width - 2, el.height - 2)
  }
}

function pick(event: MouseEvent) {
  const el = canvas.value
  const set = props.map.tilesets[props.tilesetIndex]
  if (!el || !set) return
  const rect = el.getBoundingClientRect()
  const sx = ((event.clientX - rect.left) / rect.width) * el.width
  const sy = ((event.clientY - rect.top) / rect.height) * el.height
  const col = Math.floor(sx / set.tileSize)
  const row = Math.floor(sy / set.tileSize)
  const tile = row * set.columns + col
  if (col < 0 || col >= set.columns || tile < 0 || tile >= set.tileCount) return
  emit('select', props.tilesetIndex, tile)
}

watch(
  () => [props.tilesetIndex, props.selected, props.images, props.map.tilesets] as const,
  paint,
  { deep: true },
)

onMounted(paint)
</script>

<template>
  <div class="min-w-0">
    <canvas
      ref="canvas"
      class="w-full cursor-pointer rounded-md border border-line"
      style="image-rendering: pixelated"
      @click="pick"
    />
    <div class="mt-1 flex items-center justify-between font-mono text-[10px] text-faint">
      <span>#{{ selected.set }} · 瓦片 {{ selected.tile }}</span>
      <span>{{ map.tilesets[tilesetIndex]?.tileCount ?? 0 }} 格</span>
    </div>
  </div>
</template>