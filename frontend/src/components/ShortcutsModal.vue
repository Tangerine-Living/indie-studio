<script setup lang="ts">
import { SHORTCUT_GROUPS } from '../lib/shortcuts'
import AppModal from './AppModal.vue'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()
</script>

<template>
  <AppModal
    :open="open"
    title="快捷键"
    description="输入框中不触发工具快捷键。"
    wide
    @update:open="emit('update:open', $event)"
  >
    <div class="grid gap-5 sm:grid-cols-2">
      <section v-for="group in SHORTCUT_GROUPS" :key="group.title">
        <h3 class="text-[11px] font-medium tracking-wide text-faint">{{ group.title }}</h3>
        <ul class="mt-2 space-y-1.5">
          <li v-for="[keys, action] in group.items" :key="keys" class="flex items-start gap-2 text-[11px]">
            <span class="pb-kbd shrink-0">{{ keys }}</span>
            <span class="leading-relaxed text-muted">{{ action }}</span>
          </li>
        </ul>
      </section>
    </div>
  </AppModal>
</template>