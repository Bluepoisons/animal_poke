# Animal Poke · Grok 可执行 Issue Backlog

> 基线：`main@6dc0986`  
> 原则：每个 Issue 独立实现、独立验证、尽量独立 PR；不要把多个 P0 合成一次大重构。

## Grok 执行约定

把任意一条 Issue 交给 Grok 时，同时附上以下约定：

1. 先读 Issue 指定文件和相邻测试，再修改代码。
2. 分支绝对不能以 `codex/` 开头；使用 `fix/`、`feat/`、`test/`、`chore/`。
3. 不允许在 `main` 直接 commit/push。
4. 只处理当前 Issue，禁止顺手重写无关模块。
5. 不得输出、提交或记录 `.env`、Token、API Key、照片、精确坐标。
6. API 变更必须同步 `docs/openapi.yaml`、生成类型和契约测试。
7. 前端改动必须从生产入口 `App -> features/animal-poke/AnimalPokeApp` 验证，不能只测旧 `src/components/*`。
8. 每个 PR 必须说明：根因、改动、测试、兼容性、风险、回滚方法。
9. 必须执行与改动相称的测试；无法执行时明确说明原因，不能把静态推断写成真机结论。
10. 完成标准以验收条件为准，不以“代码已写”或“单测绿色”为准。

## 依赖顺序

建议按以下顺序执行：

`AP-002 -> AP-003 -> AP-001 -> AP-004 -> AP-005 -> AP-006 -> AP-014`

部署止血可并行：

`AP-010 + AP-011 + AP-015`

经济与外围玩法可并行：

`AP-008 + AP-009 + AP-026 + AP-027`

---

## P0 · 阻断核心玩法、数据安全或生产发布

### AP-001 [P0][Frontend][Game Loop] 接通生产版“发现 -> 识别 -> 捕获”状态机

**问题与证据**

- 生产入口由 `frontend/src/App.tsx` 挂载 `features/animal-poke/AnimalPokeApp`。
- `AnimalPokeApp.tsx:61-68` 未向发现页传 `onFrame`。
- `features/animal-poke/screens/DiscoverScreen.tsx:85-89` 即使没有有效帧也直接进入捕获页。
- `features/animal-poke/screens/CaptureScreen.tsx:13` 默认物种固定为 `goose`。

**实现范围**

- 新增唯一的 `CaptureFlowState`：`idle | camera_ready | detecting | target_confirmed | capturing | generating | saving | syncing | completed | failed`。
- 状态至少保存 `photoBlob`、`detectInferenceId`、`detection`、`selectedBox`、`targetId`、`captureAttemptId`。
- 发现页必须调用真实 `getVisionDetector()`；只有受支持物种且达到配置阈值才能进入捕获。
- 多动物时要求玩家选择目标；直接访问 `#capture` 必须经过流程守卫。
- 相机拒绝、相机忙、无帧、离线、无动物、低置信度时禁止创建捕获会话。

**验收条件**

- 猫、狗、鹅会把正确物种、框、置信度和 inference ID 传入捕获页。
- Network 可观察到一次受控 `/api/v1/vision/detect` 请求；UI 不再无请求显示“VLM 实时识别中”。
- 无动物或未知物种不会进入捕获。
- 刷新、后退、切 Tab 后不会用默认鹅继续旧流程。

**测试**

- 生产入口集成测试：三物种、空图、多动物、低置信度、401/403/429/5xx、超时、取消。
- E2E 模拟相机 Blob，断言 detect 完成前 capture 不可达。
- 路由测试：非法 `#capture` 重定向到 discover。

### AP-002 [P0][Backend][Config] 删除 Vision 到 LLM 的隐式拼接回退

**问题与证据**

- 当前本地环境只有 `LLM_*` 有值，独立 `VISION_*`/`VLM_*` 未完整配置。
- `backend/internal/config/config.go:145-150` 分字段回退 endpoint/key/model，可能拼成错误组合并让 readiness 误判已配置。

**实现范围**

- 将 Vision 配置视为原子三元组：`VISION_ENDPOINT + VISION_KEY + VISION_MODEL` 必须同时存在或同时为空。
- 兼容 `VLM_*` 时只能整组三元组兼容，禁止逐字段混合。
- 默认禁止回退到 `LLM_*`；若确需复用，增加显式 `VISION_REUSE_LLM=true`，并校验模型能力清单。
- production 要求 HTTPS，拒绝 localhost、明文 HTTP、空模型和不完整三元组。
- readiness 输出安全的 capability 状态，不输出 endpoint/key。

**验收条件**

- 只配置 LLM 时 Vision 被判定为未配置，不会向文本模型发送图片。
- 混合 `VISION_ENDPOINT + VLM_KEY + LLM_MODEL` 启动失败。
- 显式复用时使用同一完整配置，并有清晰日志 fingerprint。

**测试**

- 配置表驱动测试覆盖完整、缺 1 项、混合前缀、显式复用、生产 HTTP endpoint。
- HTTP stub 断言文本模型从未收到 `image_url` 请求。

### AP-003 [P0][Privacy][Frontend+Backend] 打通同意记录、scope 与版本

**问题与证据**

- 前端 `compliance/index.ts` 只保存本地版本 `1.0.0`。
- 后端生产 Vision 要求数据库中存在 `v1` 同意记录。
- 拒绝授权后弹窗永久遮挡，和“仍可浏览本地图鉴”的文案矛盾。

**实现范围**

- 定义服务端唯一同意版本和枚举 scope：`photo`、`location`、`precise_location`。
- 首次鉴权成功后，同意按钮调用 `POST /api/v1/privacy/consent`；服务端成功后才写本地缓存。
- 撤回必须同步服务端，并立即影响 Vision/Geo/Weather/精确位置保存。
- 同意版本升级时重新弹窗；离线同意只进入 pending，不伪装为服务端已授权。
- 拒绝授权时进入只读模式，允许图鉴和设置，不允许发现/捕获。

**验收条件**

- production 新设备同意后 Vision 不再返回 `consent_missing`。
- photo 和 location 可分别授权/撤回。
- DB 不可用时保护接口 fail-closed，不绕过授权。
- UI 能查看当前 scope、版本、时间和撤回入口。

**测试**

- 首次同意、重复同意、撤回、版本升级、服务端失败、离线恢复、跨 Tab。
- 后端逐 scope 授权矩阵；撤回后旧 Token 请求立即被拒绝。

### AP-004 [P0][Sync][Data Loss] 传播真实 inference_id 并正确分类 409

**问题与证据**

- `frontend/src/services/capturePipeline.ts:223` 用本地 session ID 冒充 inference ID。
- Analyze/Value 的真实 `inference_id` 被 `validateAnalysis`/`validateValue` 丢弃。
- 后端返回 `inference_invalid` 409 后，`syncQueue.ts:154-164` 把所有 409 当作 synced。

**实现范围**

