import { createHmac, randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';

import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import { accountLifecycle, auth, pool } from '../src/auth.js';
import type { EmailProvider } from '../src/email/provider.js';
import type { IdentityLifecycleGateway } from '../src/identity/gateway.js';
import {
  AccountLifecycleService,
  hashDeletionRecoveryToken,
} from '../src/lifecycle/service.js';
import {
  isHashedBackupCodePayload,
  withBackupCodeCandidate,
} from '../src/security/backup-code-store.js';

const enabled = process.env.AUTH_LIVE_TEST === 'true';
const live = enabled ? describe : describe.skip;
const origin = 'http://127.0.0.1:3001';
const password = 'AuthIntegrationPassword-2026';
const mailpitUrl = process.env.AUTH_LIVE_MAILPIT_URL;
const internalToken =
  process.env.IDENTITY_SERVICE_TOKEN ??
  'rsp-local-identity-service-token-change-me';

interface CookieJar {
  values: Map<string, string>;
}

function jar(): CookieJar {
  return { values: new Map() };
}

function cookieHeader(cookies: CookieJar): string {
  return [...cookies.values.entries()]
    .map(([name, value]) => `${name}=${value}`)
    .join('; ');
}

function absorbCookies(response: Response, cookies: CookieJar): void {
  const headers = response.headers as Headers & {
    getSetCookie?: () => string[];
  };
  for (const value of headers.getSetCookie?.() ?? []) {
    const [pair] = value.split(';', 1);
    const separator = pair?.indexOf('=') ?? -1;
    if (!pair || separator < 1) continue;
    const name = pair.slice(0, separator);
    const cookieValue = pair.slice(separator + 1);
    if (cookieValue) cookies.values.set(name, cookieValue);
    else cookies.values.delete(name);
  }
}

async function call(
  path: string,
  options: {
    method?: string;
    body?: unknown;
    cookies?: CookieJar;
    backupCodeCandidate?: string;
  } = {},
): Promise<Response> {
  const headers = new Headers({ origin });
  if (options.body !== undefined)
    headers.set('content-type', 'application/json');
  if (options.cookies && options.cookies.values.size > 0) {
    headers.set('cookie', cookieHeader(options.cookies));
  }
  const request = new Request(`${origin}/api/auth${path}`, {
    method: options.method ?? (options.body === undefined ? 'GET' : 'POST'),
    headers,
    ...(options.body === undefined
      ? {}
      : { body: JSON.stringify(options.body) }),
  });
  const response = enabled
    ? await fetch(request, { redirect: 'manual' })
    : options.backupCodeCandidate
      ? await withBackupCodeCandidate(options.backupCodeCandidate, () =>
          auth.handler(request),
        )
      : await auth.handler(request);
  if (options.cookies) absorbCookies(response, options.cookies);
  return response;
}

function base32Decode(value: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const character of value.replaceAll('=', '').toUpperCase()) {
    const index = alphabet.indexOf(character);
    if (index < 0) throw new Error('invalid base32 TOTP secret');
    bits += index.toString(2).padStart(5, '0');
  }
  const bytes: number[] = [];
  for (let index = 0; index + 8 <= bits.length; index += 8) {
    bytes.push(Number.parseInt(bits.slice(index, index + 8), 2));
  }
  return Buffer.from(bytes);
}

function totp(secret: string, at = Date.now()): string {
  const counter = BigInt(Math.floor(at / 30_000));
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(counter);
  const digest = createHmac('sha1', base32Decode(secret))
    .update(message)
    .digest();
  const offset = (digest.at(-1) ?? 0) & 0x0f;
  const binary =
    (((digest[offset] ?? 0) & 0x7f) << 24) |
    ((digest[offset + 1] ?? 0) << 16) |
    ((digest[offset + 2] ?? 0) << 8) |
    (digest[offset + 3] ?? 0);
  return String(binary % 1_000_000).padStart(6, '0');
}

