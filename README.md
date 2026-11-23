# Indie Studio · 像素工坊

面向中文独立开发者的 **2D / 2.5D 俯视角游戏创作工作台**：把「项目 + 任务（quests）」的组织方式，和「素材生成 / 像素编辑 / 关卡编辑」的工具链放在同一个本地桌面应用里。

目标不是再造一个游戏引擎，而是把素材与关卡做好，再导出到 Godot / Tiled / 通用格式。

**以 AI 素材管理与生成为主流程**：ComfyUI / 云接口出草稿 → 切图 / 像素完美后处理 → 人工采用进入素材库。程序化建筑生成只作为离线备用，不抢占主流程。

## 当前状态

**M0 · 项目骨架**（v0.1.0-dev）已可用：

- 新建 / 打开 / 关闭项目，最近项目列表，项目锁与强制打开
- 普通文件夹 + JSON 落盘（`project.json`、`assets/`、`quests/`、`palettes/`），可直接用 git 管理
- 总览、任务看板 / 列表、素材库（含详情与版本）、设置页
- 任务与素材双向关联、验收清单、评论、里程碑
- Go 端缩略图 / 图片通过带 token 的 AssetServer 提供，不走 Wails JSON
- 浏览器 mock 模式：不启动 Go 也能跑完整 UI（含示例项目）

**M1 · 素材库（进行中）**，第一块已可用：

- 导入 PNG / JPEG / GIF / WebP / BMP；原件保存在 `source/`，永不修改
- SHA-256 按内容去重，同一文件重复导入会跳过并提示
- Go 生成缩略图：小尺寸像素图整数最近邻放大（最多 4×），大图等比缩小
- 导入类型、名称、尺寸、锚点与来源哈希写入 `asset.json`
- 网格 / 列表两种视图（选择按机器记忆）；列表显示类型、状态、尺寸与最近更新
- 多选（卡片复选框、Ctrl+点击、Shift+范围）后批量改评审状态、加 / 移除标签（不区分大小写）、批量导出或批量删除；批准仍会联动完成任务
- 素材详情显示哪些地图把它用作图集或对象，点一下直接跳到地图编辑器打开该地图
- 版本历史任选两版并排对比（最近邻显示，只读 `versions/<版本>/output.png`，不改动文件）
- 删除（单条或批量）把整个素材目录移到 `.workbench/trash/assets/`，确认框会提示地图图集与任务引用将失效；回收站保留在磁盘上可手工找回

**M1 · 像素编辑器**，第二块已可用：

- 从素材详情进入，按素材原始尺寸建立文档；图层可新增 / 删除 / 显隐 / 调不透明度
- 工具：铅笔、橡皮、油漆桶、吸管、直线、矩形、椭圆（可填充），矩形选区、套索、魔棒、移动，1–4px 方形笔刷
- 选区：全选 / 反选 / 取消，复制 / 剪切 / 粘贴（应用内与系统 PNG 两种剪贴板），Delete 清除选区内容；Shift 加选
- 像素完美手绘（1px 笔刷去掉折角处多余像素）与左右 / 上下 / 双轴对称绘制
- 油漆桶与魔棒共享 0–255 容差和「邻近填充」开关；「替换颜色」按原色 + 容差把当前图层（或选区）换成当前颜色
- 整数缩放（1×–32×）与适应窗口、中键或 Alt 拖动画布、棋盘格与网格、悬停像素提示
- 撤销 / 重做（含图层、帧结构与不透明度，上限 60 步），快捷键 B/E/G/I/L/R/O/M/A/W/V、`[` `]`、`+` `-`、Ctrl+A/D/C/X/V
- 帧动画时间轴：每图层独立帧缓冲，复制当前帧 / 删除帧、1–60 FPS 循环播放、空格播放、← → 切帧
- 洋葱皮：上一帧红色、下一帧青色半透明叠示，播放时自动隐藏
- 精灵表切片：按帧宽高、外边距与间隙从原始素材切分，网格预览，应用后可撤销
- 精灵表导出：全部帧按列数打包为一张 PNG，保存为不可变版本；Go 同时生成逐帧 PNG、Aseprite JSON 与 TexturePacker JSON，缩略图取第一帧
- 保存当前帧：合成所有可见图层写成 `versions/NNNN/`；动画素材再次打开时从 `frames/000.png` 载入
- 逐帧时长：时间轴「帧 ms」设置当前帧停留时间，播放按每帧各自时长推进；修改基准 FPS 会重置为均匀时长
- 动画标签：保存精灵表时填写 idle / walk 等标签，写入 Aseprite `frameTags` 与 TexturePacker `animations`；留空时使用 `default`
- 调色板取项目调色板，颜色输入框与取色器回填

