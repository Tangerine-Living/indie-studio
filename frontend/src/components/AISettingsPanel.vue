<script setup lang="ts">
import {
  Bot,
  Check,
  Image as ImageIcon,
  Loader2,
  MessageSquareText,
  Save,
  Wifi,
} from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { AI_PROVIDERS } from '../domain/types'
import type {
  AIProviderId,
  AISettings,
  AISettingsInput,
  AITestResult,
  AITextProviderMode,
} from '../domain/types'
import { useAppStore } from '../stores/app'
import FieldBox from './FieldBox.vue'
import SectionHeader from './SectionHeader.vue'

const store = useAppStore()

const aiForm = reactive<AISettingsInput>({
  defaultProvider: 'comfyui',
  comfy: { baseUrl: 'http://127.0.0.1:8188', workflowName: '默认工作流', workflow: '' },
  openai: {
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-image-1',
    editModel: '',
    textModel: 'gpt-4o-mini',
    size: '1024x1024',
    textProvider: 'shared',
    textBaseUrl: '',
    apiKey: '',
    clearKey: false,
    textApiKey: '',
    textClearKey: false,
  },
})
const testing = ref<AIProviderId | null>(null)
const testMessage = ref<AITestResult | null>(null)
const savedNotice = ref<string | null>(null)
let savedTimer: ReturnType<typeof setTimeout> | null = null

const ownsText = computed(() => aiForm.openai.textProvider === 'own')

function syncAIForm(next: AISettings | null) {
  if (!next) return
  aiForm.defaultProvider = next.defaultProvider
  aiForm.comfy = { ...next.comfy }
  aiForm.openai = {
    baseUrl: next.openai.baseUrl,
    model: next.openai.model,
    editModel: next.openai.editModel,
    textModel: next.openai.textModel,
    size: next.openai.size,
    textProvider: next.openai.textProvider,
    textBaseUrl: next.openai.textBaseUrl,
    apiKey: '',
    clearKey: false,
    textApiKey: '',
    textClearKey: false,
  }
}

watch(() => store.aiSettings, syncAIForm, { immediate: true })

onMounted(() => {
  if (!store.aiSettings) void store.loadAISettings()
})

function snapshot(): AISettingsInput {
  return {
    defaultProvider: aiForm.defaultProvider,
    comfy: { ...aiForm.comfy },
    openai: { ...aiForm.openai },
  }
}

function setTextProvider(mode: AITextProviderMode) {
  aiForm.openai.textProvider = mode
  if (mode === 'shared') {
    aiForm.openai.textApiKey = ''
    aiForm.openai.textClearKey = false
  }
}

function showSaved(text: string) {
  savedNotice.value = text
  if (savedTimer) clearTimeout(savedTimer)
  savedTimer = setTimeout(() => {
    if (savedNotice.value === text) savedNotice.value = null
  }, 4000)
}

async function saveAI() {
  const saved = await store.saveAISettings(snapshot())
  if (!saved) return
  syncAIForm(saved)
  testMessage.value = null
  showSaved('已保存 AI 设置')
}

/**
 * The backend tests the *stored* settings, so save the form first — otherwise
 * the check would silently validate stale values.
 */
async function testAI(id: AIProviderId) {
  testing.value = id
  testMessage.value = null
  try {
    const saved = await store.saveAISettings(snapshot())
    if (!saved) {
      testMessage.value = { ok: false, message: store.error?.message ?? '保存设置失败' }
      return
    }
    syncAIForm(saved)
    const result = await store.testAIProvider(id)
    testMessage.value = result ?? {
      ok: false,
      message: store.error?.message ?? '连接失败',
      detail: store.error?.detail,
    }
  } finally {
    testing.value = null
  }
}
</script>

