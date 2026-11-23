<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { computed, onBeforeUnmount, watch } from 'vue'

// Every open modal is tracked so Escape closes only the topmost one, and the
// body stays scroll-locked until the last one closes.
const openModals: symbol[] = []
const token = Symbol('modal')

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    wide?: boolean
    /** `raised` sits above other modals, for confirmations on top of a panel. */
    layer?: 'base' | 'raised'
  }>(),
  { wide: false, layer: 'base' },
)

const overlayClass = computed(() => (props.layer === 'raised' ? 'z-[80]' : 'z-40'))
const panelClass = computed(() => (props.layer === 'raised' ? 'z-[81]' : 'z-50'))

const emit = defineEmits<{ 'update:open': [boolean] }>()

function close() {
  emit('update:open', false)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || openModals[openModals.length - 1] !== token) return
  event.preventDefault()
  close()
}

function detach() {
  const index = openModals.indexOf(token)
  if (index >= 0) openModals.splice(index, 1)
  document.removeEventListener('keydown', onKeydown)
}

watch(
  () => props.open,
  (open) => {
    if (typeof document === 'undefined') return
    if (open) {
      if (!openModals.includes(token)) openModals.push(token)
      document.addEventListener('keydown', onKeydown)
    } else {
      detach()
    }
    document.body.style.overflow = openModals.length ? 'hidden' : ''
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (typeof document === 'undefined') return
  detach()
  document.body.style.overflow = openModals.length ? 'hidden' : ''
})
</script>

<template>
  <Teleport to="body">
    <template v-if="props.open">
      <div class="fixed inset-0 bg-black/65 backdrop-blur-[2px]" :class="overlayClass" @click="close" />
      <div
        role="dialog"
        aria-modal="true"
        class="fixed top-1/2 left-1/2 flex max-h-[88vh] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-2xl border border-line bg-panel shadow-[0_24px_80px_rgba(0,0,0,0.55)] outline-none"
        :class="[panelClass, props.wide ? 'w-[min(96vw,900px)]' : 'w-[min(94vw,620px)]']"
      >
        <div class="flex shrink-0 items-start justify-between gap-4 border-b border-line px-5 py-4">
          <div class="min-w-0">
            <h2 class="text-base font-semibold text-ink">{{ props.title }}</h2>
            <p v-if="props.description" class="mt-0.5 text-xs text-muted">{{ props.description }}</p>
          </div>
          <button type="button" class="pb-icon-btn size-8 shrink-0" @click="close">
            <X :size="15" />
          </button>
        </div>
        <div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <slot />
        </div>
        <div
          v-if="$slots.footer"
          class="flex shrink-0 items-center justify-end gap-2 border-t border-line bg-panel-2/60 px-5 py-3"
        >
          <slot name="footer" />
        </div>
      </div>
    </template>
  </Teleport>
</template>