**M1 · 调色板管理（已完成）**：

- 「设置 → 调色板」列出项目调色板：新建副本、设为项目默认、重命名、逐色增删
- 导入 GPL（GIMP）/ JASC PAL / 十六进制列表与纯 RGB 列表；导出为 `.gpl`、`.pal` 或 `.hex`
- 颜色统一小写、`#abc` 展开为 `#aabbcc` 并去重；「裁剪到 N 色」按风格指南的 `maxColors` 保留前 N 色
- 调色板写入 `palettes/<id>.json`；标记为锁定的调色板只读，先新建副本再改

**M2 · 程序化建筑（离线备用，AI 为主流程）**：

- 参数化 2.5D 俯视建筑：占地宽深、层数、瓦片尺寸、墙体 / 屋顶 / 窗框 / 门颜色、每层窗户、招牌、做旧程度、投影与 1px 描边
- 同一组参数与种子结果可复现；一次生成 1–8 个变体，先预览，再逐个采用
- 采用后写成草稿素材：`source/generated.png` 与 `source/generated.base.png` / `.roof.png`，`versions/0001/` 保存合成图、分层图与 `meta.json`（生成器、种子、参数、锚点、碰撞）
- 锚点在底部中心，碰撞为占地矩形，直接对齐地图瓦片网格
- 桌面端由 Go 栅格化为 PNG；浏览器 mock 用同一几何的 SVG 预览，便于纯前端开发与测试

**M2 · 像素完美后处理（已完成）**：

- 链条：裁切透明边 → 面积平均降采样（小图最近邻放大）→ OKLab 调色板量化 → 去杂点 → 1px 描边 → 瓦片接缝检查
- 降采样按 alpha 覆盖率平均、缩完再二值化，软边 AI 图不会在缩放前就被裁掉边缘
- 保存版本时按新旧尺寸重算锚点，大图缩到 32px 后锚点仍落在原位置
- 抖动：无 / Floyd–Steinberg（蛇形扫描）/ Bayer 4×4 / 确定性噪声，强度可调
- 调色板：项目调色板，或自动取色（OKLab 加权中位切分，2–64 色）；显式颜色去重后直接使用
- 1px 描边默认取调色板最深色，结果不越出调色板；贴边素材的描边不会被裁掉，画布自动补 1px
- 「长边 16 / 24 / 32 / 48 / 64」一键设目标尺寸并重新预览，方便把 AI 大图缩到常用像素尺寸
- 素材详情里 A/B 预览原图与处理结果、实际用色与左右/上下接缝差异；确认后保存为不可变版本
- `versions/NNNN/meta.json` 的 `process` 记录归一化参数、实际调色板与接缝结果，便于复现与审计
- 浏览器 mock 无法解码像素，返回同尺寸/调色板的模拟预览并明确标注

**M2 · AI 适配器（已完成）**：

- 两个供应商适配器：本地 ComfyUI（`/prompt` 提交、轮询 `/history`、经 `/view` 取图）与 OpenAI 兼容图像接口（`/images/generations`，支持 `b64_json` 与 `url` 两种返回）
- ComfyUI 工作流以 API 格式 JSON 保存；节点用 `_meta.title` 标记 `PROMPT` / `NEGATIVE` / `SEED` / `SIZE`，提交前自动替换，未标记时按节点类型兜底
- 顺序任务队列：排队 / 生成中 / 完成 / 失败 / 已取消，进度事件推送到前端；任务 JSON 落在 `.workbench/jobs/`，应用重启后把中断任务标为失败并可重试
- 已结束任务可逐条删除或「清空已结束」，同时删除任务 JSON 与 `.workbench/generated/<id>/` 结果；进行中的任务必须先取消，已采用的素材保留自己的副本不受影响
- 结果图保存在 `.workbench/generated/<jobId>/`，经带 token 的 `/pb-gen/` 提供；确认后「采用」为草稿素材（记录 provider、提示词、种子与尺寸）
- API Key 用 Windows DPAPI 加密，保存在 `%APPDATA%\PixelBench\ai.json`，不写入项目目录、不进入日志；云调用只发送表单里的提示词、尺寸与种子
- 提交云 API 前弹出确认框，原样列出提示词 / 负面提示词 / 尺寸 / 数量 / 种子并说明可能产生费用；本地 ComfyUI 不弹确认
- 「设置 → AI 生成」可保存 / 测试两个通道；「生成台 → AI 草稿」负责提交、查看队列进度、取消与采用；浏览器 mock 用定时器与 SVG 模拟整条链路

