# Animal Poke 全面审计报告

> 审计日期：2026-07-10  
> 审计基线：`main` / `6dc0986`  
> 视角：游戏设计、玩家体验、前端、后端、API、数据、安全、测试、部署与运营

## 1. 结论

当前项目并非“某一个识别接口坏了”，而是存在一条由多处断点组成的系统性故障链：

1. 当前生产入口没有调用动物识别 API，拍摄帧会被丢弃。
2. 捕获页默认物种固定为鹅，只执行客户端随机结算。
3. 即使接入真实识别，本地 Vision 配置为空时会逐字段回退到 LLM 配置，视觉请求可能被发送给文本模型。
4. 客户端隐私同意只写入 `localStorage`，生产后端却要求数据库中存在 `v1` 同意记录。
5. 已存在但未挂载的 Analyze → Value 管线会丢失服务端 `inference_id`；随后同步会被后端拒绝。
6. 同步队列把所有 HTTP 409 都当成“已同步”，包括 `inference_invalid`，会造成静默数据丢失。
7. 图鉴、商店、签到、地图、天气和战斗大量使用静态或组件私有状态，没有形成真实游戏进程。
8. 预发 Kustomize 配置仍指向生产 MySQL，生产和预发镜像都被固定为 `:dev`，而 CI 校验不会发现。
9. 前端生产镜像默认 API 地址为空，但 Nginx/Ingress 没有 `/api` 反向代理，生产请求可能收到 SPA HTML 而不是 JSON。
10. 现有 488 个前端测试和全绿的 Go 测试主要验证模块或旧界面，不足以证明生产入口可用。

因此，当前状态应定义为：**功能模块较多、单元测试较多，但生产入口仍是可点击原型，核心循环与服务端权威链路没有闭环。**

## 2. 已执行验证

| 验证 | 结果 | 说明 |
|---|---|---|
| `cd frontend && npm test` | 通过 | 43 个测试文件、488 个测试全部通过 |
| `cd frontend && npm run build` | 失败 | 当前工作区缺少已声明的 `@types/node`，报 `TS2688`；本机 Node 26 也超出项目 Node 22 engine |
| `cd backend && go test ./...` | 通过 | 需允许测试使用本地监听端口 |
| `cd backend && go vet ./...` | 通过 | 使用临时 Go cache |
| `cd backend && go test -race ./...` | 通过 | 当前测试覆盖内未发现竞态 |
| `cd backend && go test -cover ./...` | 部分覆盖 | handlers 34.1%、repo 23.1%、middleware 51.5%、services 61.2% |
| `kubectl kustomize .../production` | 构建成功但内容错误 | backend/frontend 镜像均为 `:dev` |
| `kubectl kustomize .../staging` | 构建成功但内容危险 | `DB_HOST=mysql.production.svc.cluster.local`，镜像仍为 `:dev` |
| 本地前端 Dev Server | 可启动 | `http://127.0.0.1:5173/` |
| 浏览器交互/截图 | 未完成 | 当前会话没有可用浏览器实例；交互结论来自生产入口代码、状态流与 CSS，未冒充真机验证 |

诊断命令生成了被 Git 忽略的 `frontend/tsconfig.tsbuildinfo`；未删除该文件，避免未经确认执行删除操作。Git tracked 状态保持干净。

## 3. 核心故障链证据

### 3.1 生产入口没有识别动物

- `frontend/src/App.tsx:1-9` 挂载的是 `features/animal-poke/AnimalPokeApp`。
- `frontend/src/features/animal-poke/AnimalPokeApp.tsx:61-68` 没有向发现页传入 `onFrame`。
- `frontend/src/features/animal-poke/screens/DiscoverScreen.tsx:85-89` 捕获一帧后，即使没有得到 Blob 或消费者，也会直接进入捕获页。
- 同文件 `72-83` 在相机仅仅 ready 时就显示“VLM 实时识别中”，没有任何检测状态或请求。
- `frontend/src/features/animal-poke/screens/CaptureScreen.tsx:13-35` 默认 `species='goose'`，力度固定为 55，只调用客户端 `Math.random()` 结算。

直接结果：玩家对准猫或狗也不会产生 `/api/v1/vision/detect` 请求，随后进入默认鹅捕获页。

### 3.2 配置会把 Vision 回退到 LLM

