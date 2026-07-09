# Animal Poke UI Working Plan

本 plan 面向“没有视觉能力的 GLM5.2”继续执行前端实现。不要让后续模型再依赖图片判断样式；所有关键视觉信息都已翻译成文本规则。

## 0. 输入资产与结论

源目录：`/Users/bluepoisonss/Downloads/export`

设计稿清单：

- `Board Title.jpg`：产品名 `Animal Poke / 动物捕捉`
- `Board Subtitle.jpg`：定位语 `现实世界探索 · VLM 动物识别 · 投掷捕获 · 图鉴战斗经济系统`
- `Mini Design System.jpg`：颜色、稀有度、物种 token、组件名
- `01 Discover ! VLM Scan.jpg`：发现 / VLM 实时识别页
- `02 LBS Hunt Map.jpg`：LBS 猎取地图页
- `03 Capture Mode.jpg`：捕获投掷页
- `04 Pokedex Collection.jpg`：图鉴收藏页
- `05 Battle Arena.jpg`：战斗竞技场页
- `06 Store + Check-in.jpg`：商店 + 签到页

移动端主设计稿尺寸均为 `1170 x 2532 px`，按 `@3x` 处理，前端逻辑画布为 `390 x 844 dp`。后续所有位置和尺寸默认使用这个逻辑画布。实现时采用响应式：宽度小于 `430px` 时按 `390px` 设计比例缩放；宽度大于 `430px` 时内容最大宽度限制为 `430px`，居中显示。

## 1. 产品体验目标

实现一个 mobile-first 的动物捕捉游戏 UI 原型，核心循环是：

1. 发现页用 VLM 扫描现实动物。
2. 地图页展示附近捕捉点与距离。
3. 捕获页通过投掷道具捕获动物。
4. 图鉴页查看已收集动物和未解锁槽位。
5. 战斗页使用已收集动物进行策略战斗。
6. 商店页通过签到和金币购买捕获道具。

第一阶段只实现前端 UI 和本地 mock 状态，不接真实相机、定位、支付或后端。遵循 KISS / YAGNI：先把视觉、布局、交互状态和数据结构做扎实，不预留复杂业务系统。

## 2. 全局设计规范

### 2.1 设计基准

- App 外层：全屏深色背景，内部为一个手机画布容器。
- 手机画布：`width: min(100vw, 430px)`，`min-height: 100dvh`，设计基准 `390 x 844`。
- 页面内边距：左右 `24px` 为主；少数卡片用 `26px` 起始。
- 页面顶部标题 Y 坐标通常 `36-44px`。
- 外框边线：每个主页面都有 2px 高饱和边框，圆角 `34px`，贴近手机屏幕边缘。
- 不做营销 landing page；打开即进入实际可用的游戏 UI。

### 2.2 色彩 token

从设计稿明确给出的基础色：

```css
:root {
  --ap-orange: #ff8c42;
  --ap-orange-deep: #e67300;
  --ap-cream: #fff8f0;
  --ap-brown: #4a2c1a;
  --ap-yellow: #ffd23f;
  --ap-gold: #ffb300;
}
```

需要补充的派生色：

```css
:root {
  --ap-black: #050606;
  --ap-panel: #111313;
  --ap-panel-soft: rgba(255, 248, 240, 0.08);
  --ap-border-muted: rgba(255, 248, 240, 0.22);
  --ap-text: #fff8f0;
  --ap-text-muted: rgba(255, 248, 240, 0.72);

  --ap-bg-discover-top: #10241a;
  --ap-bg-discover-bottom: #071009;
  --ap-bg-map-top: #315f2f;
  --ap-bg-map-bottom: #07100d;
  --ap-bg-capture-top: #4a210e;
  --ap-bg-capture-bottom: #050403;
  --ap-bg-pokedex-top: #26183f;
  --ap-bg-pokedex-bottom: #05050c;
  --ap-bg-battle-top: #2a1038;
  --ap-bg-battle-bottom: #05040b;
  --ap-bg-store-top: #4a2c1a;
  --ap-bg-store-bottom: #070403;

  --ap-rarity-common: #777777;
  --ap-rarity-uncommon: #67dd67;
  --ap-rarity-rare: #45b9ff;
  --ap-rarity-epic: #9b5cff;
  --ap-rarity-legendary: #ffd23f;

  --ap-hp-player: #67dd67;
  --ap-hp-enemy: #ff4a89;
  --ap-map-road-blue: rgba(76, 190, 255, 0.46);
  --ap-map-road-olive: rgba(170, 184, 116, 0.45);
  --ap-map-pin-blue: #45b9ff;
  --ap-map-pin-green: #82e36e;
}
```

