import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import compression from 'vite-plugin-compression'
import { visualizer } from 'rollup-plugin-visualizer'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      strategies: 'generateSW',
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        maximumFileSizeToCacheInBytes: 5 * 1024 * 1024,
        runtimeCaching: [
          // 禁止缓存鉴权/敏感 API（auth/vision/value/sync/privacy/security/commerce/errors）
          // 不对 /api/ 做通用 NetworkFirst，避免跨设备复用 Token 响应。
          // 仅允许明确公开且安全的 GET（当前无公开业务缓存资源）。
          {
            urlPattern: /\.(?:png|jpg|jpeg|svg|webp)$/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'image-cache',
              expiration: { maxEntries: 100, maxAgeSeconds: 7 * 86400 },
            },
          },
          {
            urlPattern: /\.(?:woff2?|ttf|otf)$/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'font-cache',
              expiration: { maxEntries: 10, maxAgeSeconds: 30 * 86400 },
            },
          },
          {
            urlPattern: /\.(?:js|css)$/,
            handler: 'StaleWhileRevalidate',
            options: {
              cacheName: 'static-cache',
              expiration: { maxEntries: 50, maxAgeSeconds: 7 * 86400 },
            },
          },
        ],
      },
      manifest: {
        name: 'AnimalPoke',
        short_name: 'AnimalPoke',
        description: 'LBS animal collection game',
        display: 'standalone',
        background_color: '#FFF8F0',
        theme_color: '#FF8C42',
        icons: [
          { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml' },
        ],
      },
    }),
    compression({
      algorithm: 'gzip',
      threshold: 10240,
    }),
    compression({
      algorithm: 'brotliCompress',
      threshold: 10240,
    }),
    visualizer({
      open: false,
      filename: 'dist/stats.html',
      gzipSize: true,
      brotliSize: true,
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    target: 'es2020',
    cssCodeSplit: true,
    minify: 'esbuild',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom'],
          'idb-vendor': ['idb'],
        },
      },
    },
  },
})
