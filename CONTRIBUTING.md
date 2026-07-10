# Contributing

## 开发流程
1. 从 `main` 拉分支；`git pull --rebase` 保持线性历史。
2. Issue 驱动：PR 描述写 `Fixes #N`。
3. 本地验证：
   - 后端：`cd backend && go test ./...`
   - 前端：`cd frontend && npm run build && npm test`
4. 提交前确认无 `.env`、对话日志、工具记忆进入变更。

## 代码归属
见 `.github/CODEOWNERS`。

## API 变更
- 先改 `docs/openapi.yaml`，再生成 `frontend/src/api/generated/schema.d.ts`。
- 破坏性变更需升 major 或保留兼容字段至少一个版本。

## 安全
见 `SECURITY.md`。