**M2 · 动画时长 / 标签与 exports 导出（已完成）**：

- 精灵表版本记录逐帧毫秒时长与动画标签；Aseprite JSON 每帧 `duration` 独立，`frameTags` 与 TexturePacker `animations` 按标签生成
- 素材详情「导出」把当前版本复制到 `exports/<名称>/<版本>/`：`output.png`、`meta.json`、精灵表的两个 JSON 与逐帧 PNG，另写 `export.json` 标记归属
- 同名素材不会互相覆盖（第二个起使用名称 + 素材 ID 后缀）；导出只读取版本文件，不修改素材库
- 「打开导出目录」直接打开项目的 `exports/`；浏览器 mock 返回与实际一致的路径与文件清单

**P1 · AI 方向图 / 特效精灵表（已完成）**：

- 生成台提供「4 方向 / 8 方向 / 特效序列」布局按钮：把网格、等距、脚底对齐、透明背景与推荐尺寸追加到提示词，让模型直接按 4×1 / 4×2 / 横向 4 帧出图
- 结果卡片的「切图」把一张 AI 图切成草稿动画素材：逐格可选去纯色背景（从边缘泛洪）、按 alpha 裁边、以最大帧统一缩放、共用一份调色板量化、1px 描边、底部居中对齐
- 每帧画布为「帧尺寸 + 2」，给描边留边；逐帧裁边但统一缩放，避免动画抖动；共用调色板避免颜色闪烁
- 可设置列 / 行 / 帧尺寸 / FPS / 动画标签；保存为 `ai-sheet` 来源的草稿素材，带 `animation` 元数据（帧数、列数、帧宽高、FPS），可直接在素材库回看或放入地图

**P1 · 地图扩展与 AI 补全（已完成）**：

- 地图编辑器右侧「扩展画布」按上 / 右 / 下 / 左增加格数（单边最多 256，总尺寸不超过 1024），现有瓦片与对象整体平移
- 可选「沿用边界地形」：自动拼接图层用基准瓦片填充新增格并重算边界，让旧地形自然延伸；扩展立即写入磁盘、不进入撤销栈
- 「AI 补全新区域」把地图名、尺寸、瓦片大小、现有图集与项目风格指南拼成提示词，跳到生成台并预填为地形瓦片 / 装饰道具 / 特效；生成并采用后再画入新区域

**M3 · 任务 brief 模板与评审联动（已完成）**：

- 素材 brief 模板：角色 / 建筑 / 瓦片 / 道具 / 界面 / 特效各有风格、提示词骨架与验收清单，占位符 `{title}`、`{tile}` 自动替换
- 改变素材类型时，仅在风格与提示词都为空的情况下自动填入；「套用模板」可显式覆盖已填内容并追加缺失的验收项
- 「从任务生成草稿」先保存任务，再带着 brief 跳到生成台：切到 AI 草稿、预填素材类型 / 名称 / 提示词 / 瓦片尺寸，并显示来源任务横幅
- 素材详情可修改评审状态（草稿 / 待审核 / 已批准 / 已弃用）；批准素材时，关联且处于「待验收」的任务自动转为「已完成」
- 总览加入任务与素材合并的最近活动、里程碑新增 / 删除、AI 生成队列状态卡；移除过期的 M1 占位提示
- 总览封面（从素材中任选，随删除自动清除失效引用）与制作阶段步进条（概念 → 原型 → 制作 → 打磨 → 发布），都写入 `project.json`

**M4 · 2.5D 俯视角地图编辑器（已完成）**：

