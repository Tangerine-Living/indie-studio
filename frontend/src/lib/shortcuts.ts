export interface ShortcutGroup {
  title: string
  items: ReadonlyArray<readonly [keys: string, action: string]>
}

/** One place for the shortcut sheet in settings and the ? dialog. */
export const SHORTCUT_GROUPS: ReadonlyArray<ShortcutGroup> = [
  {
    title: '全局',
    items: [
      ['Ctrl + S', '保存当前项目'],
      ['Ctrl + K', '打开命令面板与全局搜索'],
      ['?', '打开快捷键速查'],
      ['Esc', '关闭弹窗 / 退出预览'],
    ],
  },
  {
    title: '像素编辑器',
    items: [
      ['B / E / G / I', '铅笔 / 橡皮 / 油漆桶 / 吸管'],
      ['L / R / O', '直线 / 矩形 / 椭圆'],
      ['M / A / W / V', '矩形选区 / 套索 / 魔棒 / 移动'],
      ['Ctrl + A / Ctrl + D', '全选 / 取消选区'],
      ['Ctrl + C / X / V', '复制 / 剪切 / 粘贴'],
      ['Delete', '清除选区内容'],
      ['[ / ]', '减小 / 增大笔刷'],
      ['+ / -', '缩放画布'],
      ['空格拖动 / 中键拖动', '平移画布'],
      ['右键拖动', '用橡皮擦除像素'],
      ['Enter', '播放 / 暂停帧动画'],
      ['← / →', '上一帧 / 下一帧'],
      ['Ctrl + Z / Ctrl + Y', '撤销 / 重做'],
    ],
  },
  {
    title: '地图编辑器',
    items: [
      ['B / E / R / I', '画笔 / 橡皮 / 矩形 / 吸管'],
      ['C / V', '碰撞笔刷 / 对象工具'],
      ['G', '显示 / 隐藏网格'],
      ['中键拖动 / 空格拖动', '平移画布'],
      ['滚轮', '以光标为中心缩放'],
      ['Delete', '删除选中对象'],
      ['Ctrl + Z / Ctrl + Y', '撤销 / 重做'],
      ['Ctrl + S', '立即保存地图'],
    ],
  },
  {
    title: '行走预览',
    items: [
      ['WASD / 方向键', '移动角色'],
      ['鼠标滚轮', '缩放镜头'],
      ['Esc', '退出预览'],
    ],
  },
]