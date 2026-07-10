# Animal Poke · 全 API 测试矩阵

> 覆盖范围：`backend/internal/routes/router.go` 当前注册的 35 个唯一 Endpoint。  
> 目标：每个 Endpoint 均覆盖正向、参数、鉴权/归属、异常/恢复、并发/幂等和 OpenAPI 契约。

## 1. 测试分层

### 1.1 PR 必跑

- Go handler/router tests：SQLite 或 in-memory 依赖，只测确定性逻辑。
- OpenAPI request/response validation：每个用例的 status、header、body 均验证。
- Fuzz：JSON decoder、图片头、cursor、时间、provider response parser。
- 前端 contract tests：使用生成 operation client，不允许任意 path 字符串。

### 1.2 CI 必跑

- MySQL 8 service container + 真实迁移。
- Redis service + 多实例共享限流/nonce 测试。
- AI/Geo/Weather HTTP stub，覆盖超时、429、5xx、畸形响应和取消。
- Playwright 启动 frontend/backend/MySQL/Redis/stub，跑核心 E2E。

### 1.3 Staging/Nightly

- 真实 Vision/LLM Provider 黄金集认证，不使用真实用户照片。
- OWASP ZAP/DAST、k6 spike/soak、备份恢复和 canary smoke。

## 2. 全局通用断言

所有 JSON API 都要满足：

- 成功和失败均回传 `X-Request-ID`；错误体包含 `error`、`reason_code`、`request_id`。
- 401 与 403 语义稳定；跨设备资源不得泄露是否存在。
- 不返回数据库原始错误、Provider 完整响应、Token、Key、照片或精确坐标。
- JSON 拒绝 unknown field、重复 field、尾随 JSON 和超大 body。
- 非幂等写请求默认不自动重试；带 `Idempotency-Key` 时按服务端记录重放。
- CORS 只允许白名单 Origin；预检 method/header 与实际路由一致。
- OPTIONS、HEAD、错误 method 的 204/405 行为写入 OpenAPI 或统一约定。
- 429 的 `Retry-After`、`X-RateLimit-*` 可解析且有上限。
- 所有 DB 写操作检查 RowsAffected，并覆盖事务失败与并发。

## 3. Endpoint 矩阵

