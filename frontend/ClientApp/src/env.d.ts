/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}

interface ViteTypeOptions {
  strictImportMetaEnv: unknown
}

interface ImportMetaEnv {
  readonly DCS_API_PATH: string
  readonly DCS_UI_PATH: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
