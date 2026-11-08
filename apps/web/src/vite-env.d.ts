/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_USE_DEMO_DATA?: string;
  readonly VITE_RSP_CANARY?: string;
  readonly VITE_SHOW_SIDEBAR_SIGN_OUT?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
