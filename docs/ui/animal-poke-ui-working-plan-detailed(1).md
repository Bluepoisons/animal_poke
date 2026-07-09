# Animal Poke UI Detailed Working Plan for GLM5.2

本文档是给“没有视觉能力的 GLM5.2 / 后续前端模型 / 工程师”继续实现用的执行规格。它不要求读图，不要求凭审美推断；只需要按这里的组件、尺寸、状态、文案、验收规则实现。

关联参考 HTML：

- `outputs/animal-poke-ui-reference.html`

源设计资产：

- `/Users/bluepoisonss/Downloads/export/Board Title.jpg`
- `/Users/bluepoisonss/Downloads/export/Board Subtitle.jpg`
- `/Users/bluepoisonss/Downloads/export/Mini Design System.jpg`
- `/Users/bluepoisonss/Downloads/export/01 Discover ! VLM Scan.jpg`
- `/Users/bluepoisonss/Downloads/export/02 LBS Hunt Map.jpg`
- `/Users/bluepoisonss/Downloads/export/03 Capture Mode.jpg`
- `/Users/bluepoisonss/Downloads/export/04 Pokedex Collection.jpg`
- `/Users/bluepoisonss/Downloads/export/05 Battle Arena.jpg`
- `/Users/bluepoisonss/Downloads/export/06 Store + Check-in.jpg`

## 1. 最终目标

把 `export` 文件夹中的 Animal Poke UI 重新实现为前端组件，而不是把 JPG 贴成背景图。

第一阶段交付范围：

1. 一个可进入的 Animal Poke UI 页面或模块。
2. 6 个主屏：Discover、Hunt Map、Capture、Pokedex、Battle、Store。
3. 所有内容由 mock data 驱动。
4. 所有关键视觉元素用真实 HTML/CSS/SVG 实现。
5. 手机端优先，基准尺寸 `390 x 844 CSS px`。
6. 不接真实相机、真实定位、后端、支付或复杂游戏算法。

不要做：

- 不要把设计稿 JPG 当页面背景。
- 不要引入大型 UI 框架重做一套设计系统。
- 不要在用户未要求时创建 git 分支、commit、push、PR。
- 不要删除或移动 `/Users/bluepoisonss/Downloads/export`。
- 不要做真实 GPS / VLM / 支付 / 后端接口。

## 2. 设计稿换算规则

所有主页面 JPG 尺寸为 `1170 x 2532 px`，按 3x 设计稿处理。

换算公式：

```txt
CSS_X = JPG_X / 3
CSS_Y = JPG_Y / 3
CSS_W = JPG_W / 3
CSS_H = JPG_H / 3
```

所以：

- 设计稿逻辑宽度：`390px`
- 设计稿逻辑高度：`844px`
- 页面容器最大宽度：`430px`
- 默认实现宽度：`390px`

响应式规则：

- `viewport width <= 430px`：页面宽度为 `100vw`，内容按 390 基准做比例压缩或使用相对宽度。
- `viewport width > 430px`：页面宽度固定或最大为 `430px`，居中。
- 页面高度至少 `100dvh`；若设备高度小于 844px，允许纵向滚动，但核心 CTA 与底部导航不能重叠。

## 3. 信息架构

Animal Poke 是现实世界动物捕捉小游戏。页面之间的逻辑关系：

```txt
Discover
  -> Capture
  -> Pokedex

Discover
  -> Hunt Map
  -> Capture

Bottom Tabs
  -> Discover
  -> Pokedex
  -> Battle
  -> Store
  -> Achievements placeholder
```

首屏建议：Discover。

如果项目已有路由：

- `/animal-poke/discover`
- `/animal-poke/map`
- `/animal-poke/capture/:targetId`
- `/animal-poke/pokedex`
- `/animal-poke/battle`
- `/animal-poke/store`

如果项目没有路由：

- 做一个 `AnimalPokeApp` 组件。
- 内部用 `activeScreen` state 切换。
- 后续再接正式路由。

## 4. 文件组织建议

React 项目建议：