- 前端类型保留 detect/analyze/value 各阶段 `inference_id`、model、prompt version、source、degraded。
- 同步只使用 value 阶段服务端返回的 inference ID。
- 后端 409 reason code 至少拆分：`duplicate_animal`、`idempotency_conflict`、`inference_invalid`、`inference_consumed`、`inference_expired`。
- 队列仅把 `duplicate_animal` 或服务端明确的幂等重放视为成功。
- 对永久失败、可重试失败、需用户重新生成分别建状态和 UI。

**验收条件**

- 正常捕获可通过 `GET /sync/animals` 拉回。
- 错误 inference ID 不会被标记 synced。
- “服务端已提交但响应丢失”重试后返回原结果，不产生第二只动物。

**测试**

- 正常、重复 UUID、错误 kind、跨设备、已消费、已过期、响应丢失。
- 队列单测逐个验证 reason code，而不是只看 status 409。

### AP-005 [P0][Backend][Server Authority] 强制服务端推理血缘与原子消费

**问题与证据**

- `syncRequest` 允许不传 inference ID，并接受客户端自报 species/rarity/stats。
- `InferenceRepo.Consume` 先读后写，没有 `status='success'` 条件或行锁，并发可重复消费。
- detect/analyze/value 凭证未建立父子关系。

**实现范围**

- production 同步强制使用 value inference；value 必须引用 analyze，analyze 必须引用 detect + 目标框。
- inference 表增加 `parent_inference_id`、规范化结果摘要/JSON、过期时间、配置版本。
- sync 从服务端 inference 结果构造关键字段，或严格比对客户端提交值；客户端不得重抽稀有度。
- 使用条件 UPDATE 或 `SELECT ... FOR UPDATE` 原子消费，要求 `RowsAffected == 1`。
- kind、device、species、目标框和有效期必须一致。

**验收条件**

- 同一 inference 100 并发仅 1 个请求成功。
- detect/analyze inference 不能用于创建动物。
- 伪造 legendary、超范围属性、错物种全部拒绝且不落库。

**测试**

- MySQL 并发集成测试、跨设备测试、过期测试、篡改字段测试、事务回滚测试。

### AP-006 [P0][Backend][Cost] 为 AI 与同步写接口实现服务端幂等

**问题与证据**

- 客户端向 analyze/value/sync 发送 `Idempotency-Key`，后端没有读取或存储。
- 前端和后端都可能重试，单次操作可能触发多次付费 Provider 请求并生成不同属性。

**实现范围**

- 新增 `(device_id, route, idempotency_key)` 唯一记录，保存请求摘要、处理中状态、最终状态码和响应。
- 相同 key + 相同请求返回原响应；相同 key + 不同摘要返回 409 `idempotency_conflict`。
- 并发相同请求只执行一次上游调用，其余等待或返回处理中。
- 定义失败缓存策略：永久 4xx、可重试 5xx、处理中超时和记录清理 TTL。

**验收条件**

- 同一 key 重放 20 次只产生一次 Provider 调用和一次 inference。
- 超时后重试不会得到不同 rarity/stats。
- sync 丢响应重试不会重复写库或重复扣资源。

**测试**

- 并发、相同 key 不同 body、处理中崩溃恢复、上游 429/5xx、数据库唯一冲突。

### AP-007 [P0][Vision][Taxonomy] 禁止未知物种静默映射为鹅

**问题与证据**

- `frontend/src/services/visionDetect.ts:97-102` 对非猫、非狗结果一律返回 `goose`。
- 后端 Detect prompt 允许 `bird`，服务端只要求 species 非空。

**实现范围**

- 建立后端权威 taxonomy：`cat | dog | goose | unknown | unsupported`。
- 明确有限别名表，不使用“默认鹅”。
- 后端在返回前做枚举校验；未知类保留原始 label 仅用于审计，不进入捕获。
- 多动物时返回稳定排序和目标 ID，不由前端只取最高置信度后忽略其余动物。

**验收条件**

- 鸭、天鹅、鸟、人、玩偶、屏幕照片、空标签均不会生成鹅。
- Mock/降级结果有明显标识，不能伪装为真实识别。

**测试**

- 猫狗鹅别名、中文标签、未知类、空数组、非法置信度、越界框、多动物。

### AP-008 [P0][Economy][Frontend] 修复商店不扣款与无限签到

**问题与证据**

- Store 页面绕过 `ShopContext`，购买后的负 delta 被上层忽略。
- 购买不入背包；签到状态随切屏卸载，可反复领取第 1 天奖励。
- feature 数据使用 `toy-ball`，真实 ShopContext 使用 `toy_ball` 等不同 ID。

**实现范围**

- 页面只调用 `shop.buyItem()`、`shop.checkIn()`、`shop.useItem()`；删除私有经济状态。
- 统一 canonical item ID、价格、效果、限购和显示配置。
- 购买必须原子更新金币、库存、每日限购、经济流水；失败整体回滚。
- 防止双击、并发 Tab 和时钟回拨重复领奖。

**验收条件**

- 购买成功后金币减少、库存增加、刷新后仍保持。
- 当天只能签到一次；切 Tab、刷新、离线重开都不能重复领取。
- 余额不足、达到限购或状态保存失败不会显示成功。

**测试**

- 余额边界、快速双击、多 Tab、跨日、断签、时钟回拨、存储失败和数据迁移。

### AP-009 [P0][LBS][Weather] 接通真实定位、天气、发现点和鉴权

**问题与证据**

- 生产 UI 硬编码“宁波 · 雨”和三个静态发现点。
- WeatherContext 调用 API 时未传 Token 或 API base，后端路由却要求 JWT，失败后被静默降级掩盖。
- 客户端本地随机生成发现点，无法支持公平排行和反作弊。

**实现范围**

- 前端 Geo/Weather 统一走 `authedRequest` 和公开配置 base URL。
- 生产 UI 使用 `LbsContext`/`WeatherContext` 的真实状态，展示定位精度、来源、降级原因。
- 将可领取发现点和范围判定迁到服务端权威接口；客户端只渲染。
- 极端天气、低精度、超出范围、拒绝定位时建立明确行为。

**验收条件**

- 城市变化会更新天气和服务端发现点。
- 超出范围不能开始捕获，客户端改坐标不能绕过服务端。
- production provider 缺失返回明确 503，不伪装成真实城市/天气。

**测试**

- 定位成功/拒绝/超时/低精度/GPS 跳点、跨城市、JWT 续签、天气 429/5xx、发现点过期。

### AP-010 [P0][Deploy][Frontend] 修复生产 API 地址与前端镜像 smoke

**问题与证据**

- 前端镜像的 `VITE_API_BASE_URL` 默认空。
- Nginx 只有 SPA fallback，app Ingress 也不把 `/api` 转发给后端。
- CI 没有构建、启动、smoke 前端镜像。

**实现范围**

- 选择并只保留一种策略：运行时 `/config.js`，或每环境构建时强制注入 API URL，或同域 `/api` 反代。
- 空/非法生产配置必须启动失败或显示明确配置错误，不能返回 index.html 冒充 API。
- CI 构建前端镜像，启动 frontend+backend，校验 index/assets/manifest/sw/auth JSON。
- CSP `connect-src` 只允许实际 API 域，不使用宽泛 `https:`。

