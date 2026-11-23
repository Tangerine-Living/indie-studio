import { describe, expect, it, vi } from 'vitest'
import { createMockPlatform } from './mock'
import { AppError } from '../lib/errors'
import type { PostProcessOptions } from '../domain/types'

const DEMO_ROOT = 'C:\\PixelBench\\示例项目\\小镇广场'

function platform() {
  return createMockPlatform({ persist: false, seedDemo: true })
}

/** Minimal header-only PNG data URL; the mock reads dimensions from IHDR. */
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

describe('mock platform', () => {
  it('boots with a demo project in recents but closed', async () => {
    const p = platform()
    const boot = await p.bootstrap()
    expect(boot.session).toBeNull()
    expect(boot.recent).toHaveLength(1)
    expect(boot.recent[0]?.name).toBe('小镇广场')
    expect(boot.info.mode).toBe('browser')
  })

  it('opens the demo and exposes seeded data', async () => {
    const p = platform()
    const session = await p.openProject(DEMO_ROOT)
    expect(session.path).toBe(DEMO_ROOT)
    expect(session.project.name).toBe('小镇广场')
    const quests = await p.listQuests()
    const assets = await p.listAssets()
    const palettes = await p.listPalettes()
    expect(quests).toHaveLength(5)
    // Six seeded assets: grass, tile atlas, shop, traveler, icon and barrel.
    expect(assets).toHaveLength(6)
    expect(palettes[0]?.colors).toHaveLength(16)
    expect((await p.bootstrap()).session?.path).toBe(DEMO_ROOT)
  })

  it('opens when pointed at project.json', async () => {
    const p = platform()
    const session = await p.openProject(DEMO_ROOT + '\\project.json')
    expect(session.path).toBe(DEMO_ROOT)
  })

  it('saves project edits and updates recents', async () => {
    const p = platform()
    const session = await p.openProject(DEMO_ROOT)
    const saved = await p.saveProject({ ...session.project, name: '港口小镇' })
    expect(saved.project.name).toBe('港口小镇')
    expect(new Date(saved.project.updatedAt).getTime()).toBeGreaterThanOrEqual(
      new Date(session.project.updatedAt).getTime(),
    )
    const recent = await p.listRecent()
    expect(recent[0]?.name).toBe('港口小镇')
  })

  it('persists a new quest and syncs asset links', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const assets = await p.listAssets()
    const asset = assets[0]!
    const created = await p.saveQuest({
      schemaVersion: 1,
      id: '',
      title: '  新任务  ',
      type: 'asset',
      status: 'todo',
      priority: 'low',
      brief: {
        assetType: 'tile',
        tileSize: 32,
        paletteId: 'default',
        style: '',
        references: [],
        prompt: '',
      },
      acceptance: [],
      links: { assetIds: [asset.id], mapIds: [], urls: [] },
      comments: [],
      createdAt: '',
      updatedAt: '',
    })
    expect(created.id).not.toBe('')
    expect(created.title).toBe('新任务')
    const quests = await p.listQuests()
    expect(quests.some((quest) => quest.id === created.id)).toBe(true)
    const linked = (await p.listAssets()).find((item) => item.id === asset.id)
    expect(linked?.questIds).toContain(created.id)

    await p.deleteQuest(created.id)
    expect((await p.listQuests()).some((quest) => quest.id === created.id)).toBe(false)
    const unlinked = (await p.listAssets()).find((item) => item.id === asset.id)
    expect(unlinked?.questIds).not.toContain(created.id)
  })

  it('rejects invalid quest titles', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    await expect(
      p.saveQuest({
        ...(await p.listQuests())[0]!,
        title: '   ',
      }),
    ).rejects.toMatchObject({ code: 'INVALID_QUEST' })
  })

  it('returns thumbnails as data URLs', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets())[0]!
    const url = await p.assetURL(asset.id, 'thumb.png')
    expect(url.startsWith('data:image/svg+xml')).toBe(true)
  })

  it('creates a project and refuses duplicates', async () => {
    const p = platform()
    const session = await p.createProject({
      parentDir: 'C:\\Work',
      name: '新项目',
      engine: 'godot',
      tileSize: 32,
      template: 'blank',
      paletteId: 'default',
    })
    expect(session.path).toBe('C:\\Work\\新项目')
    expect(session.project.name).toBe('新项目')
    expect((await p.bootstrap()).session?.path).toBe(session.path)

    await expect(
      p.createProject({
        parentDir: 'C:\\Work',
        name: '新项目',
        engine: 'godot',
        tileSize: 32,
        template: 'blank',
        paletteId: 'default',
      }),
    ).rejects.toMatchObject({ code: 'PROJECT_EXISTS' })
  })

  it('creates a blank project with no demo content', async () => {
    const p = platform()
    const session = await p.createProject({
      parentDir: 'C:\\Work',
      name: '空白项目',
      engine: 'tiled',
      tileSize: 16,
      template: 'blank',
      paletteId: 'default',
    })
    expect(session.project.settings.template).toBe('blank')
    expect(await p.listQuests()).toHaveLength(0)
    expect(await p.listAssets()).toHaveLength(0)
    expect(await p.listMaps()).toHaveLength(0)
    expect(await p.listEntities()).toHaveLength(0)
    expect(await p.listTables()).toHaveLength(0)
    expect(await p.listPalettes()).toHaveLength(1)
    expect((await p.listPalettes())[0]?.colors).toHaveLength(16)
  })

  it('creates a blank hand-drawn asset without a version', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = await p.createBlankAsset({
      name: ' 手绘图标 ',
      type: 'ui',
      width: 20,
      height: 12,
    })
    expect(asset.name).toBe('手绘图标')
    expect(asset.type).toBe('ui')
    expect(asset.status).toBe('draft')
    expect(asset.dimensions).toEqual({ w: 20, h: 12 })
    expect(asset.pivot).toEqual({ x: 10, y: 11 })
    expect(asset.source.kind).toBe('blank')
    expect(asset.versions).toHaveLength(0)
    expect((await p.listAssets()).some((item) => item.id === asset.id)).toBe(true)

    await expect(
      p.createBlankAsset({ name: '   ', type: 'ui', width: 20, height: 12 }),
    ).rejects.toMatchObject({ code: 'INVALID_ASSET' })
    await expect(
      p.createBlankAsset({ name: '空白', type: 'ui', width: 0, height: 12 }),
    ).rejects.toMatchObject({ code: 'INVALID_ASSET' })
  })

  it('scales a hand-drawn asset pivot when a sheet changes the frame size', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = await p.createBlankAsset({
      name: '锚点表',
      type: 'sprite',
      width: 32,
      height: 32,
    })
    const saved = await p.saveAssetSheet(asset.id, pngDataURL(16, 8), {
      frameWidth: 8,
      frameHeight: 8,
      frameCount: 2,
      columns: 2,
      rows: 1,
      margin: 0,
      spacing: 0,
      fps: 8,
      loop: true,
    })
    expect(saved.dimensions).toEqual({ w: 8, h: 8 })
    expect(saved.pivot).toEqual({ x: 4, y: 7 })
    expect(saved.animation?.frameCount).toBe(2)

    // A later single-frame save replaces the animation and rescales the pivot.
    const still = await p.saveAssetVersion(asset.id, pngDataURL(16, 16))
    expect(still.animation).toBeUndefined()
    expect(still.dimensions).toEqual({ w: 16, h: 16 })
    expect(still.pivot).toEqual({ x: 8, y: 14 })
  })

  it('rejects invalid names and missing parents', async () => {
    const p = platform()
    await expect(
      p.createProject({
        parentDir: 'C:\\Work',
        name: 'CON',
        engine: 'godot',
        tileSize: 32,
        template: 'blank',
        paletteId: 'default',
      }),
    ).rejects.toBeInstanceOf(AppError)
    await expect(
      p.createProject({
        parentDir: '  ',
        name: '项目',
        engine: 'godot',
        tileSize: 32,
        template: 'blank',
        paletteId: 'default',
      }),
    ).rejects.toMatchObject({ code: 'INVALID_PARENT' })
  })

  it('reports missing projects', async () => {
    const p = platform()
    await expect(p.openProject('C:\\missing')).rejects.toMatchObject({
      code: 'PROJECT_NOT_FOUND',
    })
  })

  it('imports assets and skips duplicates by content hash', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const before = (await p.listAssets()).length

    const first = await p.importAssets(['C:\\导入\\杂货铺.png', 'C:\\导入\\草地.png'], 'building')
    expect(first.imported).toHaveLength(2)
    expect(first.duplicates).toHaveLength(0)
    const imported = first.imported[0]!
    expect(imported.status).toBe('draft')
    expect(imported.type).toBe('building')
    expect(imported.source.kind).toBe('imported')
    expect(imported.sourceHash).toBeTruthy()
    expect(imported.dimensions.w).toBeGreaterThan(0)

    const again = await p.importAssets(['C:\\导入\\杂货铺.png'], 'building')
    expect(again.imported).toHaveLength(0)
    expect(again.duplicates[0]?.assetId).toBe(imported.id)
    expect((await p.listAssets()).length).toBe(before + 2)
  })

  it('saves editor output as immutable versions and serves it as the thumb', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets()).find((item) => item.versions.length === 0)!
    const firstURL = pngDataURL(24, 18)

    const first = await p.saveAssetVersion(asset.id, firstURL)
    expect(first.currentVersion).toBe('0001')
    expect(first.dimensions).toEqual({ w: 24, h: 18 })
    expect(first.versions).toHaveLength(1)
    expect(await p.assetURL(asset.id, 'thumb.png')).toBe(firstURL)
    const source = await p.assetSource(asset.id)
    expect(source.url).toBe(firstURL)

    const second = await p.saveAssetVersion(asset.id, pngDataURL(32, 32))
    expect(second.currentVersion).toBe('0002')
    expect(second.versions).toHaveLength(2)
    expect(second.dimensions).toEqual({ w: 32, h: 32 })
  })

  it('saves a sprite-sheet version and reopens on its first frame', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets()).find((item) => item.versions.length === 0)!
    const info = {
      frameWidth: 8,
      frameHeight: 8,
      frameCount: 2,
      columns: 2,
      rows: 1,
      margin: 0,
      spacing: 0,
      fps: 10,
      loop: true,
    }

    const saved = await p.saveAssetSheet(asset.id, pngDataURL(16, 8), info)
    expect(saved.currentVersion).toBe('0001')
    expect(saved.dimensions).toEqual({ w: 8, h: 8 })
    expect(saved.animation).toMatchObject({ fps: 10, frameCount: 2, frameWidth: 8, frameHeight: 8 })

    const source = await p.assetSource(asset.id)
    expect(source.file).toBe('frames/000.png')
    expect(source.url.startsWith('data:image/svg+xml')).toBe(true)

    await expect(
      p.saveAssetSheet(asset.id, pngDataURL(16, 8), { ...info, columns: 3, frameCount: 3 }),
    ).rejects.toMatchObject({ code: 'INVALID_SHEET' })
  })

  it('saves per-frame durations and tags, then exports the version', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets()).find((item) => item.versions.length === 0)!
    const saved = await p.saveAssetSheet(asset.id, pngDataURL(16, 8), {
      frameWidth: 8,
      frameHeight: 8,
      frameCount: 2,
      columns: 2,
      rows: 1,
      margin: 0,
      spacing: 0,
      fps: 10,
      loop: true,
      durations: [60, 240],
      tags: [' idle ', 'walk', 'idle'],
    })
    expect(saved.animation?.frameDurations).toEqual([60, 240])
    expect(saved.animation?.tags).toEqual(['idle', 'walk'])

    const exported = await p.exportAsset(saved.id)
    expect(exported.dir).toBe(`${DEMO_ROOT}\\exports\\${saved.name}\\0001`)
    expect(exported.files).toEqual(
      expect.arrayContaining([
        'output.png',
        'meta.json',
        'frames-aseprite.json',
        'frames-texturepacker.json',
        'frames/000.png',
        'frames/001.png',
        'export.json',
      ]),
    )
    await p.openExportsFolder()
  })

  it('saves a review status change through saveAsset', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets()).find((item) => item.status === 'draft')!

    const saved = await p.saveAsset({ ...asset, status: 'review' })
    expect(saved.status).toBe('review')
    expect(saved.name).toBe(asset.name)
    expect(new Date(saved.updatedAt).getTime()).toBeGreaterThanOrEqual(
      new Date(asset.updatedAt).getTime(),
    )
    expect((await p.listAssets()).find((item) => item.id === asset.id)?.status).toBe('review')

    await expect(p.saveAsset({ ...asset, id: 'missing' })).rejects.toMatchObject({
      code: 'ASSET_NOT_FOUND',
    })
    await expect(p.saveAsset({ ...asset, name: '   ' })).rejects.toMatchObject({
      code: 'INVALID_ASSET',
    })
  })

  it('refuses to export an asset without a version', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets()).find((item) => item.versions.length === 0)!
    await expect(p.exportAsset(asset.id)).rejects.toMatchObject({ code: 'NO_VERSION' })
    await expect(p.exportAsset('missing')).rejects.toMatchObject({ code: 'ASSET_NOT_FOUND' })
  })

  it('draws deterministic building drafts and adopts one as an asset', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const options = {
      seed: 20261121,
      width: 4,
      depth: 3,
      tileSize: 32,
      floors: 2,
      wallColor: '#c8a87c',
      roofColor: '#b04e3e',
      trimColor: '#78543a',
      doorColor: '#5c3a24',
      windows: 0,
      sign: true,
      weathered: 0.25,
      shadow: true,
      outline: true,
      variants: 3,
    }

    const drafts = await p.generateBuildings(options)
    expect(drafts).toHaveLength(3)
    expect(drafts[0]?.width).toBe(140)
    expect(drafts[0]?.height).toBe(160)
    expect(drafts[0]?.pivot).toEqual({ x: 70, y: 150 })
    expect(drafts[0]?.collision).toEqual({ x: 6, y: 6, w: 128, h: 96 })
    expect(drafts[0]?.image.startsWith('data:image/svg+xml')).toBe(true)
    expect(new Set(drafts.map((draft) => draft.image)).size).toBe(3)

    const again = await p.generateBuildings(options)
    expect(again.map((draft) => draft.image)).toEqual(drafts.map((draft) => draft.image))

    const before = (await p.listAssets()).length
    const adopted = await p.adoptBuilding(options, drafts[0]!.seed, '  杂货铺  ')
    expect(adopted.name).toBe('杂货铺')
    expect(adopted.type).toBe('building')
    expect(adopted.status).toBe('draft')
    expect(adopted.source).toMatchObject({ kind: 'generated', workflow: 'building-v1' })
    expect(adopted.currentVersion).toBe('0001')
    expect(adopted.dimensions).toEqual({ w: 140, h: 160 })
    expect(adopted.pivot).toEqual({ x: 70, y: 150 })
    expect((await p.listAssets()).length).toBe(before + 1)
    expect((await p.assetURL(adopted.id, 'thumb.png')).startsWith('data:image/svg+xml')).toBe(true)
  })

  it('clamps building options like the Go generator', async () => {
    const p = platform()
    const options = {
      seed: 1,
      width: 0,
      depth: 99,
      tileSize: 33,
      floors: 0,
      wallColor: 'nope',
      roofColor: 'zzz',
      trimColor: '',
      doorColor: '',
      windows: 99,
      sign: false,
      weathered: 2,
      shadow: false,
      outline: false,
      variants: 99,
    }

    const drafts = await p.generateBuildings(options)
    expect(drafts).toHaveLength(8)
    // width clamps to 1, depth to 16, tile falls back to 32.
    expect(drafts[0]?.width).toBe(6 * 2 + 32)
    expect(drafts[0]?.height).toBe(6 * 2 + 16 * 32 + 32)
    expect(drafts[0]?.pivot.y).toBe(6 + 16 * 32 + 32)
  })

  it('simulates the pixel-perfect chain and saves a processed version', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets())[0]!
    const options: PostProcessOptions = {
      width: 8,
      height: 6,
      trim: true,
      alphaThreshold: 128,
      maxColors: 4,
      colors: [],
      dither: 'floyd',
      ditherStrength: 0.8,
      cleanup: true,
      outline: true,
      outlineColor: '#0c1016',
      checkSeam: true,
    }

    const preview = await p.previewPostProcess(asset.id, options)
    expect(preview.simulated).toBe(true)
    expect(preview.width).toBe(8)
    expect(preview.height).toBe(6)
    expect(preview.colors).toHaveLength(4)
    expect(preview.image.startsWith('data:image/')).toBe(true)

    const ratio = await p.previewPostProcess(asset.id, { ...options, height: 0 })
    expect(ratio.height).toBe(Math.max(1, Math.round((asset.dimensions.h * 8) / asset.dimensions.w)))

    const saved = await p.savePostProcessedVersion(asset.id, options)
    expect(saved.dimensions).toEqual({ w: 8, h: 6 })
    expect(saved.currentVersion).toBeTruthy()
    expect(saved.versions.length).toBe(asset.versions.length + 1)
    await expect(p.previewPostProcess('missing', options)).rejects.toMatchObject({
      code: 'ASSET_NOT_FOUND',
    })
  })

  it('rejects malformed editor payloads and missing assets', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const asset = (await p.listAssets())[0]!
    await expect(p.saveAssetVersion(asset.id, 'not-a-png')).rejects.toMatchObject({
      code: 'INVALID_VERSION',
    })
    await expect(p.saveAssetVersion('missing', pngDataURL(8, 8))).rejects.toMatchObject({
      code: 'ASSET_NOT_FOUND',
    })
  })

  it('reports unsupported files without failing the batch', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const summary = await p.importAssets(['C:\\导入\\说明.txt', 'C:\\导入\\新图.png'], 'sprite')
    expect(summary.imported).toHaveLength(1)
    expect(summary.failed).toHaveLength(1)
    expect(summary.failed[0]?.message).toContain('不支持的格式')
  })

  it('stores AI settings and never echoes the API key back', async () => {
    const p = platform()
    const initial = await p.getAISettings()
    expect(initial.defaultProvider).toBe('comfyui')
    expect(initial.openai.hasKey).toBe(false)
    expect(initial.comfy.baseUrl).toBe('http://127.0.0.1:8188')
    expect(initial.comfy.workflow).toContain('KSampler')

    const saved = await p.saveAISettings({
      defaultProvider: 'openai',
      comfy: initial.comfy,
      openai: { ...initial.openai, apiKey: '  test-key-55  ' },
    })
    expect(saved.defaultProvider).toBe('openai')
    expect(saved.openai.hasKey).toBe(true)
    expect(JSON.stringify(saved)).not.toContain('test-key-55')

    const kept = await p.saveAISettings({
      defaultProvider: 'openai',
      comfy: initial.comfy,
      openai: { ...saved.openai, apiKey: '' },
    })
    expect(kept.openai.hasKey).toBe(true)
    const tested = await p.testAIProvider('openai')
    expect(tested).toMatchObject({ ok: true })
    expect(tested.text).toMatchObject({ ok: true, message: expect.stringContaining('文本模型') })

    const cleared = await p.saveAISettings({
      defaultProvider: 'comfyui',
      comfy: initial.comfy,
      openai: { ...saved.openai, clearKey: true },
    })
    expect(cleared.openai.hasKey).toBe(false)
    await expect(p.testAIProvider('openai')).rejects.toMatchObject({ code: 'PROVIDER_AUTH' })
  })

  it('keeps a separate text provider and tests it on its own endpoint', async () => {
    const p = platform()
    const initial = await p.getAISettings()
    const saved = await p.saveAISettings({
      defaultProvider: 'openai',
      comfy: initial.comfy,
      openai: {
        ...initial.openai,
        apiKey: 'img-key',
        textProvider: 'own',
        textBaseUrl: 'https://text.example.com/v1',
        textApiKey: 'text-key',
      },
    })
    expect(saved.openai.textProvider).toBe('own')
    expect(saved.openai.textBaseUrl).toBe('https://text.example.com/v1')
    expect(saved.openai.textHasKey).toBe(true)
    expect(JSON.stringify(saved)).not.toContain('text-key')

    const tested = await p.testAIProvider('openai')
    expect(tested.ok).toBe(true)
    expect(tested.text).toMatchObject({ ok: true })
    expect(tested.text?.detail).toContain('https://text.example.com/v1')

    const cleared = await p.saveAISettings({
      defaultProvider: 'openai',
      comfy: initial.comfy,
      openai: { ...saved.openai, textClearKey: true },
    })
    expect(cleared.openai.textHasKey).toBe(false)
    const retested = await p.testAIProvider('openai')
    expect(retested.text).toMatchObject({ ok: false, message: expect.stringContaining('文本接口未配置') })
    await expect(p.generateText({ system: 'x', user: '写一段' })).rejects.toMatchObject({
      code: 'PROVIDER_AUTH',
    })
  })

  it('runs an AI job through the simulated queue and adopts a result', async () => {
    vi.useFakeTimers()
    try {
      const p = platform()
      await p.openProject(DEMO_ROOT)
      const eventCounts: number[] = []
      const off = p.onAIJobs((jobs) => eventCounts.push(jobs.length))

      await expect(
        p.submitAIJob({
          provider: 'comfyui',
          prompt: '   ',
          negativePrompt: '',
          width: 512,
          height: 512,
          count: 1,
          seed: 1,
          assetType: 'building',
          name: '',
        }),
      ).rejects.toMatchObject({ code: 'INVALID_PROMPT' })

      const job = await p.submitAIJob({
        provider: 'comfyui',
        prompt: '  杂货铺，3/4 俯视  ',
        negativePrompt: '模糊',
        width: 512,
        height: 512,
        count: 2,
        seed: 7,
        assetType: 'building',
        name: '杂货铺',
      })
      expect(job.state).toBe('queued')
      expect(job.prompt).toBe('杂货铺，3/4 俯视')

      await vi.advanceTimersByTimeAsync(400)
      const jobs = await p.listAIJobs()
      expect(jobs[0]?.state).toBe('done')
      expect(jobs[0]?.progress).toBe(100)
      expect(jobs[0]?.images).toHaveLength(2)
      expect(jobs[0]?.images[0]?.url.startsWith('data:image/svg+xml')).toBe(true)
      expect(eventCounts.length).toBeGreaterThan(0)

      const before = (await p.listAssets()).length
      const asset = await p.adoptAIJobImage(job.id, 1, '  杂货铺草稿  ')
      expect(asset.name).toBe('杂货铺草稿')
      expect(asset.type).toBe('building')
      expect(asset.status).toBe('draft')
      expect(asset.source).toMatchObject({ kind: 'generated', provider: 'comfyui', workflow: 'ai-image' })
      expect((await p.listAssets()).length).toBe(before + 1)
      expect((await p.assetURL(asset.id, 'thumb.png')).startsWith('data:image/svg+xml')).toBe(true)

      off()
    } finally {
      vi.useRealTimers()
    }
  })

  it('loads and normalizes the seeded 2.5D map', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const summaries = await p.listMaps()
    expect(summaries).toHaveLength(1)
    expect(summaries[0]).toMatchObject({ name: '小镇广场', width: 40, height: 30, tileSize: 32 })

    const map = await p.loadMap(summaries[0]!.id)
    expect(map.layers).toHaveLength(6)
    expect(map.tilesets[0]).toMatchObject({ columns: 4, tileCount: 16 })
    expect(map.objects).toHaveLength(4)
    expect(map.objects.some((object) => object.kind === 'spawn')).toBe(true)

    const dirty = {
      ...map,
      name: '  小镇广场  ',
      layers: map.layers.map((layer) =>
        layer.type === 'ground'
          ? {
              ...layer,
              tiles: [
                ...layer.tiles,
                { x: -1, y: 0, set: 0, tile: 0 },
                { x: 0, y: 0, set: 0, tile: 99 },
                { x: 1, y: 1, set: 0, tile: 0 },
                { x: 1, y: 1, set: 0, tile: 3 },
              ],
            }
          : layer,
      ),
    }
    const saved = await p.saveMap(dirty)
    expect(saved.name).toBe('小镇广场')
    const ground = saved.layers.find((layer) => layer.type === 'ground')!
    expect(ground.tiles.some((tile) => tile.x === -1)).toBe(false)
    expect(ground.tiles.some((tile) => tile.tile === 99)).toBe(false)
    expect(ground.tiles.filter((tile) => tile.x === 1 && tile.y === 1)).toHaveLength(1)
    expect(ground.tiles.find((tile) => tile.x === 1 && tile.y === 1)?.tile).toBe(3)

    await expect(p.saveMap({ ...saved, name: '   ' })).rejects.toMatchObject({
      code: 'INVALID_MAP',
    })
    await expect(p.saveMap({ ...saved, tileSize: 33 })).rejects.toMatchObject({
      code: 'INVALID_TILE_SIZE',
    })
    await expect(p.loadMap('missing')).rejects.toMatchObject({ code: 'MAP_NOT_FOUND' })
  })

  it('exports a map to Godot as a self-contained project folder', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const map = (await p.listMaps())[0]!
    const result = await p.exportMap(map.id, 'godot')
    expect(result.dir).toBe(`${DEMO_ROOT}\\exports\\maps\\小镇广场-godot`)
    expect(result.format).toBe('godot')
    expect(result.files).toEqual(
      expect.arrayContaining([
        'project.godot',
        '小镇广场.tscn',
        '小镇广场.tres',
        '小镇广场.json',
        'pixelbench_tiles.gd',
        'roof_fade.gd',
        'tileset-1-小镇瓦片图集.png',
      ]),
    )

    const both = await p.exportMap(map.id, 'all')
    expect(both.format).toBe('all')
    expect(both.files).toEqual(expect.arrayContaining(['小镇广场.tmx', '小镇广场.tscn']))
  })

  it('exports a map as TMX, TSX, tileset image and JSON', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const map = (await p.listMaps())[0]!
    const result = await p.exportMap(map.id)
    expect(result.dir).toBe(`${DEMO_ROOT}\\exports\\maps\\小镇广场`)
    expect(result.files).toEqual(
      expect.arrayContaining([
        'tileset-1-小镇瓦片图集.tsx',
        'tileset-1-小镇瓦片图集.png',
        '小镇广场.tmx',
        '小镇广场.json',
      ]),
    )
  })

  it('deletes a map and refuses unknown ids', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const map = (await p.listMaps())[0]!
    await p.deleteMap(map.id)
    expect(await p.listMaps()).toHaveLength(0)
    await expect(p.deleteMap(map.id)).rejects.toMatchObject({ code: 'MAP_NOT_FOUND' })
  })

  it('simulates a masked inpaint job and keeps it out of the adopt flow', async () => {
    vi.useFakeTimers()
    try {
      const p = platform()
      await p.openProject(DEMO_ROOT)
      const base = pngDataURL(32, 32)
      const mask = pngDataURL(32, 32)

      await expect(
        p.submitAIJob({
          provider: 'comfyui',
          prompt: '修手',
          negativePrompt: '',
          width: 32,
          height: 32,
          count: 1,
          seed: 3,
          assetType: 'sprite',
          name: '',
          mode: 'inpaint',
        }),
      ).rejects.toMatchObject({ code: 'INVALID_INPAINT' })

      const job = await p.submitAIJob({
        provider: 'comfyui',
        prompt: '把右手改成握着灯笼',
        negativePrompt: '',
        width: 32,
        height: 32,
        count: 4,
        seed: 3,
        assetType: 'sprite',
        name: 'AI 素材 1',
        mode: 'inpaint',
        targetAssetId: 'asset-1',
        baseImage: base,
        mask,
      })
      expect(job.mode).toBe('inpaint')
      expect(job.count).toBe(1)
      expect(job.targetAssetId).toBe('asset-1')

      await vi.advanceTimersByTimeAsync(400)
      const done = (await p.listAIJobs())[0]!
      expect(done.state).toBe('done')
      expect(done.images).toHaveLength(1)
      expect(done.images[0]?.url.startsWith('data:image/svg+xml')).toBe(true)
      expect(done.images[0]?.width).toBe(32)

      await expect(p.adoptAIJobImage(job.id, 0, '')).rejects.toMatchObject({
        code: 'INPAINT_ADOPT',
      })
      await p.deleteAIJob(job.id)
      expect(await p.listAIJobs()).toHaveLength(0)
    } finally {
      vi.useRealTimers()
    }
  })

  it('cancels queued AI jobs and refuses unknown ids', async () => {
    vi.useFakeTimers()
    try {
      const p = platform()
      await p.openProject(DEMO_ROOT)
      const job = await p.submitAIJob({
        provider: 'openai',
        prompt: '角色行走循环',
        negativePrompt: '',
        width: 64,
        height: 64,
        count: 1,
        seed: 2,
        assetType: 'sprite',
        name: '',
      })
      const canceled = await p.cancelAIJob(job.id)
      expect(canceled.state).toBe('canceled')
      await vi.advanceTimersByTimeAsync(400)
      expect((await p.listAIJobs())[0]?.state).toBe('canceled')
      await expect(p.cancelAIJob('missing')).rejects.toMatchObject({ code: 'JOB_NOT_FOUND' })
    } finally {
      vi.useRealTimers()
    }
  })
})
describe('data tables', () => {
  it('seeds items, skills and enemies with a cross-table reference', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const tables = await p.listTables()
    expect(tables.map((table) => table.key).sort()).toEqual(['enemies', 'items', 'skills'])

    const items = tables.find((table) => table.key === 'items')!
    const enemies = tables.find((table) => table.key === 'enemies')!
    expect(items.rows).toHaveLength(4)
    expect(enemies.rows).toHaveLength(3)

    const drop = enemies.columns.find((column) => column.key === 'drop')!
    expect(drop.type).toBe('ref')
    expect(drop.refTableId).toBe(items.id)
    const apple = items.rows.find((row) => Object.values(row.values).includes('苹果'))!
    expect(enemies.rows.some((row) => row.values[drop.id] === apple.id)).toBe(true)
  })

  it('normalises keys and cells, and refuses a duplicate table key', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const saved = await p.saveTable({
      schemaVersion: 1,
      id: '',
      name: '  掉落表  ',
      key: 'Drop Table!',
      note: '',
      columns: [
        { id: '', name: ' 名称 ', key: 'Name', type: 'text', required: true, options: [] },
        { id: '', name: '数量', key: 'amount', type: 'int', required: false, options: [] },
        { id: '', name: '', key: 'amount', type: 'bool', required: false, options: [] },
      ],
      rows: [
        { id: '', values: {} },
        { id: '', values: {} },
      ],
      createdAt: '',
      updatedAt: '',
    })
    expect(saved.name).toBe('掉落表')
    expect(saved.key).toBe('drop_table')
    expect(saved.columns.map((column) => column.key)).toEqual(['name', 'amount', 'amount_2'])
    expect(saved.columns[2]?.name).toBe('第 3 列')
    expect(saved.rows[0]?.id).not.toBe('')
    expect(saved.rows[0]?.id).not.toBe(saved.rows[1]?.id)
    expect((await p.listTables()).some((table) => table.id === saved.id)).toBe(true)

    await expect(
      p.saveTable({ ...saved, id: '', name: '另一张', key: 'items' }),
    ).rejects.toMatchObject({ code: 'TABLE_KEY_TAKEN' })
    await expect(p.saveTable({ ...saved, name: '   ' })).rejects.toMatchObject({
      code: 'INVALID_TABLE',
    })
  })

  it('coerces cells to their column type on save', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const [items] = await p.listTables()
    const price = items!.columns.find((column) => column.key === 'price')!
    const name = items!.columns.find((column) => column.key === 'name')!
    const saved = await p.saveTable({
      ...items!,
      rows: [
        {
          id: '',
          values: {
            [price.id]: 12.9,
            [name.id]: '  苹果  ',
          },
        },
        { id: '', values: { [price.id]: 'not-a-number' } },
      ],
    })
    expect(saved.rows[0]?.values[price.id]).toBe(12)
    // Text keeps its own whitespace; only the cell type gate applies.
    expect(saved.rows[0]?.values[name.id]).toBe('  苹果  ')
    expect(saved.rows[1]?.values[price.id]).toBeUndefined()
  })

  it('refuses to delete a table other tables reference', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const items = (await p.listTables()).find((table) => table.key === 'items')!
    await expect(p.deleteTable(items.id)).rejects.toMatchObject({ code: 'TABLE_IN_USE' })

    const enemies = (await p.listTables()).find((table) => table.key === 'enemies')!
    const drop = enemies.columns.find((column) => column.type === 'ref')!
    await p.saveTable({
      ...enemies,
      columns: enemies.columns.map((column) =>
        column.id === drop.id ? { ...column, type: 'text', refTableId: undefined } : column,
      ),
    })
    await p.deleteTable(items.id)
    expect((await p.listTables()).some((table) => table.key === 'items')).toBe(false)
    await expect(p.deleteTable(items.id)).rejects.toMatchObject({ code: 'TABLE_NOT_FOUND' })
  })

  it('exports a valid table to JSON and CSV under the table key', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const enemies = (await p.listTables()).find((table) => table.key === 'enemies')!
    const result = await p.exportTable(enemies.id)
    expect(result.dir).toBe(`${DEMO_ROOT}\\exports\\tables\\enemies`)
    expect(result.files).toEqual(['enemies.json', 'enemies.csv'])
  })

  it('blocks export while a reference dangles', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const items = (await p.listTables()).find((table) => table.key === 'items')!
    await p.saveTable({ ...items, rows: items.rows.slice(0, 1) })
    const enemies = (await p.listTables()).find((table) => table.key === 'enemies')!
    await expect(p.exportTable(enemies.id)).rejects.toMatchObject({ code: 'TABLE_INVALID' })
  })
})