```txt
src/features/animal-poke/
  AnimalPokeApp.tsx
  animalPoke.css
  data/
    animals.ts
    huntTargets.ts
    inventory.ts
  components/
    PhoneFrame.tsx
    PageTitle.tsx
    TopResourceBar.tsx
    BottomTabBar.tsx
    ActionButton.tsx
    AnimalIcon.tsx
    RarityCard.tsx
    DiscoveryPin.tsx
    BattleLog.tsx
    StoreItemRow.tsx
    CaptureProbabilityBar.tsx
  screens/
    DiscoverScreen.tsx
    HuntMapScreen.tsx
    CaptureScreen.tsx
    PokedexScreen.tsx
    BattleArenaScreen.tsx
    StoreScreen.tsx
```

Vue 项目建议：

```txt
src/features/animal-poke/
  AnimalPokeApp.vue
  animalPoke.css
  data/
  components/
  screens/
```

普通 HTML/原型项目：

```txt
animal-poke-ui-reference.html
```

实现原则：

- 页面组件只负责组合、状态和事件。
- 视觉样式集中在 CSS token 和组件 class。
- 动物、道具、地图目标全部来自 mock data。
- 不要在每个页面重复写外框、标题、卡片、按钮样式。

## 5. 全局 Design Tokens

### 5.1 基础颜色

设计稿明确给出的颜色：

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

派生颜色：

```css
:root {
  --ap-black: #050606;
  --ap-off-black: #080a09;
  --ap-panel: #111313;
  --ap-panel-soft: rgba(255, 248, 240, 0.09);
  --ap-panel-medium: rgba(255, 248, 240, 0.13);
  --ap-border-muted: rgba(255, 248, 240, 0.22);
  --ap-text: #fff8f0;
  --ap-text-muted: rgba(255, 248, 240, 0.72);
  --ap-text-dim: rgba(255, 248, 240, 0.48);

  --ap-cyan: #88f4ff;
  --ap-blue: #45b9ff;
  --ap-green: #67dd67;
  --ap-lime: #82e36e;
  --ap-pink: #ff4a89;
  --ap-purple: #9b5cff;

  --ap-common: #777777;
  --ap-uncommon: #67dd67;
  --ap-rare: #45b9ff;
  --ap-epic: #9b5cff;
  --ap-legendary: #ffd23f;
}
```

### 5.2 页面背景

每个页面都有深色渐变，不同页面用不同主色：

```css
:root {
  --ap-bg-discover: linear-gradient(180deg, #10241a 0%, #132b1d 44%, #071009 100%);
  --ap-bg-map: linear-gradient(180deg, #315f2f 0%, #112a20 48%, #07100d 100%);
  --ap-bg-capture: linear-gradient(180deg, #4a210e 0%, #160d08 56%, #050403 100%);
  --ap-bg-pokedex: linear-gradient(180deg, #26183f 0%, #100b1c 56%, #05050c 100%);
  --ap-bg-battle: linear-gradient(180deg, #2a1038 0%, #12081b 56%, #05040b 100%);
  --ap-bg-store: linear-gradient(180deg, #4a2c1a 0%, #190d05 56%, #070403 100%);
}
```

页面外框色：

```txt
discover: #ff8c42
map:      #67dd67
capture:  #ffd23f
pokedex:  #9b5cff
battle:   #ff4a89
store:    #ffb300
```

### 5.3 尺寸 token

```css
:root {
  --ap-frame-width: 390px;
  --ap-frame-max-width: 430px;
  --ap-frame-min-height: 844px;
  --ap-frame-radius: 34px;
  --ap-frame-border: 2px;

  --ap-page-pad-x: 24px;
  --ap-title-top: 38px;
  --ap-radius-sm: 10px;
  --ap-radius-md: 18px;
  --ap-radius-lg: 24px;
  --ap-radius-xl: 30px;
  --ap-touch: 44px;
}
```

### 5.4 Typography

```css
.ap-root {
  font-family: Inter, "PingFang SC", "Microsoft YaHei", system-ui, sans-serif;
  color: var(--ap-text);
  letter-spacing: 0;
}

.ap-title-xl {
  font-size: 32px;
  line-height: 1.05;
  font-weight: 900;
}

.ap-title-lg {
  font-size: 28px;
  line-height: 1.12;
  font-weight: 900;
}

.ap-label {
  font-size: 15px;
  line-height: 1.2;
  font-weight: 900;
}

.ap-body-strong {
  font-size: 20px;
  line-height: 1.25;
  font-weight: 900;
}

.ap-body {
  font-size: 16px;
  line-height: 1.35;
  font-weight: 800;
}
```