**验收条件**

- production 浏览器请求命中 API Service，Content-Type 为 JSON，无 CORS/CSP 错误。
- 错误地址在发布前被 smoke 阻断。

**测试**

- production/staging/local 三环境；绝对 URL、同域代理、空配置、错误域、CORS preflight。

### AP-011 [P0][Kubernetes][Data Isolation] 修复 staging 生产库、`:dev` 镜像和 CI 假通过

**问题与证据**

- `kubectl kustomize` 实测 production/staging 都输出 backend/frontend `:dev`。
- staging 输出 `DB_HOST=mysql.production.svc.cluster.local`。
- CI 替换不存在的 `__IMAGE_TAG__` 后只 grep 服务名，因此不会失败。

**实现范围**

- staging overlay 显式设置独立 DB host/name/user/secret；环境间使用不同凭据与网络策略。
- 发布使用不可变 commit SHA 或 digest，通过受控 Kustomize image override 注入。
- CI 禁止 `dev`、`ci`、`latest`、占位符、生产 host 出现在 staging manifest。
- 校验最终 Pod `imageID` 与发布清单一致。

**验收条件**

- staging manifest 中不存在 `.production.svc` 或生产 Secret 引用。
- production/staging 两个镜像都等于本次发布 digest。
- 任一字段错误时 CI 红灯。

**测试**

- 渲染两套 overlay 后做结构化断言；在 staging 写哨兵数据，生产库无变化。

### AP-012 [P0][Commerce][Security] 在真实商店验签完成前关闭生产履约

**问题与证据**

- 未知商品会被自动创建为订阅。
- 任意长度至少 8 的 receipt 都能履约。
- 幂等 key 全局查询未按 device 隔离，可能返回其他设备订单。
- 普通设备 Token 可直接调用退款。

**实现范围**

- production feature flag 默认关闭 commerce create/fulfill/refund，未完成时返回明确 501/503。
- 商品目录只读白名单，禁止请求时自动造商品。
- 对接 Apple/Google 服务端验签/Webhook，校验 bundle、product、transaction、amount、currency、environment。
- 幂等作用域改为 device+operation；receipt hash nullable unique。
- 退款只信平台通知或管理员流程，不能由普通客户端决定。

**验收条件**

- 伪造、重放、沙盒回执到生产、跨设备订单、篡改金额均拒绝。
- 并发履约恰好一次，不泄露其他用户订单 ID。

**测试**

- 第二个未履约订单、相同 key 跨设备、相同 receipt 跨设备、重复退款、续期、平台 webhook 签名。

### AP-013 [P0][Auth][Security] 防止仅凭 device_id 冒领身份并严格验证 JWT

**问题与证据**

- `/auth/device` 对任意已存在 device_id 直接签发 Token；device_id 存在本地且可复制。
- JWT 中间件未要求 iss/aud/exp 必须存在；DeviceChecker 出错时 fail-open。

**实现范围**

- 首次注册生成 installation secret/challenge，后续换 Token 必须证明持有；可选平台 attestation。
- JWT 使用 issuer/audience/expiration required 校验，强制 sub/device_id/jti/token_version 类型。
- Checker DB 错误返回 503，不允许已禁用或已吊销设备在依赖故障时通过。
- 配置 trusted proxies，防止伪造转发头绕过 IP 限流。
- 设计签名密钥轮换和 `kid`。

**验收条件**

- 只知道别人的 device ID 无法换取 Token。
- 缺失、错误、过期 claims 全部 401；依赖故障为 503。
- 设备禁用和 token version 提升立即生效。

**测试**

- 并发注册、secret 轮换、claims 表驱动、DB 超时、伪造 X-Forwarded-For、暴力注册限流。

### AP-014 [P0][QA][E2E] 建立生产入口全链路硬门禁

**问题与证据**

- 43 个测试文件、488 个测试全绿，但核心生产入口没有调用识别 API。
- `AnimalPokeApp.test.tsx` 主要断言“不崩溃”。
- CI 对 `npm test` 使用 `continue-on-error: true`，仓库无 Playwright/Cypress。

**实现范围**

- 引入 Playwright，启动 frontend、backend、MySQL 和确定性 AI stub。
- 模拟 camera、geolocation、IndexedDB、网络切换、Token 过期、权限拒绝。
- 覆盖 `授权 -> 鉴权 -> detect -> capture -> analyze -> value -> IDB -> sync -> pull -> 刷新恢复`。
- 移除前端测试 soft-fail；加入只统计生产可达代码的覆盖率阈值。

**验收条件**

- 一次捕获只扣一次体力、生成一只正确物种、消费一次 inference、同步一次。
- 刷新后图鉴仍存在，队列为空；失败阶段可恢复且不重复奖励。
- 故意破坏任一核心 API 调用时 CI 必须失败。

**测试**

- 正常、取消、超时、401 刷新、403 consent、429、断网重连、重复点击、页面刷新、IDB 故障。

### AP-015 [P0][Backup][DR] 建立持久化、加密且可恢复的备份链路

**问题与证据**

- 当前 MySQL CronJob 把备份写入 Pod `emptyDir`，Pod 结束后即丢失。
- 对象存储上传仍是 TODO，CronJob 也未纳入主 Kustomization。

**实现范围**

- 优先采用云数据库 PITR；否则上传到版本化、加密、最小权限对象存储。
- 增加校验和、保留/删除策略、最近成功时间指标和告警。
- 建立隔离环境自动恢复演练和数据一致性检查。

**验收条件**

- 随机时间点恢复达到 RPO <= 15 分钟、RTO <= 60 分钟。
- 备份不可匿名访问；过期或失败在规定时间内告警。
- 恢复后关键表数量和核心 API 校验通过。

**测试**

- 删除测试库后恢复、损坏备份、权限失效、上传中断、保留策略和季度演练。

### AP-016 [P0][Privacy][Data Lifecycle] 修复导出/删除不完整和删除数据回流

**问题与证据**

- export 只读取前 200 只动物。
- delete 只处理动物、推理和授权；历史导出、安全报告、订单等未覆盖。
- `ListSinceVersion` 未过滤 `deleted_at`，软删动物仍可能被 pull 回客户端。
- 精确坐标只有过期时间，没有清理任务。

**实现范围**

- 建立完整数据资产/保留清单；导出分页覆盖所有合法数据。
- 删除使用可靠异步任务或事务+补偿，覆盖法定范围和历史导出 payload。
- 引入 tombstone 协议；删除后 pull 只能下发删除标记，不得回传原内容。
- 物理清理过期精确位置；删除完成后吊销旧 Token。

**验收条件**

- 201+ 动物导出完整。
- 中途故障不会出现半删状态；失败可重试。
- 删除后普通查询、pull、旧 Token 均不能恢复已删数据。

**测试**

- 0/200/201+ 数据、事务故障、跨设备请求、历史 export、订单保留例外、tombstone 合并。

---

## P1 · 影响可玩性、可靠性、可维护性或安全基线

### AP-017 [P1][Capture UX] 实现真实投掷输入、attempt 状态与可恢复失败

