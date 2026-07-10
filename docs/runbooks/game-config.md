# 版本化游戏配置 / Feature Flag / 回滚（AP-059）

## 权威来源

| 层 | 路径 |
|----|------|
| 默认常量 | `frontend/src/stamina/constants.ts`, `shop/constants.ts`, `battle/constants.ts` |
| 前端聚合 | `frontend/src/config/gameConfig.ts` |
| 后端服务 | `GET /api/v1/config/game` |
| 运维写入 | `PUT /api/v1/ops/game-config`（需 `FEATURE_OPS` + ops token/role） |
| 回滚 | `POST /api/v1/ops/game-config/rollback` |

## 硬边界

捕获/派遣/战斗体力消耗 ∈ [1,120]；恢复/小时 ∈ [1,60]；价格 ∈ [1,10000]。越界拒绝（后端 400 / 前端 clamp）。

## 不发版回滚

1. `PUT` 错误配置后立刻 `POST .../rollback`  
2. 或前端运行时注入 `window.__AP_CONFIG__.game = { version, economy, features }`（Nginx `config.js`）后刷新  
3. 旧客户端保留本地 defaultGameConfig() 作为兼容默认

## 验证

```bash
cd backend && go test ./internal/services -run GameConfig
cd frontend && npx vitest run src/config/gameConfig.test.ts
```
