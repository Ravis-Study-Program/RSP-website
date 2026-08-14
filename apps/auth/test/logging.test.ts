import { describe, expect, it } from 'vitest';

import { sanitizeLogValue } from '../src/logging.js';

describe('structured log redaction', () => {
  it('removes nested secrets, PII, provider credentials, and URL query capabilities', () => {
    const serialized = JSON.stringify(
      sanitizeLogValue({
        password: 'SENTINEL_PASSWORD',
        nested: {
          authorization: 'Bearer SENTINEL_TOKEN',
          refreshToken: 'SENTINEL_REFRESH',
          message:
            'failure for sentinel.person@example.test at https://rsp.test/reset?token=SENTINEL_QUERY',
        },
        error: new Error(
          'token SENTINEL_TOKEN for sentinel.person@example.test',
        ),
        details: ['SENTINEL_RAW_PROVIDER_TOKEN', 'SENTINEL_RAW_PASSWORD'],
      }),
    );
    for (const sentinel of [
      'SENTINEL_PASSWORD',
      'SENTINEL_TOKEN',
      'SENTINEL_REFRESH',
      'SENTINEL_QUERY',
      'sentinel.person@example.test',
      'SENTINEL_RAW_PROVIDER_TOKEN',
      'SENTINEL_RAW_PASSWORD',
    ])
      expect(serialized).not.toContain(sentinel);
    expect(serialized).toContain('[redacted]');
  });
});