- 当前 `backend/.env` 中 `VLM_ENDPOINT`、`VLM_KEY` 为空，只有 `LLM_ENDPOINT`、`LLM_KEY`、`LLM_MODEL` 已设置；审计只检查了“是否设置”，没有输出任何密钥值。
- `backend/internal/config/config.go:145-150` 将 Vision endpoint/key/model 分别回退到 `VLM_*`，最后再回退到 `LLM_*`。
- `backend/internal/services/ai.go:284-297` 会把图片作为 `image_url` 发送给最终得到的 Vision model。

直接结果：配置校验认为 Vision 已配置，但实际可能把图片请求发给文本模型或错误端点，mock 也不会生效。

### 3.3 同意记录在生产环境必然断链

- `frontend/src/compliance/index.ts:24-62` 只把版本 `1.0.0` 写入本地存储。
- 前端没有调用 `/api/v1/privacy/consent`。
- `backend/internal/routes/router.go:145-152` 在 production 对 Vision 开启同意门禁，要求版本 `v1`。
- `backend/internal/repo/device.go:108-124` 使用相等比较验证版本。

直接结果：即使把识别 API 接到生产 UI，新设备也会收到 403 `consent_missing`。

### 3.4 推理凭证丢失并被伪装成同步成功

- `frontend/src/services/capturePipeline.ts:223` 把客户端 `sessionId` 当作 `inferenceRequestId`。
- 同文件 `252-261`、`281-290` 对 Analyze/Value 响应做字段过滤时没有保留后端 `inference_id`。
- `backend/internal/handlers/sync.go:138-160` 会消费真实 inference 记录；不存在、重复或状态错误时返回 409 `inference_invalid`。
- `frontend/src/services/syncQueue.ts:154-164` 把所有 409 都标记为 synced，没有检查 `reason_code`。

直接结果：本地显示同步完成，服务器实际上没有保存动物，刷新或换设备后数据丢失。

### 3.5 商店与签到可无限产币且不扣款

- `frontend/src/features/animal-poke/screens/StoreScreen.tsx:34-40` 计算购买后的更低金币数。
- `frontend/src/features/animal-poke/AnimalPokeApp.tsx:51-56` 只处理正 delta，负 delta 被直接忽略。
- Store 页面虽然调用 `useShop()`，但没有使用其 `buyItem` 或 `checkIn`。
- 签到状态是页面内 `useState`，切换 Tab 后组件卸载，重新进入又回到第 1 天未领取状态。

直接结果：购买提示成功但金币不减少、背包不增加；重复切页可反复领取签到金币。

### 3.6 天气、地图和生产 API 地址均未真正接通

- 生产发现页把城市和天气硬编码为“宁波”“雨”。
- 生产地图使用 `features/animal-poke/data/huntTargets.ts` 的三个固定点，刷新倒计时归零后只是重置数字。
- `frontend/src/weather/WeatherContext.tsx:96-103` 调用 `fetchWeekWeather(lat,lng)` 时没有传 Token 或 API base。
- `frontend/src/weather/api.ts:49-75` 因此会请求同源 `/api/v1/weather/week`，收到 401/HTML/404 后静默返回 null。
- `deploy/Dockerfile.frontend:11-13` 的 `VITE_API_BASE_URL` 默认空。
- `deploy/nginx-frontend.conf:40-43` 只做 SPA fallback，没有 `/api` 代理。

直接结果：开发和生产都可能长期显示随机/静态天气；生产前端若没有构建时注入地址，全部 API 请求都会打到错误主机。

### 3.7 部署配置存在跨环境风险

- `deploy/k8s/base/kustomization.yaml:15-19` 把 backend/frontend `newTag` 固定为 `dev`。
- production 和 staging overlay 都没有覆盖镜像 tag。
- staging overlay 没有覆盖 `backend-config` 的 `DB_HOST`，实际渲染仍为 `mysql.production.svc.cluster.local`。
- `.github/workflows/ci.yml:142-145` 仍尝试替换已经不存在的 `__IMAGE_TAG__`，之后只 grep 服务名，因此错误 manifest 仍可通过。

直接结果：预发可能写入生产数据库；发布可能部署旧的 `dev` 镜像；CI 给出错误的绿色信号。

### 3.8 测试绿灯不代表生产可用

- `frontend/src/features/animal-poke/AnimalPokeApp.test.tsx:21-38` 主要断言 body/PhoneFrame 存在以及点击不崩溃。
- 真正有“识别、投掷、生成”行为的测试主要覆盖未被生产入口引用的 `frontend/src/components/*` 旧界面。
- 仓库没有 Playwright/Cypress E2E。
- `.github/workflows/ci.yml:71-76` 对全量 `npm test` 设置 `continue-on-error: true`。
- OpenAPI 未包含 router 中的 errors/ranking/PvP/social/ops 七条路由，且大量 schema 使用 `additionalProperties: true`。

