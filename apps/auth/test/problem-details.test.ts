import { afterAll, describe, expect, it } from 'vitest';

process.env.AUTH_DISABLE_LISTEN = 'true';
const { buildProblem } = await import('../src/server.js');
const { pool } = await import('../src/auth.js');

afterAll(async () => {
  await pool.end();
});

describe('auth problem details', () => {
  it('returns the complete stable application/problem+json contract', () => {
    expect(
      buildProblem(
        403,
        'reauthentication_required',
        'A fresh sign-in is required',
        'req-1',
        '/api/auth/connect-password',
      ),
    ).toEqual({
      type: 'https://rsp.example/problems/reauthentication_required',
      title: 'Forbidden',
      status: 403,
      detail: 'A fresh sign-in is required',
      instance: '/api/auth/connect-password',
      code: 'reauthentication_required',
      requestId: 'req-1',
      errors: [],
    });
  });
});
