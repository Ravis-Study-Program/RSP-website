import { createHmac } from 'node:crypto';

export interface RateLimitPolicy {
  readonly max: number;
  readonly windowSeconds: number;
}

export interface RateLimitResult {
  readonly allowed: boolean;
  readonly limit: number;
  readonly remaining: number;
  readonly retryAfterSeconds: number;
  readonly resetAt: Date;
}

interface Counter {
  count: number;
  resetAtMs: number;
}

export class FixedWindowRateLimiter {
  readonly #counters = new Map<string, Counter>();
  readonly #now: () => number;

  constructor(now: () => number = Date.now) {
    this.#now = now;
  }

  consume(key: string, policy: RateLimitPolicy): RateLimitResult {
    const now = this.#now();
    const current = this.#counters.get(key);
    const counter =
      !current || current.resetAtMs <= now
        ? { count: 0, resetAtMs: now + policy.windowSeconds * 1_000 }
        : current;
    counter.count += 1;
    this.#counters.set(key, counter);

    if (this.#counters.size > 10_000) this.prune();

    const retryAfterSeconds = Math.max(
      1,
      Math.ceil((counter.resetAtMs - now) / 1_000),
    );
    return {
      allowed: counter.count <= policy.max,
      limit: policy.max,
      remaining: Math.max(0, policy.max - counter.count),
      retryAfterSeconds,
      resetAt: new Date(counter.resetAtMs),
    };
  }

  prune(): void {
    const now = this.#now();
    for (const [key, counter] of this.#counters) {
      if (counter.resetAtMs <= now) this.#counters.delete(key);
    }
  }

  clear(): void {
    this.#counters.clear();
  }
}

const FIVE_PER_MINUTE_PATHS = [
  '/sign-in/email',
  '/sign-up/email',
  '/send-verification-email',
  '/request-password-reset',
  '/forget-password',
  '/reset-password',
  '/two-factor/enable',
  '/two-factor/verify-totp',
  '/two-factor/verify-backup-code',
  '/two-factor/generate-backup-codes',
  '/connect-password',
  '/account/deletion/request',
  '/account/deletion/recovery',
  '/mfa/step-up/totp',
  '/mfa/step-up/backup-code',
] as const;

export function authRateLimitPolicy(pathname: string): RateLimitPolicy {
  const authPath = pathname.startsWith('/api/auth')
    ? pathname.slice('/api/auth'.length) || '/'
    : pathname;
  if (
    FIVE_PER_MINUTE_PATHS.includes(
      authPath as (typeof FIVE_PER_MINUTE_PATHS)[number],
    )
  ) {
    return { max: 5, windowSeconds: 60 };
  }
  if (authPath === '/token') return { max: 20, windowSeconds: 60 };
  return { max: 30, windowSeconds: 60 };
}

function hashIdentifier(value: string, secret: string): string {
  return createHmac('sha256', secret).update(value).digest('base64url');
}

function sessionCookie(cookieHeader: string | undefined): string | undefined {
  if (!cookieHeader) return undefined;
  for (const item of cookieHeader.split(';')) {
    const separator = item.indexOf('=');
    if (separator < 1) continue;
    const name = item.slice(0, separator).trim();
    if (name !== 'rsp-auth.session_token') continue;
    const value = item.slice(separator + 1).trim();
    if (value) return value;
  }
  return undefined;
}

export function authRateLimitKey(options: {
  readonly pathname: string;
  readonly ipAddress: string;
  readonly cookieHeader?: string;
  readonly requestBody?: unknown;
  readonly secret: string;
}): string {
  let accountHint: string | undefined;
  if (options.requestBody && typeof options.requestBody === 'object') {
    const email = Reflect.get(options.requestBody, 'email');
    if (typeof email === 'string' && email.trim()) {
      accountHint = `email:${email.trim().toLowerCase()}`;
    }
  }
  const stableSession = sessionCookie(options.cookieHeader);
  const stableIdentity =
    accountHint ??
    (stableSession ? `session:${stableSession}` : `ip:${options.ipAddress}`);
  return `${options.pathname}:${hashIdentifier(stableIdentity, options.secret)}`;
}