字号不要使用 `vw` 动态缩放。小屏幕只允许容器宽度缩放或文本换行。

## 6. Mock Data

### 6.1 TypeScript 类型

```ts
export type ScreenId =
  | "discover"
  | "map"
  | "capture"
  | "pokedex"
  | "battle"
  | "store";

export type Rarity =
  | "common"
  | "uncommon"
  | "rare"
  | "epic"
  | "legendary";

export type Species = "cat" | "goose" | "dog";

export interface AnimalEntry {
  id: string;
  species: Species;
  name: string;
  rarity: Rarity;
  collected: boolean;
  region?: string;
  location?: string;
  trait?: string;
  captureRate?: number;
}

export interface HuntTarget {
  id: string;
  species: Species;
  rarity: Rarity;
  distanceMeters: number;
  label: string;
  x: number;
  y: number;
}

export interface InventoryItem {
  id: string;
  icon: string;
  name: string;
  effect: string;
  price: number;
  disabled?: boolean;
}

export interface BattleState {
  playerSpecies: Species;
  enemySpecies: Species;
  playerHp: number;
  enemyHp: number;
  logLines: string[];
  activeStrategy: "aggressive" | "balanced" | "defensive";
}
```

### 6.2 Animals

```ts
export const animals: AnimalEntry[] = [
  {
    id: "000014",
    species: "cat",
    name: "猫",
    rarity: "legendary",
    collected: true,
    region: "海曙区",
    location: "月湖",
    trait: "标准抛物线",
    captureRate: 0.42,
  },
  {
    id: "000057",
    species: "dog",
    name: "狗",
    rarity: "rare",
    collected: true,
    region: "江北区",
    location: "滨江",
    trait: "下落更快",
    captureRate: 0.56,
  },
  {
    id: "000058",
    species: "goose",
    name: "鹅",
    rarity: "uncommon",
    collected: true,
    trait: "弹跳略强",
    captureRate: 0.78,
  },
  {
    id: "locked-001",
    species: "goose",
    name: "未知",
    rarity: "common",
    collected: false,
  },
];
```

### 6.3 Hunt Targets

```ts
export const huntTargets: HuntTarget[] = [
  {
    id: "target-rare-230",
    species: "dog",
    rarity: "rare",
    distanceMeters: 230,
    label: "稀有 · 230m",
    x: 0.24,
    y: 0.37,
  },
  {
    id: "target-legendary-480",
    species: "cat",
    rarity: "legendary",
    distanceMeters: 480,
    label: "传说 · 480m",
    x: 0.78,
    y: 0.30,
  },
  {
    id: "target-uncommon-50",
    species: "goose",
    rarity: "uncommon",
    distanceMeters: 50,
    label: "发现点 · 50m 内可捕获",
    x: 0.72,
    y: 0.70,
  },
];
```

### 6.4 Inventory

```ts
export const inventoryItems: InventoryItem[] = [
  {
    id: "toy-ball",
    icon: "🎾",
    name: "玩具球",
    effect: "捕获 +15%",
    price: 50,
  },
  {
    id: "advanced-ball",
    icon: "⚾",
    name: "高级玩具球",
    effect: "捕获 +25%",
    price: 120,
  },
  {
    id: "bait",
    icon: "🧀",
    name: "诱饵",
    effect: "30 分钟稀有提升",
    price: 100,
  },
  {
    id: "energy-potion",
    icon: "🧪",
    name: "体力药剂",
    effect: "体力 +3",
    price: 150,
  },
];
```

## 7. 组件规格

### 7.1 PhoneFrame

职责：

- 提供手机画布、页面背景、圆角外框、溢出裁切。
- 不负责页面内容。

Props：

```ts
interface PhoneFrameProps {
  variant: "discover" | "map" | "capture" | "pokedex" | "battle" | "store";
  children: React.ReactNode;
}
```

DOM：

```tsx
<section className={`ap-phone ap-phone--${variant}`}>
  {children}
</section>
```

CSS 要点：

```css
.ap-phone {
  position: relative;
  width: min(100vw, var(--ap-frame-width));
  min-height: var(--ap-frame-min-height);
  overflow: hidden;
  border: 2px solid var(--ap-frame-color);
  border-radius: var(--ap-frame-radius);
  background: var(--ap-frame-bg);
}
```