- 地图文档 `maps/<mapId>.map.json`：`tileSize` 16/24/32/48，格宽高 1–1024，图集引用素材，图层带类型 / 高程 / 可见 / 不透明度 / 稀疏瓦片，另有对象与地图设置
- 六个默认图层：地面、装饰、建筑、屋顶、对象、碰撞；另可添加光照图层，瓦片只存已绘制的格，保存时排序、去重、丢弃越界与悬空引用
- 全高画布编辑器（Canvas 2D，无额外渲染库）：整数缩放 50%–600%、中键 / 空格平移、滚轮缩放、视口裁剪、像素最近邻、网格与悬停格
- 工具：画笔、橡皮、矩形、吸管、碰撞笔刷、对象选择 / 拖动；右键擦除，B/E/R/I/C/V 切换，Ctrl+Z/Y 撤销重做，Ctrl+S 保存，700ms 自动保存
- 图集拾取：多图集切换、瓦片网格预览与高亮；自动拼接 16 格（四邻）与 47 格（四邻 + 内角），图集基准瓦片可重设，「重算边界」可修复已有地形
- 对象：出生点 / NPC / 宝箱 / 触发器 / 建筑 / 道具，矩形与半格拖动，名称、类型、Y 排序、碰撞、屋檐淡出、高程与自定义属性
- 深度 = `(y + h) × tileSize + elevation × heightStep`；行走预览执行 WASD / 方向键移动、碰撞与相机跟随，靠近屋顶时按设置自动淡出
- 导出到 `exports/maps/<名称>/`：Tiled 1.10 TMX + TSX（CSV 编码、翻转标志、图层属性、对象属性）、图集 PNG，以及通用 `pixelbench-map` JSON（图层与稀疏瓦片、碰撞网格、对象深度）
- 示例项目内建 40×30「小镇广场」与程序化瓦片图集，Go 与浏览器 mock 使用同一布局，打开即可编辑与导出
- 光照图层：用图集里的柔光块绘制，渲染时以 `screen` 叠加在屋顶之上，亮度受图层不透明度控制；图层面板新增不透明度滑杆
- 高程当前按图层 `elevation × heightStep` 做垂直偏移；自动选择悬崖 / 台阶边缘瓦片需要专用图集，留待后续迭代

**M5 · Godot 导出、打包与引导（已完成）**：

- Godot 4 导出写入 `exports/maps/<名称>-godot/`：自包含工程（`project.godot`）、`<名称>.tres` TileSet、`<名称>.tscn` 场景、图集 PNG 与通用 `<名称>.json`
- TileSet 按图集列出全部瓦片，`texture_region_size = tileSize`，每瓦片 `y_sort_origin = tileSize`；翻转 / 旋转映射到 Godot `TRANSFORM_FLIP_H` / `FLIP_V` / `TRANSPOSE`
- 场景按图层拆成 4 个 `TileMapLayer`（地面 / 装饰 / 建筑 / 屋顶），带 `z_index` 与高程 `position`；对象导出为底部中心锚点的 `Marker2D`（含 `pb_*` metadata 与自定义属性）；碰撞合并为 `StaticBody2D` 上的矩形
- 地块不写无法核对的打包 `tile_map_data`：`pixelbench_tiles.gd` 是 `@tool` 脚本，打开场景时从同名 JSON 读取瓦片并 `set_cell`；`roof_fade.gd` 是可选的角色靠近屋檐淡出脚本
- 地图编辑器与新增「导出」页都可选择 Tiled / Godot / 两者；后端 `ExportMap(id, format)` 统一分派
- 自动更新：`cmd/pbmanifest` 生成 Ed25519 密钥、对清单 `latest.json` 产出分离签名；应用端校验签名与频道后才提示更新，下载后校验 SHA-256，只落盘不执行
- 设置页可切换 stable / beta、开关启动检查、填写清单地址；手动「检查更新」显示版本、说明与下载结果
- 启动检查已接入：已填写清单地址且开关打开时，启动后在后台静默校验一次，失败不弹错误；发现新版本时顶栏出现「新版本 x.y.z」入口，点击进入设置的更新面板
- 引导：首次进入项目的三步浮层（任务 → 素材/生成 → 地图/导出，可跳过且只出现一次），`?` 打开按模块分组快捷键表，设置页展示同一份速查
- 命令面板与全局搜索：`Ctrl+K`（或点顶栏搜索框）呼出，按名称 / 类型 / 状态 / 标签搜索任务、素材与地图，↑↓ + Enter 直接跳到对应模块并打开详情；同样可以跳转各模块、保存项目与打开快捷键表
- 打包：`scripts/package.ps1` 产出便携 zip（附 README）与 NSIS 安装包；安装包默认简体中文、英文备选，带 WebView2 检测
- 已实测产物：`build/bin/indie-studio-0.1.0-portable.zip`（约 5.5 MB）与 `build/bin/indie-studio-0.1.0-installer.exe`（约 7.6 MB，文件版本 0.1.0，公司 / 产品名 Indie Studio，内含 WebView2 引导安装器）
- 应用图标取自界面里的 5×5 像素标记（琥珀 / 薄荷绿），由 `scripts/make-icon.py` 生成多尺寸 `icon.ico`（BMP 条目，NSIS 兼容）与 1024 的 `appicon.png`

