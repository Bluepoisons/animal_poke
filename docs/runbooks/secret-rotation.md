# 密钥轮换与吊销 Runbook

## 原则
- 服务端 Key 唯一所有者：`backend/.env`（本地）/ 云 Secret Manager 或 K8s Secret（生产）。
- 前端**永不**持有第三方 Key；轮换 Key **无需**重建前端（仅后端滚动发布）。
- 根目录不应再维护第二份服务端 `.env`。


## 按用途密钥（AP-087）
生产必须独立配置且禁止互相复用：
| 用途 | 环境变量 | 兼容别名 | previous 双读 |
|------|----------|----------|---------------|
| JWT 签名 | `JWT_SIGNING_KEY` | `JWT_SECRET` | `JWT_SIGNING_KEY_PREVIOUS` / `JWT_SECRET_PREVIOUS` |
| 账号 token pepper | `ACCOUNT_TOKEN_PEPPER` | — | `ACCOUNT_TOKEN_PEPPER_PREVIOUS` |
| 数值 HMAC | `STATS_HMAC_KEY` | — | `STATS_HMAC_KEY_PREVIOUS` |
| 时间签名 | `TIME_SIGNING_KEY` | — | `TIME_SIGNING_KEY_PREVIOUS` |

轮换原则：
- 单写双读：签发/新哈希只用 current；校验可接受 previous。
- 轮换 JWT **不影响** 账号凭证哈希或 rarity/stats 种子。
- 缺任一必需 key 时 production fail-fast。
- 日志与错误不得输出 key 明文或完整指纹以外内容。

## 轮换步骤（生产）
1. 在 Secret Manager 生成新 Key（腾讯地图 / 彩云 / Vision / LLM / JWT / Admin）。
2. 更新 ExternalSecret / `backend-secrets`，**不要**提交到 git。
3. 滚动重启 backend Deployment（新 Pod 读新 Secret）。
4. 验证：
   - `/readyz` = 200
   - 设备鉴权 + `/geo/city` + `/weather/week` smoke
   - Vision detect 成功（或预期限流）
5. 在旧供应商控制台吊销旧 Key。
6. JWT 签名密钥轮换（推荐双密钥窗口）：
   1. 将当前 `JWT_SIGNING_KEY`（或 `JWT_SECRET`）复制为对应 `*_PREVIOUS`。
   2. 生成新强随机密钥写入 current（签发始终用当前密钥；Header 可选 `kid=v1`）。
   3. 滚动发布 backend；旧 Token 在 TTL 内仍可用 previous 校验。
   4. 等待最长 `JWT_ACCESS_TTL` 后清空 previous 并再次滚动。
   5. 紧急吊销：提升设备 `token_version` 或清空 previous 使旧签立刻失效。
   6. 账号 pepper / stats HMAC / time signing 使用同一双读流程，**分别**轮换，禁止串用。
7. 设备 `installation_secret` 仅首次注册返回；客户端须安全持久化。轮换 JWT 密钥**不**要求重发 installation_secret。

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

## MySQL / Redis TLS（AP-088）
- 生产 `DB_TLS` 必须为 `require` / `verify-ca` / `verify-full`（Kustomize 静态门禁拒绝 `false` / `skip-verify`）。
- 自定义 CA：将 PEM 挂载为文件，设置 `DB_TLS_CA`；可选 `DB_TLS_SERVER_NAME`、客户端 `DB_TLS_CERT`/`DB_TLS_KEY`。
- CA 轮换：先挂载新 CA（或双 CA bundle），滚动 backend，再撤旧 CA。TLS 配置名包含材料哈希，换文件即换注册名。
- Redis：生产 `REDIS_URL` 必须为 `rediss://:password@host:port/db`（TLS + 密码 + 主机名校验）；禁止 `skip_verify`。
- `/readyz` 在 DB 失败时返回 `db_reason`=`cert|auth|pool|network|unavailable`（不含密钥/DSN）。
