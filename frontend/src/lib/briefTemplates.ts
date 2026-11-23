import type { AssetType, QuestBrief } from '../domain/types'

/**
 * Per-asset-type brief scaffolds. They fill the style line, a prompt skeleton
 * and an acceptance checklist so a new quest starts with something reviewable
 * instead of a blank form. Placeholders: {title}, {tile}.
 */
export interface BriefTemplate {
  label: string
  style: string
  prompt: string
  acceptance: string[]
}

const TEMPLATES: Record<AssetType, BriefTemplate> = {
  sprite: {
    label: '角色 / 精灵',
    style: '3/4 俯视，四方向，1px 深色描边，脚底对齐 {tile}×{tile} 网格',
    prompt: '「{title}」四方向行走循环，每方向 4 帧，主体居中，透明背景',
    acceptance: ['四方向各 4 帧', '脚底对齐瓦片网格', '不超过 16 色', '无抗锯齿杂色'],
  },
  building: {
    label: '建筑',
    style: '3/4 俯视正交，木结构，暖色招牌，光来自左上，1px 深色描边',
    prompt: '「{title}」小镇商铺，占地 4×3 格（{tile}px 瓦片），两层，带招牌与屋檐阴影，透明背景',
    acceptance: ['锚点在底部中心', '碰撞与占地一致', '屋檐阴影方向统一', '不超过 16 色'],
  },
  tile: {
    label: '瓦片',
    style: '正俯视地面瓦片，可无缝平铺，1px 深色描边',
    prompt: '「{title}」地面瓦片，{tile}×{tile}，四边可无缝平铺，2–3 种变体',
    acceptance: ['四边无缝', '与项目瓦片尺寸一致', '与默认调色板一致'],
  },
  prop: {
    label: '道具',
    style: '3/4 俯视，与建筑同一光源，1px 深色描边',
    prompt: '「{title}」场景道具，主体居中，脚底对齐 {tile}×{tile} 网格，透明背景',
    acceptance: ['比例与角色一致', '锚点在底部中心', '不超过 16 色'],
  },
  ui: {
    label: '界面图标',
    style: '平面像素图标，1px 深色描边，单一强调色',
    prompt: '「{title}」界面图标，正面视角，居中构图，透明背景',
    acceptance: ['与既有图标尺寸一致', '缩小到 16px 仍可辨识', '最多 4 色'],
  },
  fx: {
    label: '特效',
    style: '像素特效，逐帧动画，深色背景上清晰',
    prompt: '「{title}」像素特效动画，4–6 帧循环，透明背景',
    acceptance: ['帧间过渡自然', '循环首尾衔接', '不超过 8 色'],
  },
  other: {
    label: '其他',
    style: '像素风，单一主体，1px 深色描边',
    prompt: '「{title}」单件像素素材，主体居中，透明背景',
    acceptance: ['尺寸与用途匹配', '不超过 16 色'],
  },
}

export function briefTemplateFor(type: AssetType | ''): BriefTemplate | null {
  if (!type) return null
  return TEMPLATES[type] ?? null
}

function expand(text: string, title: string, tile: number): string {
  return text.replace(/\{title\}/g, title).replace(/\{tile\}/g, String(tile))
}

export interface ApplyTemplateInput {
  brief: QuestBrief
  acceptance: string[]
  /** Quest title used for the {title} placeholder. */
  title?: string
  /** Replace existing style/prompt instead of only filling blank fields. */
  overwrite?: boolean
}

/** Brief and acceptance list after merging the template for `brief.assetType`. */
export interface ApplyTemplateResult {
  brief: QuestBrief
  acceptance: string[]
}

/**
 * Merges the template for the brief's asset type. Acceptance items are
 * appended, trimmed, deduplicated and capped at 12.
 */
export function applyBriefTemplate({
  brief,
  acceptance,
  title = '',
  overwrite = false,
}: ApplyTemplateInput): ApplyTemplateResult {
  const template = briefTemplateFor(brief.assetType)
  if (!template) return { brief: { ...brief }, acceptance: [...acceptance] }

  const name = title.trim() || '新素材'
  const tile = Math.max(1, Math.round(brief.tileSize) || 32)
  const style = expand(template.style, name, tile)
  const prompt = expand(template.prompt, name, tile)

  const mergedBrief: QuestBrief = {
    ...brief,
    style: overwrite || !brief.style.trim() ? style : brief.style,
    prompt: overwrite || !brief.prompt.trim() ? prompt : brief.prompt,
  }

  const merged: string[] = []
  const seen = new Set<string>()
  for (const raw of [...acceptance, ...template.acceptance.map((item) => expand(item, name, tile))]) {
    const item = raw.trim()
    if (!item || seen.has(item)) continue
    seen.add(item)
    merged.push(item)
    if (merged.length >= 12) break
  }

  return { brief: mergedBrief, acceptance: merged }
}

/** True when the brief has a type but no style or prompt yet. */
export function briefNeedsTemplate(brief: QuestBrief): boolean {
  return Boolean(brief.assetType) && !brief.style.trim() && !brief.prompt.trim()
}

/** Expanded acceptance items of one type's template (used to drop stale ones). */
export function templateAcceptanceFor(
  type: AssetType | '',
  title = '',
  tileSize = 32,
): string[] {
  const template = briefTemplateFor(type)
  if (!template) return []
  const name = title.trim() || '新素材'
  const tile = Math.max(1, Math.round(tileSize) || 32)
  return template.acceptance.map((item) => expand(item, name, tile))
}