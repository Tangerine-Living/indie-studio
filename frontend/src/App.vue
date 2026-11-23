<script setup lang="ts">
import { AlertTriangle, Loader2 } from 'lucide-vue-next'
import { onBeforeUnmount, onMounted } from 'vue'
import ConfirmHost from './components/ConfirmHost.vue'
import ErrorToast from './components/ErrorToast.vue'
import ImageLightbox from './components/ImageLightbox.vue'
import PixelMark from './components/PixelMark.vue'
import ProjectPicker from './screens/ProjectPicker.vue'
import Workspace from './screens/Workspace.vue'
import { useAppStore } from './stores/app'

const store = useAppStore()

/** Desktop windows keep their layout fixed; only image previews zoom. */
function blockPageZoom(event: WheelEvent) {
  if (event.ctrlKey) event.preventDefault()
}

function blockZoomKeys(event: KeyboardEvent) {
  if (!event.ctrlKey && !event.metaKey) return
  if (['+', '-', '=', '_', '0'].includes(event.key)) event.preventDefault()
}

onMounted(() => {
  void store.boot()
  window.addEventListener('wheel', blockPageZoom, { passive: false })
  window.addEventListener('keydown', blockZoomKeys)
})

onBeforeUnmount(() => {
  window.removeEventListener('wheel', blockPageZoom)
  window.removeEventListener('keydown', blockZoomKeys)
})
</script>

<template>
  <div class="h-screen">
    <div v-if="store.status === 'loading'" class="flex h-full flex-col items-center justify-center gap-5 bg-canvas">
      <PixelMark :size="40" />
      <div class="flex items-center gap-2 text-xs text-muted">
        <Loader2 :size="14" class="animate-spin text-accent" />
        正在加载项目…
      </div>
      <span class="font-mono text-[10px] text-faint">Indie Studio {{ store.info.version }}</span>
    </div>

    <div
      v-else-if="store.status === 'error'"
      class="flex h-full flex-col items-center justify-center gap-4 bg-canvas px-6"
    >
      <div class="flex size-12 items-center justify-center rounded-xl border border-red/30 bg-red/10 text-red">
        <AlertTriangle :size="20" />
      </div>
      <div class="text-center">
        <div class="text-sm font-medium text-ink">Indie Studio 无法启动</div>
        <p class="mt-1 max-w-md text-xs leading-relaxed text-muted">
          {{ store.bootError ?? '启动失败' }}
        </p>
      </div>
      <button type="button" class="pb-btn pb-btn-primary" @click="store.boot()">重试</button>
    </div>

    <template v-else>
      <Workspace v-if="store.session" />
      <ProjectPicker v-else />
    </template>

    <ErrorToast />
    <ImageLightbox />
    <ConfirmHost />
  </div>
</template>