状态：

- 无内部状态。

验收：

- 所有页面外框圆角一致。
- 页面内容超出外框时被裁切。
- 宽屏下画布居中。

### 7.2 PageTitle

职责：

- 渲染页面左侧标题与可选右侧状态。

Props：

```ts
interface PageTitleProps {
  title: string;
  rightText?: string;
  rightTone?: "cream" | "yellow" | "purple";
  top?: number;
}
```

布局：

- `position: absolute`
- 左侧：`left: 24px; top: 38px`
- 右侧：`right: 58px` 或 `right: 52px`，根据页面视觉平衡微调。

标题文案：

- Hunt Map：`HUNT MAP`
- Capture：`CAPTURE MODE`
- Pokedex：`图鉴 POKEDEX`
- Battle：`BATTLE ARENA`
- Store：`商店 STORE`

验收：

- 标题不能换行。
- 右侧状态不能盖住标题；窄屏可略微减小右侧字号，但不能小于 `15px`。

### 7.3 TopResourceBar

只用于 Discover。

文案：

- 左：`宁波 · 雨`
- 中：`⚡ 84`
- 右：金币圆点 + `1260`

尺寸：

```txt
left: 18px
top: 18px
width: 354px
height: 54px
border-radius: 24px
```

颜色：

- 背景：`rgba(255, 248, 240, 0.10)`
- 边框：`rgba(255, 248, 240, 0.22)`
- 数字黄色：`#ffd23f` 或 `#ffb300`

布局：

- 用 `display: grid`
- 三列：`1.4fr 1fr 1fr`
- 左侧对齐，另外两列居中。

验收：

- 三段文本在一个胶囊内。
- 不要拆成三个独立卡片。

### 7.4 AnimalIcon

职责：

- 渲染猫、鹅、狗、未知四种图标。
- 使用 SVG 或现有项目图标，不要用 JPG。

Props：

```ts
interface AnimalIconProps {
  species: "cat" | "goose" | "dog" | "unknown";
  size?: number;
  tone?: "light" | "dark" | "muted";
}
```

颜色：

- `light`: `#fff8f0`
- `dark`: `#4a2c1a`
- `muted`: `#777777`

要求：

- 图标必须是线性 / 实心结合的简化风格。
- 图标不可带复杂阴影。
- 同一尺寸下视觉重量接近。

### 7.5 ActionButton

文案：

- Discover 主按钮：`开始捕获`

尺寸：

```txt
width: 274px
height: 58px
border-radius: 29px
```

颜色：

- 背景：`#ff8c42`
- 下边阴影 / 边：`#e67300`
- 文本：`#fff8f0`

交互：

- hover：亮度 +4%。
- active：下移 1px，阴影减弱。
- disabled：透明度 0.45，不响应点击。

### 7.6 BottomTabBar

标签：

```txt
发现 图鉴 战斗 商店 成就
```

尺寸：

```txt
left: 18px
bottom: 20px
width: 354px
height: 54px
border-radius: 22px
```

布局：

- `display: grid`
- `grid-template-columns: repeat(5, 1fr)`
- 文本居中。

状态：

- active：白色。
- inactive：`rgba(255,248,240,0.72)`。
- 按钮没有单独卡片背景。

### 7.7 RarityCard

职责：

- 图鉴页单卡片。

Props：

```ts
interface RarityCardProps {
  entry: AnimalEntry;
  selected?: boolean;
  onClick?: () => void;
}
```

尺寸：

```txt
width: 156px
height: 205px
border-radius: 22px
border-width: 4px
padding: 22px 20px
```

颜色映射：

```txt
legendary: border #ffd23f, background #fff8f0, text #4a2c1a, icon #4a2c1a
rare:      border #45b9ff, background #132638, text #fff8f0, subtext #bcecff
uncommon:  border #67dd67, background #102a18, text #fff8f0
common:    border #777777, background #090a0a, text #777777
```

内部布局：

- 图标区域：上半部分，高约 `95px`，居中。
- 主文案：`#000058 · 少见`
- 副文案：`江北区 · 滨江`
- 未解锁：居中显示 `???`。

验收：

- 4 张卡片在两列网格中对齐。
- 文案位置稳定，不因无副文案导致主文案乱跳。

