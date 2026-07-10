# 前端发布与回滚

## 策略（AP-010）

生产默认 **同域 `/api` 反代**：
- 构建时 `VITE_API_BASE_URL` 为空
- 浏览器请求相对路径 `/api/v1/...`
- 前端 Nginx 将 `/api/` 反代到 `BACKEND_UPSTREAM`（默认 `animal-poke-backend:80`）
- 可选运行时 `API_BASE_URL` 写入 `/config.js` 的 `window.__AP_CONFIG__`（绝对 URL 场景）
- `/api/*` 上游失败返回 JSON 502，**永不** SPA fallback 到 `index.html`

## 构建

```bash
# 推荐：同域 /api（勿写死跨域 API host）
docker build -f deploy/Dockerfile.frontend \
  --build-arg VITE_API_BASE_URL= \
  -t registry.cn-beijing.aliyuncs.com/animal-poke/frontend:<git-sha> .

# 仅跨域部署时才注入绝对 API（并同步 CSP）
# docker build -f deploy/Dockerfile.frontend \
#   --build-arg VITE_API_BASE_URL=https://api.animal-poke.example.com \
#   -t registry.cn-beijing.aliyuncs.com/animal-poke/frontend:<git-sha> .
```

## 部署

- 清单：`deploy/k8s/base/frontend.yaml`（`BACKEND_UPSTREAM` / `API_BASE_URL`）
- 镜像 tag 使用 commit SHA，禁止 `latest`
- Ingress：`deploy/k8s/base/ingress.yaml`（`app` host → frontend；`api` host 仍可直达 backend）

## Smoke

```bash
# 本地/CI（frontend 需已启动，默认 http://127.0.0.1:18081）
./deploy/scripts/frontend-smoke.sh http://127.0.0.1:18081
```

检查：`/healthz`、`/index.html`、`/config.js`、`/assets/*`、manifest/sw（若有）、`/api/*` 非 HTML。

## 缓存与 PWA

- `index.html` / `sw.js` / `config.js`：`Cache-Control: no-cache`
- `/assets/*`：immutable 长期缓存
- SPA fallback：仅非 `/api` 路由 `try_files ... /index.html`
- Service Worker：`registerType: autoUpdate`；用户刷新后获取新版本

## CSP

- `connect-src 'self'`（同域 `/api`）
- 禁止 broad `https:`
- 若使用绝对 `API_BASE_URL`，须在 `deploy/nginx-frontend.conf` 中显式加入该 host

## 回滚

1. 将 Deployment image 改回上一 SHA
2. 若 SW 卡住：用户硬刷新或清站点数据；新 `sw.js` no-cache 保证下次加载更新
3. 验证 `/healthz` 与 `./deploy/scripts/frontend-smoke.sh`