## 4. 玩家体验与游戏设计判断

### 4.1 当前核心循环

当前玩家实际经历的是：

`同意弹窗 → 相机或鹅占位图 → 点击开始 → 默认鹅随机判定 → Toast → 离开页面`

设计文档承诺的：

`发现 → 真实识别 → 捕获交互 → 属性生成 → 稀有揭晓 → 图鉴收藏 → 养成/战斗`

在生产入口中没有形成闭环。最优先工作不是继续增加玩法，而是先建立一条可追踪、可恢复、服务端权威的核心状态机。

### 4.2 交互问题

- “VLM 实时识别中”是虚假状态反馈。
- 相机被拒绝或不可用时仍允许进入捕获，违背产品和反作弊规则。
- 捕获力度固定，点击整个画面即结算，没有投掷学习、预判或技巧空间。
- 捕获失败提示“再试一次”，但同一 session 已结算，再点只会提示“本轮已结算”。
- 成功后没有稀有揭晓、属性解释、收藏落位和下一目标，奖励反馈断裂。
- 地图点、天气、城市和战斗日志都是静态展示，玩家行为不会改变世界或资源。
- 商店和战斗反馈声称发放资源，但真实状态没有同步改变，严重破坏信任。

### 4.3 游戏进程问题

- 新用户默认看到三只假收藏，削弱首捕价值。
- 成就入口长期显示“暂未开放”，但代码中已有成就模块，暴露架构断层。
- 图鉴、战斗、商店、派遣、状态、天气之间没有互相消费或产出真实资源。
- 没有账号绑定和服务端拉取，清除浏览器数据后进度不可恢复。
- LBS 发现点在客户端随机生成，无法支撑公平排行、区域竞技或反作弊。
- 设计文档中“每次投掷 1 体力”与“单次捕获 20 体力”冲突，代码又存在多套配置。
- 设计要求 2~5 FPS 上传，但后端每日 detect 限额只有 100 次，理论上 20~50 秒就会耗尽全天额度。

### 4.4 动物福利与现实安全

当前设计用猫粮、面包、骨头作为投掷物，尤其“给鹅投面包”可能鼓励现实喂食和追逐野生动物。建议将交互改成纯虚拟的“友好信号、镜头对焦、贴纸或光点”，并明确：

- 不追逐、不触摸、不投喂、不惊扰动物。
- 不在道路、水边、施工区、私人区域生成引导点。
- 极端天气、夜间和定位精度过低时暂停户外捕获。
- 识别人脸、儿童、车牌或住宅时先裁剪/遮挡，再发送给第三方模型。

## 5. 优先级建议

### P0：先恢复可信的最小闭环

1. 生产状态机和真实 Vision 请求。
2. Vision/LLM 配置强隔离。
3. 客户端与服务端同意版本同步。
4. inference provenance 与同步错误语义。
5. 服务端权威的动物创建与数值校验。
6. 商店/签到经济漏洞。
7. 生产 API 路由与前端镜像 smoke。
8. staging 数据库隔离、镜像 digest 和 manifest 门禁。
9. 全链路 E2E 与 OpenAPI 契约门禁。

### P1：让游戏可持续运行

1. 图鉴真实数据、捕获结果页、战斗和商店接回 Context。
2. LBS/天气/地图接入以及范围校验。
3. 相机、网络、取消、重试、幂等和离线恢复。
4. 识别黄金集、阈值校准、未知类拒绝。
5. 隐私导出/删除完整性、JWT/代理/限流/指标加固。
6. 移动布局、无障碍、PWA 更新和国际化。

### P2：增强留存与差异化

1. 首捕教学、纠错反馈和识别解释。
2. 动物福利安全设计。
3. 重复收藏转化、日/周目标、派遣和战斗解锁节奏。
4. 音效、触觉、低电量/低流量模式。
5. 账号绑定、跨设备恢复和长期运营配置。
6. 漏斗、经济平衡、识别质量和成本监控。

## 6. 配套交付物

- `docs/grok-issue-backlog-2026-07-10.md`：可直接复制给 Grok 的分级 Issue。
- `docs/api-test-matrix-2026-07-10.md`：覆盖全部 35 个当前路由的 API 测试矩阵。
- `docs/game-design-roadmap-2026-07-10.md`：从止血到可玩、再到留存的实施路线。