<template>
  <section class="pb-card p-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <SectionHeader
        title="AI 生成"
        hint="图像、文本与本地工作流分开配置；密钥只保存在本机"
      />
      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <span
          class="pb-chip font-mono text-[10px]"
          :class="store.aiSettings?.openai.hasKey ? 'text-green' : 'text-faint'"
        >
          图像 Key {{ store.aiSettings?.openai.hasKey ? '已保存' : '未保存' }}
        </span>
        <span
          v-if="ownsText"
          class="pb-chip font-mono text-[10px]"
          :class="store.aiSettings?.openai.textHasKey ? 'text-green' : 'text-faint'"
        >
          文本 Key {{ store.aiSettings?.openai.textHasKey ? '已保存' : '未保存' }}
        </span>
        <span v-else class="pb-chip font-mono text-[10px] text-faint">文本沿用图像接口</span>
      </div>
    </div>

    <div class="mt-4 rounded-lg border border-line bg-panel-2/40 px-3 py-2.5">
      <div class="flex flex-wrap items-center gap-3">
        <span class="text-[11px] text-muted">默认供应商</span>
        <select v-model="aiForm.defaultProvider" class="pb-input h-7 px-2 text-[11px]">
          <option v-for="provider in AI_PROVIDERS" :key="provider" :value="provider">
            {{ provider === 'comfyui' ? 'ComfyUI（本地）' : 'OpenAI 兼容（云）' }}
          </option>
        </select>
      </div>
      <p class="mt-1 text-[11px] text-faint">新建 AI 草稿时默认选中的通道</p>
    </div>

    <div class="mt-4 grid gap-4 xl:grid-cols-2">
      <div class="rounded-lg border border-line bg-panel-2/30 p-3">
        <div class="flex items-center gap-2">
          <ImageIcon :size="13" class="text-accent" />
          <h3 class="text-xs font-medium text-ink">图像模型（OpenAI 兼容）</h3>
        </div>
        <div class="mt-3 grid gap-3">
          <FieldBox label="接口地址">
            <input
              v-model="aiForm.openai.baseUrl"
              class="pb-input font-mono text-xs"
              placeholder="https://api.openai.com/v1"
            />
          </FieldBox>
          <div class="grid gap-3 sm:grid-cols-2">
            <FieldBox label="图像模型">
              <input
                v-model="aiForm.openai.model"
                class="pb-input font-mono text-xs"
                placeholder="gpt-image-1"
              />
            </FieldBox>
            <FieldBox label="默认尺寸" hint="生成台会按宽高比自动换算">
              <select v-model="aiForm.openai.size" class="pb-input font-mono text-xs">
                <option value="1024x1024">1024 × 1024（方）</option>
                <option value="1536x1024">1536 × 1024（横）</option>
                <option value="1024x1536">1024 × 1536（竖）</option>
              </select>
            </FieldBox>
          </div>
          <FieldBox
            label="编辑模型（局部重绘）"
            hint="留空沿用图像模型；若图像模型不支持蒙版编辑（如 MiniMax Image），换成支持 /images/edits 的模型"
          >
            <input
              v-model="aiForm.openai.editModel"
              class="pb-input font-mono text-xs"
              placeholder="留空沿用图像模型"
            />
          </FieldBox>
          <FieldBox
            label="图像 API Key"
            :hint="store.aiSettings?.openai.hasKey ? '已保存；留空表示不改' : undefined"
          >
            <input
              v-model="aiForm.openai.apiKey"
              type="password"
              class="pb-input font-mono text-xs"
              autocomplete="new-password"
              :placeholder="store.aiSettings?.openai.hasKey ? '••••••••（已保存，留空不改）' : 'sk-...'"
            />
          </FieldBox>
          <label class="flex items-center gap-2 text-[11px] text-muted">
            <input v-model="aiForm.openai.clearKey" type="checkbox" class="size-3.5 accent-[var(--color-accent)]" />
            清除已保存的图像 API Key
          </label>
        </div>
      </div>

      <div class="rounded-lg border border-line bg-panel-2/30 p-3">
        <div class="flex items-center gap-2">
          <MessageSquareText :size="13" class="text-accent" />
          <h3 class="text-xs font-medium text-ink">文本模型（AI 文案与提示词）</h3>
        </div>
        <div class="mt-3 grid gap-3">
          <div>
            <span class="pb-label">接口来源</span>
            <div class="mt-1.5 inline-flex overflow-hidden rounded-md border border-line">
              <button
                type="button"
                class="px-2.5 py-1 text-[11px] transition-colors"
                :class="!ownsText ? 'bg-accent text-canvas' : 'bg-panel-2 text-muted hover:text-ink'"
                :aria-pressed="!ownsText"
                @click="setTextProvider('shared')"
              >
                沿用图像接口
              </button>
              <button
                type="button"
                class="px-2.5 py-1 text-[11px] transition-colors"
                :class="ownsText ? 'bg-accent text-canvas' : 'bg-panel-2 text-muted hover:text-ink'"
                :aria-pressed="ownsText"
                @click="setTextProvider('own')"
              >
                独立接口
              </button>
            </div>
            <p class="mt-1 text-[11px] leading-relaxed text-faint">
              {{ ownsText
                ? '文本请求走下面单独的地址与密钥，适合文本和图像来自不同供应商。'
                : '文本请求复用上面的图像接口地址与 API Key。' }}
            </p>
          </div>
          <FieldBox label="文本模型" hint="填接口实际支持的对话模型名；带线路号就写完整，如 gemini-4-argon@156">
            <input
              v-model="aiForm.openai.textModel"
              class="pb-input font-mono text-xs"
              placeholder="gpt-4o-mini"
            />
          </FieldBox>
          <template v-if="ownsText">
            <FieldBox label="文本接口地址">
              <input
                v-model="aiForm.openai.textBaseUrl"
                class="pb-input font-mono text-xs"
                placeholder="https://api.example.com/v1"
              />
            </FieldBox>
            <FieldBox
              label="文本 API Key"
              :hint="store.aiSettings?.openai.textHasKey ? '已保存；留空表示不改' : undefined"
            >
              <input
                v-model="aiForm.openai.textApiKey"
                type="password"
                class="pb-input font-mono text-xs"
                autocomplete="new-password"
                :placeholder="store.aiSettings?.openai.textHasKey ? '••••••••（已保存，留空不改）' : 'sk-...'"
              />
            </FieldBox>
            <label class="flex items-center gap-2 text-[11px] text-muted">
              <input v-model="aiForm.openai.textClearKey" type="checkbox" class="size-3.5 accent-[var(--color-accent)]" />
              清除已保存的文本 API Key
            </label>
          </template>
        </div>
      </div>

      <div class="rounded-lg border border-line bg-panel-2/30 p-3 xl:col-span-2">
        <div class="flex items-center gap-2">
          <Bot :size="13" class="text-accent" />
          <h3 class="text-xs font-medium text-ink">本地 ComfyUI</h3>
        </div>
        <div class="mt-3 grid gap-3 sm:grid-cols-2">
          <FieldBox label="ComfyUI 地址">
            <input
              v-model="aiForm.comfy.baseUrl"
              class="pb-input font-mono text-xs"
              placeholder="http://127.0.0.1:8188"
            />
          </FieldBox>
          <FieldBox label="工作流名称" hint="仅用于本项目内的标识">
            <input v-model="aiForm.comfy.workflowName" class="pb-input" placeholder="默认文生图" />
          </FieldBox>
        </div>
        <FieldBox label="工作流（API 格式 JSON）" hint="节点用 _meta.title 标记 PROMPT / NEGATIVE / SEED / SIZE；局部重绘另需 IMAGE 与 MASK 节点">
          <textarea
            v-model="aiForm.comfy.workflow"
            rows="6"
            spellcheck="false"
            class="pb-input h-36 resize-y font-mono text-[10px] leading-relaxed"
            placeholder="从 ComfyUI 的「Save (API Format)」导出后粘贴到这里"
          />
        </FieldBox>
      </div>
    </div>

    <div
      v-if="testMessage"
      class="pb-fade-in mt-3 rounded-lg border px-3 py-2 text-[11px]"
      :class="testMessage.ok
        ? 'border-line bg-panel-2 text-ink'
        : 'border-[#c94f4f]/40 bg-[#c94f4f]/10 text-ink'"
    >
      <div class="flex items-center gap-2">
        <Wifi :size="12" :class="testMessage.ok ? 'text-green' : 'text-[#e07a7a]'" />
        <span class="flex-1">{{ testMessage.message }}</span>
      </div>
      <p v-if="testMessage.detail" class="mt-1 break-all font-mono text-[10px] leading-relaxed text-faint">
        {{ testMessage.detail }}
      </p>
      <div v-if="testMessage.text" class="mt-2 flex items-start gap-2 border-t border-line-soft pt-2">
        <MessageSquareText
          :size="12"
          class="mt-0.5 shrink-0"
          :class="testMessage.text.ok ? 'text-green' : 'text-[#e07a7a]'"
        />
        <div class="min-w-0 flex-1">
          <div>{{ testMessage.text.message }}</div>
          <p v-if="testMessage.text.detail" class="mt-0.5 break-all font-mono text-[10px] text-faint">
            {{ testMessage.text.detail }}
          </p>
        </div>
      </div>
    </div>

    <div class="mt-4 flex flex-wrap items-center gap-2">
      <button
        type="button"
        class="pb-btn pb-btn-primary px-3 py-1.5 text-xs"
        :disabled="store.busy"
        @click="saveAI()"
      >
        <Save :size="13" />
        保存 AI 设置
      </button>
      <button
        type="button"
        class="pb-btn px-3 py-1.5 text-xs"
        :disabled="testing !== null"
        @click="testAI('openai')"
      >
        <Loader2 v-if="testing === 'openai'" :size="13" class="animate-spin" />
        <Wifi v-else :size="13" />
        测试接口
      </button>
      <button
        type="button"
        class="pb-btn px-3 py-1.5 text-xs"
        :disabled="testing !== null"
        @click="testAI('comfyui')"
      >
        <Loader2 v-if="testing === 'comfyui'" :size="13" class="animate-spin" />
        <Bot v-else :size="13" />
        测试 ComfyUI
      </button>
      <span
        v-if="savedNotice"
        class="pb-fade-in flex items-center gap-1.5 rounded-md border border-green/30 bg-green/10 px-2.5 py-1.5 text-[11px] text-green"
      >
        <Check :size="12" />
        {{ savedNotice }}
      </span>
    </div>
  </section>
</template>