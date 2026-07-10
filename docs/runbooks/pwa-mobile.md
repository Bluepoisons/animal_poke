# 移动端 PWA 安装 / 安全区 / 性能预算

## 图标
- `frontend/public/icon.svg` + manifest icons
- 建议补充 192/512 PNG maskable（设计导出后放入 `public/icons/`）

## 安全区
- `.ap-root` 使用 `env(safe-area-inset-*)`

## 性能预算（发布）
- JS gzip 主包 < 250KB（见 rollup visualizer `dist/stats.html`）
- LCP < 2.5s（中端机）
- 安装包/缓存体积受 SW 控制；敏感 API 不缓存

## 验证
1. Chrome DevTools → Application → Manifest
2. Lighthouse PWA 项
3. iOS 主屏幕打开检查刘海安全区