### 7.8 DiscoveryPin

职责：

- 地图页目标点。

Props：

```ts
interface DiscoveryPinProps {
  target: HuntTarget;
  selected?: boolean;
  onSelect: () => void;
}
```

圆点规则：

- rare：蓝色圆 `34px`，外层浅蓝描边 `5px`。
- legendary：黄色圆 `58px` 外层 + `45px` 内层。
- uncommon：绿色圆 `34px`，外层浅绿描边 `5px`。
- user：亮蓝圆 `43px`。

标签：

- 字号 `16px`
- 字重 `900`
- 白色
- 与圆点间距 `10-14px`

### 7.9 CaptureProbabilityBar

职责：

- 渲染捕获力度 / 成功区间条。

结构：

```tsx
<div className="ap-probability">
  <span style={{ width: "33%" }} />
  <span style={{ width: "34%" }} />
  <span style={{ width: "7%" }} />
  <span style={{ width: "26%" }} />
</div>
```

颜色：

```txt
segment-1: #bf6a33
segment-2: #d8ad52
segment-3: #67dd67
segment-4: rgba(0,0,0,0.35)
```

验收：

- 总宽固定。
- 分段圆角只在最左和最右显示。
- 不能因为百分比文字导致条形变宽。

### 7.10 BattleLog

职责：

- 显示战斗日志，不管理战斗逻辑。

尺寸：

```txt
left: 28px
top: 475px
width: 334px
height: 153px
border-radius: 22px
```

文案：

```txt
战斗日志
猫 使用 激进策略，暴击 x2
获得金币 +70 · 掉落诱饵
```

### 7.11 StoreItemRow

职责：

- 商店道具行。

布局：

```txt
grid-template-columns: 36px 1fr 64px
column-gap: 12px
min-height: 52px
```

规则：

- 图标列固定。
- 文案列允许换行。
- 价格列固定右对齐。
- 禁用状态降低透明度，价格变灰。

## 8. 页面详细规格

### 8.1 Discover Screen

页面用途：VLM 实时动物识别与进入捕获。

背景：

- `PhoneFrame variant="discover"`
- 背景 `--ap-bg-discover`
- 外框 `#ff8c42`

绝对定位参考：

```txt
TopResourceBar       x=18  y=18   w=354 h=54
Eyebrow              x=24  y=105  w=180 h=20
MainTitle            x=24  y=136  w=330 h=40
AmbientCircle        x=217 y=96   w=220 h=220
ScanBox              x=104 y=294  w=188 h=158
ScanLine             x=88  y=376  w=222 h=2
GooseIcon            center x=195 y=374 size=96
ResultPill           x=84  y=470  w=222 h=49
DarkLowerBand        x=0   y=590  w=390 h=254
PrimaryButton        x=58  y=684  w=274 h=58
BottomTabBar         x=18  y=779  w=354 h=54
```

DOM 建议：

```tsx
<PhoneFrame variant="discover">
  <TopResourceBar city="宁波" weather="雨" energy={84} coins={1260} />
  <div className="ap-discover__eyebrow">DISCOVER MODE</div>
  <h1>VLM 实时动物识别中</h1>
  <div className="ap-discover__ambient" />
  <div className="ap-scan-box">
    <AnimalIcon species="goose" size={96} />
    <div className="ap-scan-line" />
  </div>
  <div className="ap-result-pill">鹅 · 置信度 94%</div>
  <ActionButton>开始捕获</ActionButton>
  <BottomTabBar active="discover" />
</PhoneFrame>
```

动画：

- 扫描线做上下移动，范围 `-42px` 到 `42px`。
- 周期 `1.8s`。
- 使用 `transform`，不要改 `top`，减少重排。

状态：

```ts
type ScanState = "scanning" | "recognized" | "failed";
```

状态文案：

```txt
scanning:   VLM 实时动物识别中
recognized: VLM 实时动物识别中 + 结果 pill
failed:     未识别到动物
```

第一阶段默认 `recognized`，显示鹅。

点击行为：

- 点击 `开始捕获`：进入 Capture，并带上 targetId `000058`。
- 底部 `图鉴`：进入 Pokedex。
- 底部 `战斗`：进入 Battle。
- 底部 `商店`：进入 Store。
- `成就` 第一阶段可 toast `暂未开放`。

