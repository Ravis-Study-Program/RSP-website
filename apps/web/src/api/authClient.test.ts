import {
  connectPassword,
  sendVerificationEmail,
  signInWithEmail,
  signOut,
  signUpWithEmail,
} from '@/api/authClient';

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('Better Auth browser requests', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('posts email sign-in and sign-up to Better Auth with same-origin cookies', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ redirect: false }))
      .mockResolvedValueOnce(jsonResponse({ token: null }));
    vi.stubGlobal('fetch', fetchMock);

    await signInWithEmail(
      'member@example.test',
      'correct horse battery staple',
    );
    await signUpWithEmail(
      'RSP Member',
      'member@example.test',
      'correct horse battery staple',
    );

    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/auth/sign-in/email');
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: 'POST',
      credentials: 'include',
    });
    expect(
      JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body)),
    ).toMatchObject({
      email: 'member@example.test',
      rememberMe: true,
    });
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/auth/sign-up/email');
    expect(
      JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body)),
    ).toMatchObject({
      name: 'RSP Member',
      email: 'member@example.test',
    });
  });

  it('uses POST for verification resend and sign-out', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ status: true }))
      .mockResolvedValueOnce(jsonResponse({ success: true }));
    vi.stubGlobal('fetch', fetchMock);

    await sendVerificationEmail('member@example.test');
    await signOut();

    expect(
      fetchMock.mock.calls.map(([url, init]) => [url, init?.method]),
    ).toEqual([
      ['/api/auth/send-verification-email', 'POST'],
      ['/api/auth/sign-out', 'POST'],
    ]);
  });

  it('connects a password method with the exact fresh-session contract', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ status: true, reauthenticationRequired: true }),
      );
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      connectPassword('a sufficiently long password'),
    ).resolves.toEqual({ status: true, reauthenticationRequired: true });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/connect-password',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        body: JSON.stringify({ password: 'a sufficiently long password' }),
      }),
    );
  });
});
