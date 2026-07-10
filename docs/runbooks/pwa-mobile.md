# 移动端 PWA / 浏览器矩阵 / 受控升级（AP-040）

## 支持矩阵

| 目标 | CI | 手动 / 真机 |
|------|----|-------------|
| Desktop Chromium | ✅ `npm run test:e2e` (project chromium) | DevTools + Lighthouse |
| Desktop WebKit (Safari 代理) | ✅ Playwright project `webkit` | macOS Safari 冒烟 |
| Android Chrome | 可选 `PLAYWRIGHT_MOBILE=1` | 真机 / BrowserStack |
| iOS Safari / 主屏幕 PWA | 可选 mobile-safari project | 真机必测：刘海安全区、相机、定位权限 |

CI 默认跑 **chromium + webkit**。真机矩阵不阻断 PR，但上线前必须手动勾选。

## 受控升级（捕获中不打断）

- `vite-plugin-pwa`：`registerType: 'prompt'`（禁止捕获中 auto skipWaiting）
- `frontend/src/pwa/updateGate.ts`：
  - 捕获进行中 `setCaptureActive(true)` → 新 SW **defer**
  - 捕获结束或用户确认 `applyPendingUpdate()` 才应用
- 调试：`window.__AP_APPLY_UPDATE__()`

## 敏感 API 不缓存

- Workbox **不对** `/api/` 做通用 NetworkFirst
- 禁止缓存 auth / vision / value / sync / privacy 等响应
- 单测：`src/pwa/cachePolicy.test.ts`

## IDB 跨版本

- `animal-poke-db` v1→v2→v3（animals / settings / sync_queue）
- 单测：`src/db/migration.test.ts`

## 性能预算（发布）

- JS gzip 主包合计见 AP-058 `check:bundle-budget`（默认 < 250KB）
- LCP < 2.5s（中端机，手动 Lighthouse）

## 验证清单

1. Chrome DevTools → Application → Manifest / Service Workers
2. 捕获流程中触发 update → 不应立即 reload
3. 捕获结束后 update 可应用，无白屏、IDB 不丢
4. `npx vitest run src/pwa src/db/migration.test.ts`
5. `npx playwright test --project=chromium --project=webkit`