### 2.3 字体与排版

- 字体族：优先使用系统无衬线，`font-family: Inter, "PingFang SC", "Microsoft YaHei", system-ui, sans-serif;`
- 标题使用粗字重：`font-weight: 900`。
- 正文和按钮使用 `font-weight: 800`。
- 不使用负 letter-spacing；保持 `letter-spacing: 0`。
- 中英混排标题保留空格，例如 `图鉴 POKEDEX`、`商店 STORE`。
- 文案必须完整显示，按钮和卡片内文字不可溢出；中文场景允许换行。

推荐字号：

- 页面大标题：`32px`，行高 `1.05`，字重 `900`。
- 副标题 / eyebrow：`15px`，行高 `1.2`，字重 `900`，可用橙色。
- 卡片主文案：`18-21px`，行高 `1.25`，字重 `900`。
- 次要说明：`14-16px`，行高 `1.35`，字重 `800`。
- 底部导航：`16px`，字重 `900`。

## 3. 全局组件拆分

优先建立这些可复用组件，避免重复实现：

1. `PhoneFrame`
   - 负责页面背景渐变、外框边线、最大宽度、圆角、最小高度。
   - props：`variant: "discover" | "map" | "capture" | "pokedex" | "battle" | "store"`。
   - 根据 `variant` 设置背景渐变和外框色。

2. `TopResourceBar`
   - 用在发现页顶部。
   - 左侧城市天气：`宁波 · 雨`。
   - 中间体力：闪电图标 + `84`。
   - 右侧金币：圆点图标 + `1260`。
   - 尺寸：`left: 18px; top: 18px; width: 354px; height: 54px; border-radius: 24px`。

3. `PageTitle`
   - 大标题统一组件。
   - 支持右侧状态文本，例如 `刷新 04:32`、`体力 -20`、`已收集 59`、`金币 1260`。

4. `ActionButton`
   - 主按钮：橙色填充、深橙投影或下边线。
   - 发现页按钮文案：`开始捕获`。
   - 尺寸：`width: 275px; height: 58px; border-radius: 28px`，居中。

5. `BottomTabBar`
   - 用在发现页，也可以扩展到主 App。
   - 标签：`发现`、`图鉴`、`战斗`、`商店`、`成就`。
   - 尺寸：`left: 18px; bottom: 20px; width: 354px; height: 54px; border-radius: 22px`。
   - 背景：近黑 `#080a09`，边框 `rgba(255,255,255,0.12)`。

6. `AnimalIcon`
   - 物种：`cat`、`goose`、`dog`、`unknown`。
   - 大图标用白色线性 SVG；小 token 可用 emoji。
   - 如果项目没有现成 SVG，先实现简化线性图标，不要引入大型图标包。

7. `RarityCard`
   - 图鉴卡片组件。
   - props：`animalId`、`species`、`rarity`、`region`、`location`、`locked`、`active`。
   - 通过稀有度控制边框色和背景。

8. `DiscoveryPin`
   - 地图目标点。
   - props：`rarity`、`label`、`distance`、`x`、`y`。
   - 点位是圆形，传奇点有双层黄圈。

9. `BattleLog`
   - 战斗日志面板。
   - 半透明深色背景，柔和边框，圆角 `22px`。

10. `StoreItemRow`
   - 商店道具行。
   - 三列：图标 `32px`、说明 flex、价格固定宽度 `64px` 右对齐。

## 4. 数据结构建议

只做前端 mock，后续接真实数据时再替换数据源。

```ts
type Rarity = "common" | "uncommon" | "rare" | "epic" | "legendary";
type Species = "cat" | "goose" | "dog";

interface AnimalEntry {
  id: string;              // "000058"
  species: Species;
  name: string;            // "鹅"
  rarity: Rarity;
  region?: string;         // "江北区"
  location?: string;       // "滨江"
  collected: boolean;
  trait?: string;          // "弹跳略强"
}

interface HuntTarget {
  id: string;
  rarity: Rarity;
  distanceMeters: number;
  x: number;               // 0-1 relative to map canvas width
  y: number;               // 0-1 relative to map canvas height
}

interface InventoryItem {
  id: string;
  icon: string;
  name: string;
  effect: string;
  price: number;
}
```