describe('setting library', () => {
  it('opens with linked seed entries', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const entities = await p.listEntities()
    expect(entities.map((entity) => entity.name)).toEqual(
      expect.arrayContaining(['小镇广场', '旅人']),
    )
    const plaza = entities.find((entity) => entity.name === '小镇广场')
    const traveler = entities.find((entity) => entity.name === '旅人')
    expect(plaza?.mapIds.length).toBeGreaterThan(0)
    expect(plaza?.assetIds.length).toBeGreaterThan(0)
    expect(traveler?.relations[0]?.entityId).toBe(plaza?.id)
  })

  it('normalises a saved entry', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const [target] = await p.listEntities()
    const saved = await p.saveEntity({
      schemaVersion: 1,
      id: '',
      kind: 'faction',
      name: '  盐商行会 ',
      status: 'stable',
      aliases: [' 盐商 ', '盐商', ''],
      summary: ' 控制着码头。 ',
      body: '',
      tags: [],
      fields: [
        { key: ' ', value: '丢弃' },
        { key: ' 总部 ', value: ' 雾湾 ' },
      ],
      relations: [
        { entityId: target!.id, label: ' 位于 ' },
        { entityId: 'bad id', label: '无效' },
      ],
      assetIds: [],
      mapIds: [],
      artPrompt: '',
      createdAt: '',
      updatedAt: '',
    })
    expect(saved.name).toBe('盐商行会')
    expect(saved.aliases).toEqual(['盐商'])
    expect(saved.summary).toBe('控制着码头。')
    expect(saved.fields).toEqual([{ key: '总部', value: '雾湾' }])
    expect(saved.relations).toEqual([{ entityId: target!.id, label: '位于' }])
    expect(saved.id).not.toBe('')
    expect((await p.listEntities()).some((entity) => entity.id === saved.id)).toBe(true)
  })

  it('removes relations when the target is deleted', async () => {
    const p = platform()
    await p.openProject(DEMO_ROOT)
    const [target] = await p.listEntities()
    const source = await p.saveEntity({
      schemaVersion: 1,
      id: '',
      kind: 'character',
      name: '账房',
      status: 'draft',
      aliases: [],
      summary: '',
      body: '',
      tags: [],
      fields: [],
      relations: [{ entityId: target!.id, label: '受雇于' }],
      assetIds: [],
      mapIds: [],
      artPrompt: '',
      createdAt: '',
      updatedAt: '',
    })
    await p.deleteEntity(target!.id)
    const entities = await p.listEntities()
    const kept = entities.find((entity) => entity.id === source.id)
    expect(kept?.relations).toEqual([])
    await expect(p.deleteEntity(target!.id)).rejects.toMatchObject({ code: 'ENTITY_NOT_FOUND' })
  })
})
