# Animal Poke · API 测试矩阵

> 覆盖范围：OpenAPI `49` 个 operationId（与 Gin runtime 双向对齐）。
> 本文件由 `node scripts/api-test-matrix-gate.mjs --write` 从 inventory/matrix 生成；禁止手工改计数。

## 门禁

- `node scripts/openapi-contract-gate.mjs` — `/api/v1` Gin ↔ OpenAPI（AP-033）
- `node scripts/api-test-matrix-gate.mjs` — 全量 operation inventory + 矩阵 + 契约测试引用（AP-091）
- `go test ./internal/routes -run TestContractMatrix` — 每个 operation 至少一个 success + failure

## Endpoint 矩阵

| ID | Method | Path | operationId | Success tests | Failure tests |
|---|---|---|---|---|---|
| API-accountDefaults | `GET` | `/api/v1/account/defaults` | `accountDefaults` | TestContractMatrix/accountDefaults/success | TestContractMatrix/accountDefaults/failure |
| API-listAuditLogs | `GET` | `/api/v1/admin/audit/logs` | `listAuditLogs` | TestContractMatrix/listAuditLogs/success | TestContractMatrix/listAuditLogs/failure |
| API-authAccount | `GET` | `/api/v1/auth/account` | `authAccount` | TestContractMatrix/authAccount/success | TestContractMatrix/authAccount/failure |
| API-authListDevices | `GET` | `/api/v1/auth/devices` | `authListDevices` | TestContractMatrix/authListDevices/success | TestContractMatrix/authListDevices/failure |
| API-listEntitlements | `GET` | `/api/v1/commerce/entitlements` | `listEntitlements` | TestContractMatrix/listEntitlements/success | TestContractMatrix/listEntitlements/failure |
| API-getOrder | `GET` | `/api/v1/commerce/orders/{id}` | `getOrder` | TestContractMatrix/getOrder/success | TestContractMatrix/getOrder/failure |
| API-getGameConfig | `GET` | `/api/v1/config/game` | `getGameConfig` | TestContractMatrix/getGameConfig/success | TestContractMatrix/getGameConfig/failure |
| API-getCity | `GET` | `/api/v1/geo/city` | `getCity` | TestContractMatrix/getCity/success | TestContractMatrix/getCity/failure |
| API-opsMetricsSummary | `GET` | `/api/v1/ops/metrics-summary` | `opsMetricsSummary` | TestContractMatrix/opsMetricsSummary/success | TestContractMatrix/opsMetricsSummary/failure |
| API-ping | `GET` | `/api/v1/ping` | `ping` | TestContractMatrix/ping/success | TestContractMatrix/ping/failure |
| API-getPrivacyRequest | `GET` | `/api/v1/privacy/requests/{id}` | `getPrivacyRequest` | TestContractMatrix/getPrivacyRequest/success | TestContractMatrix/getPrivacyRequest/failure |
| API-rankingDaily | `GET` | `/api/v1/ranking/daily` | `rankingDaily` | TestContractMatrix/rankingDaily/success | TestContractMatrix/rankingDaily/failure |
| API-socialFriends | `GET` | `/api/v1/social/friends` | `socialFriends` | TestContractMatrix/socialFriends/success | TestContractMatrix/socialFriends/failure |
| API-pullAnimals | `GET` | `/api/v1/sync/animals` | `pullAnimals` | TestContractMatrix/pullAnimals/success | TestContractMatrix/pullAnimals/failure |
| API-getTime | `GET` | `/api/v1/time` | `getTime` | TestContractMatrix/getTime/success | TestContractMatrix/getTime/failure |
| API-getWeatherWeek | `GET` | `/api/v1/weather/week` | `getWeatherWeek` | TestContractMatrix/getWeatherWeek/success | TestContractMatrix/getWeatherWeek/failure |
| API-getHealth | `GET` | `/health` | `getHealth` | TestContractMatrix/getHealth/success | TestContractMatrix/getHealth/failure |
| API-getLivez | `GET` | `/livez` | `getLivez` | TestContractMatrix/getLivez/success | TestContractMatrix/getLivez/failure |
| API-getMetrics | `GET` | `/metrics` | `getMetrics` | TestContractMatrix/getMetrics/success | TestContractMatrix/getMetrics/failure |
| API-getReady | `GET` | `/ready` | `getReady` | TestContractMatrix/getReady/success | TestContractMatrix/getReady/failure |
| API-getReadyz | `GET` | `/readyz` | `getReadyz` | TestContractMatrix/getReadyz/success | TestContractMatrix/getReadyz/failure |
| API-ackAuditLog | `POST` | `/api/v1/admin/audit/logs/{id}/ack` | `ackAuditLog` | TestContractMatrix/ackAuditLog/success | TestContractMatrix/ackAuditLog/failure |
| API-adminRefundOrder | `POST` | `/api/v1/admin/commerce/orders/refund` | `adminRefundOrder` | TestContractMatrix/adminRefundOrder/success | TestContractMatrix/adminRefundOrder/failure |
| API-webhookRefundOrder | `POST` | `/api/v1/admin/commerce/webhooks/refund` | `webhookRefundOrder` | TestContractMatrix/webhookRefundOrder/success | TestContractMatrix/webhookRefundOrder/failure |
| API-analyticsIngest | `POST` | `/api/v1/analytics/events` | `analyticsIngest` | TestContractMatrix/analyticsIngest/success | TestContractMatrix/analyticsIngest/failure |
| API-authBind | `POST` | `/api/v1/auth/bind` | `authBind` | TestContractMatrix/authBind/success | TestContractMatrix/authBind/failure |
| API-authDevice | `POST` | `/api/v1/auth/device` | `authDevice` | TestContractMatrix/authDevice/success | TestContractMatrix/authDevice/failure |
| API-authRevokeDevice | `POST` | `/api/v1/auth/devices/revoke` | `authRevokeDevice` | TestContractMatrix/authRevokeDevice/success | TestContractMatrix/authRevokeDevice/failure |
| API-authLogin | `POST` | `/api/v1/auth/login` | `authLogin` | TestContractMatrix/authLogin/success | TestContractMatrix/authLogin/failure |
| API-authLogout | `POST` | `/api/v1/auth/logout` | `authLogout` | TestContractMatrix/authLogout/success | TestContractMatrix/authLogout/failure |
| API-createOrder | `POST` | `/api/v1/commerce/orders` | `createOrder` | TestContractMatrix/createOrder/success | TestContractMatrix/createOrder/failure |
| API-fulfillOrder | `POST` | `/api/v1/commerce/orders/fulfill` | `fulfillOrder` | TestContractMatrix/fulfillOrder/success | TestContractMatrix/fulfillOrder/failure |
| API-refundOrder | `POST` | `/api/v1/commerce/orders/refund` | `refundOrder` | TestContractMatrix/refundOrder/success | TestContractMatrix/refundOrder/failure |
| API-errorsReport | `POST` | `/api/v1/errors/report` | `errorsReport` | TestContractMatrix/errorsReport/success | TestContractMatrix/errorsReport/failure |
| API-rollbackGameConfig | `POST` | `/api/v1/ops/game-config/rollback` | `rollbackGameConfig` | TestContractMatrix/rollbackGameConfig/success | TestContractMatrix/rollbackGameConfig/failure |
| API-putConsent | `POST` | `/api/v1/privacy/consent` | `putConsent` | TestContractMatrix/putConsent/success | TestContractMatrix/putConsent/failure |
| API-deleteData | `POST` | `/api/v1/privacy/delete` | `deleteData` | TestContractMatrix/deleteData/success | TestContractMatrix/deleteData/failure |
| API-exportData | `POST` | `/api/v1/privacy/export` | `exportData` | TestContractMatrix/exportData/success | TestContractMatrix/exportData/failure |
| API-pvpMatch | `POST` | `/api/v1/pvp/match` | `pvpMatch` | TestContractMatrix/pvpMatch/success | TestContractMatrix/pvpMatch/failure |
| API-pvpResult | `POST` | `/api/v1/pvp/result` | `pvpResult` | TestContractMatrix/pvpResult/success | TestContractMatrix/pvpResult/failure |
| API-safetyReport | `POST` | `/api/v1/safety/report` | `safetyReport` | TestContractMatrix/safetyReport/success | TestContractMatrix/safetyReport/failure |
| API-securityReport | `POST` | `/api/v1/security/report` | `securityReport` | TestContractMatrix/securityReport/success | TestContractMatrix/securityReport/failure |
| API-socialShare | `POST` | `/api/v1/social/share` | `socialShare` | TestContractMatrix/socialShare/success | TestContractMatrix/socialShare/failure |
| API-syncAnimal | `POST` | `/api/v1/sync/animal` | `syncAnimal` | TestContractMatrix/syncAnimal/success | TestContractMatrix/syncAnimal/failure |
| API-syncAnimalsBatch | `POST` | `/api/v1/sync/animals` | `syncAnimalsBatch` | TestContractMatrix/syncAnimalsBatch/success | TestContractMatrix/syncAnimalsBatch/failure |
| API-valueGenerate | `POST` | `/api/v1/value/generate` | `valueGenerate` | TestContractMatrix/valueGenerate/success | TestContractMatrix/valueGenerate/failure |
| API-visionAnalyze | `POST` | `/api/v1/vision/analyze` | `visionAnalyze` | TestContractMatrix/visionAnalyze/success | TestContractMatrix/visionAnalyze/failure |
| API-visionDetect | `POST` | `/api/v1/vision/detect` | `visionDetect` | TestContractMatrix/visionDetect/success | TestContractMatrix/visionDetect/failure |
| API-putGameConfig | `PUT` | `/api/v1/ops/game-config` | `putGameConfig` | TestContractMatrix/putGameConfig/success | TestContractMatrix/putGameConfig/failure |

## 全局断言

- 成功/失败均回传 `X-Request-ID`（JSON API）。
- 错误体包含稳定 `reason_code`（若适用），不泄露 Secret/DSN/Provider 原文。
- 401/403 语义稳定；缺依赖时 503 而非 404。
- 矩阵与 inventory 数量必须一致；CI 禁止 skip 掩盖缺口。
