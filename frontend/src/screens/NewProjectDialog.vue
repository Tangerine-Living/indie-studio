<script setup lang="ts">
import { FolderOpen, Loader2 } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import type { Engine } from '../domain/types'
import { TILE_SIZES } from '../domain/types'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import AppModal from '../components/AppModal.vue'
import FieldBox from '../components/FieldBox.vue'

const open = defineModel<boolean>('open', { required: true })

const store = useAppStore()

const TEMPLATES = [
  { id: 'blank', name: '空白项目', hint: '只有项目文件与默认调色板' },
  { id: 'town-square', name: '小镇广场', hint: '附带示例任务、地图与占位素材' },
] as const

const name = ref('')
const parentDir = ref('')
const engine = ref<string>('godot')
const tileSize = ref(32)
const templateId = ref<string>('blank')
const formError = ref<string | null>(null)

// Every new project starts empty: no name, and the blank template selected.
watch(open, (isOpen) => {
  if (!isOpen) return
  name.value = ''
  templateId.value = 'blank'
  formError.value = null
})

/** Shown in the footer so the final folder is obvious before creating. */
const targetPath = computed(() => {
  const parent = parentDir.value.trim().replace(/[\\/]+$/, '')
  const projectName = name.value.trim()
  return parent && projectName ? `${parent}\\${projectName}` : ''
})

/** Describes the files actually written for the selected template. */
const templateSummary = computed(() =>
  templateId.value === 'town-square'
    ? '会额外写入一条示例任务、草地占位素材、一张示例地图、示例设定与数值表；这些内容都可以删除。'
    : '只创建 project.json、默认 16 色调色板与说明文件；素材、任务、地图与 .workbench 缓存会在首次使用时生成。',
)

function selectTemplate(id: string) {
  templateId.value = id
}

async function browse() {
  const dir = await platform.pickDirectory('选择项目存放的文件夹')
  if (dir) parentDir.value = dir
}

async function submit() {
  formError.value = null
  if (!name.value.trim()) {
    formError.value = '请输入项目名称'
    return
  }
  if (!parentDir.value.trim()) {
    formError.value = '请选择项目存放的父文件夹'
    return
  }
  const ok = await store.createProject({
    parentDir: parentDir.value.trim(),
    name: name.value.trim(),
    engine: engine.value as Engine,
    tileSize: tileSize.value,
    template: templateId.value,
    paletteId: 'default',
  })
  if (ok) open.value = false
}
</script>

<template>
  <AppModal
    v-model:open="open"
    title="新建项目"
    description="项目会作为一个普通文件夹保存到你选择的位置，可直接用 git 管理。"
  >
    <div class="space-y-4">
      <FieldBox label="项目名称" hint="支持中文、空格；不能以点或空格结尾">
        <input
          v-model="name"
          class="pb-input"
          maxlength="80"
          autofocus
          placeholder="例如：我的小镇"
        />
      </FieldBox>

      <FieldBox label="存放位置">
        <div class="flex gap-2">
          <input
            v-model="parentDir"
            class="pb-input font-mono text-xs"
            placeholder="选择或输入一个文件夹路径"
          />
          <button type="button" class="pb-btn shrink-0 px-2.5" @click="browse()">
            <FolderOpen :size="15" />
            浏览
          </button>
        </div>
      </FieldBox>

      <div class="grid grid-cols-2 gap-4">
        <FieldBox label="目标引擎" hint="导出时默认使用">
          <select v-model="engine" class="pb-input">
            <option value="godot">Godot 4</option>
            <option value="tiled">Tiled</option>
            <option value="generic">通用格式</option>
          </select>
        </FieldBox>
        <FieldBox label="瓦片尺寸" hint="之后可在设置中修改">
          <select v-model.number="tileSize" class="pb-input">
            <option v-for="size in TILE_SIZES" :key="size" :value="size">
              {{ size }} × {{ size }} px
            </option>
          </select>
        </FieldBox>
      </div>

      <div>
        <span class="pb-label">模板</span>
        <div class="mt-2 grid grid-cols-2 gap-2">
          <button
            v-for="item in TEMPLATES"
            :key="item.id"
            type="button"
            class="rounded-lg border px-3 py-2.5 text-left transition-colors"
            :class="
              templateId === item.id
                ? 'border-accent/50 bg-accent-soft'
                : 'border-line bg-panel-2 hover:bg-panel-3'
            "
            @click="selectTemplate(item.id)"
          >
            <span class="block text-sm font-medium text-ink">{{ item.name }}</span>
            <span class="mt-0.5 block text-[11px] leading-snug text-muted">{{ item.hint }}</span>
          </button>
        </div>
      </div>

      <p v-if="formError" class="rounded-lg border border-red/25 bg-red/10 px-3 py-2 text-xs text-red">
        {{ formError }}
      </p>

      <p class="rounded-lg border border-line bg-panel-2/60 px-3 py-2 text-[11px] leading-relaxed text-muted">
        <span v-if="targetPath" class="mb-1 block break-all font-mono text-[10px] text-ink">
          {{ targetPath }}
        </span>
        {{ templateSummary }}
      </p>
    </div>

    <template #footer>
      <button type="button" class="pb-btn" @click="open = false">取消</button>
      <button type="button" class="pb-btn pb-btn-primary" :disabled="store.busy" @click="submit()">
        <Loader2 v-if="store.busy" :size="15" class="animate-spin" />
        创建项目
      </button>
    </template>
  </AppModal>
</template>