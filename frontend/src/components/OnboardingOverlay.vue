<script setup lang="ts">
import { ArrowRight, Images, Map as MapIcon, Sparkles } from 'lucide-vue-next'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useAppStore } from '../stores/app'

// Kept in one place so every build uses the same first-run flag.
const STORAGE_KEY = ['pixelbench', 'onboarding', 'v1'].join('.')

const store = useAppStore()
const visible = ref(false)
const step = ref(0)
const card = ref<HTMLElement | null>(null)
const primary = ref<HTMLButtonElement | null>(null)

try {
  visible.value = localStorage.getItem(STORAGE_KEY) !== 'done'
} catch {
  visible.value = false
}

const STEPS = [
  {
    icon: Sparkles,
    title: '任务先行',
    body: '在任务看板里把想法拆成可验收的活。任务可以关联素材、套用 brief 模板，或直接跳到生成台起草。',
    route: 'quests' as const,
    action: '去看看任务',
  },
  {
    icon: Images,
    title: '素材与生成',
    body: '导入 PNG / GIF / WebP，或用参数化建筑与 AI 适配器生成草稿；像素编辑器修细节，后处理链把 AI 图变成干净像素图。',
    route: 'assets' as const,
    action: '打开素材库',
  },
  {
    icon: MapIcon,
    title: '地图与导出',
    body: '在 2.5D 地图里自动拼接地形、放对象与碰撞，用行走预览验证比例，然后导出 Tiled 或 Godot 4。',
    route: 'maps' as const,
    action: '进入地图',
  },
]

const current = computed(() => STEPS[step.value]!)
const last = computed(() => step.value === STEPS.length - 1)

function finish(destination?: 'quests' | 'assets' | 'maps') {
  visible.value = false
  try {
    localStorage.setItem(STORAGE_KEY, 'done')
  } catch {
    // Private browsing may block storage; the guide simply reappears next time.
  }
  if (destination) store.setRoute(destination)
}

function next() {
  if (last.value) {
    finish(current.value.route)
    return
  }
  step.value += 1
}

/** The guide can be replayed from the command palette. */
function replay() {
  step.value = 0
  visible.value = true
}

/**
 * Modal behaviour for the guide: Esc skips it, Tab stays inside the card, and
 * the primary action takes focus on every step.
 */
function onKeyDown(event: KeyboardEvent) {
  if (!visible.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    finish()
    return
  }
  if (event.key !== 'Tab') return
  const focusables = Array.from(
    card.value?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? [],
  )
  if (focusables.length === 0) return
  const first = focusables[0]!
  const last = focusables[focusables.length - 1]!
  const active = document.activeElement
  const inside = card.value?.contains(active ?? null) ?? false
  if (event.shiftKey && (!inside || active === first)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (!inside || active === last)) {
    event.preventDefault()
    first.focus()
  }
}

watch([visible, step], async () => {
  if (!visible.value) return
  await nextTick()
  primary.value?.focus({ preventScroll: true })
})

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('pixelbench:onboarding', replay)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('pixelbench:onboarding', replay)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-30 flex items-center justify-center bg-black/70 p-4 backdrop-blur-[2px]">
      <div
        ref="card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="onboarding-title"
        class="pb-fade-in w-[min(94vw,560px)] rounded-2xl border border-line bg-panel p-6 shadow-[0_24px_80px_rgba(0,0,0,0.55)]"
      >
        <div class="flex items-center justify-between gap-3">
          <span class="pb-chip border-accent/25 bg-accent-soft text-accent">3 分钟上手 · {{ step + 1 }}/{{ STEPS.length }}</span>
          <button type="button" class="text-[11px] text-faint hover:text-ink" @click="finish()">跳过</button>
        </div>

        <div class="mt-5 flex items-start gap-3">
          <span class="flex size-10 shrink-0 items-center justify-center rounded-xl border border-line bg-panel-2 text-accent">
            <component :is="current.icon" :size="18" />
          </span>
          <div class="min-w-0">
            <h2 id="onboarding-title" class="text-base font-semibold tracking-tight text-ink">
              {{ current.title }}
            </h2>
            <p class="mt-1.5 text-xs leading-relaxed text-muted">{{ current.body }}</p>
          </div>
        </div>

        <div class="mt-5 flex items-center gap-1.5">
          <button
            v-for="(entry, index) in STEPS"
            :key="entry.title"
            type="button"
            class="h-1.5 flex-1 rounded-full transition"
            :class="index <= step ? 'bg-accent' : 'bg-panel-3'"
            :title="entry.title"
            :aria-label="`第 ${index + 1} 步：${entry.title}`"
            :aria-current="index === step ? 'step' : undefined"
            @click="step = index"
          />
        </div>

        <div class="mt-5 flex items-center justify-between gap-2">
          <button
            type="button"
            class="pb-btn px-3 py-1.5 text-xs"
            :disabled="step === 0"
            @click="step -= 1"
          >
            上一步
          </button>
          <button
            ref="primary"
            type="button"
            class="pb-btn pb-btn-primary px-3 py-1.5 text-xs"
            @click="next()"
          >
            {{ last ? current.action : '下一步' }}
            <ArrowRight :size="13" />
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>