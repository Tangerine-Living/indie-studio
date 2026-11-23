<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    label: string
    value: string | number
    sub?: string
    tone?: 'accent' | 'green' | 'blue' | 'neutral'
  }>(),
  { tone: 'accent' },
)

const TONES: Record<string, string> = {
  accent: 'bg-accent-soft text-accent',
  green: 'bg-green/10 text-green',
  blue: 'bg-blue/10 text-blue',
  neutral: 'bg-panel-3 text-muted',
}

const iconTone = computed(() => TONES[props.tone])
</script>

<template>
  <div class="pb-card flex items-start gap-3 p-4">
    <div
      v-if="$slots.icon"
      class="flex size-9 shrink-0 items-center justify-center rounded-lg"
      :class="iconTone"
    >
      <slot name="icon" />
    </div>
    <div class="min-w-0">
      <div class="text-[11px] font-medium tracking-wide text-muted">{{ label }}</div>
      <div class="mt-1 font-mono text-2xl leading-none font-semibold text-ink">{{ value }}</div>
      <div v-if="sub" class="mt-1.5 text-[11px] text-faint">{{ sub }}</div>
    </div>
  </div>
</template>