Mock 数据必须覆盖设计稿：

- 猫：`#000014 · 传说`，`海曙区 · 月湖`
- 狗：`#000057 · 稀有`，`江北区 · 滨江`
- 鹅：`#000058 · 少见`
- 未解锁槽位：显示 `???`
- 商店金币：`1260`
- 体力：发现页 `84`，捕获页展示消耗 `体力 -20`

## 5. 页面规格

### 5.1 Discover / VLM Scan

对应图片：`01 Discover ! VLM Scan.jpg`

目标：表现实时 VLM 动物识别，用户可点击 `开始捕获` 进入捕获页。

布局：

- `PhoneFrame variant="discover"`，外框色 `--ap-orange`。
- 顶部资源条：`x=18 y=18 w=354 h=54`。
- eyebrow：`DISCOVER MODE`，橙色，`x=24 y=105`。
- 主标题：`VLM 实时动物识别中`，白色，`x=24 y=136`，字号 `30-32px`。
- 装饰圆：右上棕色半透明圆，直径约 `210px`，中心约 `x=322 y=202`，允许超出右侧。
- 扫描框：`x=104 y=294 w=188 h=158`，圆角 `20px`，边框 `4px solid --ap-yellow`，背景透明。
- 横向扫描线：青色 `#87f3ff`，高度 `2px`，`x=88 y=376 w=222`。
- 扫描框中间放鹅图标，白色线性，宽高约 `90px`。
- 识别结果胶囊：`x=84 y=470 w=222 h=49`，黑色背景，金色细边框，圆角 `23px`。
- 识别结果文案：`鹅 · 置信度 94%`，左侧有鹅 token。
- 中下部有一条黑色水平遮罩/暗区，从 `y≈590` 开始到页面底部，让 CTA 更突出。
- 主按钮：`开始捕获`，`x=58 y=684 w=274 h=58`。
- 底部导航：`x=18 y=779 w=354 h=54`。

交互：

- 扫描线做上下轻微动画，周期 `1.8s`。
- 点击 `开始捕获`：路由到 capture 页面或切换状态。
- 识别中状态可用 mock：先显示 `VLM 实时动物识别中`，1 秒后保留当前识别结果。

验收：

- 390px 宽屏幕下资源条、按钮、底部导航均不贴边。
- 识别框和按钮水平居中。
- 文字不被装饰圆遮挡。

### 5.2 Hunt Map

对应图片：`02 LBS Hunt Map.jpg`

目标：抽象地图展示附近目标，不需要真实地图瓦片。

布局：

- `PhoneFrame variant="map"`，外框色 `--ap-rarity-uncommon`。
- 标题：`HUNT MAP`，`x=24 y=38`，字号 `32px`。
- 右上刷新：`刷新 04:32`，`x≈247 y=47`，字号 `17px`。
- 蓝色斜向道路：宽 `82px`，高 `690px`，圆角 `41px`，颜色 `--ap-map-road-blue`，旋转约 `18deg`，起点从左下角越界进入。
- 橄榄色横向道路：宽 `344px`，高 `44px`，圆角 `22px`，颜色 `--ap-map-road-olive`，旋转约 `-11deg`，`y≈423`。
- 目标点 1：蓝色稀有点，位置 `x=92 y=309`，圆直径 `34px`，标签 `稀有 · 230m` 放在点下方。
- 目标点 2：黄色传奇点，位置 `x=305 y=257`，外圈直径 `58px`，内圈直径 `45px`，标签 `传说 · 480m`。
- 用户位置：亮蓝点，位置 `x=196 y=421`，直径 `43px`，标签 `你的位置` 放在下方。
- 绿色点：位置 `x=280 y=594`，直径 `34px`，无标签。
- 底部信息卡：`x=24 y=656 w=342 h=139`，圆角 `24px`，背景 `rgba(5,6,6,0.88)`，边框 `rgba(255,255,255,0.15)`。
- 信息卡标题：`鹅  发现点 · 50m 内可捕获`，字号 `22px`。
- 信息卡说明：`500m 范围内 7 个目标，诱饵会提升稀有出现率。`

交互：