### 8.2 Hunt Map Screen

页面用途：LBS 附近捕获点探索。

背景：

- `PhoneFrame variant="map"`
- 背景 `--ap-bg-map`
- 外框 `#67dd67`

绝对定位参考：

```txt
Title                x=24  y=38   text=HUNT MAP
RefreshText          x=247 y=47   text=刷新 04:32
BlueRoad             x=50  y=150  w=82  h=690 rotate=18deg
OliveRoad            x=28  y=430  w=344 h=44  rotate=-11deg
RarePin              x=92  y=309  diameter=34 label=稀有 · 230m
LegendaryPin         x=305 y=257  diameter=58 label=传说 · 480m
UserPin              x=196 y=421  diameter=43 label=你的位置
GreenPin             x=280 y=594  diameter=34
InfoCard             x=24  y=656  w=342 h=139
```

道路实现：

```css
.ap-map__road-blue {
  position: absolute;
  width: 82px;
  height: 690px;
  border-radius: 999px;
  background: rgba(76, 190, 255, 0.42);
  transform: rotate(18deg);
}
```

底部卡文案：

```txt
鹅  发现点 · 50m 内可捕获
500m 范围内 7 个目标，诱饵会提升稀有出现率。
```

倒计时：

- 初始 `04:32`。
- 每秒递减。
- 到 `00:00` 后恢复 `04:32`，目标点位置保持 mock 不变。

选择目标：

- 点击 pin 后更新信息卡。
- `selectedTargetId` 默认为 `target-uncommon-50`。

### 8.3 Capture Screen

页面用途：投掷捕获。

背景：

- `PhoneFrame variant="capture"`
- 背景 `--ap-bg-capture`
- 外框 `#ffd23f`

绝对定位参考：

```txt
Title                x=24  y=38   text=CAPTURE MODE
EnergyCost           x=252 y=48   text=体力 -20
TargetGoose          center x=195 y=305 size=112
ThrowLine            x=85  y=492  w=262 h=4 rotate=-15deg
ItemIcon             x=89  y=535  w=54  h=54
ProbabilityCard      x=34  y=616  w=322 h=120
CardTitle            x=54  y=642  text=鹅 · 面包屑球 · 弹跳略强
ProbabilityBar       x=54  y=678  w=282 h=16
CardFooter           x=54  y=708  text=捕获成功率 78% · 最佳力度 35-75
```

交互模型：

```ts
interface CaptureUiState {
  power: number;             // 0-100
  bestMin: number;           // 35
  bestMax: number;           // 75
  captureRate: number;       // 78
  phase: "aiming" | "throwing" | "success" | "failed";
}
```

第一阶段可做简化：

- 画面默认展示 `power=55`。
- 点击页面或按钮触发 mock 捕获。
- 如果 `power` 在 `35-75`，显示成功 toast：`捕获成功：鹅已加入图鉴`。
- 如果不在范围，显示失败 toast：`捕获失败，再试一次`。

如果做拖拽：

- 不需要可见 slider。
- 可用 pointer down / move 控制投掷线角度或 power。
- 但第一阶段不是必须。

### 8.4 Pokedex Screen

页面用途：图鉴收藏。

背景：

- `PhoneFrame variant="pokedex"`
- 背景 `--ap-bg-pokedex`
- 外框 `#9b5cff`

绝对定位参考：

```txt
Title                x=24  y=43   text=图鉴 POKEDEX
Count                x=267 y=50   text=已收集 59
Tabs                 x=28  y=91   text=全部 猫 鹅 狗
Grid                 x=26  y=130
CardWidth            156
CardHeight           205
ColumnGap            30
RowGap               32
```

卡片位置：

```txt
Card 1 legendary cat    x=26  y=130
Card 2 rare dog         x=212 y=130
Card 3 uncommon goose   x=26  y=367
Card 4 locked           x=212 y=367
```

Tab 过滤：

```ts
type PokedexFilter = "all" | "cat" | "goose" | "dog";
```

Tab 文案：

```txt
全部 猫 鹅 狗
```

已收集卡片点击：

- 设置 `selectedAnimalId`。
- 可在卡片上加 2px 内发光或轻微 scale。
- 第一阶段不强制做详情 modal。

未解锁卡片点击：

- toast：`尚未发现`

### 8.5 Battle Arena Screen

