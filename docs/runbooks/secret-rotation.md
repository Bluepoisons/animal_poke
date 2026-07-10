# 密钥轮换与吊销 Runbook

## 原则
- 服务端 Key 唯一所有者：`backend/.env`（本地）/ 云 Secret Manager 或 K8s Secret（生产）。
- 前端**永不**持有第三方 Key；轮换 Key **无需**重建前端（仅后端滚动发布）。
- 根目录不应再维护第二份服务端 `.env`。

## 轮换步骤（生产）
1. 在 Secret Manager 生成新 Key（腾讯地图 / 彩云 / Vision / LLM / JWT / Admin）。
2. 更新 ExternalSecret / `backend-secrets`，**不要**提交到 git。
3. 滚动重启 backend Deployment（新 Pod 读新 Secret）。
4. 验证：
   - `/readyz` = 200
   - 设备鉴权 + `/geo/city` + `/weather/week` smoke
   - Vision detect 成功（或预期限流）
5. 在旧供应商控制台吊销旧 Key。
6. 若 JWT_SECRET 轮换：旧 Token 失效，客户端会重新 `POST /auth/device`。

## 泄漏应急
1. 立即在供应商侧禁用泄露 Key。
2. 轮换全部相关 Secret。
3. 检查 git 历史 / 镜像层 / CI 日志（gitleaks）。
4. 评估是否需要强制设备 `token_version` 提升以吊销会话。

## 本地
```bash
cd backend
cp .env.example .env   # 仅此一份服务端配置
# 编辑 .env 填入真实值；切勿提交
```
