<script setup lang="ts">
import { AlertCircle, Check, Loader2 } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'

const store = useAppStore()
</script>

<template>
  <span v-if="store.saveState === 'saving'" class="inline-flex items-center gap-1.5 text-[11px] text-accent">
    <Loader2 :size="12" class="animate-spin" />
    保存中…
  </span>
  <span v-else-if="store.saveState === 'saved'" class="inline-flex items-center gap-1.5 text-[11px] text-green">
    <Check :size="13" />
    已保存
  </span>
  <span v-else-if="store.saveState === 'error'" class="inline-flex items-center gap-1.5 text-[11px] text-red">
    <AlertCircle :size="12" />
    保存失败
  </span>
  <span v-else class="inline-flex items-center gap-1.5 text-[11px] text-faint">
    <span class="size-1.5 rounded-full bg-faint" />
    已保存
  </span>
</template>