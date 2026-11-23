<script setup lang="ts">
defineProps<{
  title: string
  hint?: string
}>()
</script>

<template>
  <div
    class="flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-line px-6 py-14 text-center"
  >
    <div v-if="$slots.icon" class="mb-1 text-faint">
      <slot name="icon" />
    </div>
    <div class="text-sm font-medium text-ink">{{ title }}</div>
    <div v-if="hint" class="max-w-md text-xs leading-relaxed text-muted">{{ hint }}</div>
    <div v-if="$slots.action" class="mt-3">
      <slot name="action" />
    </div>
  </div>
</template>