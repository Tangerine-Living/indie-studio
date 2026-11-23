import { createI18n } from 'vue-i18n'

/**
 * v1 ships Chinese-first. English copy is in place so the layout and switch
 * can land before translation is complete; missing keys fall back to zh-CN.
 */
const zhCN = {
  app: {
    name: 'Indie Studio',
    tagline: '2D 与 2.5D 俯视角独立游戏的素材与关卡工作台',
  },
  nav: {
    dashboard: '总览',
    quests: '任务',
    lore: '设定',
    tables: '数值',
    assets: '素材',
    generate: '生成台',
    maps: '地图',
    export: '导出',
    settings: '设置',
  },
  action: {
    save: '保存',
    cancel: '取消',
    delete: '删除',
    create: '创建',
    edit: '编辑',
    close: '关闭',
    search: '搜索',
    retry: '重试',
    back: '返回',
  },
  status: {
    backlog: '待整理',
    todo: '待办',
    doing: '进行中',
    review: '待验收',
    done: '已完成',
  },
  priority: {
    low: '低',
    medium: '中',
    high: '高',
  },
  questType: {
    asset: '素材',
    code: '程序',
    design: '设计',
    audio: '音频',
    marketing: '宣发',
  },
  assetType: {
    sprite: '角色',
    building: '建筑',
    tile: '瓦片',
    prop: '道具',
    ui: '界面',
    fx: '特效',
    other: '其他',
  },
  assetStatus: {
    draft: '草稿',
    review: '待审核',
    approved: '已批准',
    deprecated: '已弃用',
  },
  stage: {
    concept: '概念',
    prototype: '原型',
    production: '制作',
    polish: '打磨',
    release: '发布',
  },
  entityKind: {
    character: '角色',
    place: '地点',
    faction: '势力',
    item: '物品',
    event: '事件',
    term: '术语',
  },
  entityStatus: {
    idea: '构想',
    draft: '草稿',
    stable: '定稿',
  },
}

const enUS = {
  app: {
    name: 'Indie Studio',
    tagline: 'Asset and level workbench for 2D and 2.5D top-down indie games',
  },
  nav: {
    dashboard: 'Overview',
    quests: 'Quests',
    lore: 'Setting',
    tables: 'Data',
    assets: 'Assets',
    generate: 'Generate',
    maps: 'Maps',
    export: 'Export',
    settings: 'Settings',
  },
  action: {
    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    create: 'Create',
    edit: 'Edit',
    close: 'Close',
    search: 'Search',
    retry: 'Retry',
    back: 'Back',
  },
  status: {
    backlog: 'Backlog',
    todo: 'To do',
    doing: 'In progress',
    review: 'In review',
    done: 'Done',
  },
  priority: { low: 'Low', medium: 'Medium', high: 'High' },
  questType: {
    asset: 'Asset',
    code: 'Code',
    design: 'Design',
    audio: 'Audio',
    marketing: 'Marketing',
  },
  assetType: {
    sprite: 'Sprite',
    building: 'Building',
    tile: 'Tile',
    prop: 'Prop',
    ui: 'UI',
    fx: 'FX',
    other: 'Other',
  },
  assetStatus: {
    draft: 'Draft',
    review: 'In review',
    approved: 'Approved',
    deprecated: 'Deprecated',
  },
  stage: {
    concept: 'Concept',
    prototype: 'Prototype',
    production: 'Production',
    polish: 'Polish',
    release: 'Release',
  },
  entityKind: {
    character: 'Character',
    place: 'Place',
    faction: 'Faction',
    item: 'Item',
    event: 'Event',
    term: 'Term',
  },
  entityStatus: {
    idea: 'Idea',
    draft: 'Draft',
    stable: 'Settled',
  },
}

export const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

export default i18n