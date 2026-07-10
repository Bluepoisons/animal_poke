# Security Policy

## 支持版本
当前 `main` 分支为唯一积极维护线。

## 报告漏洞
请**不要**在公开 GitHub Issue 中披露可利用细节。

1. 通过仓库 Maintainer 私信，或邮件联系仓库 Owner（`Bluepoisons` / `isaac-sun` / `ji233-Sun`）。
2. 提供：影响组件、复现步骤、影响范围、是否已有利用迹象。
3. 我们会在 3 个工作日内确认，并协调修复与披露时间线。

## 密钥与配置
- 所有第三方 Key **仅**存放在 `backend/.env`（本地）或 K8s / 云 Secret Manager（生产）。
- 前端只允许 `VITE_API_BASE_URL` 等公开配置。
- 发现泄漏：立即轮换 Key，参考 `docs/runbooks/secret-rotation.md`。
