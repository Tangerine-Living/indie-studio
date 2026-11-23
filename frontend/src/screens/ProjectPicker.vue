<script setup lang="ts">
import { FolderOpen, Loader2, Plus } from 'lucide-vue-next'
import { ref } from 'vue'
import { platform } from '../platform'
import { useAppStore } from '../stores/app'
import { shortPath } from '../lib/utils'
import EmptyState from '../components/EmptyState.vue'
import PixelMark from '../components/PixelMark.vue'
import NewProjectDialog from './NewProjectDialog.vue'

const store = useAppStore()
const newOpen = ref(false)

async function chooseExisting() {
  const file = await platform.pickProjectFile()
  if (file) await store.openProject(file)
}
</script>

<template>
  <div class="h-full overflow-y-auto bg-canvas">
    <div class="mx-auto flex min-h-full w-full max-w-4xl flex-col px-10 py-10">
      <header class="flex items-center gap-3">
        <PixelMark :size="26" />
        <div class="flex items-baseline gap-2">
          <span class="text-sm font-semibold tracking-tight text-ink">Indie Studio</span>
          <span class="text-[11px] text-faint">像素工坊</span>
        </div>
        <span class="ml-auto font-mono text-[10px] text-faint">
          v{{ store.info.version }} · {{ store.info.mode === 'browser' ? '浏览器预览' : store.info.platform }}
        </span>
      </header>

      <section class="mt-16 grid gap-12 lg:grid-cols-[0.95fr_1.05fr] lg:items-start">
        <div>
          <h1 class="max-w-sm text-[28px] leading-[1.25] font-semibold tracking-tight text-ink">
            打开项目，<br />开始做像素素材。
          </h1>
          <p class="mt-4 max-w-xs text-[13px] leading-relaxed text-muted">
            项目、素材与生成结果都保存在本地文件夹，可以直接用 git 管理。
          </p>

          <div class="mt-8 flex flex-wrap items-center gap-2.5">
            <button type="button" class="pb-btn pb-btn-primary px-4 py-2" @click="newOpen = true">
              <Plus :size="15" />
              新建项目
            </button>
            <button type="button" class="pb-btn px-4 py-2" :disabled="store.busy" @click="chooseExisting()">
              <Loader2 v-if="store.busy" :size="14" class="animate-spin" />
              <FolderOpen v-else :size="14" />
              打开已有项目
            </button>
          </div>

          <p class="mt-6 text-[11px] text-faint">本地优先 · 无账号 · 不上传项目内容</p>
        </div>

        <div class="rounded-xl border border-line bg-panel/60">
          <div class="flex items-baseline justify-between px-4 py-3">
            <span class="text-[12px] font-medium text-ink">最近项目</span>
            <span class="font-mono text-[10px] text-faint">{{ store.recent.length }}</span>
          </div>
          <EmptyState
            v-if="store.recent.length === 0"
            class="m-3 border-0"
            title="还没有最近项目"
            hint="新建一个项目，或打开已有项目里的 project.json。"
          >
            <template #icon><FolderOpen :size="20" /></template>
          </EmptyState>
          <ul v-else class="border-t border-line-soft">
            <li v-for="project in store.recent" :key="project.path">
              <button
                type="button"
                class="group flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-panel-2 disabled:opacity-60"
                :disabled="store.busy"
                @click="store.openProject(project.path)"
              >
                <FolderOpen :size="15" class="shrink-0 text-faint group-hover:text-accent" />
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-[13px] font-medium text-ink">{{ project.name }}</span>
                  <span class="mt-0.5 block truncate font-mono text-[10px] text-faint">
                    {{ shortPath(project.path, 48) }}
                  </span>
                </span>
                <span class="shrink-0 font-mono text-[10px] text-faint">
                  {{ new Date(project.lastOpenedAt).toLocaleDateString('zh-CN') }}
                </span>
              </button>
            </li>
          </ul>
        </div>
      </section>
    </div>

    <NewProjectDialog v-model:open="newOpen" />
  </div>
</template>