**问题**：生产捕获页力度固定为 55，点击整个画面立即随机结算；失败提示“再试一次”，但 session 已 settled，下一次只会显示“本轮已结算”。

**实现范围**

- 使用 Pointer Events 实现按住蓄力或滑动方向/力度；提供键盘和开关控制替代操作。
- 一个 encounter 可包含多个 attempt，但每个 attempt 有独立 ID、成本和结算状态。
- 明确每次失败是否允许重试、消耗什么资源、最多几次；成功/最终失败后锁定。
- 为猫/狗/鹅配置可解释的速度、弹跳和最佳区间，而不是复制三套逻辑。
- 增加轨迹预览、命中反馈、成功/失败动画和结果下一步。

**验收/测试**

- 力度与输入一致；快速连击、pointer cancel、移出按钮、后台切换不会重复结算。
- 每个 attempt 最多扣一次体力/道具；失败后能按规则创建新 attempt。
- Touch、Mouse、Pen、Keyboard、reduced-motion 全部覆盖。

### AP-018 [P1][Camera] 修复相机异步生命周期、权限和设备选择

**问题**：离开页面时 `getUserMedia` 可能仍在 pending；延迟成功的 stream 可能无人停止。相机拒绝/占用时仍可开始捕获。

**实现范围**

- 为每次 start 创建 generation token；过期 Promise 成功后立即停止所有 tracks。
- 对 StrictMode 双 effect、visibilitychange、锁屏、切 Tab、路由卸载做一致清理。
- 支持后置/前置摄像头选择与后置不可用 fallback。
- 权限状态分别处理 denied、dismissed、busy、unsupported、insecure context。
- camera 未 ready 时禁用捕获，并提供可执行的设置指引。

**验收/测试**

- 任意离开路径后浏览器摄像头指示灯关闭。
- 100 次快速 start/stop 无泄漏、无残留 track、无重复权限请求。
- 延迟 Promise、权限撤回、设备拔出、iOS 后台恢复测试通过。

### AP-019 [P1][Privacy][Image Pipeline] 在上传前最小化照片与元数据

**问题**：后端把完整原始图片 base64 发送给 Provider，可能包含 EXIF GPS、人脸、儿童、车牌和住宅背景；WebP magic 通过时即使解码失败也被接受。

**实现范围**

- 客户端或后端重编码为规范 JPEG/WebP，移除 EXIF/ICC/附加 chunk。
- detect 后只裁剪所选动物区域用于 analyze；必要时对人脸和车牌做遮挡。
- 严格解码所有支持格式，拒绝截断/伪造 WebP、图片炸弹和异常尺寸。
- 记录输入摘要和尺寸，不记录文件名、原图或精确坐标。
- 明确临时内存、日志、Provider 保留和删除策略。

**验收/测试**

- 上传到 Provider 的 fixture 不含 EXIF GPS；多人场景只发送必要区域。
- JPEG/PNG/WebP、旋转 EXIF、超大像素、截断文件和恶意 RIFF fuzz 全覆盖。

### AP-020 [P1][Vision] 建立多动物目标一致性与严格输出校验

**问题**：detect 可能识别多只动物，前端只取最高置信度；analyze 对整图重新分析，可能分析另一只动物。Analyze 输出没有完整范围校验。

**实现范围**

- detect 为每个目标返回稳定 `target_id`、box、species、confidence。
- 用户选择目标后，analyze 必须引用 detect inference + target ID/box。
- 后端用 JSON Schema/structured output 校验枚举、字符串长度、1-10 分值、box 面积和边界。
- detect/analyze/value species 必须一致，否则返回 `target_mismatch`。

**验收/测试**

- 猫狗同框时选择猫只会生成猫。
- 模型返回错类型、缺字段、越界值、Markdown、多个 JSON 块时不会静默修成成功。
- 多动物、遮挡、交叠框、目标离开画面测试通过。

### AP-021 [P1][Backend][Resilience] 标准化上游错误、总超时、熔断与并发隔离

**问题**：AI 单次 client timeout 为 30 秒且最多重试 3 次；`Retry-After` 没有上限，可能超过服务端 write timeout。Vision 上游失败多被压成 500。

**实现范围**

- 为 Geo、Weather、Vision、LLM 分别配置总 deadline、单次 timeout、最大重试和并发 semaphore。
- 只重试网络错误、429、502、503；退避增加 jitter 和硬上限。
- 增加 circuit breaker/bulkhead；返回 429/502/503/504、`reason_code`、`retryable` 和 request ID。
- 客户端对 daily quota、短时限流和 Provider 故障给出不同 UX。

**验收/测试**

- 429 携带巨大 Retry-After、慢响应、半响应、断连、畸形 JSON、5xx 风暴均在总预算内结束。
- 取消能传到上游；恢复后 half-open 成功；无 goroutine/连接泄漏。

### AP-022 [P1][Game Design][Cost] 统一扫描频率、每日额度、延迟和成本模型

**问题**：设计文档要求 2~5 FPS 上传，但每日 detect 限额只有 100 次，理论上 20~50 秒耗尽；当前 UI又只做单帧且声称“实时”。

**实现范围**

- 明确 MVP 是“手动扫描”还是“低频连续扫描”，禁止文案和实现不一致。
- 使用本地非 AI 质量门禁：模糊度、亮度、运动幅度、重复帧 hash，减少无效上传。
- 命中后停止上传，使用短期目标跟踪；低电量/弱网切到手动扫描。
- 重新设计 session budget、每日 quota、免费次数、成本告警和玩家提示。

**验收/测试**

- 典型 5 分钟流程不会耗尽全天额度。
- 记录每次成功捕获的平均 detect 次数、带宽、P95 延迟和 Provider 成本。
- 模拟 1k/10k 并发玩家，成本和吞吐不超过预算。

### AP-023 [P1][Distributed Systems] 接入 Redis 限流、配额和 nonce replay 防护

**问题**：`REDIS_URL` 被加载但未使用；Router 永远使用进程内 Counter。生产 3+ Pod 时配额各算各的。Security nonce 使用无锁、无 TTL 的本地 map。

**实现范围**

- 用 Redis Lua 实现原子 token bucket/sliding window、daily quota、nonce `SET NX EX`。
- key 必须有 TTL；小数 rate 不得被截断成错误固定窗口。
- 按 device、account、IP、inference digest 多维限流；明确 Redis 故障时各接口 fail-open/closed。
- 删除无界 map；记录限流和 replay 指标。

**验收/测试**

- 3 Pod 下总限额一致；同 nonce 跨 Pod/重启仅首个成功。
- `go test -race` 并发安全；Redis 故障行为与文档一致。
- 1 万随机 key 后内存/Redis key 数量按 TTL 回落。

### AP-024 [P1][Sync API] 统一单条/批量校验并禁止内部错误泄露

**问题**：batch 元素没有执行 Gin binding 约束，`syncOne` 与单条逻辑不一致；可提交非法 species/rarity/stats/坐标，batch 还可能返回原始 DB error。

**实现范围**