Godot 工程已用 **Godot 4.3 stable（headless）** 实测：`--import` 导入图集、`--editor` 打开工程、直接运行主场景均无报错；场景加载后 4 个图层分别写入 1200 / 17 / 12 / 12 个格，TileSet 的 `tile_size`、图集区域与 16 个瓦片正确，对象 Marker2D 的深度与自定义属性、两块合并碰撞矩形的尺寸和位置均与地图数据一致；翻转 / 旋转在 Godot 内得到的 `alternative_tile` 与 `TRANSFORM_FLIP_H`(4096) / `FLIP_V`(8192) / `TRANSPOSE`(16384) 组合完全对应。另外用 OpenGL 窗口渲染了一张 640×480 截图，确认草地、道路、池塘、建筑 / 屋顶与对象都能正常出画（截图在 `build/verify/godot-export-preview.png`），但没有逐像素与编辑器画面比对。安装包已在本机用 NSIS 3.13 构建，未在干净虚拟机上做安装 / 卸载回归。

全部里程碑：

| 阶段 | 内容 |
|---|---|
| M1 | 精灵表切片与导出（已完成：导入、像素编辑器、帧时间轴、切片、精灵表版本） |
| M2 | 程序化建筑、像素完美后处理链、ComfyUI / OpenAI 适配器、逐帧时长 / 标签、exports 导出（均已完成） |
| M3 | 总览与任务增强、brief 模板、从任务生成草稿、评审联动（已完成） |
| M4 | 2.5D 地图编辑器（地图文档、自动拼接、Y 排序、行走预览、Tiled 导出，已完成） |
| M5 | Godot 导出、打包脚本、签名更新清单、引导与快捷键（已完成） |
| P1 | AI 方向 / 特效精灵表、地图画布扩展与 AI 补全（已完成） |

## 技术栈

| 层 | 选择 |
|---|---|
| 桌面壳 | Wails v2 + Go 1.25 |
| 前端 | Vue 3.5（`<script setup>`）+ Pinia + vue-i18n + TypeScript |
| 样式 | Tailwind CSS v4 + lucide-vue-next |
| 校验 | Zod（所有后端返回值都经过运行时校验） |
| 测试 | Go `testing` + Vitest / @vue/test-utils |

前端只通过 `src/platform` 适配器访问壳层：`wails.ts` 调 `window.go.main.App`，`mock.ts` 用 localStorage 提供内存项目。UI 不直接触碰 Wails binding。

## 目录结构

```
pixelbench/
  app.go / main.go             Wails 入口与绑定方法
  internal/project/            项目创建、打开、迁移、锁、原子保存
  internal/assets/             导入、缩略图、版本、精灵表、生成素材落盘、像素化版本、exports 导出
  internal/generators/         程序化像素生成（俯视建筑）
  internal/imageproc/          裁切/降采样、OKLab 量化、抖动、去杂点、描边、接缝
  internal/generate/           AI 供应商适配器、任务队列、密钥设置
  internal/export/             素材版本导出、Tiled TMX/TSX、通用地图 JSON 与 Godot 4 工程
  internal/update/             更新清单校验（Ed25519）、版本比较、带哈希校验的下载
  cmd/pbmanifest/              发布用：生成 Ed25519 密钥、给 latest.json 签名
  scripts/package.ps1          便携 zip 与（可选）NSIS 安装包
  scripts/make-icon.py         由界面 PixelMark 生成 icon.ico / appicon.png
  internal/secure/             Windows DPAPI（非 Windows 退化为 base64，仅供开发）
  internal/apperr/             统一错误码与中文文案
  frontend/
    src/domain/                与 Go model 对齐的类型和 Zod schema
    src/platform/              types.ts / wails.ts / mock.ts
    src/stores/app.ts          Pinia 应用状态与自动保存
    src/editor/                像素工具、图层/帧文档、精灵表几何与 JSON（pixels / document / sheet）
    src/maps/                  地图模型、自动拼接、深度/碰撞与 Canvas 2D 渲染（model / render / MapCanvas）
    src/screens/               总览、任务、素材、像素编辑器、生成台、地图编辑器、导出、设置
    src/components/            共享 UI（Modal、缩略图、保存指示等）
```