页面用途：动物对战。

背景：

- `PhoneFrame variant="battle"`
- 背景 `--ap-bg-battle`
- 外框 `#ff4a89`

绝对定位参考：

```txt
Title                x=24  y=38   text=BATTLE ARENA
CatIcon              center x=109 y=257 size=112
VS                   center x=195 y=260 text=VS
DogIcon              center x=285 y=257 size=112
PlayerHp             x=34  y=333  w=134 h=14
EnemyHp              x=226 y=333  w=134 h=14
AdvantageText        x=88  y=386  text=火 > 草 · 光克暗
BattleLog            x=28  y=475  w=334 h=153
StrategyButtons      y=696        text=激进 平衡 防守
```

策略：

```ts
const strategies = {
  aggressive: {
    label: "激进",
    playerDamage: 24,
    selfDamage: 12,
    log: ["猫 使用 激进策略，暴击 x2", "获得金币 +70 · 掉落诱饵"],
  },
  balanced: {
    label: "平衡",
    playerDamage: 16,
    selfDamage: 8,
    log: ["猫 使用 平衡策略，稳定命中", "获得金币 +45"],
  },
  defensive: {
    label: "防守",
    playerDamage: 9,
    selfDamage: 3,
    log: ["猫 使用 防守策略，减少伤害", "获得金币 +25"],
  },
};
```

血条：

- 外层尺寸固定。
- 内层 `width: ${hp}%`。
- `playerHp` 默认 100。
- `enemyHp` 默认 100。

按钮：

- 三个文字按钮。
- active 策略用黄色。
- 点击后更新日志、血条。

### 8.6 Store + Check-in Screen

页面用途：签到奖励与道具购买。

背景：

- `PhoneFrame variant="store"`
- 背景 `--ap-bg-store`
- 外框 `#ffb300`

绝对定位参考：

```txt
Title                x=24  y=43   text=商店 STORE
Coins                x=254 y=51   text=金币 1260
CheckInCard          x=26  y=93   w=338 h=137
CheckInTitle         x=48  y=123  text=7 日签到轨道
RewardNumbers        x=48  y=166  text=20 30 40 50 60 80 150
RewardNote           x=48  y=202  text=第 7 天额外送：玩具球 🎾
SectionTitle         x=28  y=278  text=道具背包
ItemList             x=36  y=328  w=314
```

签到状态：

```ts
interface CheckInState {
  currentDay: number;       // 1-7
  claimedToday: boolean;
  rewards: number[];        // [20,30,40,50,60,80,150]
}
```

点击签到卡：

- 如果 `claimedToday=false`，金币增加当前奖励，设置 true。
- 高亮当前 day 数字。
- 如果已经领取，toast：`今日已签到`。

购买道具：

- 点击 item row。
- 如果金币足够：金币减少价格，toast `已购买：玩具球`。
- 如果金币不足：toast `金币不足`，行 disabled。

## 9. HTML 参考稿说明

已提供 `outputs/animal-poke-ui-reference.html`，作用：

- 让后续无视觉模型可以从 DOM 结构、class 名、CSS token 直接迁移。
- 让工程师快速打开检查六个页面的大致视觉。
- 作为 React/Vue 组件实现前的低成本 reference。

HTML 文件要求：

- 单文件可打开。
- 不依赖 npm、CDN、外部图片。
- 不把 JPG 作为背景。
- 包含 6 个 screen。
- 包含基础点击切换。

这个 HTML 不是最终生产代码，但 class 命名、token、组件层次可以迁移到项目。

## 10. 后续模型执行步骤

### Step 1：识别项目技术栈

执行：

```sh
rg --files
```

重点找：

```txt
package.json
vite.config.*
next.config.*
src/
app/
pages/
components/
tailwind.config.*
```

然后读：

```txt
package.json
src 入口文件
现有路由文件
现有样式系统
```

不要上来就改代码。

### Step 2：决定接入方式

如果是 React SPA：

- 新增 `src/features/animal-poke/AnimalPokeApp.tsx`
- 在现有 route 或 app entry 挂载。

如果是 Next.js：

- App Router：`app/animal-poke/page.tsx`
- Pages Router：`pages/animal-poke.tsx`
- 样式放同级 CSS module 或全局 feature CSS。

如果是 Vue：

