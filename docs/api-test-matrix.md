# Animal Poke · API 测试矩阵

> 覆盖范围：OpenAPI `131` 个 operationId（与 Gin runtime 双向对齐）。
> 本文件由 `node scripts/api-test-matrix-gate.mjs --write` 从 inventory/matrix 生成；禁止手工改计数。

## 门禁

- `node scripts/openapi-contract-gate.mjs` — `/api/v1` Gin ↔ OpenAPI（AP-033）
- `node scripts/api-test-matrix-gate.mjs` — 全量 operation inventory + 矩阵 + 契约测试引用（AP-091）
- `go test ./internal/routes -run TestContractMatrix` — 每个 operation 至少一个 success + failure

## Endpoint 矩阵

| ID | Method | Path | operationId | Success tests | Failure tests |
|---|---|---|---|---|---|
| API-deleteCollectionAnimal | `DELETE` | `/api/v1/collection/{uuid}` | `deleteCollectionAnimal` | TestContractMatrix/deleteCollectionAnimal/success | TestContractMatrix/deleteCollectionAnimal/failure |
| API-deleteAnimal | `DELETE` | `/api/v1/sync/animals/{uuid}` | `deleteAnimal` | TestContractMatrix/deleteAnimal/success | TestContractMatrix/deleteAnimal/failure |
| API-accountDefaults | `GET` | `/api/v1/account/defaults` | `accountDefaults` | TestContractMatrix/accountDefaults/success | TestContractMatrix/accountDefaults/failure |
| API-listAuditLogs | `GET` | `/api/v1/admin/audit/logs` | `listAuditLogs` | TestContractMatrix/listAuditLogs/success | TestContractMatrix/listAuditLogs/failure |
| API-adminGetSecurityReport | `GET` | `/api/v1/admin/security/reports/{id}` | `adminGetSecurityReport` | TestContractMatrix/adminGetSecurityReport/success | TestContractMatrix/adminGetSecurityReport/failure |
| API-authAccount | `GET` | `/api/v1/auth/account` | `authAccount` | TestContractMatrix/authAccount/success | TestContractMatrix/authAccount/failure |
| API-authListDevices | `GET` | `/api/v1/auth/devices` | `authListDevices` | TestContractMatrix/authListDevices/success | TestContractMatrix/authListDevices/failure |
| API-battleCatalog | `GET` | `/api/v1/battle/catalog` | `battleCatalog` | TestContractMatrix/battleCatalog/success | TestContractMatrix/battleCatalog/failure |
| API-getCollectionAnimal | `GET` | `/api/v1/collection/{uuid}` | `getCollectionAnimal` | TestContractMatrix/getCollectionAnimal/success | TestContractMatrix/getCollectionAnimal/failure |
| API-listEntitlements | `GET` | `/api/v1/commerce/entitlements` | `listEntitlements` | TestContractMatrix/listEntitlements/success | TestContractMatrix/listEntitlements/failure |
| API-getOrder | `GET` | `/api/v1/commerce/orders/{id}` | `getOrder` | TestContractMatrix/getOrder/success | TestContractMatrix/getOrder/failure |
| API-getGameConfig | `GET` | `/api/v1/config/game` | `getGameConfig` | TestContractMatrix/getGameConfig/success | TestContractMatrix/getGameConfig/failure |
| API-getCity | `GET` | `/api/v1/geo/city` | `getCity` | TestContractMatrix/getCity/success | TestContractMatrix/getCity/failure |
| API-getGrowthCatalog | `GET` | `/api/v1/growth/catalog` | `getGrowthCatalog` | TestContractMatrix/getGrowthCatalog/success | TestContractMatrix/getGrowthCatalog/failure |
| API-listGrowthCompanions | `GET` | `/api/v1/growth/companions` | `listGrowthCompanions` | TestContractMatrix/listGrowthCompanions/success | TestContractMatrix/listGrowthCompanions/failure |
| API-getGrowthCompanion | `GET` | `/api/v1/growth/companions/{animal_uuid}` | `getGrowthCompanion` | TestContractMatrix/getGrowthCompanion/success | TestContractMatrix/getGrowthCompanion/failure |
| API-listGrowthEvents | `GET` | `/api/v1/growth/events` | `listGrowthEvents` | TestContractMatrix/listGrowthEvents/success | TestContractMatrix/listGrowthEvents/failure |
| API-getResearcherGrowth | `GET` | `/api/v1/growth/researcher` | `getResearcherGrowth` | TestContractMatrix/getResearcherGrowth/success | TestContractMatrix/getResearcherGrowth/failure |
| API-listInventory | `GET` | `/api/v1/inventory` | `listInventory` | TestContractMatrix/listInventory/success | TestContractMatrix/listInventory/failure |
| API-narrativeCatalog | `GET` | `/api/v1/narrative/catalog` | `narrativeCatalog` | TestContractMatrix/narrativeCatalog/success | TestContractMatrix/narrativeCatalog/failure |
| API-narrativeListClues | `GET` | `/api/v1/narrative/clues` | `narrativeListClues` | TestContractMatrix/narrativeListClues/success | TestContractMatrix/narrativeListClues/failure |
| API-narrativeEndingSummary | `GET` | `/api/v1/narrative/ending-summary` | `narrativeEndingSummary` | TestContractMatrix/narrativeEndingSummary/success | TestContractMatrix/narrativeEndingSummary/failure |
| API-narrativeGetNode | `GET` | `/api/v1/narrative/nodes/{node_id}` | `narrativeGetNode` | TestContractMatrix/narrativeGetNode/success | TestContractMatrix/narrativeGetNode/failure |
| API-narrativeProgress | `GET` | `/api/v1/narrative/progress` | `narrativeProgress` | TestContractMatrix/narrativeProgress/success | TestContractMatrix/narrativeProgress/failure |
| API-narrativePullAllProgress | `GET` | `/api/v1/narrative/progress/all` | `narrativePullAllProgress` | TestContractMatrix/narrativePullAllProgress/success | TestContractMatrix/narrativePullAllProgress/failure |
| API-narrativeListSeen | `GET` | `/api/v1/narrative/seen` | `narrativeListSeen` | TestContractMatrix/narrativeListSeen/success | TestContractMatrix/narrativeListSeen/failure |
| API-opsMetricsSummary | `GET` | `/api/v1/ops/metrics-summary` | `opsMetricsSummary` | TestContractMatrix/opsMetricsSummary/success | TestContractMatrix/opsMetricsSummary/failure |
| API-photoGetCalibration | `GET` | `/api/v1/photo/calibration` | `photoGetCalibration` | TestContractMatrix/photoGetCalibration/success | TestContractMatrix/photoGetCalibration/failure |
| API-photoPersonalBest | `GET` | `/api/v1/photo/personal-best` | `photoPersonalBest` | TestContractMatrix/photoPersonalBest/success | TestContractMatrix/photoPersonalBest/failure |
| API-photoDailyTheme | `GET` | `/api/v1/photo/theme/daily` | `photoDailyTheme` | TestContractMatrix/photoDailyTheme/success | TestContractMatrix/photoDailyTheme/failure |
| API-ping | `GET` | `/api/v1/ping` | `ping` | TestContractMatrix/ping/success | TestContractMatrix/ping/failure |
| API-getPrivacyRequest | `GET` | `/api/v1/privacy/requests/{id}` | `getPrivacyRequest` | TestContractMatrix/getPrivacyRequest/success | TestContractMatrix/getPrivacyRequest/failure |
| API-listQuests | `GET` | `/api/v1/quests` | `listQuests` | TestContractMatrix/listQuests/success | TestContractMatrix/listQuests/failure |
| API-getQuest | `GET` | `/api/v1/quests/{quest_id}` | `getQuest` | TestContractMatrix/getQuest/success | TestContractMatrix/getQuest/failure |
| API-getQuestCatalog | `GET` | `/api/v1/quests/catalog` | `getQuestCatalog` | TestContractMatrix/getQuestCatalog/success | TestContractMatrix/getQuestCatalog/failure |
| API-rankingDaily | `GET` | `/api/v1/ranking/daily` | `rankingDaily` | TestContractMatrix/rankingDaily/success | TestContractMatrix/rankingDaily/failure |
| API-socialBlocksList | `GET` | `/api/v1/social/blocks` | `socialBlocksList` | TestContractMatrix/socialBlocksList/success | TestContractMatrix/socialBlocksList/failure |
| API-socialFriends | `GET` | `/api/v1/social/friends` | `socialFriends` | TestContractMatrix/socialFriends/success | TestContractMatrix/socialFriends/failure |
| API-socialFriendRequests | `GET` | `/api/v1/social/friends/requests` | `socialFriendRequests` | TestContractMatrix/socialFriendRequests/success | TestContractMatrix/socialFriendRequests/failure |
| API-socialSearch | `GET` | `/api/v1/social/search` | `socialSearch` | TestContractMatrix/socialSearch/success | TestContractMatrix/socialSearch/failure |
| API-socialSettingsGet | `GET` | `/api/v1/social/settings` | `socialSettingsGet` | TestContractMatrix/socialSettingsGet/success | TestContractMatrix/socialSettingsGet/failure |
| API-socialShareGet | `GET` | `/api/v1/social/share/{token}` | `socialShareGet` | TestContractMatrix/socialShareGet/success | TestContractMatrix/socialShareGet/failure |
| API-pullAnimals | `GET` | `/api/v1/sync/animals` | `pullAnimals` | TestContractMatrix/pullAnimals/success | TestContractMatrix/pullAnimals/failure |
| API-getAnimalDetail | `GET` | `/api/v1/sync/animals/{uuid}` | `getAnimalDetail` | TestContractMatrix/getAnimalDetail/success | TestContractMatrix/getAnimalDetail/failure |
| API-getTime | `GET` | `/api/v1/time` | `getTime` | TestContractMatrix/getTime/success | TestContractMatrix/getTime/failure |
| API-getClientVersion | `GET` | `/api/v1/version` | `getClientVersion` | TestContractMatrix/getClientVersion/success | TestContractMatrix/getClientVersion/failure |
| API-getWallet | `GET` | `/api/v1/wallet` | `getWallet` | TestContractMatrix/getWallet/success | TestContractMatrix/getWallet/failure |
| API-listWalletLedger | `GET` | `/api/v1/wallet/ledger` | `listWalletLedger` | TestContractMatrix/listWalletLedger/success | TestContractMatrix/listWalletLedger/failure |
| API-getWeatherWeek | `GET` | `/api/v1/weather/week` | `getWeatherWeek` | TestContractMatrix/getWeatherWeek/success | TestContractMatrix/getWeatherWeek/failure |
| API-getHealth | `GET` | `/health` | `getHealth` | TestContractMatrix/getHealth/success | TestContractMatrix/getHealth/failure |
| API-getLivez | `GET` | `/livez` | `getLivez` | TestContractMatrix/getLivez/success | TestContractMatrix/getLivez/failure |
| API-getMetrics | `GET` | `/metrics` | `getMetrics` | TestContractMatrix/getMetrics/success | TestContractMatrix/getMetrics/failure |
| API-getReady | `GET` | `/ready` | `getReady` | TestContractMatrix/getReady/success | TestContractMatrix/getReady/failure |
| API-getReadyz | `GET` | `/readyz` | `getReadyz` | TestContractMatrix/getReadyz/success | TestContractMatrix/getReadyz/failure |
| API-patchCollectionAnimal | `PATCH` | `/api/v1/collection/{uuid}` | `patchCollectionAnimal` | TestContractMatrix/patchCollectionAnimal/success | TestContractMatrix/patchCollectionAnimal/failure |
| API-socialSettingsPatch | `PATCH` | `/api/v1/social/settings` | `socialSettingsPatch` | TestContractMatrix/socialSettingsPatch/success | TestContractMatrix/socialSettingsPatch/failure |
| API-patchAnimal | `PATCH` | `/api/v1/sync/animals/{uuid}` | `patchAnimal` | TestContractMatrix/patchAnimal/success | TestContractMatrix/patchAnimal/failure |
| API-ackAuditLog | `POST` | `/api/v1/admin/audit/logs/{id}/ack` | `ackAuditLog` | TestContractMatrix/ackAuditLog/success | TestContractMatrix/ackAuditLog/failure |
| API-adminIssueToken | `POST` | `/api/v1/admin/auth/token` | `adminIssueToken` | TestContractMatrix/adminIssueToken/success | TestContractMatrix/adminIssueToken/failure |
| API-adminRefundOrder | `POST` | `/api/v1/admin/commerce/orders/refund` | `adminRefundOrder` | TestContractMatrix/adminRefundOrder/success | TestContractMatrix/adminRefundOrder/failure |
| API-webhookRefundOrder | `POST` | `/api/v1/admin/commerce/webhooks/refund` | `webhookRefundOrder` | TestContractMatrix/webhookRefundOrder/success | TestContractMatrix/webhookRefundOrder/failure |
| API-adminRevokeSession | `POST` | `/api/v1/admin/sessions/revoke` | `adminRevokeSession` | TestContractMatrix/adminRevokeSession/success | TestContractMatrix/adminRevokeSession/failure |
| API-analyticsIngest | `POST` | `/api/v1/analytics/events` | `analyticsIngest` | TestContractMatrix/analyticsIngest/success | TestContractMatrix/analyticsIngest/failure |
| API-authBind | `POST` | `/api/v1/auth/bind` | `authBind` | TestContractMatrix/authBind/success | TestContractMatrix/authBind/failure |
| API-authDevice | `POST` | `/api/v1/auth/device` | `authDevice` | TestContractMatrix/authDevice/success | TestContractMatrix/authDevice/failure |
| API-authRevokeDevice | `POST` | `/api/v1/auth/devices/revoke` | `authRevokeDevice` | TestContractMatrix/authRevokeDevice/success | TestContractMatrix/authRevokeDevice/failure |
| API-authEmailVerify | `POST` | `/api/v1/auth/email/verify` | `authEmailVerify` | TestContractMatrix/authEmailVerify/success | TestContractMatrix/authEmailVerify/failure |
| API-authEmailVerifyRequest | `POST` | `/api/v1/auth/email/verify/request` | `authEmailVerifyRequest` | TestContractMatrix/authEmailVerifyRequest/success | TestContractMatrix/authEmailVerifyRequest/failure |
| API-authLogin | `POST` | `/api/v1/auth/login` | `authLogin` | TestContractMatrix/authLogin/success | TestContractMatrix/authLogin/failure |
| API-authLogout | `POST` | `/api/v1/auth/logout` | `authLogout` | TestContractMatrix/authLogout/success | TestContractMatrix/authLogout/failure |
| API-authPasswordChange | `POST` | `/api/v1/auth/password/change` | `authPasswordChange` | TestContractMatrix/authPasswordChange/success | TestContractMatrix/authPasswordChange/failure |
| API-authPasswordForgot | `POST` | `/api/v1/auth/password/forgot` | `authPasswordForgot` | TestContractMatrix/authPasswordForgot/success | TestContractMatrix/authPasswordForgot/failure |
| API-authPasswordReset | `POST` | `/api/v1/auth/password/reset` | `authPasswordReset` | TestContractMatrix/authPasswordReset/success | TestContractMatrix/authPasswordReset/failure |
| API-authReauth | `POST` | `/api/v1/auth/reauth` | `authReauth` | TestContractMatrix/authReauth/success | TestContractMatrix/authReauth/failure |
| API-authRefresh | `POST` | `/api/v1/auth/refresh` | `authRefresh` | TestContractMatrix/authRefresh/success | TestContractMatrix/authRefresh/failure |
| API-authUnbind | `POST` | `/api/v1/auth/unbind` | `authUnbind` | TestContractMatrix/authUnbind/success | TestContractMatrix/authUnbind/failure |
| API-battlePveSettle | `POST` | `/api/v1/battle/pve/settle` | `battlePveSettle` | TestContractMatrix/battlePveSettle/success | TestContractMatrix/battlePveSettle/failure |
| API-battlePveStart | `POST` | `/api/v1/battle/pve/start` | `battlePveStart` | TestContractMatrix/battlePveStart/success | TestContractMatrix/battlePveStart/failure |
| API-battleSimulate | `POST` | `/api/v1/battle/simulate` | `battleSimulate` | TestContractMatrix/battleSimulate/success | TestContractMatrix/battleSimulate/failure |
| API-createOrder | `POST` | `/api/v1/commerce/orders` | `createOrder` | TestContractMatrix/createOrder/success | TestContractMatrix/createOrder/failure |
| API-fulfillOrder | `POST` | `/api/v1/commerce/orders/fulfill` | `fulfillOrder` | TestContractMatrix/fulfillOrder/success | TestContractMatrix/fulfillOrder/failure |
| API-refundOrder | `POST` | `/api/v1/commerce/orders/refund` | `refundOrder` | TestContractMatrix/refundOrder/success | TestContractMatrix/refundOrder/failure |
| API-errorsReport | `POST` | `/api/v1/errors/report` | `errorsReport` | TestContractMatrix/errorsReport/success | TestContractMatrix/errorsReport/failure |
| API-postGrowthEvent | `POST` | `/api/v1/growth/events` | `postGrowthEvent` | TestContractMatrix/postGrowthEvent/success | TestContractMatrix/postGrowthEvent/failure |
| API-resetGrowth | `POST` | `/api/v1/growth/reset` | `resetGrowth` | TestContractMatrix/resetGrowth/success | TestContractMatrix/resetGrowth/failure |
| API-consumeInventory | `POST` | `/api/v1/inventory/consume` | `consumeInventory` | TestContractMatrix/consumeInventory/success | TestContractMatrix/consumeInventory/failure |
| API-grantInventory | `POST` | `/api/v1/inventory/grant` | `grantInventory` | TestContractMatrix/grantInventory/success | TestContractMatrix/grantInventory/failure |
| API-narrativeSubmitChoice | `POST` | `/api/v1/narrative/choices` | `narrativeSubmitChoice` | TestContractMatrix/narrativeSubmitChoice/success | TestContractMatrix/narrativeSubmitChoice/failure |
| API-narrativeUpdateClue | `POST` | `/api/v1/narrative/clues` | `narrativeUpdateClue` | TestContractMatrix/narrativeUpdateClue/success | TestContractMatrix/narrativeUpdateClue/failure |
| API-narrativeFailForward | `POST` | `/api/v1/narrative/fail-forward` | `narrativeFailForward` | TestContractMatrix/narrativeFailForward/success | TestContractMatrix/narrativeFailForward/failure |
| API-narrativeObservation | `POST` | `/api/v1/narrative/observation` | `narrativeObservation` | TestContractMatrix/narrativeObservation/success | TestContractMatrix/narrativeObservation/failure |
| API-narrativeMarkSeen | `POST` | `/api/v1/narrative/seen` | `narrativeMarkSeen` | TestContractMatrix/narrativeMarkSeen/success | TestContractMatrix/narrativeMarkSeen/failure |
| API-rollbackGameConfig | `POST` | `/api/v1/ops/game-config/rollback` | `rollbackGameConfig` | TestContractMatrix/rollbackGameConfig/success | TestContractMatrix/rollbackGameConfig/failure |
| API-photoCalibrate | `POST` | `/api/v1/photo/calibrate` | `photoCalibrate` | TestContractMatrix/photoCalibrate/success | TestContractMatrix/photoCalibrate/failure |
| API-photoScore | `POST` | `/api/v1/photo/score` | `photoScore` | TestContractMatrix/photoScore/success | TestContractMatrix/photoScore/failure |
| API-photoThemeProgress | `POST` | `/api/v1/photo/theme/progress` | `photoThemeProgress` | TestContractMatrix/photoThemeProgress/success | TestContractMatrix/photoThemeProgress/failure |
| API-putConsent | `POST` | `/api/v1/privacy/consent` | `putConsent` | TestContractMatrix/putConsent/success | TestContractMatrix/putConsent/failure |
| API-deleteData | `POST` | `/api/v1/privacy/delete` | `deleteData` | TestContractMatrix/deleteData/success | TestContractMatrix/deleteData/failure |
| API-exportData | `POST` | `/api/v1/privacy/export` | `exportData` | TestContractMatrix/exportData/success | TestContractMatrix/exportData/failure |
| API-pvpCancel | `POST` | `/api/v1/pvp/cancel` | `pvpCancel` | TestContractMatrix/pvpCancel/success | TestContractMatrix/pvpCancel/failure |
| API-pvpMatch | `POST` | `/api/v1/pvp/match` | `pvpMatch` | TestContractMatrix/pvpMatch/success | TestContractMatrix/pvpMatch/failure |
| API-pvpResult | `POST` | `/api/v1/pvp/result` | `pvpResult` | TestContractMatrix/pvpResult/success | TestContractMatrix/pvpResult/failure |
| API-claimQuest | `POST` | `/api/v1/quests/{quest_id}/claim` | `claimQuest` | TestContractMatrix/claimQuest/success | TestContractMatrix/claimQuest/failure |
| API-compensateQuests | `POST` | `/api/v1/quests/compensate` | `compensateQuests` | TestContractMatrix/compensateQuests/success | TestContractMatrix/compensateQuests/failure |
| API-applyQuestEvent | `POST` | `/api/v1/quests/events` | `applyQuestEvent` | TestContractMatrix/applyQuestEvent/success | TestContractMatrix/applyQuestEvent/failure |
| API-rankingScore | `POST` | `/api/v1/ranking/score` | `rankingScore` | TestContractMatrix/rankingScore/success | TestContractMatrix/rankingScore/failure |
| API-rankingSettle | `POST` | `/api/v1/ranking/settle` | `rankingSettle` | TestContractMatrix/rankingSettle/success | TestContractMatrix/rankingSettle/failure |
| API-safetyReport | `POST` | `/api/v1/safety/report` | `safetyReport` | TestContractMatrix/safetyReport/success | TestContractMatrix/safetyReport/failure |
| API-securityReport | `POST` | `/api/v1/security/report` | `securityReport` | TestContractMatrix/securityReport/success | TestContractMatrix/securityReport/failure |
| API-socialBlock | `POST` | `/api/v1/social/block` | `socialBlock` | TestContractMatrix/socialBlock/success | TestContractMatrix/socialBlock/failure |
| API-socialFriendRequestAccept | `POST` | `/api/v1/social/friends/accept` | `socialFriendRequestAccept` | TestContractMatrix/socialFriendRequestAccept/success | TestContractMatrix/socialFriendRequestAccept/failure |
| API-socialFriendRequestCancel | `POST` | `/api/v1/social/friends/cancel` | `socialFriendRequestCancel` | TestContractMatrix/socialFriendRequestCancel/success | TestContractMatrix/socialFriendRequestCancel/failure |
| API-socialFriendRequestReject | `POST` | `/api/v1/social/friends/reject` | `socialFriendRequestReject` | TestContractMatrix/socialFriendRequestReject/success | TestContractMatrix/socialFriendRequestReject/failure |
| API-socialFriendRemove | `POST` | `/api/v1/social/friends/remove` | `socialFriendRemove` | TestContractMatrix/socialFriendRemove/success | TestContractMatrix/socialFriendRemove/failure |
| API-socialFriendRequestCreate | `POST` | `/api/v1/social/friends/request` | `socialFriendRequestCreate` | TestContractMatrix/socialFriendRequestCreate/success | TestContractMatrix/socialFriendRequestCreate/failure |
| API-socialMute | `POST` | `/api/v1/social/mute` | `socialMute` | TestContractMatrix/socialMute/success | TestContractMatrix/socialMute/failure |
| API-socialReportUser | `POST` | `/api/v1/social/report` | `socialReportUser` | TestContractMatrix/socialReportUser/success | TestContractMatrix/socialReportUser/failure |
| API-socialShareCreate | `POST` | `/api/v1/social/share` | `socialShareCreate` | TestContractMatrix/socialShareCreate/success | TestContractMatrix/socialShareCreate/failure |
| API-socialShareRevoke | `POST` | `/api/v1/social/share/{token}/revoke` | `socialShareRevoke` | TestContractMatrix/socialShareRevoke/success | TestContractMatrix/socialShareRevoke/failure |
| API-socialUnblock | `POST` | `/api/v1/social/unblock` | `socialUnblock` | TestContractMatrix/socialUnblock/success | TestContractMatrix/socialUnblock/failure |
| API-socialUnmute | `POST` | `/api/v1/social/unmute` | `socialUnmute` | TestContractMatrix/socialUnmute/success | TestContractMatrix/socialUnmute/failure |
| API-syncAnimal | `POST` | `/api/v1/sync/animal` | `syncAnimal` | TestContractMatrix/syncAnimal/success | TestContractMatrix/syncAnimal/failure |
| API-syncAnimalsBatch | `POST` | `/api/v1/sync/animals` | `syncAnimalsBatch` | TestContractMatrix/syncAnimalsBatch/success | TestContractMatrix/syncAnimalsBatch/failure |
| API-valueGenerate | `POST` | `/api/v1/value/generate` | `valueGenerate` | TestContractMatrix/valueGenerate/success | TestContractMatrix/valueGenerate/failure |
| API-visionAnalyze | `POST` | `/api/v1/vision/analyze` | `visionAnalyze` | TestContractMatrix/visionAnalyze/success | TestContractMatrix/visionAnalyze/failure |
| API-visionDetect | `POST` | `/api/v1/vision/detect` | `visionDetect` | TestContractMatrix/visionDetect/success | TestContractMatrix/visionDetect/failure |
| API-creditWallet | `POST` | `/api/v1/wallet/credit` | `creditWallet` | TestContractMatrix/creditWallet/success | TestContractMatrix/creditWallet/failure |
| API-debitWallet | `POST` | `/api/v1/wallet/debit` | `debitWallet` | TestContractMatrix/debitWallet/success | TestContractMatrix/debitWallet/failure |
| API-reconcileWallet | `POST` | `/api/v1/wallet/reconcile` | `reconcileWallet` | TestContractMatrix/reconcileWallet/success | TestContractMatrix/reconcileWallet/failure |
| API-adminWriteGameConfig | `PUT` | `/api/v1/admin/config/game` | `adminWriteGameConfig` | TestContractMatrix/adminWriteGameConfig/success | TestContractMatrix/adminWriteGameConfig/failure |
| API-putGameConfig | `PUT` | `/api/v1/ops/game-config` | `putGameConfig` | TestContractMatrix/putGameConfig/success | TestContractMatrix/putGameConfig/failure |

## 全局断言

- 成功/失败均回传 `X-Request-ID`（JSON API）。
- 错误体包含稳定 `reason_code`（若适用），不泄露 Secret/DSN/Provider 原文。
- 401/403 语义稳定；缺依赖时 503 而非 404。
- 矩阵与 inventory 数量必须一致；CI 禁止 skip 掩盖缺口。
