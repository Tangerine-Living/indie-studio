<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import AppModal from './AppModal.vue'
import { dialogState, settleDialog } from '../lib/dialogs'

const confirmButton = ref<HTMLButtonElement | null>(null)
const promptInput = ref<HTMLInputElement | null>(null)

const canConfirm = computed(
  () => dialogState.mode !== 'prompt' || dialogState.promptValue.trim().length > 0,
)

watch(
  () => dialogState.open,
  async (open) => {
    if (!open) return
    await nextTick()
    if (dialogState.mode === 'prompt') promptInput.value?.focus()
    else confirmButton.value?.focus()
  },
)

function onOpenChange(open: boolean) {
  if (!open) settleDialog(false)
}

function confirm() {
  if (canConfirm.value) settleDialog(true)
}

// Unmounting the shell must not leave a caller awaiting a dialog that can
// never be answered.
onBeforeUnmount(() => {
  if (dialogState.open) settleDialog(false)
})
</script>

<template>
  <AppModal
    :open="dialogState.open"
    :title="dialogState.title"
    layer="raised"
    @update:open="onOpenChange"
  >
    <p v-if="dialogState.message" class="text-xs leading-relaxed whitespace-pre-line text-muted">
      {{ dialogState.message }}
    </p>
    <pre
      v-if="dialogState.detail"
      class="mt-3 max-h-52 overflow-auto rounded-lg border border-line bg-canvas p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-muted"
    >{{ dialogState.detail }}</pre>
    <input
      v-if="dialogState.mode === 'prompt'"
      ref="promptInput"
      v-model="dialogState.promptValue"
      class="pb-input mt-3"
      :placeholder="dialogState.promptPlaceholder"
      @keydown.enter.prevent="confirm()"
    />

    <template #footer>
      <button type="button" class="pb-btn text-xs" @click="settleDialog(false)">
        {{ dialogState.cancelText }}
      </button>
      <button
        ref="confirmButton"
        type="button"
        class="pb-btn text-xs"
        :class="dialogState.tone === 'danger' ? 'pb-btn-danger' : 'pb-btn-primary'"
        :disabled="!canConfirm"
        @click="confirm()"
      >
        {{ dialogState.confirmText }}
      </button>
    </template>
  </AppModal>
</template>