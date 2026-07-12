/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string
  /** AP-063: enable Mock OAuth UI in development only (never production) */
  readonly VITE_AUTH_MOCK_OAUTH?: string
  readonly VITE_LOG_LEVEL?: string
  /** Build-time switch for deterministic Playwright hooks; never set in release builds. */
  readonly VITE_ENABLE_E2E_HOOKS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
