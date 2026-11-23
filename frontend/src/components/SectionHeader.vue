<script setup lang="ts">
defineProps<{
  title: string
  hint?: string
}>()
</script>

<template>
  <div class="flex items-end justify-between gap-3">
    <div>
      <h2 class="text-sm font-semibold text-ink">{{ title }}</h2>
      <p v-if="hint" class="mt-0.5 text-xs text-muted">{{ hint }}</p>
    </div>
    <slot name="action" />
  </div>
</template>