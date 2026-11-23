<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{ size?: number }>(), { size: 28 })

const PATTERN = ['accca', 'a...a', 'a...a', 'accca', 'a....']
const CELL = computed(() => props.size / 5)
const CELLS = computed(() =>
  PATTERN.flatMap((row) =>
    row
      .split('')
      .map((char) => (char === 'a' ? '#f2b84b' : char === 'c' ? '#6ee7a8' : 'transparent')),
  ),
)
</script>

<template>
  <div
    class="grid shrink-0 rounded-[7px] border border-accent/30 bg-[#1a1205] p-[3px]"
    :style="{
      width: `${props.size + 6}px`,
      height: `${props.size + 6}px`,
      gridTemplateColumns: `repeat(5, ${CELL}px)`,
    }"
    aria-hidden="true"
  >
    <span
      v-for="(color, index) in CELLS"
      :key="index"
      :style="{ width: `${CELL}px`, height: `${CELL}px`, background: color }"
    />
  </div>
</template>