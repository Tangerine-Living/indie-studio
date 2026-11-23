<script setup lang="ts">
defineProps<{
  label: string
  hint?: string
}>()
</script>

<template>
  <label class="block">
    <span class="pb-label">{{ label }}</span>
    <span class="mt-1.5 block"><slot /></span>
    <span v-if="hint" class="mt-1 block text-[11px] text-faint">{{ hint }}</span>
  </label>
</template>