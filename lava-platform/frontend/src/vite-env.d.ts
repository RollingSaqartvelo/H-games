/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE?: string
  readonly VITE_WS_URL?: string
  readonly VITE_DEV_TOKEN?: string
  readonly VITE_API_TARGET?: string
  readonly VITE_WS_TARGET?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