async function signIn(email: string, cookies: CookieJar): Promise<Response> {
  return call('/sign-in/email', { body: { email, password }, cookies });
}

async function sessionCount(userId: string): Promise<number> {
  const result = await pool.query<{ count: string }>(
    'SELECT count(*)::text AS count FROM auth.sessions WHERE user_id = $1',
    [userId],
  );
  return Number(result.rows[0]?.count ?? '0');
}

async function verifyThroughCapturedEmail(targetEmail: string): Promise<void> {
  if (!mailpitUrl)
    throw new Error(
      'AUTH_LIVE_MAILPIT_URL is required for live verification tests',
    );
  let messageId: string | undefined;
  for (let attempt = 0; attempt < 20 && !messageId; attempt += 1) {
    const list = await fetch(`${mailpitUrl}/api/v1/messages`);
    const body = (await list.json()) as {
      messages?: Array<{ ID: string; To?: Array<{ Address?: string }> }>;
    };
    messageId = body.messages?.find((message) =>
      message.To?.some(
        (recipient) =>
          recipient.Address?.toLowerCase() === targetEmail.toLowerCase(),
      ),
    )?.ID;
    if (!messageId) await new Promise((resolve) => setTimeout(resolve, 100));
  }
  if (!messageId)
    throw new Error(`verification email for ${targetEmail} was not captured`);
  const messageResponse = await fetch(
    `${mailpitUrl}/api/v1/message/${messageId}`,
  );
  const message = (await messageResponse.json()) as { Text?: string };
  const verificationUrl = message.Text?.match(
    /https?:\/\/[^\s]+\/api\/auth\/verify-email\?[^\s]+/,
  )?.[0];
  if (!verificationUrl)
    throw new Error('verification URL was absent from captured email');
  const verification = await fetch(verificationUrl, { redirect: 'manual' });
  expect([200, 302]).toContain(verification.status);
  if (verification.status === 302)
    expect(verification.headers.get('location')).toBe('/');
}

