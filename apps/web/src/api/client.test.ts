import { clearAccessToken, getAccessToken } from '@/api/client';

function jwt(payload: Record<string, unknown>) {
  const encoded = btoa(JSON.stringify(payload))
    .replaceAll('+', '-')
    .replaceAll('/', '_')
    .replace(/=+$/, '');
  return `header.${encoded}.signature`;
}

function tokenResponse(token: string) {
  return new Response(JSON.stringify({ token }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('Better Auth access token acquisition', () => {
  afterEach(() => {
    clearAccessToken();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('uses the JWT plugin GET endpoint and caches the token in memory until its exp claim', async () => {
    const token = jwt({
      sub: 'auth-user',
      exp: Math.floor(Date.now() / 1000) + 300,
    });
    const fetchMock = vi.fn().mockResolvedValue(tokenResponse(token));
    vi.stubGlobal('fetch', fetchMock);

    await expect(getAccessToken()).resolves.toBe(token);
    await expect(getAccessToken()).resolves.toBe(token);

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith('/api/auth/token', {
      method: 'GET',
      credentials: 'include',
      headers: { Accept: 'application/json' },
    });
  });

  it('refreshes a token whose JWT expiry is inside the safety window', async () => {
    const expiring = jwt({ exp: Math.floor(Date.now() / 1000) + 10 });
    const replacement = jwt({ exp: Math.floor(Date.now() / 1000) + 300 });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(tokenResponse(expiring))
      .mockResolvedValueOnce(tokenResponse(replacement));
    vi.stubGlobal('fetch', fetchMock);

    await expect(getAccessToken()).resolves.toBe(expiring);
    await expect(getAccessToken()).resolves.toBe(replacement);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('rejects a token without a numeric exp claim', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(tokenResponse(jwt({ sub: 'auth-user' }))),
    );

    await expect(getAccessToken()).rejects.toMatchObject({
      problem: { code: 'invalid_auth_token' },
    });
  });
});
