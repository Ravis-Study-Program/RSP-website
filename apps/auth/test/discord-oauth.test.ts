import { randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from 'vitest';
import { auth, pool } from '../src/auth.js';

// Real PostgreSQL, cookies, OAuth state, callbacks, and HTTP middleware. Only
// Discord's token/userinfo responses are substituted; no credentials are needed.
const live =
  process.env.AUTH_DISCORD_LIVE_TEST === 'true' ? describe : describe.skip;
const origin = 'http://127.0.0.1:3001';
const password = 'DiscordIntegration-Password-2026';
type Jar = Map<string, string>;
const profile = {
  id: '123456789012345678',
  email: 'discord-personal@example.test',
  verified: true,
  avatar: '0123456789abcdef0123456789abcdef',
};

live.sequential('Discord explicit linking and login', () => {
  let identity: Server;
  let close: (() => Promise<void>) | undefined;
  let resetRateLimits: (() => void) | undefined;
  let userId: string;
  let email: string;
  let failNextRevocation = false;
  const events: Array<{
    type: string;
    authUserId: string;
    avatarUrl?: string | null;
  }> = [];
  let discordProfile = { ...profile } as {
    id: string;
    email: string;
    verified: boolean;
    avatar: string | null;
  };

  async function call(path: string, jar: Jar = new Map(), body?: unknown) {
    const response = await fetch(`${origin}/api/auth${path}`, {
      method: body === undefined ? 'GET' : 'POST',
      redirect: 'manual',
      headers: {
        origin,
        cookie: [...jar].map(([key, value]) => `${key}=${value}`).join('; '),
        ...(body === undefined ? {} : { 'content-type': 'application/json' }),
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });
    for (const cookie of response.headers.getSetCookie()) {
      const pair = cookie.split(';')[0]!;
      const i = pair.indexOf('=');
      if (pair.slice(i + 1)) jar.set(pair.slice(0, i), pair.slice(i + 1));
      else jar.delete(pair.slice(0, i));
    }
    return response;
  }

  async function signIn() {
    const cookies: Jar = new Map();
    expect(
      (await call('/sign-in/email', cookies, { email, password })).status,
    ).toBe(200);
    return cookies;
  }

  async function oauth(cookies: Jar, link = false, provider = 'discord') {
    const start = await call(
      link ? '/link-social' : '/sign-in/social',
      cookies,
      {
        provider,
        callbackURL: '/dashboard',
        errorCallbackURL: '/sign-in',
        disableRedirect: true,
        // An explicit signup request must not override Discord's signup ban.
        requestSignUp: true,
      },
    );
    expect(start.status).toBe(200);
    const { url } = (await start.json()) as { url: string };
    const state = new URL(url).searchParams.get('state');
    return call(`/callback/${provider}?code=test-code&state=${state}`, cookies);
  }

  beforeAll(async () => {
    identity = createServer((request, response) => {
      let body = '';
      request.on('data', (chunk) => {
        body += String(chunk);
      });
      request.on('end', () => {
        const event = JSON.parse(body) as (typeof events)[number];
        if (failNextRevocation && event.type === 'sessions_revoked') {
          failNextRevocation = false;
          response.writeHead(503).end();
          return;
        }
        events.push(event);
        response.writeHead(204).end();
      });
    });
    await new Promise<void>((resolve) =>
      identity.listen(4000, '127.0.0.1', resolve),
    );
    const server = await import('../src/server.js');
    close = server.closeAuthServerForTest;
    resetRateLimits = server.resetAuthRateLimiterForTest;
  });

  beforeEach(async () => {
    const context = await auth.$context;
    for (const provider of context.socialProviders) {
      if (!['discord', 'google'].includes(provider.id)) continue;
      vi.spyOn(provider, 'validateAuthorizationCode').mockImplementation(
        async () => ({
          accessToken: 'test-token',
          scopes: ['identify', 'email'],
        }),
      );
      vi.spyOn(provider, 'getUserInfo').mockImplementation(async () => ({
        user: {
          name: 'Discord Name',
          email: discordProfile.email,
          emailVerified: discordProfile.verified,
          image: undefined,
        },
        data: { ...discordProfile, sub: discordProfile.id },
      }));
    }
    await pool.query('DELETE FROM rate_limits');
    resetRateLimits?.();
    discordProfile = { ...profile };
    events.length = 0;
    failNextRevocation = false;
    email = `discord-test-${randomUUID()}@example.test`;
    const signup = await call('/sign-up/email', new Map(), {
      name: 'RSP Name',
      email,
      password,
    });
    expect(signup.status).toBe(200);
    const result = await pool.query<{ id: string }>(
      'UPDATE users SET email_verified=true WHERE email=$1 RETURNING id',
      [email],
    );
    userId = result.rows[0]!.id;
  });

  afterEach(async () => {
    await pool.query(
      "DELETE FROM accounts WHERE provider_id='discord' AND user_id IN (SELECT id FROM users WHERE email LIKE 'discord-test-%@example.test')",
    );
  });

  afterAll(async () => {
    await close?.();
    await new Promise<void>((resolve) => identity?.close(() => resolve()));
    await pool.query(
      "DELETE FROM users WHERE email LIKE 'discord-test-%@example.test'",
    );
    await pool.end();
  });

  it('rejects new Discord users and never auto-links matching emails', async () => {
    let response = await oauth(new Map());
    expect(response.headers.get('location')).toContain('signup_disabled');
    discordProfile.email = email;
    response = await oauth(new Map());
    expect(response.headers.get('location')).toContain('account_not_linked');
    expect(
      (
        await pool.query(
          "SELECT id FROM accounts WHERE provider_id='discord' AND user_id=$1",
          [userId],
        )
      ).rows,
    ).toHaveLength(0);
  });

  it('requires a fresh authenticated session to connect', async () => {
    expect(
      (await call('/link-social', new Map(), { provider: 'discord' })).status,
    ).toBe(401);
    const cookies = await signIn();
    await pool.query(
      "UPDATE sessions SET created_at=now()-interval '6 minutes' WHERE user_id=$1",
      [userId],
    );
    expect(
      (await call('/link-social', cookies, { provider: 'discord' })).status,
    ).toBe(403);
  });

  it('links different emails, preserves the RSP identity, syncs the photo, then clears it on disconnect', async () => {
    const cookies = await signIn();
    const response = await oauth(cookies, true);
    expect(response.headers.get('location')).toBe('/dashboard');
    expect(
      (await pool.query('SELECT email,name FROM users WHERE id=$1', [userId]))
        .rows[0],
    ).toEqual({ email, name: 'RSP Name' });
    expect(
      (await pool.query('SELECT id FROM sessions WHERE user_id=$1', [userId]))
        .rows,
    ).toHaveLength(0);
    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'discord_avatar_updated',
        authUserId: userId,
        avatarUrl: expect.stringContaining(profile.avatar),
      }),
    );

    discordProfile.avatar = 'abcdef0123456789abcdef0123456789';
    const returning: Jar = new Map();
    expect((await oauth(returning)).headers.get('location')).toBe('/dashboard');
    const session = (await (await call('/get-session', returning)).json()) as {
      user: { id: string; email: string };
    };
    expect(session.user).toMatchObject({ id: userId, email });
    expect(events).toContainEqual(
      expect.objectContaining({
        avatarUrl: expect.stringContaining(discordProfile.avatar),
      }),
    );
    const accounts = (await (
      await call('/list-accounts', returning)
    ).json()) as Array<{ id: string; providerId: string }>;
    expect(
      (
        await call('/unlink-account', returning, {
          accountId: accounts.find((a) => a.providerId === 'credential')!.id,
        })
      ).status,
    ).toBe(400);
    expect(
      (
        await call('/unlink-account', returning, {
          accountId: accounts.find((a) => a.providerId === 'discord')!.id,
        })
      ).status,
    ).toBe(200);
    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'discord_avatar_updated',
        authUserId: userId,
        avatarUrl: null,
      }),
    );
    expect((await oauth(new Map())).headers.get('location')).not.toBe(
      '/dashboard',
    );
  });

  it('rejects an account linked to a different member and keeps Google email matching', async () => {
    const cookies = await signIn();
    const conflict = await pool.query<{ user_id: string }>(
      "SELECT user_id FROM accounts WHERE provider_id='credential' AND user_id<>$1 LIMIT 1",
      [userId],
    );
    await pool.query(
      "INSERT INTO accounts(id,account_id,provider_id,user_id,created_at,updated_at) VALUES($1,$2,'discord',$3,now(),now())",
      [randomUUID(), profile.id, conflict.rows[0]!.user_id],
    );
    expect((await oauth(cookies, true)).headers.get('location')).toContain(
      'account_already_linked_to_different_user',
    );
    expect(
      (await oauth(cookies, true, 'google')).headers.get('location'),
    ).toContain('email_does_not_match');
    await pool.query(
      "DELETE FROM accounts WHERE provider_id='discord' AND account_id=$1",
      [profile.id],
    );
  });

  it('clears the photo when unlinking succeeds but session revocation delivery fails', async () => {
    const cookies = await signIn();
    expect((await oauth(cookies, true)).headers.get('location')).toBe(
      '/dashboard',
    );
    const returning = await signIn();
    const accounts = (await (
      await call('/list-accounts', returning)
    ).json()) as Array<{
      id: string;
      providerId: string;
    }>;
    events.length = 0;
    failNextRevocation = true;
    const response = await call('/unlink-account', returning, {
      accountId: accounts.find((account) => account.providerId === 'discord')!
        .id,
    });
    expect(response.status).toBe(500);
    expect(
      (
        await pool.query(
          "SELECT id FROM accounts WHERE user_id=$1 AND provider_id='discord'",
          [userId],
        )
      ).rows,
    ).toHaveLength(0);
    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'discord_avatar_updated',
        authUserId: userId,
        avatarUrl: null,
      }),
    );
  });

  it('preserves matching-email Google linking, session revocation, and MFA', async () => {
    discordProfile.email = email;
    const cookies = await signIn();
    expect((await oauth(cookies, true, 'google')).headers.get('location')).toBe(
      '/dashboard',
    );
    expect(
      (await pool.query('SELECT id FROM sessions WHERE user_id=$1', [userId]))
        .rows,
    ).toHaveLength(0);
    await pool.query('UPDATE users SET two_factor_enabled=true WHERE id=$1', [
      userId,
    ]);
    const returning: Jar = new Map();
    expect(
      (await oauth(returning, false, 'google')).headers.get('location'),
    ).toContain('/two-factor');
    expect(await (await call('/get-session', returning)).json()).toBeNull();
  });

  it('clears removed Discord avatars and requires MFA on returning Discord login', async () => {
    const cookies = await signIn();
    discordProfile.avatar = null;
    expect((await oauth(cookies, true)).headers.get('location')).toBe(
      '/dashboard',
    );
    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'discord_avatar_updated',
        avatarUrl: null,
      }),
    );
    await pool.query('UPDATE users SET two_factor_enabled=true WHERE id=$1', [
      userId,
    ]);
    const returning: Jar = new Map();
    const response = await oauth(returning);
    expect(response.headers.get('location')).toContain('/two-factor');
    expect(
      (await pool.query('SELECT id FROM sessions WHERE user_id=$1', [userId]))
        .rows,
    ).toHaveLength(0);
    expect(await (await call('/get-session', returning)).json()).toBeNull();
  });

  it('refreshes the picture on a visit at most once a day, keeping tokens server-side', async () => {
    const cookies = await signIn();
    await oauth(cookies, true);
    const returning: Jar = new Map();
    expect((await oauth(returning)).headers.get('location')).toBe('/dashboard');
    const refresh = async () => {
      const response = await call('/discord/avatar/refresh', returning, {});
      expect(response.status).toBe(200);
      return ((await response.json()) as { result: string }).result;
    };
    const age = () =>
      pool.query(
        "UPDATE accounts SET updated_at=now()-interval '25 hours' WHERE user_id=$1 AND provider_id='discord'",
        [userId],
      );
    const avatarEvents = () =>
      events.filter((event) => event.type === 'discord_avatar_updated');

    // The Discord sign-in just now counts as a check.
    expect(await refresh()).toBe('skipped');
    // A day later, the same picture sends nothing.
    await age();
    const before = avatarEvents().length;
    expect(await refresh()).toBe('unchanged');
    expect(avatarEvents()).toHaveLength(before);
    // A new picture is sent once, and the next check waits another day.
    discordProfile.avatar = 'fedcba9876543210fedcba9876543210';
    await age();
    expect(await refresh()).toBe('updated');
    expect(avatarEvents().at(-1)).toMatchObject({
      authUserId: userId,
      avatarUrl: expect.stringContaining(discordProfile.avatar),
    });
    expect(await refresh()).toBe('skipped');
    // Discord failing keeps the current picture.
    const context = await auth.$context;
    const discord = context.socialProviders.find(
      (provider) => provider.id === 'discord',
    )!;
    vi.mocked(discord.getUserInfo).mockRejectedValueOnce(new Error('down'));
    await age();
    expect(await refresh()).toBe('unavailable');
    expect(avatarEvents().at(-1)!.avatarUrl).toContain(discordProfile.avatar);

    // Signed out, and the stored tokens are never handed to the browser.
    expect((await call('/discord/avatar/refresh', new Map(), {})).status).toBe(
      401,
    );
    for (const path of ['/get-access-token', '/refresh-token'])
      expect((await call(path, returning, { accountId: 'any' })).status).toBe(
        404,
      );
  });

  it('skips members without Discord', async () => {
    const response = await call('/discord/avatar/refresh', await signIn(), {});
    expect(await response.json()).toEqual({ result: 'skipped' });
  });
});