- 提取唯一 `validateAndSyncOne` 领域服务，单条和批量复用。
- 严格验证 UUID、物种、rarity、stats、class、element、坐标、时间范围、字符串长度、inference。
- 明确 batch 是原子事务还是逐项结果；禁止在 context 中临时塞 item。
- 统一错误 reason code，不返回数据库原始错误。

**验收/测试**

- 1/100/101 items、批内重复、部分失败、零坐标、未来时间、错 inference、SQL 特殊字符全部有稳定结果。
- 单条与 batch 对同一 item 的行为完全一致。

### AP-025 [P1][Sync][Client] 修复 pull cursor、删除 tombstone 和启动时双向对账

**问题**：前端只有 push queue，没有调用 pull；后端实现与 OpenAPI 参数不一致，固定 limit 50，空页 next_version 可能回到 0，`UnixNano` 不是可靠游标。

**实现范围**

- 使用数据库单调 sequence/ID cursor，支持 `limit`、`has_more`、`next_cursor`。
- 定义 tombstone，支持服务端删除同步到客户端。
- 前端启动/登录/恢复网络时先 pull 分页，再合并本地 pending，最后 push。
- 冲突以服务端版本和 pending operation 规则解决，并给用户可见状态。

**验收/测试**

- 51/200/1000 条分页不漏不重；空页保持当前 cursor。
- 删除记录不会携带原内容重新出现。
- 新设备或清空本地缓存后可以恢复同一账号的收藏。

### AP-026 [P1][Pokedex] 使用真实收藏数据并建立迁移、空态和详情页

**问题**：新用户默认展示三只假收藏；IDB 失败被静默吞掉；DB rarity 数字被直接 cast 成 UI 字符串；捕获结果不会进入图鉴。

**实现范围**

- 生产空库显示新手空态，Demo fixture 只能通过显式开发开关加载。
- 建立后端/IDB 到统一 `AnimalViewModel` 的映射和历史数据迁移。
- 增加 loading/error/retry/empty/new 状态、分页/虚拟列表、搜索过滤和详情页。
- 详情展示照片缩略图、粗粒度地点、日期、属性、生成依据和同步状态。

**验收/测试**

- 捕获完成立即出现，刷新后保持；不存在默认假收藏和默认鹅迁移。
- 空库、损坏记录、旧 rarity、IDB 不可用、1000+ 动物性能测试通过。

### AP-027 [P1][Battle] 将生产战斗页接回 BattleContext 与权威结算

**问题**：当前页面固定“我的猫 vs 野生狗”，只在本地扣血；HP 为 0 后仍可点击，日志声称发金币但资源不变化。

**实现范围**

- 使用真实已收藏宠物进行选宠、匹配、回合、终局、奖励和战绩。
- 终局只结算一次；页面切换/刷新可恢复进行中的战斗或安全终止。
- PvE 可先本地确定性模拟并由服务端签名结算；PvP 未完成前隐藏或 501。
- 天气、状态、元素、体力和道具使用同一配置来源。

**验收/测试**

- 只有已收藏宠物可出战；零 HP 后操作锁定。
- 金币、经验、掉落和战绩真实更新且不可重复领取。
- 胜/负/平、重复点击、掉线、恢复、天气/状态修正全覆盖。

### AP-028 [P1][Progression] 接通等级、成就、派遣、状态与统一不变量

**问题**：Provider 已挂载，但生产 UI 大量忽略其状态；成就按钮只显示“暂未开放”。资源逻辑依赖本地时间和随机数，多 Tab/改时钟可破坏一致性。

**实现范围**

- 定义服务端或签名权威不变量：体力、金币、道具、签到、派遣、战斗奖励、捕获均恰好一次。
- 注入受控 clock/RNG；使用可信服务器时间处理签到、恢复和派遣。
- 把已实现的等级/成就/派遣/状态逐步接入生产导航，未完成模块用 feature flag 隐藏。
- 多 Tab 使用 BroadcastChannel/锁协调写入。

**验收/测试**

- 任意操作序列中资源不为负、不凭空增加。
- 调系统时间不能加速恢复、签到或派遣。
- 两 Tab、崩溃重启、离线重放后最终与服务端一致。

### AP-029 [P1][Mobile Layout] 建立固定栏 + 可滚动内容 + Safe Area 布局

**问题**：`.ap-phone` 使用 `overflow:hidden`，长页面没有明确滚动容器；底栏、刘海、Home Indicator、横屏和动态字体存在遮挡风险。

**实现范围**

- 使用三段布局：固定/粘性顶栏、`overflow-y:auto` 内容区、底部导航。
- 统一使用 `env(safe-area-inset-*)`，避免后续 shorthand padding 覆盖。
- 处理 320x568 到 430x932、平板、横屏、软键盘和 200% 字体。

**验收/测试**

- 所有页面最后一个控件可访问，底栏不遮内容。
- 320x568、390x667、430x932、横屏和桌面视觉回归通过。
- 真机截图确认后才能关闭 Issue。

### AP-030 [P1][Accessibility] 完成 WCAG 2.2 AA 与真机辅助技术验证

**问题**：仓库只有很短的 a11y 基线；CSS 定义 skip link 但 DOM 未使用；无限动画无 reduced-motion；战斗 HP 和策略状态语义不足。

**实现范围**

- 增加 `main`、真实 skip link、dialog focus trap、页面切换焦点和标题更新。
- 将 HP/概率/进度改为 progressbar；策略/过滤用 tab/pressed 等正确语义。
- 尊重 `prefers-reduced-motion`，提供动画、震动、音效设置。
- 修正文字/控件对比度、Toast 停留时间和 44x44 触控目标。

**验收/测试**

- axe critical/serious 为 0；200% 缩放不丢内容。
- 只用键盘可完成同意、扫描、捕获、图鉴、购买和战斗。
- VoiceOver/TalkBack 能播报识别、体力扣除、错误和结果。

### AP-031 [P1][Navigation] 修复 hash、历史、深链和流程守卫

**问题**：普通导航使用 `replaceState`，开始捕获和地图返回不更新 hash；刷新或后退行为不一致，`#capture` 可绕过识别。

**实现范围**

- 引入集中式 router/navigation reducer，定义每个屏幕的合法前置状态。
- 用户主动导航使用 push，状态修正使用 replace；地图返回和捕获完成必须更新 URL。
- 流程 state 过期时跳回最近合法页面，并说明原因。
- 页面切换移动焦点，保留必要的滚动/表单状态。

**验收/测试**

- 前进、后退、刷新、深链、会话过期行为可预测。
- 不能通过 URL 跳过 consent、detect、范围或体力检查。

### AP-032 [P1][Architecture] 合并重复 UI、Pipeline、Queue 和数据模型

**问题**：仓库存在 `src/components/*` 与 `features/animal-poke/*` 两套屏幕，以及两套 capture pipeline、两套 sync queue。大量强测试覆盖不可达旧代码。

**实现范围**

- 生成 import graph，选择唯一生产实现；先迁移有效能力和测试，再删除重复版本。
- 统一 Species、Item、AnimalRecord、CaptureSession、API Result 类型。
- 引入 knip/dependency-cruiser 或等价门禁，禁止新增不可达业务模块。
- 测试必须优先从 App/路由入口触发。

