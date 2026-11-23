<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number
    tone?: 'accent' | 'green' | 'blue' | 'red'
  }>(),
  { tone: 'accent' },
)

const TONES = {
  accent: 'bg-accent',
  green: 'bg-green',
  blue: 'bg-blue',
  red: 'bg-red',
} as const

const width = computed(() => `${Math.min(100, Math.max(0, props.value))}%`)
</script>

<template>
  <div class="h-1.5 w-full overflow-hidden rounded-full bg-line-soft">
    <div
      class="h-full rounded-full transition-[width] duration-300"
      :class="TONES[tone]"
      :style="{ width }"
    />
  </div>
</template>