- 点击目标点选中后，底部卡片更新目标物种、距离、稀有度。
- 刷新倒计时每秒递减；到 `00:00` 后重置为 mock 的 `04:32`。
- 不接真实 GPS；地图状态用 `HuntTarget[]` mock。

验收：

- 道路可以越界，但不能盖住标题。
- 目标点标签与点位距离保持 `10-14px`。
- 底部卡片不遮挡外框，底部留 `48px` 左右安全空间。

### 5.3 Capture Mode

对应图片：`03 Capture Mode.jpg`

目标：表现投掷捕获动作和捕获概率。

布局：

- `PhoneFrame variant="capture"`，外框色 `--ap-yellow`。
- 标题：`CAPTURE MODE`，`x=24 y=38`。
- 右上消耗提示：`体力 -20`，黄色，`x≈252 y=48`。
- 目标动物：鹅图标，白色线性，宽高约 `110px`，中心 `x=195 y=305`。
- 投掷轨迹线：黄色直线，高度 `4px`，长度约 `262px`，从 `x=85 y=492` 到 `x=342 y=424`，旋转约 `-15deg`。
- 道具图标：白色面包 / 玩具球样式，`x≈89 y=535 w=54 h=54`。
- 底部概率卡：`x=34 y=616 w=322 h=120`，圆角 `22px`，背景 `rgba(255,248,240,0.12)`，边框 `rgba(255,210,63,0.45)`。
- 卡片标题：`鹅 · 面包屑球 · 弹跳略强`。
- 概率条：`x=54 y=678 w=282 h=16`，圆角 `8px`。
- 概率条分段：左棕橙 `33%`，中金色 `34%`，右绿 `7%`，剩余深色 `26%`。
- 底部文案：`捕获成功率 78% · 最佳力度 35-75`。

交互：

- 支持一个可拖动/点击的投掷力度值，范围 `0-100`。
- 默认力度 `55`，位于最佳区间 `35-75`，命中时显示成功率 `78%`。
- 点击或松手后可触发 mock 捕获结果：
  - 力度在 `35-75`：成功概率高，展示成功 toast 或进入图鉴。
  - 力度不在区间：展示失败提示，但不扣真实资源。

验收：

- 轨迹线不要穿过概率卡。
- 卡片内三行文字都在可读区域内。
- 概率条分段总宽度固定，不因数值文本改变布局。

### 5.4 Pokedex Collection

对应图片：`04 Pokedex Collection.jpg`

目标：展示已收集动物、按物种过滤、显示锁定槽位。

布局：

- `PhoneFrame variant="pokedex"`，外框色 `--ap-rarity-epic`。
- 标题：`图鉴 POKEDEX`，`x=24 y=43`。
- 右上计数：`已收集 59`，淡紫色，`x≈267 y=50`。
- 过滤 tabs：`全部`、`猫`、`鹅`、`狗`，`x=28 y=91`，间距 `18px`，字号 `18px`，字重 `900`。
- 卡片网格：两列，左右内边距 `26px`，列宽 `156px`，列间距 `30px`，行间距 `32px`。
- 卡片尺寸：`w=156 h=205`，圆角 `22px`，边框 `4px`。

卡片状态：

- 传奇猫卡：
  - 背景 `--ap-cream`
  - 边框 `--ap-rarity-legendary`
  - 图标深棕
  - 文案 `#000014 · 传说`
  - 副文案 `海曙区 · 月湖`
- 稀有狗卡：
  - 背景深蓝 `#132638`
  - 边框 `--ap-rarity-rare`
  - 图标白色
  - 文案 `#000057 · 稀有`
  - 副文案 `江北区 · 滨江`
- 少见鹅卡：
  - 背景深绿 `#102a18`
  - 边框 `--ap-rarity-uncommon`
  - 图标白色
  - 文案 `#000058 · 少见`
- 锁定卡：
  - 背景近黑
  - 边框 `--ap-rarity-common`
  - 中央 `???`，灰色，字号 `30px`

交互：

- 点击 tab 过滤列表。
- 点击已收集卡片进入详情 modal 或详情页；第一阶段可只做选中态。
- 未解锁卡片不可点击，或点击后提示 `尚未发现`。

验收：

- 两列卡片必须等宽；不能因文案长度改变卡片高度。
- 卡片内物种图标垂直位置一致，主文案在下半区。
- 过滤 tab 一行展示，不换行。

### 5.5 Battle Arena

对应图片：`05 Battle Arena.jpg`