**验收/测试**

- 每个业务屏、pipeline、queue 只有一个生产实现。
- 无未引用模块；覆盖率不再被 dead code 稀释或虚高。
- 删除旧代码前测试等价迁移完成。

### AP-033 [P1][OpenAPI] 建立 Gin 路由与 OpenAPI 双向契约门禁

**问题**：Runtime 有 errors/ranking/PvP/social/ops 七条未入 spec；sync 实现 201 而 spec 声明 200；大量 schema 是 `additionalProperties:true`，前端 path 仍是任意 string。

**实现范围**

- 补齐全部 runtime route、method、状态码、header 和严格 schema。
- 自动提取 Gin route table，与 spec 双向比较。
- Handler 集成测试使用 OpenAPI request/response validator。
- 生成 operation client，业务代码不能传任意 path 字符串。
- 生成物缺失或过期必须使 CI 失败，不能自动复制后继续通过。

**验收/测试**

- 缺路由、错状态、错 schema、错 Content-Type、漏生成物均阻断 CI。
- 所有前端 API 调用使用生成 request/response 类型。

### AP-034 [P1][API] 统一错误 Envelope、JSON 严格解码和 Body Limit

**问题**：多数接口错误只有 `error`，缺 reason/request ID；JSON API 普遍没有 body limit，Security 在完整解码后才检查大小；unknown fields 被忽略。

**实现范围**

- 统一错误结构：`error`、`reason_code`、`request_id`、`retryable`、`details?`。
- 全局或逐路由 MaxBytesReader；按 sync batch、receipt、error stack 设置不同上限。
- JSON decoder 拒绝 unknown fields、重复字段、尾随 JSON。
- 建立统一 validation error 映射，不泄露内部 DB/provider 错误。

**验收/测试**

- 400/401/403/404/409/413/415/429/5xx 全部符合 schema。
- 超大/畸形 JSON、重复 key、尾随 payload、header 注入不会 panic 或产生非预期 5xx。

### AP-035 [P1][Observability] 接通真实指标、Trace、SLO 和告警

**问题**：`ObserveAICost`、`ObserveRateLimit`、`ObserveCache` 没有调用点；前端 monitoring 是 mock 类型；无 dashboard/SLO/告警。

**实现范围**

- 接入 Prometheus/OpenTelemetry，覆盖 HTTP RED、DB pool、AI provider/model、成本、置信度、空结果、同步队列年龄和游戏漏斗。
- request ID、capture session、inference ID、release SHA 可关联，但不得包含照片/精确坐标。
- 建立核心循环成功率、识别 P95、同步延迟、错误率和成本 SLO。
- 为 DB down、Provider 5xx、识别空结果激增、队列积压建立告警和 runbook。

**验收/测试**

- 故障注入后 1 分钟内可在 dashboard 定位具体 provider/model/release。
- 告警恢复自动关闭；runbook 能从 request ID 追踪到失败阶段。

### AP-036 [P1][Metrics Security] 将 `/metrics` 移出公网并消除高基数 DoS

**问题**：`/metrics` 公开注册且 Ingress 暴露 `/`；指标以原始 path 为基础，只折叠数字/UUID，随机 404 path 会无限增长 `sync.Map`。

**实现范围**

- 指标使用独立 management port/ClusterIP 或强鉴权，不通过公共 Ingress。
- 使用 `c.FullPath()` 或固定 `unknown`，限制 label 集和 series 数量。
- 迁移到标准 Prometheus collector，避免永久 sync.Map。

**验收/测试**

- 公网 metrics 403/404，Prometheus 仍能抓取。
- 请求 1 万随机 path 后 series 和内存保持有界；并发 scrape/race 测试通过。

### AP-037 [P1][Privacy][Telemetry] 对齐客户端错误上报 Schema 与私有 Source Map

**问题**：前端发送字段和后端 binding 不一致，额外字段被丢弃；后端只写日志，无聚合；生产 sourcemap 关闭，release 不是 commit SHA。

**实现范围**

- 共享错误 schema，包含 release SHA、route、component、request ID、非敏感上下文。
- 上传私有 source map 到错误平台，不公开随包发布。
- 做去重、采样、离线队列、PII/Token/坐标脱敏和保留策略。

**验收/测试**

- 构造生产异常可按 release/route/request ID 还原源码行。
- 日志和平台中不存在 Token、照片、精确坐标；重复上报幂等。

### AP-038 [P1][Database] 提供生产迁移 Job、MySQL 集成测试与约束

**问题**：production `AUTO_MIGRATE=false`，仓库没有实际迁移 Job/CLI；migrate 覆盖率 0。当前集成测试 MySQL 不可达时会 skip，数据库也缺少多项游戏数据约束。

**实现范围**

- 增加 `backend migrate up/status` 和 K8s pre-deploy Job，包含锁、超时、备份检查和幂等。
- CI 使用 MySQL 8 service，连接失败必须 fail，不得 skip。
- 为 species、rarity、stats、金额、状态、inference/idempotency 增加 CHECK/FK/复合唯一约束。
- 破坏性变更使用 expand/contract。

**验收/测试**

- 空库可升级到 CurrentVersion，重复执行无副作用，中断可安全重试。
- 绕过 API 写非法物种、rarity=99、负金额、重复凭证由 DB 拒绝。

### AP-039 [P1][Release] 建立 staging -> canary -> production 与自动回滚

**问题**：仓库只有 CI，没有完整发布 workflow；后端 container smoke 对 readyz 使用 `|| true`，不验证真实就绪；无前端/K8s 在线 smoke。

**实现范围**

- 构建、签名、推送前后端镜像并注入 digest；执行迁移、部署 staging、契约/E2E/DAST/性能 smoke。
- 人工批准后 canary，按错误率/延迟/核心漏斗自动回滚。
- 发布记录包含 git SHA、镜像 digest、DB schema、配置版本和回滚命令。

**验收/测试**

- readyz 非 200 或主循环失败必须终止。
- canary 故障能自动回滚；一键回滚演练通过，旧/新 schema 兼容。

### AP-040 [P1][PWA][Mobile QA] 建立浏览器/真机矩阵和受控升级

**问题**：没有 Playwright WebKit、BrowserStack/真机矩阵；SW autoUpdate 可能在捕获中更新；缺少跨版本 IDB/SW 测试。

**实现范围**

- 覆盖 Chromium/WebKit、主流 Android Chrome、iOS Safari/PWA。
- 测试 camera/geolocation allow/deny/revoke、后台/锁屏、横竖屏、弱网/offline、安装/启动/更新。
- 捕获进行中延迟更新；schema 不兼容时提示受控刷新。
- 验证 SW 不缓存 auth/vision/value/sync/privacy 等敏感 API。

**验收/测试**

- 支持矩阵内主循环可完成；更新不白屏、不丢 IDB、不跨身份缓存。
- 旧 SW + 新 API、旧 DB + 新前端均有迁移路径。