| ID | Endpoint | 当前契约状态 | 必测正向 | 必测异常/边界/安全 |
|---|---|---|---|---|
| API-T01 | `GET /health` | 已入 OpenAPI | 200、固定 schema、快速响应 | 错 method；不得暴露 DB/Provider/Secret；并发 1k 请求 |
| API-T02 | `GET /livez` | 已入 OpenAPI | 200、进程存活 | 即使 DB down 仍只表达 liveness；无高开销依赖 |
| API-T03 | `GET /ready` | 已入 OpenAPI | DB/schema/核心依赖 ready 返回 200 | DB down、schema mismatch、Redis/Provider 配置缺失返回 503；恢复后转 200；无 Secret |
| API-T04 | `GET /readyz` | 已入 OpenAPI | 同 API-T03 | legacy `/ready` 与 `/readyz` 响应一致；迁移失败不得假 ready |
| API-T05 | `GET /metrics` | 已入 OpenAPI但当前公网 | Prometheus 格式；真实请求后计数增长 | 公网拒绝；1 万随机 path 不增加无界 series；并发 scrape；无设备/坐标标签 |
| API-T06 | `GET /api/v1/ping` | 已入 OpenAPI | 200、request ID、预期环境信息 | production 不暴露内部 app_env/DB 拓扑；DB nil 语义；错误 method |
| API-T07 | `GET /api/v1/time` | 已入 OpenAPI | RFC3339/Unix 时间、`X-Server-Time`、签名测试向量 | 时钟回拨、签名篡改、重复请求、缺/弱 secret、跨时区；前端校验错误 |
| API-T08 | `POST /api/v1/auth/device` | 已入 OpenAPI | 首次注册、续签、并发同设备 | UUID/8/64 边界、非法字符、超大 body、禁用设备、DB error 503、暴力注册、伪造 IP、仅知 device ID 冒领、JWT claims 完整性 |
| API-T09 | `GET /api/v1/geo/city` | 已入 OpenAPI | 合法边界坐标、真实/缓存响应 | 缺参数、NaN/Inf、越界、JWT、location consent、Provider timeout/429/5xx/malformed、取消、日志无 Key |
| API-T10 | `GET /api/v1/weather/week` | 已入 OpenAPI | JWT、恰好 7 天、日期/温度/枚举完整、cache | 坐标错误、consent、Provider 异常；随机降级仅开发；缓存一致；Token 续签；前端不静默吞 401 |
| API-T11 | `POST /api/v1/errors/report` | **缺 OpenAPI** | 202、有效 message/stack/release | 16 KiB 边界、空 message、畸形 JSON、Token/Key/坐标脱敏、重复上报、离线重放、限流 |
| API-T12 | `GET /api/v1/ranking/daily` | **缺 OpenAPI；占位** | feature 完成后真实城市榜/结算日 | 未完成时 501；城市长度/字符、分页、时区边界、普通/作弊设备、缓存、限流 |
| API-T13 | `POST /api/v1/pvp/match` | **缺 OpenAPI；占位** | feature 完成后返回非空 match ID 和对手 | 未完成时 501；请求 schema、队列超时、重复匹配、跨城市/段位、并发、取消 |
| API-T14 | `POST /api/v1/pvp/result` | **缺 OpenAPI；占位** | 合法 match 状态机和 ELO 原子更新 | match 所有权、重复结果、篡改战斗日志、超时/已结束 match、并发双提交、ELO 下限 |
| API-T15 | `GET /api/v1/social/friends` | **缺 OpenAPI；占位** | feature 完成后分页好友列表 | 未完成时 501；隐私设置、跨设备、分页、封禁关系、DB error 不得假空列表 |
| API-T16 | `POST /api/v1/social/share` | **缺 OpenAPI；占位** | 合法动物生成不可猜 share ID/过期时间 | 未完成时 501；动物归属、输入长度、内容安全、幂等、过期/撤销、ID 枚举攻击 |
| API-T17 | `GET /api/v1/ops/metrics-summary` | **缺 OpenAPI；权限错误** | 管理员/内部返回真实聚合 | 普通设备拒绝；时间范围、空数据、DB error、敏感维度、缓存、审计 |
| API-T18 | `POST /api/v1/vision/detect` | 已入 OpenAPI | JPEG/PNG/WebP；猫/狗/鹅；无动物；多动物；真实 inference | JWT、photo consent、空文件、字段名错、multipart overhead、5 MiB/像素边界、伪 WebP、图片炸弹、未知物种、Provider 全错误、幂等、取消、并发、配额 |
| API-T19 | `POST /api/v1/vision/analyze` | 已入 OpenAPI但 schema 过宽 | 引用 detect target，1-10 分值、真实 inference | 错/跨设备/过期 detect ID、多动物 target mismatch、非法模型 JSON、超长字符串、provenance 写库失败、幂等 |
| API-T20 | `POST /api/v1/value/generate` | 已入 OpenAPI但 schema 过宽 | 引用 analyze，确定性 stats/叙事、真实 inference | species enum、分值边界、错 kind/跨设备、模型非法输出、重抽攻击、内容安全、幂等、Provider 429/5xx |
| API-T21 | `POST /api/v1/sync/animal` | 已入 OpenAPI；状态码漂移 | 完整 detect->analyze->value->sync，201，pull 可见 | 字段范围、错/跨设备/过期 inference、篡改属性、100 并发消费、重复 UUID、丢响应重试、DB rollback、位置最小化 |
| API-T22 | `POST /api/v1/sync/animals` | 已入 OpenAPI；schema 过宽 | 1、100 items，明确逐项/原子语义 | 0/101、嵌套字段验证、批内重复、部分失败、单条逻辑一致、无原始 DB error、事务与性能 |
| API-T23 | `GET /api/v1/sync/animals` | 已入 OpenAPI但参数漂移 | 0/1/50/51/200/1000 分页，next cursor | 非法/负数/溢出 cursor、limit、空页 cursor 不倒退、并发版本、tombstone、删除数据不回流、跨设备隔离 |
| API-T24 | `POST /api/v1/privacy/consent` | 已入 OpenAPI但 schema 过宽 | 当前版本、各 scope 授权、重复幂等 | 非法版本/scope、隐式全授权禁止、撤回、并发更新、DB error、受保护 API 立即生效 |
| API-T25 | `POST /api/v1/privacy/export` | 已入 OpenAPI | 0/200/201+ 动物和全部数据资产 | 所有权、分页、部分 DB 失败、异步状态、加密、下载过期、payload 保留、限流、并发请求 |
| API-T26 | `POST /api/v1/privacy/delete` | 已入 OpenAPI | 全范围删除/tombstone/Token 吊销 | 事务中断、重试、订单法定保留、历史 export、精确位置、删除后 pull、失败状态码不能假 200 |
| API-T27 | `GET /api/v1/privacy/requests/{id}` | 已入 OpenAPI | own request 各状态 | other device、非法/不存在 ID、payload 访问控制、状态转换、删除后访问、枚举攻击 |
| API-T28 | `POST /api/v1/security/report` | 已入 OpenAPI但 schema 过宽 | 合法 nonce/risk payload、审计记录 | body limit、同 nonce、跨 Pod/重启 replay、100 并发、DB/audit error、空/伪造 payload、Redis down、race/panic |
| API-T29 | `POST /api/v1/commerce/orders` | 已入 OpenAPI但生产不安全 | feature 开启后白名单商品、正确金额、幂等 | 未完成时 501；未知商品、第二个未履约订单、跨设备同 key、平台 enum、并发、DB unique、金额篡改 |
| API-T30 | `POST /api/v1/commerce/orders/fulfill` | 已入 OpenAPI但生产不安全 | Apple/Google 正常验签、首次/续期 | receipt 大小、伪造/沙盒到生产、跨平台/跨用户、重复 receipt、并发不同 receipt、已退款、Webhook 重放 |
| API-T31 | `POST /api/v1/commerce/orders/refund` | 已入 OpenAPI但调用方不可信 | 平台 Webhook/管理员合法退款 | 普通设备拒绝、重复退款、未履约订单、跨设备、并发、权益回收、审计、平台签名 |
| API-T32 | `GET /api/v1/commerce/orders/{id}` | 已入 OpenAPI | own created/fulfilled/refunded order | other device、非法/不存在 ID、敏感字段隐藏、DB error、枚举攻击 |
| API-T33 | `GET /api/v1/commerce/entitlements` | 已入 OpenAPI | active/expired/续期/退款结果 | DB error 不得假空；跨设备、分页、过期边界、并发续期、敏感字段 |
| API-T34 | `GET /api/v1/admin/audit/logs` | 已入 OpenAPI | 正确 Admin Key、过滤/分页/总数 | 缺/错 Key、负 offset、超大 limit、非法时间、日志脱敏、查询操作审计、constant-time key compare |
| API-T35 | `POST /api/v1/admin/audit/logs/{id}/ack` | 已入 OpenAPI | 首次 ack、actor/time/status | 非数字/0/不存在、重复 ack、RowsAffected、并发 ack、错误 Key、审计管理员身份 |

