# 前端发布与回滚

## 构建
```bash
# 注入公开 API 地址
docker build -f deploy/Dockerfile.frontend \
  --build-arg VITE_API_BASE_URL=https://api.animal-poke.example.com \
  -t registry.cn-beijing.aliyuncs.com/animal-poke/frontend:<git-sha> .
```

## 部署
- 清单：`deploy/k8s/base/frontend.yaml`
- 镜像 tag 使用 commit SHA，禁止 `latest`。
- Ingress：`deploy/k8s/base/ingress.yaml`（`__APP_HOST__`）

## 缓存与 PWA
- `index.html` / `sw.js`：`Cache-Control: no-cache`
- `/assets/*`：immutable 长期缓存
- SPA fallback：`try_files ... /index.html`
- Service Worker：`registerType: autoUpdate`；用户刷新后获取新版本

## 回滚
1. 将 Deployment image 改回上一 SHA。
2. 若 SW 卡住：用户硬刷新或清站点数据；新 `sw.js` no-cache 保证下次加载更新。
3. 验证 `/healthz` 与主路径 smoke。