### AP-041 [P1][Build][Supply Chain] 恢复可重复前端构建并增加供应链门禁

**问题**：当前工作区缺少已声明的 `@types/node`，本机 Node 26 超出 engine；CI 动态下载 `latest` 工具，缺少 npm/image/IaC/SBOM/签名门禁。

**实现范围**

- 统一 npm 或 pnpm 单一包管理器，锁定 Node 22、包管理器版本和 Corepack。
- CI 使用 clean install，验证 lockfile；修复本地依赖完整性但不盲目升级核心依赖。
- 加 OSV/npm audit、SAST、image/IaC scan、SBOM、Cosign provenance；工具固定版本或 SHA。

**验收/测试**

- 全新 checkout 在 Node 22 可重复 `install -> test -> build`。
- 高危可利用漏洞、未签名镜像、危险 K8s 或 lock drift 阻断发布。

### AP-042 [P1][Feature Flags] 占位 Ranking/PvP/Social/Ops API 不得返回假成功

**问题**：这些接口是骨架，但返回 2xx、空 match ID、空榜单或 pending share，客户端可能误认为成功；ops metrics 对普通设备开放。

**实现范围**

- 未完成能力使用 feature flag，并返回 501/503 `feature_unavailable`；前端隐藏入口。
- PvP 实现 match 所有权、状态机、重放、ELO 原子更新前不得上线。
- share 使用不可猜 ID、过期和访问控制；ops 仅管理员/内部访问。

**验收/测试**

- 关闭 flag 时前后端均不展示假成功。
- 普通设备不能访问 ops；空 match ID 永不作为成功结果。

---

## P2 · 体验优化、留存、运营和长期质量

### AP-043 [P2][Onboarding] 设计首个 10 分钟的保证成功核心循环

**目标**：让首次玩家理解权限、扫描质量、识别、投掷、属性揭晓和图鉴价值，而不是面对空相机和未知按钮。

**实现范围**

- 权限前先解释用途和可拒绝后果，再触发系统弹窗。
- 使用一个保证可完成的教学 encounter；引导距离、光线、构图和目标框。
- 首次投掷提供慢动作/轨迹和容错；首次结果展示“为什么是这个物种/稀有度”。
- 教学可跳过，可在设置重播；中途退出可续接。

**验收/测试**

- 新用户无需外部说明能完成首次收藏。
- 覆盖同意、拒绝、弱网、无动物、退出续接和回流用户。

### AP-044 [P2][Animal Welfare] 移除现实投喂和追逐暗示

**问题**：猫粮、面包、骨头投掷，尤其给鹅投面包，可能鼓励不健康投喂和惊扰野生动物。

**实现范围**

- 将投掷物改为纯虚拟“友好光点、贴纸、镜头信号或观察徽章”。
- 文案明确“不追逐、不触摸、不投喂、不惊扰”。
- 对受保护、危险、受伤或幼年动物只允许观察/上报，不提供接近式玩法。
- 增加动物福利顾问审核和地区化内容配置。

**验收/测试**

- 所有页面、道具和教学不再鼓励现实投喂。
- 安全提示不会被动画/奖励遮挡，可被屏幕阅读器读出。

### AP-045 [P2][Outdoor Safety] 增加道路、水边、夜间、极端天气与低精度保护

**实现范围**

- 服务端生成点时排除道路中心、水体、施工区、私人区域和不可达地块。
- 展示定位精度圈；精度超阈值时不允许判定 in-range。
- 极端天气、低电量、深夜和移动速度过高时暂停户外捕获。
- 捕获页增加“先停下再操作”，避免边走边看屏幕。

**验收/测试**

- 不在危险区域生成引导点；车辆速度状态不能捕获。
- GPS 漂移、跳点、跨城市边界、夜间和极端天气测试通过。

### AP-046 [P2][Recognition UX] 增加“识别错了”纠正、申诉和再扫描

**实现范围**

- 结果页提供“不是这个动物”“物种对但品种错”“框选错目标”。
- 纠正不会直接改变高价值奖励；进入安全的复核/再扫描流程。
- 保存模型、prompt、置信度、匿名样本授权状态和纠正标签。
- 给玩家解释可执行改进：靠近、稳定、增加光线、换角度。

**验收/测试**

- 玩家可在 2 步内纠正或重扫，不会被默认鹅锁死。
- 未授权样本不进入训练集；纠正事件可追踪且不泄露照片。

### AP-047 [P2][ML QA] 建立真实动物识别黄金集和模型认证流水线

**实现范围**

- 使用有授权的猫/狗/鹅以及鸭、天鹅、鸟、人、空景负样本。
- 覆盖暗光、逆光、模糊、遮挡、远景、多动物、旋转、压缩和屏幕翻拍。
- 保存期望 species/bbox；PR 跑 stub contract，nightly/staging 跑真实 Provider。
- 报告 per-class precision/recall、unknown rejection、IoU、校准误差、P50/P95/P99、成本。

**验收/测试**

- 模型或 Prompt 变更生成基线 diff；超过退化阈值阻断发布。
- 失败样本可回溯到模型和 Prompt 版本。

### AP-048 [P2][Game Balance][Fairness] 将稀有度和属性改为确定性可解释算法

**问题**：当前 LLM 直接生成 rarity/stats，重试可重抽，难以公平审计；Prompt 声称使用 random seed，但请求没有稳定 seed。

**实现范围**

- 服务端用版本化规则计算 rarity/stats，seed 来自 capture/inference ID 的 HMAC。
- LLM 只生成叙事，或只能在严格边界内补充非核心文本。
- 结果页展示影响因素：拍摄质量、目标完整度、已知物种规则、有限随机项。
- 同一幂等请求永远返回同一结果；配置版本写入动物记录。

**验收/测试**

- 重试、换设备、服务重启不会改变同一 capture 的属性。
- 运行百万次分布测试，稀有度和战斗强度符合目标区间。

### AP-049 [P2][Collection] 为重复捕获设计价值并保护首捕兴奋点

**实现范围**

- 首次发现解锁图鉴；重复个体转化为亲密度、研究点、装饰碎片或任务进度。
- 保留个体差异但限制库存膨胀，提供对比、收藏锁和安全分解确认。
- 新用户图鉴不预填假收藏，首捕有独立揭晓和定位。

**验收/测试**

- 重复捕获既不等于垃圾，也不能无限制造经济价值。
- 模拟 30 天玩家库存、资源产出和清理流程。

### AP-050 [P2][Progression] 建立 D1/D7/D30 目标、解锁节奏与回流路径

**实现范围**

- D1：首捕、图鉴、一次简单任务；D7：三物种、基础战斗/派遣；D30：区域/赛季长期目标。
- 每日/每周任务连接发现、收藏、战斗、派遣和安全探索。
- 功能按等级/行为解锁，未开放入口隐藏而非长期 Toast。
- 回流玩家获得进度摘要和一条明确的下一步，不制造过度 FOMO。

**验收/测试**

- 首日始终有 3 个可执行目标；体力耗尽后仍有非付费活动。
- 模拟 D1/D7/D30、断签、回流和新功能解锁。

