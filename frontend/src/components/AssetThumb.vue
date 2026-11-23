<script setup lang="ts">
import { Image as ImageIcon, Loader2 } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import type { Asset } from '../domain/types'
import { platform } from '../platform'

const props = withDefaults(
  defineProps<{
    asset: Asset
    size?: number | 'fill'
  }>(),
  { size: 64 },
)

const url = ref<string | null>(null)
const failed = ref(false)
let requestId = 0

watch(
  () => [props.asset.id, props.asset.updatedAt],
  () => {
    const id = (requestId += 1)
    failed.value = false
    url.value = null
    platform
      .assetURL(props.asset.id, 'thumb.png')
      .then((next) => {
        if (id === requestId) url.value = next
      })
      .catch(() => {
        if (id === requestId) failed.value = true
      })
  },
  { immediate: true },
)

const pixels = computed(() => (typeof props.size === 'number' ? props.size : 64))
const boxStyle = computed(() =>
  props.size === 'fill'
    ? { width: '100%', height: '100%' }
    : { width: `${props.size}px`, height: `${props.size}px` },
)
</script>

<template>
  <div
    class="transparency flex shrink-0 items-center justify-center overflow-hidden rounded-md border border-line"
    :style="boxStyle"
  >
    <img
      v-if="url && !failed"
      :src="url"
      :alt="props.asset.name"
      class="pixelated h-full w-full object-contain"
      draggable="false"
      @error="failed = true"
    />
    <ImageIcon v-else-if="failed" :size="Math.max(14, pixels / 4)" class="text-faint" />
    <Loader2 v-else :size="Math.max(12, pixels / 5)" class="animate-spin text-faint" />
  </div>
</template>