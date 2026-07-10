import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    exclude: ['**/node_modules/**', '**/dist/**', '**/e2e/**', 'e2e/**'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: [
        'src/features/animal-poke/**/*.{ts,tsx}',
        'src/services/visionDetect.ts',
        'src/services/capturePipeline.ts',
        'src/services/syncQueue.ts',
        'src/auth/deviceAuth.ts',
        'src/compliance/**/*.{ts,tsx}',
        'src/capture/session.ts',
      ],
      thresholds: {
        lines: 40,
        functions: 40,
        statements: 40,
      },
    },
  },
})
