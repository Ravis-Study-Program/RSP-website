import { afterAll, describe, expect, it } from 'vitest';

process.env.AUTH_DISABLE_LISTEN = 'true';
const { normalizedWebResponse } = await import('../src/server.js');
const { pool } = await import('../src/auth.js');

afterAll(async () => {
  await pool.end();
});

describe('native Better Auth response normalization', () => {
  it.each([
    [403, 'EMAIL_NOT_VERIFIED', 'Email not verified'],
    [400, 'PASSWORD_TOO_SHORT', 'Password too short'],
  ])(
    'normalizes %i %s while retaining the stable code',
    async (status, code, detail) => {
      const response = await normalizedWebResponse(
        Response.json({ code, message: detail }, { status }),
        'req-1',
        '/api/auth/sign-in/email',
      );
      expect(response.headers.get('content-type')).toContain(
        'application/problem+json',
      );
      await expect(response.json()).resolves.toMatchObject({
        type: `https://rsp.example/problems/${code}`,
        status,
        detail,
        instance: '/api/auth/sign-in/email',
        code,
        requestId: 'req-1',
        errors: [],
      });
    },
  );

  it('preserves retry and cookie headers for rate limits', async () => {
    const response = await normalizedWebResponse(
      Response.json(
        { code: 'TOO_MANY_REQUESTS', message: 'Too many requests' },
        {
          status: 429,
          headers: {
            'retry-after': '42',
            'set-cookie': 'challenge=kept; HttpOnly',
          },
        },
      ),
      'req-2',
      '/api/auth/sign-in/email',
    );
    expect(response.headers.get('retry-after')).toBe('42');
    expect(response.headers.get('set-cookie')).toContain('challenge=kept');
    expect(((await response.json()) as { title: string }).title).toBe(
      'Too Many Requests',
    );
  });
});