### 项目文件夹格式

```
MyGame/
  project.json
  assets/<assetId>/asset.json
  assets/<assetId>/source/generated.png     程序化生成的合成图（原样保留）
  assets/<assetId>/source/generated.base.png    base / roof 分层，供屋檐淡出
  assets/<assetId>/versions/NNNN/meta.json  版本元数据（生成器、种子、参数、锚点、碰撞）
  assets/<assetId>/frames/NNN.png           动画帧（由精灵表版本派生，可重建）
  quests/<questId>.json
  palettes/<paletteId>.json
  maps/<mapId>.map.json                   地图、图层、对象、碰撞与设置
  exports/<名称>/<版本>/                  从素材详情导出的引擎可用文件（可随时重建）
  exports/maps/<名称>/                    Tiled TMX/TSX、图集 PNG 与通用 JSON
  exports/maps/<名称>-godot/             Godot 4 工程（project.godot、.tres、.tscn、加载脚本）
  .workbench/                  可重建缓存、日志、自动保存、回收站
  .workbench/jobs/<jobId>.json AI 任务状态（重启后据此标记中断）
  .workbench/generated/<jobId>/   AI 生成结果图，采用后复制进 assets/
```

应用级 AI 设置（`ai.json`，含 DPAPI 加密的 API Key）保存在 `%APPDATA%\PixelBench\`，与项目目录分离。

`schemaVersion` 独立于应用版本，迁移在 `internal/project/migrations` 按版本链式执行。素材版本不可变，当前版本可回退。

## 开发

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench"; wails dev
```

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench\frontend"; npm run dev
```

只跑前端时使用 mock 适配器，不需要 Go / WebView2。

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench"; go test ./...
```

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench\frontend"; npm run typecheck
```

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench\frontend"; npm test
```

打包：

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench"; powershell -ExecutionPolicy Bypass -File scripts/package.ps1 -Version 1.0.0 -Nsis
```

产物落在 `build/bin/`：`pixelbench-<版本>-portable.zip` 与 `pixelbench-amd64-installer.exe`。`-SkipFrontend` 可跳过前端构建，只重打包。

`-Nsis` 需要 NSIS：`winget install --id NSIS.NSIS -e`。脚本会检查 `makensis`，找不到时自动尝试 `C:\Program Files (x86)\NSIS`。安装包语言以简体中文为默认、英文备选；公司 / 产品名与版权来自 `wails.json` 的 `info`，发版时同步 `productVersion`。`build/windows/installer/*.nsi|nsh` 按 ACP 编码读取，注释与字符串请保持 ASCII（中文会报 `Bad text encoding`）；中文界面文案由 NSIS 语言宏提供。

打包前先关闭正在运行的 `pixelbench.exe`，并让终端停留在项目根目录——Wails 的 `-clean` 需要删除 `build/bin`，被占用时会报 `Access is denied`。

发布更新清单（一次性生成密钥，之后每次发版签名）：

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench"; go run ./cmd/pbmanifest keygen -out release-keys
```

```powershell
cd "D:\Users\XiaoH\Documents\Code Files\pixelbench"; go run ./cmd/pbmanifest manifest -key release-keys/private.b64 -version 1.0.0 -channel stable -file build/bin/indie-studio-0.1.0-installer.exe -url https://example.com/indie-studio-installer.exe -out latest.json
```

把 `release-keys/public.b64` 用 `-ldflags "-X main.UpdatePublicKey=<base64>"` 嵌进构建；`latest.json` 与 `latest.json.sig` 一起上传。私钥不要进仓库。

浏览器预览里看不到真实更新检查（mock 固定返回「未配置更新源」），需要桌面端 + 可访问的清单地址。

## 设计原则

- 本地文件是唯一事实来源，索引与缩略图随时可重建
- 重计算在 Go（像素运算、量化、图集、导出），交互与预览在前端
- 大二进制走 HTTP AssetServer，不塞进 Wails 的 JSON 通道
- 所有写盘原子化：临时文件 + rename；项目级锁防止双开
- 默认无遥测、无账号、不上传项目内容；AI 调用只发送用户填写的提示词与尺寸
- AI Key 由 DPAPI 加密保存在应用配置目录，永不写入项目、日志或导出文件

## 许可

尚未定稿。v1 计划本地功能免费、AI 自带 Key、不代收费用。