### AP-051 [P2][Economy QA] 建立经济蒙特卡洛与资源不变量门禁

**实现范围**

- 建模金币来源/消耗、体力、道具、签到、派遣、战斗、重复捕获和商业化。
- 注入确定性 clock/RNG，运行不同玩家类型的 30/90 天模拟。
- 定义通胀、卡死、付费压力、资源负数、无限循环和囤积阈值。
- 每次数值配置变更自动生成前后分布差异。

**验收/测试**

- 不存在无成本无限产币、无限领奖或资源负数路径。
- 新手、轻度、核心、回流玩家均能在目标节奏推进。

### AP-052 [P2][Feedback] 增加可关闭的音效、触觉和稀有揭晓反馈

**实现范围**

- 为扫描锁定、蓄力、命中、失败、稀有揭晓、购买、升级定义独立反馈。
- Audio/Vibration 必须在用户手势后初始化；尊重系统静音、reduced-motion 和设置。
- 状态和反馈使用同一事件源，禁止成功音效配失败结果。

**验收/测试**

- 不支持 Audio/Vibration 时无异常；后台/锁屏停止播放。
- 设置关闭后刷新仍保持，辅助技术不依赖声音才能理解结果。

### AP-053 [P2][I18n][Settings] 接通国际化和统一设置中心

**问题**：I18nProvider 已挂载，但生产文案大量硬编码；声音、动画、隐私、语言等设置没有统一入口。

**实现范围**

- 所有生产文案使用 message key；支持中文和英文，预留日文。
- 设置中心包含语言、音效、音乐、触觉、动效、流量模式、权限和数据导出/删除。
- 设置本地持久化，并在账号绑定后可安全同步非敏感项。

**验收/测试**

- 切换英文后六个生产页面无遗漏硬编码。
- 最长英文、缺失 key、RTL 预检查、刷新和跨设备设置测试通过。

### AP-054 [P2][Performance][Battery] 增加低流量、低电量和弱机模式

**实现范围**

- 根据网络、Save-Data、电量、温度/掉帧信号切换连续扫描与手动扫描。
- 客户端在上传前缩放、压缩、去重；后台立即暂停相机和定时任务。
- 图鉴虚拟列表、图片缩略图和按需加载；设置性能预算。

**验收/测试**

- 单次 5 分钟流程满足目标流量、耗电和帧率预算。
- 中低端 Android、iPhone A12、弱网和低电量真机测试通过。

### AP-055 [P2][Account] 增加账号绑定、跨设备恢复和设备迁移

**问题**：当前身份仅依赖 localStorage device ID，清除数据后无法恢复，复制 ID 又可能冒领。

**实现范围**

- 保留游客模式，但提供手机号/邮箱/Apple/Google 等可选绑定。
- 设计游客合并、设备迁移、冲突解决、退出登录和设备撤销。
- 本地 Token 使用更安全存储；服务端收藏归属 account，不只归属 device。

**验收/测试**

- 清除浏览器数据或换设备后可恢复收藏和进度。
- 游客合并不会重复发奖；丢失设备可吊销。

### AP-056 [P2][Safety][Moderation] 增加人像、儿童、不当内容和滥用防护

**实现范围**

- 上传前检测/遮挡人脸、车牌和明显住宅信息；禁止把人物照片作为动物收藏。
- 对不当图片、虐待动物、受伤动物建立安全处理和举报路径。
- 未成年人账号使用更严格的时间、位置和社交默认值。
- 明确 Provider 不训练/保留策略，并保存审计证明。

**验收/测试**

- 纯人像、儿童、人+动物、车牌、住宅和不当内容 fixture 有稳定安全结果。
- 安全拒绝不会泄露模型细节或保留原图。

### AP-057 [P2][Analytics] 建立隐私友好的核心漏斗与实验平台

**实现范围**

- 事件：授权、相机成功、扫描、检测结果、捕获 attempt、生成阶段、收藏完成、交易、战斗终局。
- 指标：识别成功/未知率、P95、每捕获调用数、各阶段流失、重复点击、同步失败、D1/D7。
- 使用匿名/伪匿名 ID、粗粒度位置、schema registry、采样和离线队列。
- A/B 实验必须定义停止条件、样本量和动物福利/安全 guardrail。

**验收/测试**

- 事件中无照片、Token、精确坐标；同一 session 可用非敏感 ID 串联。
- schema 漂移、重复上报、离线恢复和 consent revoke 测试通过。

### AP-058 [P2][Performance QA] 将业务负载与前端性能预算变成门禁

**问题**：现有 k6 默认不测 Vision，某些 400/503 也可算通过；前端只生成 bundle 报告，没有预算判断。

**实现范围**

- k6 分 auth/detect/analyze/value/sync，使用固定账号池和预置 consent。
- 定义业务成功率、P95/P99、吞吐、并发、成本；加入 spike/soak/Provider 限流。
- LHCI/等价工具检查 LCP、INP、CLS、JS gzip、图片和 SW 缓存体积。
- 基准结果归档并与上一版本比较。

**验收/测试**

- 非预期 4xx/5xx 不算成功；性能或 bundle 退化超过阈值阻断。
- HPA、DB pool、Redis 和 Provider 并发在目标 DAU 模型下稳定。

### AP-059 [P2][Live Ops] 建立版本化游戏配置、Feature Flag 和回滚

**实现范围**

- 统一体力、价格、掉率、阈值、天气修正、物种参数和功能开关。
- 配置有 schema、版本、审核、灰度、签名、缓存和回滚；客户端只展示权威值。
- 高风险配置设置硬边界，不能把概率、价格或消耗改到非法范围。

**验收/测试**

- 同一数值不再分散在设计文档、旧组件、feature 数据和 Context 常量。
- 配置回滚不需要重新发版，旧客户端有兼容默认值。

### AP-060 [P2][Docs][Traceability] 清理历史记录并建立需求到证据追踪

**问题**：任务清单仍包含大量已退役 Godot 完成记录，容易让执行者把历史测试当成 React 当前状态；设计文档内部也有 1 与 20 体力等冲突。

**实现范围**

- 将 Godot 历史迁入 archive；当前 backlog 只引用 React/Go 交付物。
- 建立“设计要求 -> 代码 -> 自动测试 -> 运行指标 -> Owner”追踪表。
- 统一冲突数值和术语；文档链接、路径、状态由 CI 检查。

**验收/测试**

- 不再存在指向缺失交付物却标记完成的当前任务。
- 每个 P0 玩法、隐私、性能要求都有可运行验证命令或 dashboard。

---

## 完成定义

任何 Issue 只有同时满足以下条件才可关闭：

- 生产入口或真实 runtime 路径已验证，而不只是孤立单元。
- 新增/更新了正向、异常、边界和回归测试。
- API 契约、错误语义、日志和指标已同步。
- 不引入新的默认 Mock、静态假数据或“返回 2xx 但功能未完成”的行为。
- 不泄露照片、精确坐标、Token、Key 或其他敏感信息。
- 给出回滚方案，并确认不会破坏已有本地存档或数据库数据。