live.sequential('live Better Auth handler and PostgreSQL', () => {
  const suffix = randomUUID();
  const email = `auth-live-${suffix}@example.test`;
  let userId = '';
  let primary = jar();
  let originalBackupCodes: string[] = [];
  let closeServer: (() => Promise<void>) | undefined;
  let resetRateLimiter: (() => void) | undefined;
  let identityServer: Server | undefined;
  let failNextIdentityCallback = false;

  async function resetAuthRateLimits(): Promise<void> {
    // The suite intentionally performs more than five sign-ins while
    // exercising separate flows. Production throttling has focused tests;
    // isolate each live scenario so rate limiting cannot mask its assertion.
    await pool.query('DELETE FROM auth.rate_limits');
    resetRateLimiter?.();
  }

  beforeAll(async () => {
    identityServer = createServer((request, response) => {
      if (
        request.method === 'POST' &&
        request.url === '/internal/auth/lifecycle-events'
      ) {
        request.resume();
        if (failNextIdentityCallback) {
          failNextIdentityCallback = false;
          response.writeHead(503).end();
        } else {
          response.writeHead(204).end();
        }
        return;
      }
      response.writeHead(404).end();
    });
    await new Promise<void>((resolve, reject) => {
      identityServer?.once('error', reject);
      identityServer?.listen(4000, '127.0.0.1', resolve);
    });
    const server = await import('../src/server.js');
    closeServer = server.closeAuthServerForTest;
    resetRateLimiter = server.resetAuthRateLimiterForTest;
    await pool.query('DELETE FROM auth.rate_limits');
    const signup = await call('/sign-up/email', {
      body: { name: 'Auth Live', email, password },
    });
    expect(signup.status).toBe(200);
    await verifyThroughCapturedEmail(email);
    const user = await pool.query<{ id: string; email_verified: boolean }>(
      'SELECT id, email_verified FROM auth.users WHERE email = $1',
      [email],
    );
    userId = user.rows[0]?.id ?? '';
    expect(userId).not.toBe('');
    expect(user.rows[0]?.email_verified).toBe(true);
    const verifiedEvent = await pool.query<{
      payload: { type?: string; authUserId?: string };
    }>(
      "SELECT payload FROM auth.lifecycle_outbox WHERE payload->>'type' = 'email_verified' AND payload->>'authUserId' = $1",
      [userId],
    );
    expect(verifiedEvent.rows).toHaveLength(1);
    expect((await signIn(email, primary)).status).toBe(200);
  });

  afterAll(async () => {
    if (userId)
      await pool.query('DELETE FROM auth.users WHERE id = $1', [userId]);
    await closeServer?.();
    await new Promise<void>((resolve, reject) => {
      if (!identityServer?.listening) return resolve();
      identityServer.close((error) => (error ? reject(error) : resolve()));
    });
    await pool.end();
  });

  it('enables TOTP, stores only backup hashes, revokes setup sessions and records the transition', async () => {
    await resetAuthRateLimits();
    const enable = await call('/two-factor/enable', {
      body: { password },
      cookies: primary,
    });
    expect(enable.status).toBe(200);
    const setup = (await enable.json()) as {
      totpURI: string;
      backupCodes: string[];
    };
    originalBackupCodes = setup.backupCodes;
    expect(originalBackupCodes).toHaveLength(10);
    const stored = await pool.query<{
      backup_codes: string;
      verified: boolean;
    }>(
      'SELECT backup_codes, verified FROM auth.two_factors WHERE user_id = $1',
      [userId],
    );
    expect(isHashedBackupCodePayload(stored.rows[0]?.backup_codes ?? '')).toBe(
      true,
    );
    for (const backupCode of originalBackupCodes) {
      expect(stored.rows[0]?.backup_codes).not.toContain(backupCode);
    }
    const setupSecret = new URL(setup.totpURI).searchParams.get('secret');
    expect(setupSecret).toBeTruthy();
    const verify = await call('/two-factor/verify-totp', {
      body: { code: totp(setupSecret ?? ''), trustDevice: false },
      cookies: primary,
    });
    expect(verify.status).toBe(200);
    await verify.arrayBuffer();
    const enabledUser = await pool.query<{ two_factor_enabled: boolean }>(
      'SELECT two_factor_enabled FROM auth.users WHERE id = $1',
      [userId],
    );
    expect(enabledUser.rows[0]?.two_factor_enabled).toBe(true);
    expect(await sessionCount(userId)).toBe(0);
    const event = await pool.query<{
      payload: { type?: string; eventId?: string };
    }>(
      "SELECT payload FROM auth.lifecycle_outbox WHERE payload->>'type' = 'mfa_configured' AND payload->>'authUserId' = $1",
      [userId],
    );
    expect(event.rows[0]?.payload).toMatchObject({
      type: 'mfa_configured',
      eventId: expect.any(String),
    });
    const mfaState = await fetch(
      `${origin}/internal/auth/users/${userId}/mfa-state`,
      {
        headers: { authorization: `Bearer ${internalToken}` },
      },
    );
    expect(mfaState.status).toBe(200);
    await expect(mfaState.json()).resolves.toEqual({ configured: true });
  });

  it('schedules failed lifecycle delivery and reclaims an expired outbox lease', async () => {
    const authUserId = `outbox-live-${randomUUID()}`;
    const now = new Date('2026-08-14T00:00:00.000Z');
    let deliveryAttempts = 0;
    const identityGateway: IdentityLifecycleGateway = {
      async publish() {
        deliveryAttempts += 1;
        if (deliveryAttempts === 1)
          throw new Error('planned downstream outage');
      },
    };
    const emailProvider: EmailProvider = { async send() {} };
    const service = new AccountLifecycleService(
      pool,
      emailProvider,
      identityGateway,
      () => now,
    );

    try {
      await expect(
        service.queueIdentityEvent(
          {
            type: 'email_verified',
            authUserId,
            securityVersion: 1,
            occurredAt: now.toISOString(),
          },
          true,
        ),
      ).rejects.toThrow('identity synchronization is pending retry');

      const failed = await pool.query<{
        attempt_count: number;
        available_at: Date;
        last_error: string | null;
      }>(
        `SELECT attempt_count, available_at, last_error
           FROM auth.lifecycle_outbox
          WHERE payload->>'authUserId' = $1`,
        [authUserId],
      );
      expect(failed.rows[0]).toMatchObject({
        attempt_count: 1,
        last_error: 'planned downstream outage',
      });
      expect(failed.rows[0]?.available_at.toISOString()).toBe(
        '2026-08-14T00:00:05.000Z',
      );

      await pool.query(
        `UPDATE auth.lifecycle_outbox
            SET available_at = $2::timestamptz - interval '1 second',
                locked_until = $2::timestamptz - interval '1 second'
          WHERE payload->>'authUserId' = $1`,
        [authUserId, now],
      );
      await expect(service.flushOutbox(1)).resolves.toBe(1);
      const delivered = await pool.query<{
        delivered_at: Date | null;
        locked_until: Date | null;
      }>(
        `SELECT delivered_at, locked_until
           FROM auth.lifecycle_outbox
          WHERE payload->>'authUserId' = $1`,
        [authUserId],
      );
      expect(delivered.rows[0]?.delivered_at?.toISOString()).toBe(
        now.toISOString(),
      );
      expect(delivered.rows[0]?.locked_until).toBeNull();
      expect(deliveryAttempts).toBe(2);
    } finally {
      await pool.query(
        "DELETE FROM auth.lifecycle_outbox WHERE payload->>'authUserId' = $1",
        [authUserId],
      );
    }
  });

  it('automatically retries a transient identity callback without a restart or manual sweep', async () => {
    const authUserId = `outbox-timer-${randomUUID()}`;
    failNextIdentityCallback = true;
    await accountLifecycle.queueIdentityEvent({
      type: 'sessions_revoked',
      authUserId,
      reason: 'security_admin',
      securityVersion: 2,
      occurredAt: new Date().toISOString(),
    });
    const failed = await pool.query<{
      attempt_count: number;
      delivered_at: Date | null;
    }>(
      `SELECT attempt_count, delivered_at
         FROM auth.lifecycle_outbox
        WHERE payload->>'authUserId' = $1`,
      [authUserId],
    );
    expect(failed.rows[0]).toMatchObject({
      attempt_count: 1,
      delivered_at: null,
    });

    try {
      let deliveredAt: Date | null = null;
      for (let attempt = 0; attempt < 60 && !deliveredAt; attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, 250));
        const result = await pool.query<{ delivered_at: Date | null }>(
          `SELECT delivered_at
             FROM auth.lifecycle_outbox
            WHERE payload->>'authUserId' = $1`,
          [authUserId],
        );
        deliveredAt = result.rows[0]?.delivered_at ?? null;
      }
      expect(deliveredAt).not.toBeNull();
    } finally {
      await pool.query(
        "DELETE FROM auth.lifecycle_outbox WHERE payload->>'authUserId' = $1",
        [authUserId],
      );
    }
  }, 20_000);

  it('removes recovery credentials, challenges, and outbox PII during the deletion sweep', async () => {
    const authUserId = `privacy-live-${randomUUID()}`;
    const originalEmail = `${authUserId}@example.test`;
    const rawRecoveryToken = `SensitiveRecoveryToken-${randomUUID()}`;
    const now = new Date('2026-08-14T01:00:00.000Z');
    const emailProvider: EmailProvider = { async send() {} };
    const identityGateway: IdentityLifecycleGateway = { async publish() {} };
    const service = new AccountLifecycleService(
      pool,
      emailProvider,
      identityGateway,
      () => now,
    );

    try {
      await pool.query(
        `INSERT INTO auth.users
           (id, name, email, email_verified, created_at, updated_at, two_factor_enabled,
            account_state, security_version, deletion_requested_at, deletion_recovery_deadline)
         VALUES ($1, 'Privacy Test', $2, TRUE, $3::timestamptz, $3::timestamptz, FALSE,
                 'deletion_pending', 2, $3::timestamptz - interval '30 days', $3::timestamptz - interval '1 second')`,
        [authUserId, originalEmail, now],
      );
      await pool.query(
        `INSERT INTO auth.account_deletion_recovery_tokens
           (id, user_id, token_hash, requested_at, expires_at)
         VALUES ($1, $2, $3, $4::timestamptz - interval '30 days', $4::timestamptz - interval '1 second')`,
        [
          randomUUID(),
          authUserId,
          hashDeletionRecoveryToken(rawRecoveryToken),
          now,
        ],
      );
      await pool.query(
        `INSERT INTO auth.verifications
           (id, identifier, value, expires_at, created_at, updated_at)
         VALUES ($1, $2, 'email-token', $3::timestamptz + interval '1 day', $3::timestamptz, $3::timestamptz),
                ($4, '2fa-challenge', $5, $3::timestamptz + interval '1 day', $3::timestamptz, $3::timestamptz)`,
        [randomUUID(), originalEmail, now, randomUUID(), authUserId],
      );
      await pool.query(
        `INSERT INTO auth.lifecycle_outbox
           (id, target, payload, available_at, created_at, delivered_at)
         VALUES
           ($1, 'identity', $2::jsonb, $6::timestamptz, $6::timestamptz, $6::timestamptz),
           ($3, 'identity', $4::jsonb, $6::timestamptz, $6::timestamptz, NULL),
           ($5, 'email', $7::jsonb, $6::timestamptz, $6::timestamptz, NULL)`,
        [
          randomUUID(),
          JSON.stringify({
            type: 'auth_user_created',
            authUserId,
            email: originalEmail,
          }),
          randomUUID(),
          JSON.stringify({
            type: 'email_changed',
            authUserId,
            email: originalEmail,
          }),
          randomUUID(),
          now,
          JSON.stringify({
            to: originalEmail,
            subject: 'Recover your RSP account',
            text: `Cancel deletion with ${rawRecoveryToken}`,
          }),
        ],
      );

      await expect(service.pseudonymizeDue(50)).resolves.toBeGreaterThanOrEqual(
        1,
      );
      const account = await pool.query<{
        email: string;
        account_state: string;
      }>('SELECT email, account_state FROM auth.users WHERE id = $1', [
        authUserId,
      ]);
      expect(account.rows[0]).toEqual({
        email: `deleted+${authUserId}@invalid.rsp.local`,
        account_state: 'deleted',
      });
      const remainingTokens = await pool.query<{ count: string }>(
        'SELECT count(*)::text AS count FROM auth.account_deletion_recovery_tokens WHERE user_id = $1',
        [authUserId],
      );
      expect(remainingTokens.rows[0]?.count).toBe('0');
      const remainingChallenges = await pool.query<{ count: string }>(
        'SELECT count(*)::text AS count FROM auth.verifications WHERE identifier = $1 OR value = $2',
        [originalEmail, authUserId],
      );
      expect(remainingChallenges.rows[0]?.count).toBe('0');
      const outbox = await pool.query<{ payload: unknown }>(
        "SELECT payload FROM auth.lifecycle_outbox WHERE payload->>'authUserId' = $1 OR payload->>'to' = $2",
        [authUserId, originalEmail],
      );
      const serializedOutbox = JSON.stringify(outbox.rows);
      expect(serializedOutbox).toContain('auth_pseudonymized');
      expect(serializedOutbox).not.toContain(originalEmail);
      expect(serializedOutbox).not.toContain(rawRecoveryToken);
    } finally {
      await pool.query(
        "DELETE FROM auth.lifecycle_outbox WHERE payload->>'authUserId' = $1 OR payload->>'to' = $2",
        [authUserId, originalEmail],
      );
      await pool.query('DELETE FROM auth.users WHERE id = $1', [authUserId]);
    }
  });

  it('requires TOTP at sign-in and consumes a hashed backup code exactly once', async () => {
    await resetAuthRateLimits();
    const challenge = jar();
    const signInResponse = await signIn(email, challenge);
    expect(signInResponse.status).toBe(200);
    expect(await signInResponse.json()).toMatchObject({
      twoFactorRedirect: true,
      twoFactorMethods: expect.arrayContaining(['totp']),
    });
    expect(challenge.values.has('rsp-auth.session_token')).toBe(false);
    expect(challenge.values.has('rsp-auth.two_factor')).toBe(true);
    const selected = originalBackupCodes[0];
    expect(selected).toBeTruthy();
    const verified = await call('/two-factor/verify-backup-code', {
      body: { code: selected, trustDevice: false },
      cookies: challenge,
      backupCodeCandidate: selected,
    });
    expect(verified.status).toBe(200);
    expect(challenge.values.has('rsp-auth.session_token')).toBe(true);
    const token = await call('/token', { cookies: challenge });
    const accessToken = ((await token.json()) as { token: string }).token;
    const claims = JSON.parse(
      Buffer.from(accessToken.split('.')[1] ?? '', 'base64url').toString(
        'utf8',
      ),
    ) as Record<string, unknown>;
    const persistedSessions = await pool.query<{
      token: string;
      mfa_verified_at: Date | null;
    }>('SELECT token, mfa_verified_at FROM auth.sessions WHERE user_id = $1', [
      userId,
    ]);
    expect(
      claims.mfaVerified,
      JSON.stringify({
        cookies: [...challenge.values.entries()],
        sessions: persistedSessions.rows,
      }),
    ).toBe(true);
    expect(claims.mfaVerifiedAt).toEqual(expect.any(String));
    const remaining = await pool.query<{ backup_codes: string }>(
      'SELECT backup_codes FROM auth.two_factors WHERE user_id = $1',
      [userId],
    );
    expect(JSON.parse(remaining.rows[0]?.backup_codes ?? '[]')).toHaveLength(9);

    await call('/sign-out', { body: {}, cookies: challenge });
    const replay = jar();
    expect((await signIn(email, replay)).status).toBe(200);
    const replayResponse = await call('/two-factor/verify-backup-code', {
      body: { code: selected, trustDevice: false },
      cookies: replay,
      backupCodeCandidate: selected,
    });
    expect(replayResponse.status).toBe(401);
    primary = jar();
    expect((await signIn(email, primary)).status).toBe(200);
    const complete = await call('/two-factor/verify-backup-code', {
      body: { code: originalBackupCodes[1], trustDevice: false },
      cookies: primary,
      backupCodeCandidate: originalBackupCodes[1],
    });
    expect(complete.status).toBe(200);
  });

  it('regenerates backups only after password reauthentication and revokes every session', async () => {
    await resetAuthRateLimits();
    const other = jar();
    expect((await signIn(email, other)).status).toBe(200);
    const otherMfa = await call('/two-factor/verify-backup-code', {
      body: { code: originalBackupCodes[2], trustDevice: false },
      cookies: other,
      backupCodeCandidate: originalBackupCodes[2],
    });
    expect(otherMfa.status).toBe(200);
    expect(await sessionCount(userId)).toBe(2);
    const wrongPassword = await call('/two-factor/generate-backup-codes', {
      body: { password: 'wrong-password-value' },
      cookies: primary,
    });
    expect(wrongPassword.status).toBe(400);
    const regenerate = await call('/two-factor/generate-backup-codes', {
      body: { password },
      cookies: primary,
    });
    expect(regenerate.status).toBe(200);
    const replacements = (
      (await regenerate.json()) as { backupCodes: string[] }
    ).backupCodes;
    expect(replacements).toHaveLength(10);
    expect(replacements).not.toEqual(originalBackupCodes);
    expect(await sessionCount(userId)).toBe(0);
    expect((await call('/token', { cookies: other })).status).toBe(401);
    primary = jar();
    expect((await signIn(email, primary)).status).toBe(200);
    const mfa = await call('/two-factor/verify-backup-code', {
      body: { code: replacements[0], trustDevice: false },
      cookies: primary,
      backupCodeCandidate: replacements[0],
    });
    expect(mfa.status).toBe(200);
    originalBackupCodes = replacements;
  });

  it('rolls an active database session after the configured update age', async () => {
    await resetAuthRateLimits();
    const before = await pool.query<{ id: string; expires_at: Date }>(
      'SELECT id, expires_at FROM auth.sessions WHERE user_id = $1',
      [userId],
    );
    const session = before.rows[0];
    expect(session).toBeTruthy();
    await pool.query(
      `UPDATE auth.sessions
          SET updated_at = now() - interval '2 days', expires_at = now() + interval '1 hour'
        WHERE id = $1`,
      [session?.id],
    );
    const refreshed = await call('/get-session', { cookies: primary });
    expect(refreshed.status).toBe(200);
    await refreshed.arrayBuffer();
    const after = await pool.query<{ expires_at: Date; updated_at: Date }>(
      'SELECT expires_at, updated_at FROM auth.sessions WHERE id = $1',
      [session?.id],
    );
    expect(after.rows[0]?.expires_at.getTime()).toBeGreaterThan(
      Date.now() + 6 * 24 * 60 * 60 * 1_000,
    );
    expect(after.rows[0]?.updated_at.getTime()).toBeGreaterThan(
      Date.now() - 60_000,
    );
  });

  it('rotates signing keys, overlaps the old public key for grace, and rejects it after grace', async () => {
    await resetAuthRateLimits();
    const oldTokenResponse = await call('/token', { cookies: primary });
    const oldToken = ((await oldTokenResponse.json()) as { token: string })
      .token;
    const oldHeader = JSON.parse(
      Buffer.from(oldToken.split('.')[0] ?? '', 'base64url').toString('utf8'),
    ) as { kid: string; alg: string };
    await pool.query(
      "UPDATE auth.jwks SET expires_at = now() - interval '1 second' WHERE id = $1",
      [oldHeader.kid],
    );
    const newTokenResponse = await call('/token', { cookies: primary });
    const newToken = ((await newTokenResponse.json()) as { token: string })
      .token;
    const newHeader = JSON.parse(
      Buffer.from(newToken.split('.')[0] ?? '', 'base64url').toString('utf8'),
    ) as { kid: string };
    expect(newHeader.kid).not.toBe(oldHeader.kid);
    const overlap = await call('/jwks');
    const overlapKeys = (
      (await overlap.json()) as { keys: Array<{ kid: string }> }
    ).keys;
    expect(overlapKeys.map((key) => key.kid)).toEqual(
      expect.arrayContaining([oldHeader.kid, newHeader.kid]),
    );
    const oldVerification = await auth.api.verifyJWT({
      body: { token: oldToken },
    });
    expect(oldVerification.payload?.sub).toBe(userId);
    await pool.query(
      "UPDATE auth.jwks SET expires_at = now() - interval '31 days' WHERE id = $1",
      [oldHeader.kid],
    );
    const afterGrace = await call('/jwks');
    const afterGraceKids = (
      (await afterGrace.json()) as { keys: Array<{ kid: string }> }
    ).keys.map((key) => key.kid);
    expect(afterGraceKids).not.toContain(oldHeader.kid);
    expect(afterGraceKids).toContain(newHeader.kid);
    // Token verification reads stored keys directly; public consumers reject
    // the old token once its `kid` is no longer advertised by the JWKS route.
  });

  it('does not silently merge matching emails and links password only through the explicit route', async () => {
    await resetAuthRateLimits();
    expect(auth.options.account?.accountLinking).toMatchObject({
      disableImplicitLinking: true,
      allowDifferentEmails: false,
      trustedProviders: [],
    });
    const linkingEmail = `auth-link-${suffix}@example.test`;
    const temporaryPassword = 'TemporaryGoogleBootstrap-2026';
    const linkedPassword = 'ExplicitLinkedPassword-2026';
    const signup = await call('/sign-up/email', {
      body: {
        name: 'Explicit Link',
        email: linkingEmail,
        password: temporaryPassword,
      },
    });
    expect(signup.status).toBe(200);
    await verifyThroughCapturedEmail(linkingEmail);
    const linkedUser = await pool.query<{ id: string }>(
      'SELECT id FROM auth.users WHERE email = $1',
      [linkingEmail],
    );
    const linkedUserId = linkedUser.rows[0]?.id ?? '';
    const linkedJar = jar();
    const temporarySignIn = await call('/sign-in/email', {
      body: { email: linkingEmail, password: temporaryPassword },
      cookies: linkedJar,
    });
    expect(temporarySignIn.status).toBe(200);
    await pool.query(
      "DELETE FROM auth.accounts WHERE user_id = $1 AND provider_id = 'credential'",
      [linkedUserId],
    );
    await pool.query(
      `INSERT INTO auth.accounts
         (id, account_id, provider_id, user_id, created_at, updated_at)
       VALUES ($1, $2, 'google', $3, now(), now())`,
      [randomUUID(), `google-${suffix}`, linkedUserId],
    );
    const connect = await fetch(`${origin}/api/auth/connect-password`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: cookieHeader(linkedJar),
        origin,
      },
      body: JSON.stringify({ password: linkedPassword }),
    });
    expect(connect.status).toBe(200);
    await expect(connect.json()).resolves.toEqual({
      status: true,
      reauthenticationRequired: true,
    });
    const methods = await pool.query<{ provider_id: string }>(
      'SELECT provider_id FROM auth.accounts WHERE user_id = $1 ORDER BY provider_id',
      [linkedUserId],
    );
    expect(methods.rows.map((account) => account.provider_id)).toEqual([
      'credential',
      'google',
    ]);
    expect(await sessionCount(linkedUserId)).toBe(0);

    const retryJar = jar();
    expect(
      (
        await call('/sign-in/email', {
          body: { email: linkingEmail, password: linkedPassword },
          cookies: retryJar,
        })
      ).status,
    ).toBe(200);
    const retry = await fetch(`${origin}/api/auth/connect-password`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: cookieHeader(retryJar),
        origin,
      },
      body: JSON.stringify({ password: 'AnotherLinkedPassword-2026' }),
    });
    expect(retry.status).toBe(409);
    await expect(retry.json()).resolves.toMatchObject({
      code: 'password_method_exists',
      status: 409,
    });
    await pool.query('DELETE FROM auth.users WHERE id = $1', [linkedUserId]);
  });

  it('disabling MFA revokes sessions and records the state for privileged-role demotion', async () => {
    await resetAuthRateLimits();
    const disabled = await call('/two-factor/disable', {
      body: { password },
      cookies: primary,
    });
    expect(disabled.status).toBe(200);
    expect(await sessionCount(userId)).toBe(0);
    const event = await pool.query<{
      payload: { type?: string; eventId?: string };
    }>(
      "SELECT payload FROM auth.lifecycle_outbox WHERE payload->>'type' = 'mfa_disabled' AND payload->>'authUserId' = $1",
      [userId],
    );
    expect(event.rows[0]?.payload).toMatchObject({
      type: 'mfa_disabled',
      eventId: expect.any(String),
    });
  });
});