目标：展示动物对战、血条、属性克制、日志与策略按钮。

布局：

- `PhoneFrame variant="battle"`，外框色 `--ap-hp-enemy`。
- 标题：`BATTLE ARENA`，`x=24 y=38`。
- VS 区域位于页面中上部：
  - 左侧猫图标中心约 `x=109 y=257`，宽高约 `110px`。
  - 中间 `VS`，粉色 `--ap-hp-enemy`，字号 `42px`，字重 `900`。
  - 右侧狗图标中心约 `x=285 y=257`，宽高约 `110px`。
- 血条：
  - 左血条 `x=34 y=333 w=134 h=14`，绿色。
  - 右血条 `x=226 y=333 w=134 h=14`，粉色。
  - 圆角 `7px`。
- 属性提示：`火 > 草 · 光克暗`，黄色，`x=88 y=386`。
- 战斗日志：`x=28 y=475 w=334 h=153`，圆角 `22px`，背景 `rgba(255,248,240,0.10)`，边框 `rgba(255,248,240,0.22)`。
- 日志文案：
  - `战斗日志`
  - `猫 使用 激进策略，暴击 x2`
  - `获得金币 +70 · 掉落诱饵`
- 策略按钮行：`激进`、`平衡`、`防守`，位于 `y≈696`，横向等距。

交互：

- 三个策略按钮更新日志和血条 mock：
  - `激进`：玩家伤害高，自身也损血。
  - `平衡`：中等伤害。
  - `防守`：伤害低，减少受到伤害。
- 按钮用文字即可，但需要明确 hover/pressed/active 状态：
  - active：文字黄色或下划线。
  - pressed：轻微缩放 `transform: scale(0.98)`。

验收：

- VS 文本必须在两个图标之间，不遮挡图标。
- 日志卡可容纳三行中文，不截断。
- 血条宽度根据百分比变化时外容器尺寸不变。

### 5.6 Store + Check-in

对应图片：`06 Store + Check-in.jpg`

目标：展示金币余额、7 日签到奖励、道具购买列表。

布局：

- `PhoneFrame variant="store"`，外框色 `--ap-gold`。
- 标题：`商店 STORE`，`x=24 y=43`。
- 右上余额：`金币 1260`，黄色，`x≈254 y=51`。
- 签到卡：`x=26 y=93 w=338 h=137`，圆角 `22px`，背景 `rgba(255,248,240,0.10)`，边框 `rgba(255,179,0,0.55)`。
- 签到标题：`7 日签到轨道`。
- 奖励数字行：`20 30 40 50 60 80 150`，黄色，字号 `19px`，等距。
- 说明：`第 7 天额外送：玩具球 🎾`。
- 分区标题：`道具背包`，`x=28 y=278`，字号 `28px`。
- 道具列表从 `y≈328` 开始，每行高度约 `52px`，行间距 `16px`。

道具行文案：

- `🎾 玩具球  捕获 +15%`，价格 `50`
- `⚾ 高级玩具球  捕获 +25%`，价格 `120`
- `🧀 诱饵  30 分钟稀有提升`，价格 `100`
- `🧪 体力药剂  体力 +3`，价格 `150`

交互：

- 点击道具行弹出确认购买状态；第一阶段可用 toast，不需要真实扣费。
- 签到卡可点击，mock 领取当天奖励后高亮当天数字。
- 金币不足状态：价格变灰，道具行禁用。

验收：

- 价格列右对齐，纵向形成一条直线。
- 道具说明不能挤压价格列；小屏幕下说明可以换行，但价格列固定。
- 签到数字必须一行显示。

## 6. 路由与状态流

推荐路由：

- `/animal-poke/discover`
- `/animal-poke/map`
- `/animal-poke/capture/:targetId`
- `/animal-poke/pokedex`
- `/animal-poke/battle`
- `/animal-poke/store`

如果当前项目没有路由系统，就用本地 `activeScreen` 状态切换，不额外引入路由库。

状态流：

1. 初始进入 `discover`。
2. `开始捕获` 进入 `capture`，默认目标为鹅 `#000058`。
3. 底部导航可进入 `pokedex`、`battle`、`store`。
4. 地图页可从发现页或调试入口进入。
5. 捕获成功后，把鹅标记为已收集，再跳转 `pokedex`。

## 7. 实现步骤

后续 GLM5.2 按以下顺序执行：

