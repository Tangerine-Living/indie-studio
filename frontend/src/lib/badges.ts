import type { AssetStatus, Priority, QuestStatus } from '../domain/types'

export const QUEST_STATUS_CLASS: Record<QuestStatus, string> = {
  backlog: 'border-line bg-panel-3 text-muted',
  todo: 'border-blue/25 bg-blue/10 text-blue',
  doing: 'border-accent/30 bg-accent-soft text-accent',
  review: 'border-[#c99cff]/25 bg-[#c99cff]/10 text-[#c99cff]',
  done: 'border-green/25 bg-green/10 text-green',
}

export const QUEST_STATUS_DOT: Record<QuestStatus, string> = {
  backlog: 'bg-faint',
  todo: 'bg-blue',
  doing: 'bg-accent',
  review: 'bg-[#c99cff]',
  done: 'bg-green',
}

export const ASSET_STATUS_CLASS: Record<AssetStatus, string> = {
  draft: 'border-line bg-panel-3 text-muted',
  review: 'border-[#c99cff]/25 bg-[#c99cff]/10 text-[#c99cff]',
  approved: 'border-green/25 bg-green/10 text-green',
  deprecated: 'border-red/25 bg-red/10 text-red',
}

export const PRIORITY_DOT_CLASS: Record<Priority, string> = {
  low: 'bg-faint',
  medium: 'bg-blue',
  high: 'bg-red',
}