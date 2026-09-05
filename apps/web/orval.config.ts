import { defineConfig } from 'orval';

export default defineConfig({
  rsp: {
    input: '../../api/openapi.yaml',
    output: {
      target: './src/api/generated/rsp.ts',
      schemas: './src/api/generated/models',
      client: 'fetch',
      clean: true,
      formatter: 'prettier',
    },
  },
});