## 4. 重点并发用例

必须单独实现以下并发测试：

1. 同一 inference 100 并发 sync：只允许一次成功。
2. 同一 AI Idempotency-Key 100 并发：只允许一次 Provider 调用。
3. 同一 commerce receipt 并发履约：只允许一次权益变更。
4. 同一 security nonce 跨 3 个实例：只允许一次成功。
5. 同一 device 并发注册：只创建一条 device，Token claims 合法。
6. 同一签到/购买/战斗结算从两个 Tab 同时提交：奖励恰好一次。

## 5. 重点 Fuzz 用例

- `parseDetectJSON` / chat response：Markdown、多个 JSON、超深嵌套、超长字符串、NaN/Inf 表达、空 choices。
- 图片：随机 bytes、截断 JPEG/PNG/WebP、超大 dimensions、multipart boundary、压缩炸弹。
- JSON：unknown/duplicate fields、尾随对象、Unicode、极端数字、超长数组。
- Cursor/时间：`-`、`-1`、溢出 int64、前导零、未来 100 年、闰秒/时区。
- IDs：空、过长、非 UUID、Unicode 同形字符、路径编码、SQL/HTML 特殊字符。

## 6. CI 完成标准

- 任一路由未入 OpenAPI、任一 OpenAPI operation 无 runtime route，CI 失败。
- 任何测试因 MySQL/Redis/服务不可达而 skip，CI 失败。
- 前端全量测试不得 soft-fail。
- 生产主循环 E2E、API contract、MySQL integration、race、fuzz smoke 全部为硬门禁。
- 测试报告必须保存 endpoint、case、status、request ID 和失败 reason code，但不得保存敏感内容。

