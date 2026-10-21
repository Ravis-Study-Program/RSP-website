/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_USE_DEMO_DATA?: string;
  readonly VITE_RSP_CANARY?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
