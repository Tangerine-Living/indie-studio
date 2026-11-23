<script setup lang="ts">
import { AlertTriangle, LockKeyhole, X } from 'lucide-vue-next'
import { computed } from 'vue'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const canForce = computed(
  () => store.error?.code === 'PROJECT_LOCKED' && Boolean(store.lockedPath),
)
</script>

<template>
  <div
    v-if="store.error"
    role="alert"
    class="pb-fade-in fixed right-4 bottom-4 z-[70] w-[min(92vw,420px)] overflow-hidden rounded-xl border border-red/35 bg-panel shadow-[0_18px_60px_rgba(0,0,0,0.6)]"
  >
    <div class="flex items-start gap-3 px-4 py-3">
      <div class="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg bg-red/10 text-red">
        <LockKeyhole v-if="canForce" :size="14" />
        <AlertTriangle v-else :size="14" />
      </div>
      <div class="min-w-0 flex-1">
        <span class="font-mono text-[10px] tracking-wide text-red">{{ store.error.code }}</span>
        <p class="mt-0.5 text-sm leading-snug text-ink">{{ store.error.message }}</p>
        <p
          v-if="store.error.detail"
          class="mt-1 max-h-20 overflow-y-auto font-mono text-[10px] break-all text-faint"
        >
          {{ store.error.detail }}
        </p>
        <div class="mt-3 flex items-center gap-2">
          <button
            v-if="canForce"
            type="button"
            class="pb-btn pb-btn-danger px-2.5 py-1 text-xs"
            :disabled="store.busy"
            @click="store.forceOpen()"
          >
            强制打开
          </button>
          <button
            type="button"
            class="pb-btn pb-btn-ghost px-2.5 py-1 text-xs"
            @click="store.clearError()"
          >
            关闭
          </button>
        </div>
      </div>
      <button
        type="button"
        class="text-faint transition hover:text-ink"
        aria-label="关闭提示"
        @click="store.clearError()"
      >
        <X :size="14" />
      </button>
    </div>
  </div>
</template>