1. 先定位前端项目入口
   - 查找 `package.json`、`src/`、`app/`、`pages/`、`routes/`。
   - 不改变现有技术栈；如果已有 React/Vue/Svelte，就沿用。
   - 如果只是原型项目，优先 React + TypeScript + CSS Modules 或现有 CSS 方案。

2. 创建 feature 目录
   - 推荐路径：`src/features/animal-poke/`
   - 子目录：`components/`、`screens/`、`data/`、`styles/`
   - 不把所有页面塞进一个巨大文件。

3. 建立设计 token
   - 新增 `animalPoke.css` 或接入现有 token 文件。
   - 先写颜色、字号、半径、阴影、页面渐变。
   - 所有组件引用 token，不在组件里散落硬编码颜色。

4. 实现基础组件
   - `PhoneFrame`
   - `PageTitle`
   - `AnimalIcon`
   - `ActionButton`
   - `BottomTabBar`
   - `RarityCard`
   - `DiscoveryPin`
   - `BattleLog`
   - `StoreItemRow`

5. 实现 mock data
   - `animals.ts`
   - `huntTargets.ts`
   - `inventory.ts`
   - 保证字段覆盖本 plan 第 4 节。

6. 实现 6 个 screen
   - `DiscoverScreen`
   - `HuntMapScreen`
   - `CaptureScreen`
   - `PokedexScreen`
   - `BattleArenaScreen`
   - `StoreScreen`

7. 接入导航
   - 如果项目已有路由：加 route。
   - 如果项目无路由：做一个 `AnimalPokeApp`，内部维护 `activeScreen`。

8. 做响应式与可访问性
   - 主画布最大宽度 `430px`。
   - 所有可点击元素最小触控高度 `44px`。
   - 按钮加 `aria-label` 或明确文本。
   - 动态数字更新不能造成布局跳动。

9. 验证
   - 运行 lint / typecheck / test / build，按项目实际命令执行。
   - 用浏览器查看 390x844、375x812、430x932 三个尺寸。
   - 检查文字不溢出、组件不重叠、外框不被裁切。

## 8. 组件实现原则

- KISS：第一阶段只做 UI + mock 交互，不接相机、GPS、后端、支付。
- YAGNI：不要实现复杂背包系统、真实地图瓦片、真实战斗算法。
- DRY：页面渐变、外框、标题、卡片、按钮、图标都抽成复用组件。
- SOLID：
  - `PhoneFrame` 只负责容器样式。
  - `AnimalIcon` 只负责渲染物种图形。
  - `RarityCard` 只负责单张图鉴卡。
  - 页面组件负责组合和状态，不直接复制底层样式。

## 9. 视觉验收清单

逐项确认：

- 页面整体是深色游戏化 UI，不是普通白底管理后台。
- 所有主页面都有圆角外框和高饱和边线。
- `discover` 页有顶部资源条、扫描框、识别结果、橙色 CTA、底部 Tab。
- `map` 页是抽象道路 + 圆形点位 + 底部发现点说明卡。
- `capture` 页有鹅图标、黄色投掷轨迹、道具图标、概率条。
- `pokedex` 页是两列稀有度卡片，含锁定 `???` 卡。
- `battle` 页有猫 VS 狗、双血条、克制提示、日志、三策略按钮。
- `store` 页有签到卡和 4 个道具行，价格列对齐。
- 中文文案和英文标题与本 plan 完全一致，除非产品另有新文案。
- 390px 宽视口下没有横向滚动。

## 10. 不要做的事

- 不要提交 git、创建分支或发 PR，除非用户明确要求。
- 不要删除或覆盖 `/Users/bluepoisonss/Downloads/export` 原始图片。
- 不要把这些 JPG 直接当页面背景贴上去；需要实现为真实前端组件。
- 不要引入大型 UI 框架只为几个组件。
- 不要把真实定位、相机、支付、战斗算法作为第一阶段必做项。
- 不要用“看起来差不多”作为验收标准；按本 plan 的文字规格实现。

## 11. 最小交付定义

一次合格的前端交付至少包含：

1. 可运行的 `AnimalPoke` UI 入口。
2. 6 个页面均可访问。
3. mock 数据驱动页面内容。
4. 通用组件已拆分，避免重复写同类卡片和按钮。
5. 响应式适配 `375-430px` 手机宽度。
6. 至少通过项目现有的 build/typecheck/lint 中可用项。