- 新增 feature component。
- 在 router 中加 route。

如果是纯 HTML：

- 可以直接使用参考 HTML 继续扩展。

### Step 3：先落 token，再落组件

顺序固定：

1. `animalPoke.css`
2. `PhoneFrame`
3. `AnimalIcon`
4. `PageTitle`
5. `ActionButton`
6. `BottomTabBar`
7. `RarityCard`
8. `DiscoveryPin`
9. `BattleLog`
10. `StoreItemRow`
11. 6 个 screen

原因：

- token 和基础组件完成后，页面实现只是组合。
- 避免每个页面复制样式。

### Step 4：实现屏幕切换

最小状态：

```ts
const [screen, setScreen] = useState<ScreenId>("discover");
const [selectedTargetId, setSelectedTargetId] = useState("target-uncommon-50");
const [pokedexFilter, setPokedexFilter] = useState<PokedexFilter>("all");
const [coins, setCoins] = useState(1260);
const [energy, setEnergy] = useState(84);
```

事件：

```ts
onStartCapture -> setScreen("capture")
onBottomTab("pokedex") -> setScreen("pokedex")
onBottomTab("battle") -> setScreen("battle")
onBottomTab("store") -> setScreen("store")
onSelectTarget -> setSelectedTargetId(id)
onBuyItem -> update coins or toast
onStrategy -> update battle state
```

### Step 5：验收

运行项目现有命令。优先级：

```sh
npm run typecheck
npm run lint
npm run test
npm run build
```

如果项目没有对应命令，只跑存在的。

浏览器检查尺寸：

- `390 x 844`
- `375 x 812`
- `430 x 932`

视觉检查：

- 标题不换行。
- CTA 不贴边。
- 底部导航不被浏览器底栏盖住。
- 图鉴两列对齐。
- 商店价格列对齐。
- 战斗 VS 不遮挡图标。
- 地图点位标签不互相重叠。

## 11. 更具体的 CSS Skeleton

后续实现可以直接迁移以下结构。

```css
.ap-root {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  background: #030404;
  color: var(--ap-text);
  font-family: Inter, "PingFang SC", "Microsoft YaHei", system-ui, sans-serif;
}

.ap-phone {
  position: relative;
  width: min(100vw, 390px);
  min-height: 844px;
  overflow: hidden;
  border: 2px solid var(--ap-frame-color);
  border-radius: 34px;
  background: var(--ap-frame-bg);
}

.ap-phone--discover {
  --ap-frame-color: var(--ap-orange);
  --ap-frame-bg: var(--ap-bg-discover);
}

.ap-phone--map {
  --ap-frame-color: var(--ap-uncommon);
  --ap-frame-bg: var(--ap-bg-map);
}

.ap-phone--capture {
  --ap-frame-color: var(--ap-yellow);
  --ap-frame-bg: var(--ap-bg-capture);
}

.ap-phone--pokedex {
  --ap-frame-color: var(--ap-epic);
  --ap-frame-bg: var(--ap-bg-pokedex);
}

.ap-phone--battle {
  --ap-frame-color: var(--ap-pink);
  --ap-frame-bg: var(--ap-bg-battle);
}

.ap-phone--store {
  --ap-frame-color: var(--ap-gold);
  --ap-frame-bg: var(--ap-bg-store);
}
```

## 12. 组件质量标准

每个组件都要满足：

- 有明确 props。
- 没有隐式依赖全局业务状态。
- 样式通过 class 和 token 控制。
- 文案由 props 或 mock data 提供。
- 不引入暂时不需要的复杂抽象。

具体原则：

- KISS：页面状态保持简单，能用一个 state 解决就不引入状态管理库。
- YAGNI：不做真实地图、不做真实相机、不做战斗数值系统。
- DRY：外框、标题、按钮、卡片、动物图标复用。
- SOLID：组件职责单一，页面只组合。

## 13. Done Definition

完成标准：

1. Animal Poke 页面能打开。
2. 六个 screen 都能访问。
3. UI 不是图片背景，而是真实组件。
4. 页面文案与本计划一致。
5. 390px 宽度没有横向滚动。
6. 组件没有明显重叠或溢出。
7. 可点击元素高度至少 44px。
8. 构建命令通过，或明确说明项目缺少该命令。
9. 没有 git commit / push，除非用户明确要求。

