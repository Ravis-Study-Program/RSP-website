import { defineConfig } from 'orval';

export default defineConfig({
  rsp: {
    input: '../../api/openapi.yaml',
    output: {
      target: './src/api/generated/rsp.ts',
      schemas: './src/api/generated/models',
      client: 'react-query',
      httpClient: 'fetch',
      clean: true,
      prettier: true,
      override: {
        mutator: {
          path: './src/api/orvalRequest.ts',
          name: 'orvalRequest',
        },
        query: {
          useQuery: true,
          signal: true,
        },
      },
    },
  },
});
