import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { VueWrapper } from '@vue/test-utils'
import App from './App.vue'
import i18n from './i18n'
import { useAppStore } from './stores/app'
import { useDraftsStore } from './stores/drafts'

/**
 * Smoke tests that mount the real app against the browser mock platform. They
 * catch runtime template errors that `vue-tsc` cannot see, and walk every route.
 */
async function bootApp() {
  const pinia = createPinia()
  useDraftsStore(pinia).reset()
  const wrapper = mount(App, {
    global: { plugins: [pinia, i18n] },
    attachTo: document.body,
  })
  await flushPromises()
  const store = useAppStore(pinia)
  return { wrapper, store }
}

async function openDemo(store: ReturnType<typeof useAppStore>) {
  const demo = store.recent.find((project) => project.name === '小镇广场') ?? store.recent[0]
  expect(demo).toBeTruthy()
  const ok = await store.openProject(demo.path)
  expect(ok).toBe(true)
  await flushPromises()
}

/** The mock text model needs a saved key, just like the real chat endpoint. */
async function enableMockAI(store: ReturnType<typeof useAppStore>) {
  const saved = await store.saveAISettings({
    defaultProvider: 'openai',
    comfy: { baseUrl: 'http://127.0.0.1:8188', workflowName: '默认文生图', workflow: '' },
    openai: {
      baseUrl: 'https://api.openai.com/v1',
      model: 'gpt-image-1',
      textModel: 'gpt-4o-mini',
      size: '1024x1024',
      apiKey: 'sk-test',
    },
  })
  expect(saved).toBeTruthy()
}

/** Opens one of the tabbed settings sections by its label. */
async function openSettingsTab(
  wrapper: ReturnType<typeof mount>,
  label: string,
) {
  const tab = wrapper.findAll('button').find((button) => button.text() === label)
  expect(tab).toBeTruthy()
  await tab!.trigger('click')
  await flushPromises()
}

/** The mock text call resolves after 250 ms; let it finish before asserting. */
async function settleAIText() {
  await new Promise((resolve) => setTimeout(resolve, 300))
  await flushPromises()
}

/** Header-only PNG the mock reads dimensions from (no real pixels). */
function pngDataURL(width: number, height: number): string {
  const bytes = new Uint8Array(24)
  bytes.set([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a], 0)
  bytes[16] = (width >>> 24) & 0xff
  bytes[17] = (width >>> 16) & 0xff
  bytes[18] = (width >>> 8) & 0xff
  bytes[19] = width & 0xff
  bytes[20] = (height >>> 24) & 0xff
  bytes[21] = (height >>> 16) & 0xff
  bytes[22] = (height >>> 8) & 0xff
  bytes[23] = height & 0xff
  return `data:image/png;base64,${btoa(String.fromCharCode(...bytes))}`
}

/** The app-owned dialog host renders into body, not inside the wrapper. */
function activeDialog(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[role="dialog"]')
}

async function clickDialogButton(label: string) {
  const dialogs = Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).reverse()
  for (const dialog of dialogs) {
    const target = Array.from(dialog.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === label,
    )
    if (target) {
      target.click()
      await flushPromises()
      return
    }
  }
  expect.fail(`${label} button in dialog`)
}

let mounted: VueWrapper | null = null

afterEach(() => {
  mounted?.unmount()
  mounted = null
  document.body.innerHTML = ''
})

describe('Indie Studio app', () => {
  it('boots the picker and lists the seeded demo project', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    expect(store.status).toBe('ready')
    expect(store.recent.length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('新建项目')
    expect(wrapper.text()).toContain('小镇广场')
  })

  it('creates a blank project from the picker and opens an empty workspace', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    if (store.session) {
      await store.closeProject()
      await flushPromises()
    }

    await wrapper
      .findAll('button')
      .find((button) => button.text().trim() === '新建项目')!
      .trigger('click')
    await flushPromises()
    const dialog = activeDialog()!
    const name = dialog.querySelectorAll('input')[0] as HTMLInputElement
    // A new project starts with no name; the template picker must not fill one.
    expect(name.value).toBe('')
    name.value = '空白测试'
    name.dispatchEvent(new Event('input', { bubbles: true }))
    const parent = dialog.querySelectorAll('input')[1] as HTMLInputElement
    parent.value = 'C:\\Work'
    parent.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()
    expect(dialog.textContent).toContain('C:\\Work\\空白测试')
    // The default template is blank, and the summary follows the selection.
    expect(dialog.textContent).toContain('只创建 project.json')

    const town = Array.from(dialog.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('小镇广场'),
    )
    town!.click()
    await flushPromises()
    expect(dialog.textContent).toContain('示例任务、草地占位素材')

    const blank = Array.from(dialog.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('空白项目'),
    )
    blank!.click()
    await flushPromises()
    const create = Array.from(dialog.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '创建项目',
    )
    create!.click()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 30))
    await flushPromises()

    expect(store.session?.project.name).toBe('空白测试')
    expect(store.session?.project.settings.template).toBe('blank')
    expect(store.quests).toHaveLength(0)
    expect(store.assets).toHaveLength(0)
    expect(store.tables).toHaveLength(0)
    expect(wrapper.text()).toContain('还没有素材')
  })

  it('opens the demo project and renders every route', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    expect(wrapper.text()).toContain('素材总数')

    const routes = [
      ['quests', '任务'],
      ['lore', '设定'],
      ['tables', '数值表'],
      ['assets', '素材库'],
      ['generate', '提交到队列'],
      ['maps', '行走预览'],
      ['export', '打开导出目录'],
      ['settings', '风格指南'],
      ['dashboard', '任务进展'],
    ] as const

    for (const [route, text] of routes) {
      store.setRoute(route)
      await flushPromises()
      expect(wrapper.text(), route).toContain(text)
    }
  })

  it('pins the shell to the viewport and scrolls long screens inside it', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    // body is overflow:hidden; the shell must own the viewport height or tall
    // screens get clipped instead of scrolling.
    expect(wrapper.element.classList.contains('h-screen')).toBe(true)
    await openDemo(store)
    store.setRoute('generate')
    await flushPromises()
    expect(wrapper.find('main').classes()).toContain('overflow-y-auto')
  })

  it('opens the command palette, searches and focuses a quest', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()
    expect(document.querySelector('[aria-label="命令面板"]')).toBeTruthy()
    expect(document.querySelector('input[aria-label="命令面板搜索"]')).toBeTruthy()

    const input = document.querySelector<HTMLInputElement>('input[aria-label="命令面板搜索"]')
    expect(input).toBeTruthy()
    input!.value = '草地瓦片'
    input!.dispatchEvent(new Event('input'))
    await flushPromises()

    const palette = document.querySelector('[aria-label="命令面板"]')!
    const result = Array.from(palette.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('草地瓦片 16 变体'),
    )
    expect(result).toBeTruthy()
    result!.click()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 30))
    await flushPromises()

    expect(document.querySelector('[aria-label="命令面板"]')).toBeNull()
    expect(store.route).toBe('quests')
    expect(document.body.textContent).toContain('编辑任务')
  })

  it('replays the onboarding guide from the command palette', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()
    const input = document.querySelector<HTMLInputElement>('input[aria-label="命令面板搜索"]')
    expect(input).toBeTruthy()
    input!.value = '引导'
    input!.dispatchEvent(new Event('input'))
    await flushPromises()

    const command = Array.from(
      document.querySelectorAll<HTMLButtonElement>('[aria-label="命令面板"] button'),
    ).find((button) => button.textContent?.includes('重新查看新手引导'))
    expect(command).toBeTruthy()
    command!.click()
    await flushPromises()

    const guide = () => document.querySelector('[aria-labelledby="onboarding-title"]')
    expect(guide()).toBeTruthy()
    expect(guide()?.textContent).toContain('1/3')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(guide()).toBeNull()
  })

  it('edits a setting, saves it manually and lists the entries that reference it', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('lore')
    await flushPromises()

    expect(store.entities.map((entity) => entity.name)).toEqual(
      expect.arrayContaining(['小镇广场', '旅人']),
    )

    const entry = wrapper
      .findAll('button')
      .find((button) => button.text().includes('小镇广场') && button.text().includes('定稿'))
    expect(entry).toBeTruthy()
    await entry!.trigger('click')
    await flushPromises()

    const name = wrapper.find('input[aria-label="设定名称"]')
    expect((name.element as HTMLInputElement).value).toBe('小镇广场')
    // The linked map resolves, and the traveller's "初到" relation shows as a backlink.
    expect(wrapper.text()).toContain('40×30')
    expect(wrapper.text()).toContain('被这些设定引用')
    expect(wrapper.text()).toContain('初到')

    await name.setValue('老广场')
    await flushPromises()
    // The editor is manual-save now: the library keeps the old name until 保存.
    expect(wrapper.text()).toContain('未保存')
    expect(store.entities.some((entity) => entity.name === '老广场')).toBe(false)

    const save = wrapper
      .findAll('button')
      .find((button) => button.attributes('title') === '保存这条设定')
    expect(save).toBeTruthy()
    await save!.trigger('click')
    await flushPromises()
    expect(store.entities.some((entity) => entity.name === '老广场')).toBe(true)
    expect(wrapper.text()).not.toContain('未保存')
  })

  it('creates a setting from the library toolbar', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('lore')
    await flushPromises()

    const before = store.entities.length
    const create = wrapper.findAll('button').find((button) => button.text().includes('新建设定'))
    expect(create).toBeTruthy()
    await create!.trigger('click')
    await flushPromises()

    expect(store.entities.length).toBe(before + 1)
    expect(store.entities.some((entity) => entity.name === '未命名设定')).toBe(true)
    expect(wrapper.text()).toContain('未命名设定')
    expect((wrapper.find('input[aria-label="设定名称"]').element as HTMLInputElement).value).toBe(
      '未命名设定',
    )
  })

  it('edits a data-table cell, autosaves it and adds a row', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    expect(store.tables.map((table) => table.key)).toEqual(
      expect.arrayContaining(['items', 'skills', 'enemies']),
    )

    const items = wrapper
      .findAll('button')
      .find((button) => button.text().includes('物品') && button.text().includes('items'))
    expect(items).toBeTruthy()
    await items!.trigger('click')
    await flushPromises()

    const name = wrapper.find('input[aria-label="数值表名称"]')
    expect((name.element as HTMLInputElement).value).toBe('物品')

    const firstCell = wrapper.find('input[aria-label="名称 第 1 行"]')
    expect((firstCell.element as HTMLInputElement).value).toBe('苹果')
    await firstCell.setValue('红苹果')
    // The editor debounces writes past the 700 ms window.
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    const saved = store.tables.find((table) => table.key === 'items')!
    const nameColumn = saved.columns.find((column) => column.key === 'name')!
    expect(saved.rows[0]?.values[nameColumn.id]).toBe('红苹果')

    const before = saved.rows.length
    const addRow = wrapper.findAll('button').find((button) => button.text().includes('添加行'))
    expect(addRow).toBeTruthy()
    await addRow!.trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    expect(store.tables.find((table) => table.key === 'items')!.rows.length).toBe(before + 1)

    // The new row focuses its first cell; Enter at the bottom adds another.
    const focused = document.activeElement as HTMLInputElement | null
    expect(focused?.getAttribute('aria-label')).toBe(`名称 第 ${before + 1} 行`)
    const lastCell = wrapper.find(`input[aria-label="名称 第 ${before + 1} 行"]`)
    await lastCell.trigger('keydown', { key: 'Enter' })
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    expect(store.tables.find((table) => table.key === 'items')!.rows.length).toBe(before + 2)
  })

  it('selects rows and deletes them in one batch', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('物品') && button.text().includes('items'))!
      .trigger('click')
    await flushPromises()

    const before = store.tables.find((table) => table.key === 'items')!.rows.length
    // Add two filled rows, then delete them in one batch — seed rows stay intact.
    const addRow = wrapper.findAll('button').find((button) => button.text().includes('添加行'))
    await addRow!.trigger('click')
    await addRow!.trigger('click')
    await flushPromises()
    await wrapper
      .find(`input[aria-label="名称 第 ${before + 1} 行"]`)
      .setValue('批量甲')
    await wrapper
      .find(`input[aria-label="名称 第 ${before + 2} 行"]`)
      .setValue('批量乙')
    await flushPromises()
    await wrapper.find(`input[aria-label="选择第 ${before + 1} 行"]`).setValue(true)
    await wrapper.find(`input[aria-label="选择第 ${before + 2} 行"]`).setValue(true)
    await flushPromises()
    expect(wrapper.text()).toContain('已选 2 行')

    const remove = wrapper.findAll('button').find((button) => button.text().includes('删除选中'))
    expect(remove).toBeTruthy()
    await remove!.trigger('click')
    await flushPromises()
    await clickDialogButton('删除 2 行')
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    expect(store.tables.find((table) => table.key === 'items')!.rows.length).toBe(before)
  })

  it('edits a wide row in the row form', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('物品') && button.text().includes('items'))!
      .trigger('click')
    await flushPromises()

    const edit = wrapper
      .findAll('button')
      .find((button) => button.attributes('title')?.includes('编辑这一行'))
    expect(edit).toBeTruthy()
    await edit!.trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('编辑行')

    const rowDialog = Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).find(
      (dialog) => dialog.textContent?.includes('编辑行'),
    )
    expect(rowDialog).toBeTruthy()
    const items = store.tables.find((table) => table.key === 'items')!
    const nameColumn = items.columns.find((column) => column.key === 'name')!
    const original = String(items.rows[0]?.values[nameColumn.id] ?? '')
    const field = Array.from(rowDialog!.querySelectorAll('input')).find(
      (input) => input.value === original,
    )
    expect(field).toBeTruthy()
    const updated = `${original} 改`
    field!.value = updated
    field!.dispatchEvent(new Event('input', { bubbles: true }))
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    const saved = store.tables.find((table) => table.key === 'items')!
    const savedColumn = saved.columns.find((column) => column.key === 'name')!
    expect(saved.rows[0]?.values[savedColumn.id]).toBe(updated)

    await clickDialogButton('完成')
  })

  it('keeps decimals and commits numeric cells on change', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('技能') && button.text().includes('skills'))!
      .trigger('click')
    await flushPromises()

    const cooldown = wrapper.find('input[aria-label="冷却（秒） 第 1 行"]')
    expect((cooldown.element as HTMLInputElement).value).toBe('0.8')
    await cooldown.setValue('0.25')
    await cooldown.trigger('change')
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()

    const skills = store.tables.find((table) => table.key === 'skills')!
    const column = skills.columns.find((item) => item.key === 'cooldown')!
    expect(skills.rows[0]?.values[column.id]).toBe(0.25)
  })

  it('asks before dropping edits whose save failed', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('物品') && button.text().includes('items'))!
      .trigger('click')
    await flushPromises()

    const name = wrapper.find('input[aria-label="数值表名称"]')
    await name.setValue('  ')
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    expect(wrapper.text()).toContain('保存失败')

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('技能') && button.text().includes('skills'))!
      .trigger('click')
    await flushPromises()
    expect(activeDialog()?.textContent).toContain('这张表还有修改没能保存')
    await clickDialogButton('丢弃修改')
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 30))
    await flushPromises()
    expect((wrapper.find('input[aria-label="数值表名称"]').element as HTMLInputElement).value).toBe(
      '技能',
    )
  })

  it('jumps to another data table from the palette while the screen is open', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()
    const input = document.querySelector<HTMLInputElement>('input[aria-label="命令面板搜索"]')!
    input.value = '技能'
    input.dispatchEvent(new Event('input'))
    await flushPromises()

    const palette = document.querySelector('[aria-label="命令面板"]')!
    const result = Array.from(palette.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('技能'),
    )
    expect(result).toBeTruthy()
    result!.click()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 30))
    await flushPromises()

    expect(store.route).toBe('tables')
    expect((wrapper.find('input[aria-label="数值表名称"]').element as HTMLInputElement).value).toBe(
      '技能',
    )
  })

  it('asks before deleting a filled data-table row', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('物品') && button.text().includes('items'))!
      .trigger('click')
    await flushPromises()

    const before = store.tables.find((table) => table.key === 'items')!.rows.length
    // Delete the last seeded row (旧地图); enemies' drops point at other rows.
    const remove = wrapper
      .findAll('button')
      .filter((button) => button.attributes('title') === '删除这一行')[3]
    expect(remove).toBeTruthy()
    await remove!.trigger('click')
    await flushPromises()

    expect(activeDialog()?.textContent).toContain('删除第 4 行')
    await clickDialogButton('删除行')
    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    expect(store.tables.find((table) => table.key === 'items')!.rows.length).toBe(before - 1)
  })

  it('removes a freshly added blank row without a prompt', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('物品') && button.text().includes('items'))!
      .trigger('click')
    await flushPromises()

    const before = store.tables.find((table) => table.key === 'items')!.rows.length
    const addRow = wrapper.findAll('button').find((button) => button.text().includes('添加行'))
    await addRow!.trigger('click')
    await flushPromises()

    const removes = wrapper
      .findAll('button')
      .filter((button) => button.attributes('title') === '删除这一行')
    await removes[removes.length - 1].trigger('click')
    await flushPromises()
    expect(activeDialog()).toBeNull()

    await new Promise((resolve) => setTimeout(resolve, 800))
    await flushPromises()
    expect(store.tables.find((table) => table.key === 'items')!.rows.length).toBe(before)
  })

  it('creates a data table and exports a valid one', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('tables')
    await flushPromises()

    const before = store.tables.length
    const create = wrapper.findAll('button').find((button) => button.text().includes('新建表'))
    expect(create).toBeTruthy()
    await create!.trigger('click')
    await flushPromises()
    expect(store.tables.length).toBe(before + 1)
    const created = store.tables.find((table) => table.name === '未命名数值表')!
    expect(created).toBeTruthy()
    expect((wrapper.find('input[aria-label="数值表名称"]').element as HTMLInputElement).value).toBe(
      '未命名数值表',
    )

    const enemies = store.tables.find((table) => table.key === 'enemies')!
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('敌人') && button.text().includes('enemies'))!
      .trigger('click')
    await flushPromises()
    const exportButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('导出 JSON / CSV'))
    expect(exportButton).toBeTruthy()
    const exported = await store.exportTable(enemies.id)
    expect(exported?.dir).toBe('C:\\PixelBench\\示例项目\\小镇广场\\exports\\tables\\enemies')
  })

  it('opens the record behind a recent activity row', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('dashboard')
    await flushPromises()
    expect(wrapper.text()).toContain('任务进展')

    const row = wrapper
      .findAll('button')
      .find((button) => button.text().includes('小镇广场贴图验收'))
    expect(row).toBeTruthy()
    await row!.trigger('click')
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 30))
    await flushPromises()

    expect(store.route).toBe('quests')
    expect(document.body.textContent).toContain('编辑任务')
    expect(document.body.textContent).toContain('小镇广场贴图验收')
  })

  it('drafts a generation brief and copy from a lore entry', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    await enableMockAI(store)
    store.setRoute('lore')
    await flushPromises()

    const entry = wrapper
      .findAll('button')
      .find((button) => button.text().includes('旅人') && button.text().includes('草稿'))
    expect(entry).toBeTruthy()
    await entry!.trigger('click')
    await flushPromises()

    const copyButton = wrapper.findAll('button').find((button) => button.text().includes('AI 文案'))
    await copyButton!.trigger('click')
    await settleAIText()
    const dialog = activeDialog()
    expect(dialog?.textContent).toContain('AI 文案草稿')
    expect((dialog?.querySelector('textarea') as HTMLTextAreaElement).value).toContain('旅人')
    await clickDialogButton('取消')

    const generateButton = wrapper.findAll('button').find((button) => button.text().includes('生成素材'))
    await generateButton!.trigger('click')
    await settleAIText()

    // The prompt is confirmed in a dialog before the workbench opens.
    const promptDialog = Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).find(
      (dialog) => dialog.textContent?.includes('提示词确认'),
    )
    expect(promptDialog).toBeTruthy()
    const prompt = (promptDialog!.querySelector('textarea') as HTMLTextAreaElement).value
    expect(prompt).toContain('像素风')
    const open = Array.from(promptDialog!.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '打开生成台',
    )
    open!.click()
    await flushPromises()

    expect(store.route).toBe('generate')
    expect(wrapper.text()).toContain('来自设定「旅人」')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toContain('像素风')
  })

  it('moves the setting selection when a kind filter hides it', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('lore')
    await flushPromises()

    await wrapper
      .findAll('button')
      .find((button) => button.text().startsWith('地点'))!
      .trigger('click')
    await flushPromises()
    expect((wrapper.find('input[aria-label="设定名称"]').element as HTMLInputElement).value).toBe(
      '老广场',
    )

    await wrapper
      .findAll('button')
      .find((button) => button.text().startsWith('角色'))!
      .trigger('click')
    await flushPromises()
    expect((wrapper.find('input[aria-label="设定名称"]').element as HTMLInputElement).value).toBe(
      '旅人',
    )
  })

  it('summarises filtered assets and clears the filters', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const cards = () =>
      wrapper.findAll('button').filter((button) => button.classes().includes('group')).length
    const before = cards()
    expect(before).toBeGreaterThan(1)

    await wrapper.find('select[aria-label="按类型筛选"]').setValue('sprite')
    await flushPromises()
    expect(wrapper.text()).toContain('筛选出')
    expect(cards()).toBeLessThan(before)

    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('清除筛选'))!
      .trigger('click')
    await flushPromises()
    expect(cards()).toBe(before)
    expect(wrapper.text()).not.toContain('筛选出')
  })

  it('keeps versionless assets out of the export list', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('export')
    await flushPromises()

    // 杂货铺（草稿） is seeded without a saved version.
    const row = wrapper.findAll('li').find((item) => item.text().includes('杂货铺'))
    expect(row).toBeTruthy()
    const button = row!.find('button')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('title')).toContain('还没有保存过版本')
  })

  it('opens the quest editor from a board card', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('quests')
    await flushPromises()

    const card = wrapper.findAll('article')[0]
    expect(card).toBeTruthy()
    await card.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('编辑任务')
    expect(document.body.textContent).toContain('验收清单')
    expect(document.body.textContent).toContain('保存任务')
  })

  it('approving an asset completes the quests waiting in review', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    const quest = store.quests.find((item) => item.status === 'review')!
    expect(quest).toBeTruthy()
    const asset = store.assets.find(
      (item) => item.questIds.includes(quest.id) && item.status !== 'approved',
    )!
    expect(asset).toBeTruthy()

    const result = await store.setAssetStatus(asset.id, 'approved')
    expect(result?.asset.status).toBe('approved')
    expect(result?.promoted.map((item) => item.id)).toContain(quest.id)
    expect(store.quests.find((item) => item.id === quest.id)?.status).toBe('done')
    await flushPromises()
  })

  it('hands a quest brief to the generate screen and consumes it once', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    const quest = store.quests[0]!
    store.openGenerateFromQuest(quest)
    expect(store.route).toBe('generate')
    expect(store.generateIntent?.questId).toBe(quest.id)
    expect(store.generateIntent?.assetType).toBe(quest.brief.assetType)
    await flushPromises()
    // The generate screen mounts and consumes the intent exactly once.
    expect(store.generateIntent).toBeNull()
    expect(wrapper.text()).toContain(quest.title)
    expect(wrapper.text()).toContain('来自任务')
  })

  it('opens the seeded 2.5D map, paints and exports Tiled files', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('maps')
    await flushPromises()

    expect(store.maps).toHaveLength(1)
    expect(wrapper.text()).toContain('小镇广场')
    expect(wrapper.text()).toContain('地面')

    const summary = store.maps[0]!
    const map = await store.openMap(summary.id)
    expect(map).toBeTruthy()
    expect(map!.layers).toHaveLength(6)
    expect(map!.settings.ySort).toBe(true)
    expect(map!.objects.some((object) => object.kind === 'spawn')).toBe(true)

    const result = await store.exportMap(summary.id)
    expect(result?.dir).toContain('exports\\maps\\小镇广场')
    expect(result?.format).toBe('tiled')
    expect(result?.files.some((file) => file.endsWith('.tmx'))).toBe(true)
    expect(result?.files.some((file) => file.endsWith('.json'))).toBe(true)

    const godot = await store.exportMap(summary.id, 'godot')
    expect(godot?.dir).toContain('小镇广场-godot')
    expect(godot?.format).toBe('godot')
    for (const name of ['project.godot', '小镇广场.tscn', '小镇广场.tres', 'pixelbench_tiles.gd']) {
      expect(godot?.files).toContain(name)
    }
    await flushPromises()
  })

  it('imports an asset and shows it in the grid', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const before = store.assets.length
    const summary = await store.importAssets(['C:\\导入\\木头箱子.png'], 'prop')
    expect(summary?.imported).toHaveLength(1)
    await flushPromises()

    expect(store.assets.length).toBe(before + 1)
    expect(wrapper.text()).toContain('木头箱子')
    expect(wrapper.text()).toContain(`${before + 1} 个`)
  })

  it('imports through the toolbar and reports duplicates and bad files', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const prompt = vi.spyOn(window, 'prompt').mockReturnValue('C:\\导入\\工具栏导入.png')
    const importBtn = wrapper.findAll('button').find((button) => button.text().trim() === '导入')
    expect(importBtn).toBeTruthy()

    const runImport = async () => {
      await importBtn!.trigger('click')
      await flushPromises()
      await new Promise((resolve) => setTimeout(resolve, 30))
      await flushPromises()
    }

    await runImport()
    expect(wrapper.text()).toContain('导入 1 个')
    const imported = store.assets.find((asset) => asset.name === '工具栏导入')
    expect(imported?.type).toBe('sprite')
    expect(imported?.status).toBe('draft')

    prompt.mockReturnValue('C:\\导入\\工具栏导入.png')
    await runImport()
    expect(wrapper.text()).toContain('跳过 1 个重复（工具栏导入）')

    prompt.mockReturnValue('C:\\导入\\说明.txt')
    await runImport()
    expect(wrapper.text()).toContain('1 个失败：不支持的格式')

    prompt.mockRestore()
  })

  it('opens the asset detail modal', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const card = wrapper.findAll('button').find((button) => button.classes().includes('group'))
    expect(card).toBeTruthy()
    await card!.trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('素材 ID')
    expect(document.body.textContent).toContain('版本')
  })

  it('persists updater settings and records a manual check', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    const loaded = await store.loadUpdateSettings()
    expect(loaded?.channel).toBe('stable')
    expect(loaded?.autoCheck).toBe(true)

    await store.saveUpdateSettings({ channel: 'dev' })
    expect(store.updateSettings?.channel).toBe('dev')

    const status = await store.checkUpdates()
    expect(status?.channel).toBe('dev')
    expect(store.updateStatus?.checkedAt).toBeTruthy()
    expect(store.updateBusy).toBe(false)
  })

  it('creates a blank asset and opens it in the pixel editor', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const before = store.assets.length
    const openButton = wrapper
      .findAll('button')
      .find((button) => button.text().trim() === '新建素材')
    expect(openButton).toBeTruthy()
    await openButton!.trigger('click')
    await flushPromises()

    const dialog = activeDialog()!
    const name = dialog.querySelector('input[aria-label="新建素材名称"]') as HTMLInputElement
    name.value = '手绘测试'
    name.dispatchEvent(new Event('input', { bubbles: true }))
    const width = dialog.querySelector('input[aria-label="新建素材宽度"]') as HTMLInputElement
    width.value = '24'
    width.dispatchEvent(new Event('input', { bubbles: true }))
    const height = dialog.querySelector('input[aria-label="新建素材高度"]') as HTMLInputElement
    height.value = '16'
    height.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()

    const create = Array.from(dialog.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '创建并开始绘制',
    )
    create!.click()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 30))
    await flushPromises()

    expect(store.route).toBe('editor')
    expect(store.assets.length).toBe(before + 1)
    const asset = store.assets.find((item) => item.name === '手绘测试')
    expect(asset?.dimensions).toEqual({ w: 24, h: 16 })
    expect(asset?.versions).toHaveLength(0)
    expect(store.editorAssetId).toBe(asset?.id)
    expect(wrapper.text()).toContain('手绘测试')
    expect(wrapper.text()).toContain('24 × 16 px')
    expect(wrapper.text()).toContain('未保存')
  })

  it('reopens a saved sprite sheet with all its frames', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    const blank = await store.createBlankAsset({
      name: '动画面板',
      type: 'sprite',
      width: 8,
      height: 8,
    })
    expect(blank).toBeTruthy()
    const saved = await store.saveAssetSheet(blank!.id, pngDataURL(16, 8), {
      frameWidth: 8,
      frameHeight: 8,
      frameCount: 2,
      columns: 2,
      rows: 1,
      margin: 0,
      spacing: 0,
      fps: 10,
      loop: true,
      durations: [100, 200],
      tags: ['idle'],
    })
    expect(saved?.animation?.frameCount).toBe(2)

    store.openEditor(blank!.id)
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 100))
    await flushPromises()

    expect(store.route).toBe('editor')
    expect(wrapper.text()).toContain('2 帧')
    expect(wrapper.text()).toContain('第 1 / 2 帧')
    expect(wrapper.text()).toContain('v0001')
  })

  it('blocks saving a post-process result after the settings change', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const card = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('group') && button.text().includes('草地占位图'))
    await card!.trigger('click')
    await flushPromises()
    const postButton = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('像素化'),
    )
    expect(postButton).toBeTruthy()
    postButton!.click()
    await new Promise((resolve) => setTimeout(resolve, 60))
    await flushPromises()

    const modal = () =>
      Array.from(document.querySelectorAll('[role="dialog"]')).find((dialog) =>
        dialog.textContent?.includes('像素完美处理'),
      )!
    const saveButton = () =>
      Array.from(modal().querySelectorAll('button')).find((button) =>
        button.textContent?.includes('保存为新版本'),
      ) as HTMLButtonElement
    expect(saveButton().disabled).toBe(false)

    const width = modal().querySelectorAll('input[type="number"]')[0] as HTMLInputElement
    width.value = '16'
    width.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()
    expect(saveButton().disabled).toBe(true)
    expect(modal().textContent).toContain('参数已修改')

    const refresh = Array.from(modal().querySelectorAll('button')).find((button) =>
      button.textContent?.includes('重新预览'),
    )
    refresh!.click()
    await new Promise((resolve) => setTimeout(resolve, 60))
    await flushPromises()
    expect(saveButton().disabled).toBe(false)

    const before = store.assets.find((asset) => asset.name === '草地占位图')?.versions.length ?? 0
    saveButton().click()
    await new Promise((resolve) => setTimeout(resolve, 60))
    await flushPromises()
    const after = store.assets.find((asset) => asset.name === '草地占位图')?.versions.length ?? 0
    expect(after).toBe(before + 1)
    expect(store.assets.find((asset) => asset.name === '草地占位图')?.dimensions).toEqual({
      w: 16,
      h: 32,
    })
  })

  it('opens the pixel editor from the asset detail', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const card = wrapper
      .findAll('button')
      .find((button) => button.classes().includes('group') && button.text().includes('草地占位图'))
    expect(card).toBeTruthy()
    await card!.trigger('click')
    await flushPromises()

    const edit = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('像素编辑'),
    )
    expect(edit).toBeTruthy()
    edit!.click()
    await flushPromises()

    expect(store.route).toBe('editor')
    const editing = store.assets.find((asset) => asset.id === store.editorAssetId)
    expect(editing).toBeTruthy()
    expect(wrapper.text()).toContain(editing!.name)
    expect(wrapper.text()).toContain('保存版本')
    expect(wrapper.text()).toContain('图层')
    expect(wrapper.text()).toContain('洋葱皮')
    expect(wrapper.text()).toContain('帧')
    expect(wrapper.text()).toContain('精灵表')

    // Selection, pixel-perfect, symmetry and recolor are wired to the panel.
    expect(wrapper.text()).toContain('选区')
    expect(wrapper.text()).toContain('像素完美手绘')
    expect(wrapper.text()).toContain('对称绘制')
    expect(wrapper.text()).toContain('替换')
    // Opening a saved version reads as that version, not as unsaved work.
    expect(wrapper.text()).toMatch(/v\d{4}/)
    expect(wrapper.text()).not.toContain('未保存')

    await wrapper.find('button[title="新建图层"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('2 图层')

    const marquee = wrapper.find('button[title="矩形选区 (M)"]')
    expect(marquee.exists()).toBe(true)
    await marquee.trigger('click')
    expect(wrapper.text()).toContain('select')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true }))
    await flushPromises()
    const clearSelection = wrapper.find('button[title="清除选区内容 (Delete)"]')
    expect(clearSelection.exists()).toBe(true)
    expect(clearSelection.attributes('disabled')).toBeUndefined()

    store.closeEditor()
    await flushPromises()
    expect(store.route).toBe('assets')
  })

  it('multi-selects assets and applies a review status in one batch', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const targets = store.assets.slice(0, 2).map((asset) => asset.id)
    const boxes = wrapper.findAll('[role="checkbox"]')
    expect(boxes.length).toBeGreaterThanOrEqual(2)
    await boxes[0]!.trigger('click')
    await boxes[1]!.trigger('click')
    expect(wrapper.text()).toContain('已选 2 个')

    const select = wrapper.find('select[aria-label="批量设置评审状态"]')
    expect(select.exists()).toBe(true)
    await select.setValue('approved')
    await flushPromises()

    for (const id of targets) {
      expect(store.assets.find((asset) => asset.id === id)?.status).toBe('approved')
    }
    expect(wrapper.text()).toContain('2 个素材标记为')
  })

  it('adds and removes one tag across checked assets', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const boxes = wrapper.findAll('[role="checkbox"]')
    await boxes[0]!.trigger('click')
    await boxes[1]!.trigger('click')
    expect(wrapper.text()).toContain('已选 2 个')

    const ids = store.assets.slice(0, 2).map((asset) => asset.id)
    const input = wrapper.find('input[aria-label="批量标签"]')
    expect(input.exists()).toBe(true)
    await input.setValue('夜景')

    const add = wrapper.findAll('button').find((button) => button.text() === '加标签')
    expect(add).toBeTruthy()
    await add!.trigger('click')
    await flushPromises()
    for (const id of ids) {
      expect(store.assets.find((asset) => asset.id === id)?.tags).toContain('夜景')
    }
    expect(wrapper.text()).toContain('2 个素材添加标签')

    await input.setValue('夜景')
    const remove = wrapper.findAll('button').find((button) => button.text() === '移除标签')
    expect(remove).toBeTruthy()
    await remove!.trigger('click')
    await flushPromises()
    for (const id of ids) {
      expect(store.assets.find((asset) => asset.id === id)?.tags).not.toContain('夜景')
    }
  })

  it('moves checked and selected assets into the project trash', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const before = store.assets.length

    await wrapper.findAll('[role="checkbox"]')[0]!.trigger('click')
    const batchDelete = wrapper.findAll('button').find((button) => button.text().includes('删除'))
    expect(batchDelete).toBeTruthy()
    await batchDelete!.trigger('click')
    await flushPromises()

    const box = activeDialog()
    expect(box?.textContent).toContain('删除素材')
    expect(box?.textContent).toContain('.workbench/trash/assets/')
    await clickDialogButton('移入回收站')
    expect(store.assets.length).toBe(before - 1)
    expect(wrapper.text()).toContain('已移入回收站')

    const target = store.assets[0]!
    const card = wrapper.findAll('button').find((button) => button.text().includes(target.name))
    expect(card).toBeTruthy()
    await card!.trigger('click')
    await flushPromises()
    await flushPromises()

    const detailDelete = Array.from(document.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '删除',
    )
    expect(detailDelete).toBeTruthy()
    detailDelete!.click()
    await flushPromises()
    expect(activeDialog()?.textContent).toContain(target.name)
    await clickDialogButton('移入回收站')
    await flushPromises()

    expect(store.assets.some((asset) => asset.id === target.id)).toBe(false)
    expect(store.assets.length).toBe(before - 2)
  })

  it('switches the asset library between grid and list views', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const list = wrapper.find('button[title="列表视图"]')
    expect(list.exists()).toBe(true)
    await list.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('最近更新')
    expect(wrapper.text()).toContain(store.assets[0]!.name)
    expect(wrapper.find('button[title="列表视图"]').attributes('aria-pressed')).toBe('true')

    await wrapper.find('button[title="网格视图"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('最近更新')
    expect(wrapper.find('button[title="网格视图"]').attributes('aria-pressed')).toBe('true')
  })

  it('shows which maps reference an asset', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const summary = store.maps[0]!
    const map = await store.openMap(summary.id)
    const assetId = map!.tilesets[0]!.assetId
    const asset = store.assets.find((item) => item.id === assetId)!
    expect(asset).toBeTruthy()

    const card = wrapper.findAll('button').find((button) => button.text().includes(asset.name))
    expect(card).toBeTruthy()
    await card!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(document.body.textContent).toContain('地图引用')
    expect(document.body.textContent).toContain('小镇广场')
    expect(document.body.textContent).toContain('图集')
  })

  it('compares two versions of an asset side by side', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('assets')
    await flushPromises()

    const asset = store.assets.find((item) => item.versions.length > 0)!
    expect(asset).toBeTruthy()
    const card = wrapper.findAll('button').find((button) => button.text().includes(asset.name))
    expect(card).toBeTruthy()
    await card!.trigger('click')
    await flushPromises()

    const compare = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.trim() === '对比',
    )
    expect(compare).toBeTruthy()
    compare!.click()
    await flushPromises()

    expect(document.body.textContent).toContain('版本对比')
    expect(document.body.textContent).toContain('基准版本')
    expect(document.body.textContent).toContain('对比版本')
    expect(document.body.textContent).toContain('对比不会修改任何版本文件')
  })

  it('persists pixel-editor preferences from settings', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('settings')
    await flushPromises()
    await openSettingsTab(wrapper, '通用设置')
    await openSettingsTab(wrapper, '编辑器')

    const grid = wrapper.find('input[type="checkbox"]')
    expect(grid.exists()).toBe(true)
    await grid.setValue(false)
    await flushPromises()
    expect(store.editorSettings?.showGrid).toBe(false)

    const mode = wrapper.find('select')
    await mode.setValue('actual')
    await flushPromises()
    expect(store.editorSettings?.openMode).toBe('actual')
  })

  it('keeps an AI copy draft on reopen and overwrites the body', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    await enableMockAI(store)
    store.setRoute('lore')
    await flushPromises()

    const entry = wrapper
      .findAll('button')
      .find((button) => button.text().includes('旅人') && button.text().includes('草稿'))
    await entry!.trigger('click')
    await flushPromises()

    const openCopy = async () => {
      const button = wrapper.findAll('button').find((item) => item.text().includes('AI 文案'))
      await button!.trigger('click')
      await flushPromises()
      return Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).find((item) =>
        item.textContent?.includes('AI 文案草稿'),
      )
    }

    let dialog = await openCopy()
    await settleAIText()
    const draft = (dialog!.querySelector('textarea') as HTMLTextAreaElement).value
    expect(draft).toContain('旅人')
    await clickDialogButton('取消')

    // Reopening shows the cached draft immediately — no loading spinner.
    dialog = await openCopy()
    const kept = dialog!.querySelector('textarea') as HTMLTextAreaElement
    expect(kept.value).toBe(draft)
    const cover = Array.from(dialog!.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '覆盖正文',
    )
    cover!.click()
    await flushPromises()

    const body = wrapper.find('textarea[aria-label="设定正文"]').element as HTMLTextAreaElement
    expect(body.value).toBe(draft)
  })

  it('keeps AI copy drafts across route changes', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    await enableMockAI(store)
    store.setRoute('lore')
    await flushPromises()

    const openEntry = async () => {
      const entry = wrapper
        .findAll('button')
        .find((button) => button.text().includes('旅人') && button.text().includes('草稿'))
      await entry!.trigger('click')
      await flushPromises()
    }
    const dialog = () =>
      Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).find((item) =>
        item.textContent?.includes('AI 文案草稿'),
      )

    await openEntry()
    await wrapper.findAll('button').find((item) => item.text().includes('AI 文案'))!.trigger('click')
    await settleAIText()
    const draft = (dialog()!.querySelector('textarea') as HTMLTextAreaElement).value
    await clickDialogButton('取消')

    store.setRoute('dashboard')
    await flushPromises()
    store.setRoute('lore')
    await flushPromises()
    await openEntry()
    await wrapper.findAll('button').find((item) => item.text().includes('AI 文案'))!.trigger('click')
    await flushPromises()
    expect((dialog()!.querySelector('textarea') as HTMLTextAreaElement).value).toBe(draft)
    await clickDialogButton('取消')
  })

  it('fixes an asset prompt so later generations reuse it', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    await enableMockAI(store)
    store.setRoute('lore')
    await flushPromises()

    const entry = wrapper
      .findAll('button')
      .find((button) => button.text().includes('旅人') && button.text().includes('草稿'))
    await entry!.trigger('click')
    await flushPromises()

    await wrapper.findAll('button').find((item) => item.text().includes('生成素材'))!.trigger('click')
    await settleAIText()
    const promptDialog = () =>
      Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).find((item) =>
        item.textContent?.includes('提示词确认'),
      )
    const prompt = (promptDialog()!.querySelector('textarea') as HTMLTextAreaElement).value
    const fix = Array.from(promptDialog()!.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '设为固定提示词',
    )
    fix!.click()
    await flushPromises()
    expect(store.entities.find((entity) => entity.name === '旅人')?.artPrompt).toBe(prompt)

    await clickDialogButton('取消')
    await wrapper.findAll('button').find((item) => item.text().includes('生成素材'))!.trigger('click')
    await flushPromises()
    expect((promptDialog()!.querySelector('textarea') as HTMLTextAreaElement).value).toBe(prompt)
    expect(promptDialog()!.textContent).toContain('角色固定提示词')
    await clickDialogButton('取消')
  })

  it('prefills the generation negative prompt from project settings', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('settings')
    await flushPromises()
    await openSettingsTab(wrapper, '项目设置')
    await openSettingsTab(wrapper, '风格指南')

    const field = wrapper
      .findAll('textarea')
      .find((item) => (item.element as HTMLTextAreaElement).placeholder.includes('模糊'))
    expect(field).toBeTruthy()
    expect((field!.element as HTMLTextAreaElement).value).toContain('抗锯齿')
    await field!.setValue('低清晰度，色带')
    await field!.trigger('change')
    await flushPromises()

    store.setRoute('generate')
    await flushPromises()
    const values = wrapper.findAll('textarea').map((item) => (item.element as HTMLTextAreaElement).value)
    expect(values).toContain('低清晰度，色带')
  })

  it('deletes an unused palette copy and protects the default one', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('settings')
    await flushPromises()
    await openSettingsTab(wrapper, '项目设置')
    await openSettingsTab(wrapper, '调色板')

    const before = store.palettes.length
    const create = wrapper.findAll('button').find((button) => button.text().includes('新建副本'))
    await create!.trigger('click')
    await flushPromises()
    expect(store.palettes.length).toBe(before + 1)

    const remove = wrapper
      .findAll('button')
      .find((button) => button.attributes('title')?.startsWith('删除调色板'))
    expect(remove).toBeTruthy()
    await remove!.trigger('click')
    await flushPromises()
    await clickDialogButton('删除调色板')
    await flushPromises()
    expect(store.palettes.length).toBe(before)
  })

  it('creates a palette copy from settings and persists it', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('settings')
    await flushPromises()
    await openSettingsTab(wrapper, '项目设置')
    await openSettingsTab(wrapper, '调色板')

    const before = store.palettes.length
    const create = wrapper.findAll('button').find((button) => button.text().includes('新建副本'))
    expect(create).toBeTruthy()
    await create!.trigger('click')
    await flushPromises()

    expect(store.palettes.length).toBe(before + 1)
    expect(store.palettes.some((palette) => palette.name.includes('副本'))).toBe(true)
    expect(wrapper.text()).toContain('项目色数上限')
  })

  it('confirms the exact payload before calling a cloud image API', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('generate')
    await flushPromises()

    const aiTab = wrapper.findAll('button').find((button) => button.text().includes('AI 生成'))
    expect(aiTab).toBeTruthy()
    await aiTab!.trigger('click')
    await flushPromises()

    const provider = wrapper.findAll('select').find((select) => select.text().includes('OpenAI'))
    expect(provider).toBeTruthy()
    await provider!.setValue('openai')
    const prompt = wrapper.find('textarea')
    await prompt.setValue('一间杂货铺，暖色招牌')
    await flushPromises()

    const submit = wrapper.findAll('button').find((button) => button.text().includes('提交到队列'))
    expect(submit).toBeTruthy()
    const before = store.aiJobs.length

    await submit!.trigger('click')
    await flushPromises()
    const box = activeDialog()
    expect(box?.textContent).toContain('一间杂货铺，暖色招牌')
    expect(box?.textContent).toContain('不会发送项目文件')
    await clickDialogButton('取消')
    expect(store.aiJobs.length).toBe(before)

    await submit!.trigger('click')
    await flushPromises()
    await clickDialogButton('发送请求')
    await flushPromises()
    expect(store.aiJobs.length).toBe(before + 1)
  })

  it('cuts a finished AI image into an 8-direction sheet', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)

    const job = await store.submitAIJob({
      provider: 'comfyui',
      prompt: '主角，8 方向行走，4 列 2 行等距排列',
      negativePrompt: '',
      width: 512,
      height: 512,
      count: 1,
      seed: 7,
      assetType: 'sprite',
      name: '主角',
    })
    expect(job).toBeTruthy()
    // Mock jobs advance on short timers; poll so a loaded run cannot race them.
    for (let attempt = 0; attempt < 40; attempt += 1) {
      await store.refreshAIJobs()
      if (store.aiJobs.find((item) => item.id === job!.id)?.state === 'done') break
      await new Promise((resolve) => setTimeout(resolve, 50))
    }
    expect(store.aiJobs.find((item) => item.id === job!.id)?.state).toBe('done')
    await flushPromises()

    store.setRoute('generate')
    await flushPromises()
    const aiTab = wrapper.findAll('button').find((button) => button.text().includes('AI 生成'))
    await aiTab!.trigger('click')
    await flushPromises()

    // The queue list also repeats finished jobs; pick the result card with actions.
    const jobCard = wrapper
      .findAll('article')
      .find(
        (card) =>
          card.text().includes('主角') &&
          card.findAll('button').some((button) => button.text().includes('切图')),
      )
    expect(jobCard).toBeTruthy()
    const sheetButton = jobCard!.findAll('button').find((button) => button.text().includes('切图'))
    expect(sheetButton).toBeTruthy()
    await sheetButton!.trigger('click')
    await flushPromises()

    // AppModal teleports to body, so query the dialog outside the wrapper.
    const dialog = Array.from(document.querySelectorAll<HTMLElement>('[role="dialog"]')).find(
      (item) => item.textContent?.includes('切成精灵表'),
    )
    expect(dialog).toBeTruthy()
    expect(dialog!.textContent).toContain('切成精灵表')

    const eightDir = Array.from(dialog!.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === '8 方向',
    )
    expect(eightDir).toBeTruthy()
    eightDir!.click()
    await flushPromises()

    const confirm = Array.from(dialog!.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('切分并保存'),
    )
    expect(confirm).toBeTruthy()
    confirm!.click()
    await flushPromises()

    const saved = store.assets.find((asset) => asset.name === '主角 精灵表')
    expect(saved).toBeTruthy()
    expect(saved!.type).toBe('sprite')
    expect(saved!.animation?.columns).toBe(4)
    expect(saved!.animation?.frameCount).toBe(8)
    expect(saved!.animation?.frameWidth).toBe(34)
    expect(wrapper.text()).toContain('8 帧精灵表')
  })

  it('deletes finished AI jobs and refuses to drop a running one', async () => {
    const { store } = await bootApp()
    await openDemo(store)

    const job = await store.submitAIJob({
      provider: 'comfyui',
      prompt: '木箱',
      negativePrompt: '',
      width: 256,
      height: 256,
      count: 1,
      seed: 1,
      assetType: 'prop',
      name: '木箱',
    })
    expect(job).toBeTruthy()
    expect(store.aiJobs.some((item) => item.id === job!.id)).toBe(true)

    // Active jobs must be canceled first.
    expect(await store.deleteAIJob(job!.id)).toBe(false)
    expect(store.error?.code).toBe('JOB_ACTIVE')
    expect(store.aiJobs.some((item) => item.id === job!.id)).toBe(true)

    for (let attempt = 0; attempt < 40; attempt += 1) {
      await store.refreshAIJobs()
      if (store.aiJobs.find((item) => item.id === job!.id)?.state === 'done') break
      await new Promise((resolve) => setTimeout(resolve, 50))
    }
    expect(store.aiJobs.find((item) => item.id === job!.id)?.state).toBe('done')

    expect(await store.deleteAIJob(job!.id)).toBe(true)
    expect(store.aiJobs.some((item) => item.id === job!.id)).toBe(false)
  })

  it('expands the map canvas and hands the map context to AI generation', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    store.setRoute('maps')
    await flushPromises()

    const summary = store.maps[0]!
    const section = Array.from(document.querySelectorAll('section')).find((element) =>
      element.textContent?.includes('扩展画布'),
    )
    expect(section).toBeTruthy()
    expect(section!.textContent).toContain('AI 补全新区域')

    const edges = section!.querySelectorAll<HTMLInputElement>('input[type="number"]')
    expect(edges.length).toBeGreaterThanOrEqual(4)
    edges[1]!.value = '2'
    edges[1]!.dispatchEvent(new Event('input'))
    await flushPromises()

    const expand = Array.from(section!.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('扩展画布'),
    )
    expect(expand).toBeTruthy()
    expand!.click()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 20))
    await flushPromises()

    const grown = store.maps.find((map) => map.id === summary.id)
    expect(grown?.width).toBe(summary.width + 2)
    expect(wrapper.text()).toContain('已扩展至')

    const terrain = Array.from(section!.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('地形瓦片'),
    )
    expect(terrain).toBeTruthy()
    terrain!.click()
    await flushPromises()

    expect(store.route).toBe('generate')
    expect(store.mapGenerateIntent).toBeNull()
    expect(wrapper.text()).toContain('来自地图')
    const nameField = wrapper.find('input[placeholder="例如：杂货铺"]')
    expect((nameField.element as HTMLInputElement).value).toContain('扩展素材')
    const prompt = wrapper.find('textarea')
    expect((prompt.element as HTMLTextAreaElement).value).toContain(summary.name)
    expect((prompt.element as HTMLTextAreaElement).value).toContain('瓦片')
  })

  it('leaves the new-map dialog closed on an empty map list and offers AI tiles', async () => {
    const { wrapper, store } = await bootApp()
    mounted = wrapper
    await openDemo(store)
    for (const summary of [...store.maps]) await store.deleteMap(summary.id)
    store.setRoute('maps')
    await flushPromises()

    expect(store.maps).toHaveLength(0)
    expect(wrapper.text()).toContain('还没有打开地图')
    // Opening the Maps screen must not pop the dialog on its own.
    expect(document.querySelector('[role="dialog"]')).toBeNull()

    const ai = Array.from(wrapper.findAll('button')).find((button) =>
      button.text().includes('AI 生成地形图集'),
    )
    expect(ai).toBeTruthy()
    await ai!.trigger('click')
    await flushPromises()

    expect(store.route).toBe('generate')
    const prompt = wrapper.find('textarea')
    expect((prompt.element as HTMLTextAreaElement).value).toContain('16 格自动拼接瓦片')
  })
})