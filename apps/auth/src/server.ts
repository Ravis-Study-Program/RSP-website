import { randomUUID } from 'node:crypto';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';

import { APIError } from 'better-auth';
import { z } from 'zod';

import { accountLifecycle, auth, pool } from './auth.js';
import { config } from './config.js';
import { invitationEmail } from './email/templates.js';
import {
  ACCOUNT_DELETION_RECOVERY_PATH,
  ACCOUNT_DELETION_REQUEST_PATH,
  MFA_STEP_UP_BACKUP_PATH,
  MFA_STEP_UP_PATH,
  MFA_STEP_UP_TOTP_PATH,
  parseUrlEncodedField,
  renderDeletionRecoveredPage,
  renderDeletionRecoveryPage,
  renderMfaStepUpPage,
} from './lifecycle/browser.js';
import { InvalidDeletionRecoveryTokenError } from './lifecycle/service.js';
import { log } from './logging.js';
import {
  recordOriginRejection,
  recordRateLimitRejection,
  recordResponse,
  renderMetrics,
} from './metrics.js';
import { withBackupCodeCandidate } from './security/backup-code-store.js';
import { hasValidServiceToken } from './security/internal-auth.js';
import {
  evaluateExplicitLink,
  evaluateFreshSession,
} from './security/linking-policy.js';
import { evaluateBrowserOrigin } from './security/origin.js';
import {
  authRateLimitKey,
  authRateLimitPolicy,
  FixedWindowRateLimiter,
} from './security/rate-limit.js';

const MAX_BODY_BYTES = 1024 * 1024;
const FIVE_MINUTES_SECONDS = 5 * 60;
const edgeRateLimiter = new FixedWindowRateLimiter();

export function resetAuthRateLimiterForTest(): void {
  if (process.env.NODE_ENV !== 'test' && process.env.VITEST !== 'true') {
    throw new Error('auth rate limiter reset is test-only');
  }
  edgeRateLimiter.clear();
}

const revokeSchema = z.object({
  reason: z.enum([
    'suspension',
    'deletion_requested',
    'password_reset',
    'password_changed',
    'email_changed',
    'provider_linked',
    'mfa_changed',
    'security_admin',
  ]),
});

const stateSchema = z.object({
  state: z.enum(['active', 'suspended']),
  reason: z.string().trim().min(1).max(500),
  actorUserId: z
    .string()
    .trim()
    .regex(/^[A-Za-z0-9._-]{1,128}$/),
});

const deletionSchema = z.object({
  recoveryUrl: z.url(),
  recoveryDeadline: z.iso.datetime({ offset: true }),
});

function requestId(request: IncomingMessage): string {
  const supplied = request.headers['x-request-id'];
  return typeof supplied === 'string' &&
    /^[A-Za-z0-9._-]{1,128}$/.test(supplied)
    ? supplied
    : randomUUID();
}

async function readBody(request: IncomingMessage): Promise<Buffer | undefined> {
  if (request.method === 'GET' || request.method === 'HEAD') return undefined;
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += buffer.length;
    if (size > MAX_BODY_BYTES) throw new Error('request body too large');
    chunks.push(buffer);
  }
  return chunks.length ? Buffer.concat(chunks) : undefined;
}

function parseJsonBody(body: Buffer | undefined): unknown {
  if (!body?.length) return undefined;
  try {
    return JSON.parse(body.toString('utf8')) as unknown;
  } catch {
    return undefined;
  }
}

function toWebRequest(
  request: IncomingMessage,
  body: Buffer | undefined,
): Request {
  const url = new URL(request.url ?? '/', config.baseUrl);
  const headers = new Headers();
  for (const [name, value] of Object.entries(request.headers)) {
    if (Array.isArray(value)) {
      for (const entry of value) headers.append(name, entry);
    } else if (value !== undefined) {
      headers.set(name, value);
    }
  }
  return new Request(url, {
    method: request.method ?? 'GET',
    headers,
    ...(body ? { body: body.toString('utf8') } : {}),
  });
}

function clientIp(request: IncomingMessage): string {
  if (config.trustProxyHeaders) {
    const forwarded = request.headers[config.ipAddressHeader];
    if (typeof forwarded === 'string' && forwarded.length <= 128)
      return forwarded;
  }
  return request.socket.remoteAddress ?? 'unknown';
}

function send(
  response: ServerResponse,
  status: number,
  body: string | undefined,
  headers: Record<string, string | string[]> = {},
): void {
  response.writeHead(status, headers);
  response.end(body);
  recordResponse(status);
}

function sendJson(
  response: ServerResponse,
  status: number,
  body: unknown,
  id: string,
  additionalHeaders: Record<string, string> = {},
): void {
  send(response, status, JSON.stringify(body), {
    'content-type': 'application/json; charset=utf-8',
    'cache-control': 'no-store',
    'x-request-id': id,
    ...additionalHeaders,
  });
}

function sendHtml(
  response: ServerResponse,
  status: number,
  body: string,
  id: string,
  additionalHeaders: Record<string, string | string[]> = {},
): void {
  send(response, status, body, {
    'content-type': 'text/html; charset=utf-8',
    'cache-control': 'no-store',
    'content-security-policy':
      "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'",
    'referrer-policy': 'no-referrer',
    'x-content-type-options': 'nosniff',
    'x-request-id': id,
    ...additionalHeaders,
  });
}

function sendProblem(
  response: ServerResponse,
  status: number,
  code: string,
  detail: string,
  id: string,
  additionalHeaders: Record<string, string> = {},
): void {
  const instance = response.req?.url?.split('?', 1)[0] ?? '/';
  send(
    response,
    status,
    JSON.stringify(buildProblem(status, code, detail, id, instance)),
    {
      'content-type': 'application/problem+json; charset=utf-8',
      'cache-control': 'no-store',
      'x-request-id': id,
      ...additionalHeaders,
    },
  );
}

export function buildProblem(
  status: number,
  code: string,
  detail: string,
  requestId: string,
  instance: string,
): Record<string, unknown> {
  const titleByStatus: Readonly<Record<number, string>> = {
    400: 'Bad Request',
    401: 'Unauthorized',
    403: 'Forbidden',
    404: 'Not Found',
    405: 'Method Not Allowed',
    409: 'Conflict',
    429: 'Too Many Requests',
    500: 'Internal Server Error',
  };
  return {
    type: `https://rsp.example/problems/${code}`,
    title: titleByStatus[status] ?? 'Request Failed',
    status,
    detail,
    instance,
    code,
    requestId,
    errors: [],
  };
}

export async function normalizedWebResponse(
  webResponse: Response,
  id: string,
  instance: string,
): Promise<Response> {
  if (webResponse.ok || webResponse.status < 400) return webResponse;
  let body: unknown;
  try {
    body = await webResponse.clone().json();
  } catch {
    body = undefined;
  }
  const source = body && typeof body === 'object' ? body : undefined;
  const rawCode = source ? Reflect.get(source, 'code') : undefined;
  const rawDetail = source ? Reflect.get(source, 'message') : undefined;
  const code =
    typeof rawCode === 'string' && rawCode ? rawCode : 'authentication_failed';
  const detail =
    typeof rawDetail === 'string' && rawDetail
      ? rawDetail
      : 'Authentication request failed';
  const headers = new Headers(webResponse.headers);
  headers.set('content-type', 'application/problem+json; charset=utf-8');
  headers.set('cache-control', 'no-store');
  headers.set('x-request-id', id);
  return new Response(
    JSON.stringify(
      buildProblem(webResponse.status, code, detail, id, instance),
    ),
    {
      status: webResponse.status,
      statusText: webResponse.statusText,
      headers,
    },
  );
}

async function sendWebResponse(
  response: ServerResponse,
  webResponse: Response,
  id: string,
): Promise<void> {
  const normalized = await normalizedWebResponse(
    webResponse,
    id,
    response.req?.url?.split('?', 1)[0] ?? '/',
  );
  const headers: Record<string, string | string[]> = {};
  for (const [name, value] of normalized.headers) {
    if (name.toLowerCase() !== 'set-cookie') headers[name] = value;
  }
  const responseHeaders = normalized.headers as Headers & {
    getSetCookie?: () => string[];
  };
  const cookies = responseHeaders.getSetCookie?.();
  if (cookies?.length) headers['set-cookie'] = cookies;
  headers['x-request-id'] = id;
  headers['cache-control'] ??= 'no-store';
  send(
    response,
    normalized.status,
    Buffer.from(await normalized.arrayBuffer()).toString(),
    headers,
  );
}

function responseCookies(webResponse: Response): string[] | undefined {
  const headers = webResponse.headers as Headers & {
    getSetCookie?: () => string[];
  };
  return headers.getSetCookie?.();
}

function sessionTokenFromResponse(webResponse: Response): string | undefined {
  for (const cookie of responseCookies(webResponse) ?? []) {
    const match = /^rsp-auth\.session_token=([^;]+)/.exec(cookie);
    if (!match?.[1]) continue;
    const signedValue = decodeURIComponent(match[1]);
    const token = signedValue.split('.', 1)[0];
    if (token) return token;
  }
  return undefined;
}

async function markMfaSession(webResponse: Response): Promise<void> {
  let token = sessionTokenFromResponse(webResponse);
  if (!token) {
    try {
      const result = (await webResponse.clone().json()) as {
        token?: unknown;
        session?: { token?: unknown };
      };
      token =
        typeof result.token === 'string'
          ? result.token
          : typeof result.session?.token === 'string'
            ? result.session.token
            : undefined;
    } catch {
      return;
    }
  }
  if (token) {
    await pool.query(
      'UPDATE sessions SET mfa_verified_at = now(), updated_at = now() WHERE token = $1',
      [token],
    );
  }
}

async function handleBrowserExtension(
  url: URL,
  request: IncomingMessage,
  rawBody: Buffer | undefined,
  webRequest: Request,
  response: ServerResponse,
  id: string,
): Promise<boolean> {
  if (url.pathname === '/api/auth/connect-password') {
    if (request.method !== 'POST') {
      sendProblem(response, 405, 'method_not_allowed', 'POST is required', id, {
        allow: 'POST',
      });
      return true;
    }
    const session = await auth.api.getSession({ headers: webRequest.headers });
    const freshness = evaluateFreshSession({
      authenticatedUserId: session?.user.id,
      sessionCreatedAt: session?.session.createdAt,
      now: new Date(),
      freshAgeSeconds: FIVE_MINUTES_SECONDS,
    });
    if (!freshness.allowed || !session) {
      sendProblem(
        response,
        freshness.reason === 'not_authenticated' ? 401 : 403,
        freshness.reason ?? 'reauthentication_required',
        'Connecting password sign-in requires a fresh reauthenticated session',
        id,
      );
      return true;
    }
    const parsed = z
      .object({ password: z.string().min(12).max(128) })
      .safeParse(parseJsonBody(rawBody));
    if (!parsed.success) {
      sendProblem(
        response,
        400,
        'invalid_password',
        'Password must contain 12 to 128 characters',
        id,
      );
      return true;
    }
    const methods = await pool.query<{ provider_id: string }>(
      'SELECT provider_id FROM accounts WHERE user_id = $1',
      [session.user.id],
    );
    const providers = new Set(
      methods.rows.map((account) => account.provider_id),
    );
    if (providers.has('credential')) {
      sendProblem(
        response,
        409,
        'password_method_exists',
        'Password sign-in is already connected',
        id,
      );
      return true;
    }
    if (!providers.has('google')) {
      sendProblem(
        response,
        403,
        'google_method_required',
        'Password can only be connected to a Google account',
        id,
      );
      return true;
    }
    try {
      await auth.api.setPassword({
        body: { newPassword: parsed.data.password },
        headers: webRequest.headers,
      });
    } catch (error) {
      if (!(error instanceof APIError)) throw error;
      sendProblem(
        response,
        error.statusCode >= 400 && error.statusCode < 500
          ? error.statusCode
          : 400,
        typeof error.body?.code === 'string'
          ? error.body.code.toLowerCase()
          : 'password_link_failed',
        'Password sign-in could not be connected',
        id,
      );
      return true;
    }
    await accountLifecycle.revokeSessions(session.user.id, 'provider_linked');
    sendJson(
      response,
      200,
      { status: true, reauthenticationRequired: true },
      id,
    );
    return true;
  }

  if (url.pathname === ACCOUNT_DELETION_REQUEST_PATH) {
    if (request.method !== 'POST') {
      sendProblem(response, 405, 'method_not_allowed', 'POST is required', id, {
        allow: 'POST',
      });
      return true;
    }
    const session = await auth.api.getSession({ headers: webRequest.headers });
    const freshness = evaluateFreshSession({
      authenticatedUserId: session?.user.id,
      sessionCreatedAt: session?.session.createdAt,
      now: new Date(),
      freshAgeSeconds: FIVE_MINUTES_SECONDS,
    });
    if (!freshness.allowed || !session) {
      sendProblem(
        response,
        freshness.reason === 'not_authenticated' ? 401 : 403,
        freshness.reason ?? 'reauthentication_required',
        'Requesting account deletion requires a fresh sign-in',
        id,
      );
      return true;
    }
    const recoveryDeadline = new Date(Date.now() + 30 * 24 * 60 * 60 * 1_000);
    await accountLifecycle.requestDeletion({
      authUserId: session.user.id,
      recoveryUrl: `${config.baseUrl}${ACCOUNT_DELETION_RECOVERY_PATH}`,
      recoveryDeadline,
    });
    send(response, 204, undefined, {
      'cache-control': 'no-store',
      'x-request-id': id,
    });
    return true;
  }

  if (url.pathname === ACCOUNT_DELETION_RECOVERY_PATH) {
    const token =
      request.method === 'GET'
        ? (url.searchParams.get('token')?.trim() ?? '')
        : parseUrlEncodedField(rawBody, 'token');
    if (request.method === 'GET') {
      sendHtml(response, 200, renderDeletionRecoveryPage(token), id);
      return true;
    }
    if (request.method !== 'POST') {
      sendProblem(
        response,
        405,
        'method_not_allowed',
        'GET or POST is required',
        id,
        {
          allow: 'GET, POST',
        },
      );
      return true;
    }
    try {
      await accountLifecycle.cancelDeletionWithRecoveryToken(token);
      sendHtml(response, 200, renderDeletionRecoveredPage(), id);
    } catch (error) {
      if (!(error instanceof InvalidDeletionRecoveryTokenError)) throw error;
      sendHtml(
        response,
        410,
        renderDeletionRecoveryPage(
          '',
          'This recovery link is invalid, expired, or has already been used.',
        ),
        id,
      );
    }
    return true;
  }

  if (url.pathname === MFA_STEP_UP_PATH) {
    if (request.method !== 'GET') {
      sendProblem(response, 405, 'method_not_allowed', 'GET is required', id, {
        allow: 'GET',
      });
      return true;
    }
    sendHtml(response, 200, renderMfaStepUpPage(), id);
    return true;
  }

  if (
    url.pathname === MFA_STEP_UP_TOTP_PATH ||
    url.pathname === MFA_STEP_UP_BACKUP_PATH
  ) {
    if (request.method !== 'POST') {
      sendProblem(response, 405, 'method_not_allowed', 'POST is required', id, {
        allow: 'POST',
      });
      return true;
    }
    const code = parseUrlEncodedField(rawBody, 'code');
    if (code.length < 6 || code.length > 128) {
      sendHtml(
        response,
        400,
        renderMfaStepUpPage('Enter a valid verification code.'),
        id,
      );
      return true;
    }
    const endpoint =
      url.pathname === MFA_STEP_UP_TOTP_PATH
        ? '/api/auth/two-factor/verify-totp'
        : '/api/auth/two-factor/verify-backup-code';
    const headers = new Headers(webRequest.headers);
    headers.delete('content-length');
    headers.set('content-type', 'application/json');
    headers.set('origin', new URL(config.baseUrl).origin);
    const verificationRequest = new Request(new URL(endpoint, config.baseUrl), {
      method: 'POST',
      headers,
      body: JSON.stringify({ code, trustDevice: false }),
    });
    const verification = await withBackupCodeCandidate(
      url.pathname === MFA_STEP_UP_BACKUP_PATH ? code : undefined,
      () => auth.handler(verificationRequest),
    );
    if (verification.ok) await markMfaSession(verification);
    await verification.arrayBuffer();
    if (!verification.ok) {
      sendHtml(
        response,
        400,
        renderMfaStepUpPage('The verification code was not accepted.'),
        id,
      );
      return true;
    }
    const cookies = responseCookies(verification);
    send(response, 303, undefined, {
      location: '/dashboard',
      'cache-control': 'no-store',
      'x-request-id': id,
      ...(cookies?.length ? { 'set-cookie': cookies } : {}),
    });
    return true;
  }

  return false;
}

function internalUserRoute(
  pathname: string,
): { authUserId: string; action: string } | undefined {
  const match =
    /^\/internal\/auth\/users\/([^/]+)\/(revoke-sessions|state|mfa-state|deletion|deletion-cancel|pseudonymize)$/.exec(
      pathname,
    );
  if (!match?.[1] || !match[2]) return undefined;
  try {
    return { authUserId: decodeURIComponent(match[1]), action: match[2] };
  } catch {
    return undefined;
  }
}

async function handleInternal(
  route: { authUserId: string; action: string },
  request: IncomingMessage,
  body: unknown,
  response: ServerResponse,
  id: string,
): Promise<void> {
  const expectedMethod = route.action === 'mfa-state' ? 'GET' : 'POST';
  if (request.method !== expectedMethod) {
    sendProblem(
      response,
      405,
      'method_not_allowed',
      `${expectedMethod} is required`,
      id,
      { allow: expectedMethod },
    );
    return;
  }
  if (
    !hasValidServiceToken(
      request.headers.authorization ?? null,
      config.identityService.token,
    )
  ) {
    sendProblem(
      response,
      401,
      'invalid_service_token',
      'Internal authentication failed',
      id,
    );
    return;
  }
  if (!route.authUserId || route.authUserId.length > 200) {
    sendProblem(
      response,
      400,
      'invalid_auth_user_id',
      'Invalid auth user ID',
      id,
    );
    return;
  }

  switch (route.action) {
    case 'mfa-state': {
      const result = await pool.query<{ two_factor_enabled: boolean | null }>(
        "SELECT two_factor_enabled FROM users WHERE id = $1 AND account_state = 'active'",
        [route.authUserId],
      );
      if (!result.rows[0]) {
        sendProblem(
          response,
          404,
          'auth_user_not_found',
          'Active authentication user not found',
          id,
        );
        return;
      }
      sendJson(
        response,
        200,
        { configured: result.rows[0].two_factor_enabled === true },
        id,
      );
      return;
    }
    case 'revoke-sessions': {
      const parsed = revokeSchema.safeParse(body);
      if (!parsed.success) {
        sendProblem(
          response,
          400,
          'invalid_request',
          'A valid revocation reason is required',
          id,
        );
        return;
      }
      const revoked = await accountLifecycle.revokeSessions(
        route.authUserId,
        parsed.data.reason,
      );
      sendJson(response, 200, { revoked }, id);
      return;
    }
    case 'state': {
      const parsed = stateSchema.safeParse(body);
      if (!parsed.success) {
        sendProblem(
          response,
          400,
          'invalid_request',
          'State must be active or suspended',
          id,
        );
        return;
      }
      await accountLifecycle.setSuspended(
        route.authUserId,
        parsed.data.state === 'suspended',
        {
          reason: parsed.data.reason,
          actorUserId: parsed.data.actorUserId,
        },
      );
      send(response, 204, undefined, { 'x-request-id': id });
      return;
    }
    case 'deletion': {
      const parsed = deletionSchema.safeParse(body);
      if (!parsed.success) {
        sendProblem(
          response,
          400,
          'invalid_request',
          'Invalid deletion recovery details',
          id,
        );
        return;
      }
      const recoveryOrigin = new URL(parsed.data.recoveryUrl).origin;
      if (!config.trustedOrigins.includes(recoveryOrigin)) {
        sendProblem(
          response,
          400,
          'invalid_recovery_url',
          'Recovery URL origin is not trusted',
          id,
        );
        return;
      }
      const deadline = new Date(parsed.data.recoveryDeadline);
      const duration = deadline.getTime() - Date.now();
      if (
        duration < 29 * 24 * 60 * 60 * 1_000 ||
        duration > 31 * 24 * 60 * 60 * 1_000
      ) {
        sendProblem(
          response,
          400,
          'invalid_recovery_deadline',
          'Recovery deadline must be 30 days',
          id,
        );
        return;
      }
      await accountLifecycle.requestDeletion({
        authUserId: route.authUserId,
        recoveryUrl: parsed.data.recoveryUrl,
        recoveryDeadline: deadline,
      });
      send(response, 204, undefined, { 'x-request-id': id });
      return;
    }
    case 'deletion-cancel':
      await accountLifecycle.cancelDeletion(route.authUserId);
      send(response, 204, undefined, { 'x-request-id': id });
      return;
    case 'pseudonymize':
      await accountLifecycle.pseudonymize(route.authUserId);
      send(response, 204, undefined, { 'x-request-id': id });
      return;
    default:
      sendProblem(
        response,
        404,
        'not_found',
        'Internal endpoint not found',
        id,
      );
  }
}

async function handleRequest(
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  const id = requestId(request);
  const startedAt = performance.now();
  try {
    const url = new URL(request.url ?? '/', config.baseUrl);
    if (url.pathname === '/health/live') {
      sendJson(response, 200, { status: 'live' }, id);
      return;
    }
    if (url.pathname === '/health/ready') {
      await pool.query('SELECT 1');
      sendJson(response, 200, { status: 'ready' }, id);
      return;
    }
    if (url.pathname === '/metrics') {
      send(response, 200, renderMetrics(), {
        'content-type': 'text/plain; version=0.0.4; charset=utf-8',
        'x-request-id': id,
      });
      return;
    }

    const rawBody = await readBody(request);
    const jsonBody = parseJsonBody(rawBody);
    if (url.pathname === '/internal/auth/invitations') {
      if (
        !hasValidServiceToken(
          request.headers.authorization ?? null,
          config.identityService.token,
        )
      ) {
        sendProblem(
          response,
          401,
          'invalid_service_token',
          'Internal authentication failed',
          id,
        );
        return;
      }
      if (request.method !== 'POST') {
        sendProblem(
          response,
          405,
          'method_not_allowed',
          'POST is required',
          id,
          { allow: 'POST' },
        );
        return;
      }
      const parsed = z
        .object({
          id: z.uuid(),
          to: z.email(),
          name: z.string().min(1).max(100),
          season: z.string().min(1).max(200),
          role: z.enum(['student', 'mentor', 'coordinator']),
          url: z.url(),
        })
        .safeParse(jsonBody);
      if (!parsed.success) {
        sendProblem(response, 400, 'invalid_request', 'Invalid invitation', id);
        return;
      }
      const invitationUrl = new URL(parsed.data.url);
      if (
        invitationUrl.origin !== new URL(config.baseUrl).origin ||
        invitationUrl.pathname !== '/invitations/accept'
      ) {
        sendProblem(
          response,
          400,
          'invalid_request',
          'Invalid invitation URL',
          id,
        );
        return;
      }
      await accountLifecycle.queueEmail(
        parsed.data.id,
        invitationEmail(
          parsed.data.to,
          parsed.data.name,
          parsed.data.season,
          parsed.data.role,
          parsed.data.url,
        ),
      );
      sendJson(response, 202, { queued: true }, id);
      return;
    }
    const internalRoute = internalUserRoute(url.pathname);
    if (internalRoute) {
      await handleInternal(internalRoute, request, jsonBody, response, id);
      return;
    }
    if (!url.pathname.startsWith('/api/auth/')) {
      sendProblem(response, 404, 'not_found', 'Endpoint not found', id);
      return;
    }

    const origin = evaluateBrowserOrigin(
      request.method ?? 'GET',
      request.headers.origin ?? null,
      config.trustedOrigins,
    );
    if (!origin.allowed) {
      recordOriginRejection();
      sendProblem(
        response,
        403,
        'untrusted_origin',
        'Request origin is not trusted',
        id,
      );
      return;
    }

    const ratePolicy = authRateLimitPolicy(url.pathname);
    const rateKey = authRateLimitKey({
      pathname: url.pathname,
      ipAddress: clientIp(request),
      cookieHeader: request.headers.cookie,
      requestBody: jsonBody,
      secret: config.secret,
    });
    const rateResult = edgeRateLimiter.consume(rateKey, ratePolicy);
    if (!rateResult.allowed) {
      recordRateLimitRejection();
      sendProblem(
        response,
        429,
        'rate_limited',
        'Too many authentication requests',
        id,
        {
          'retry-after': String(rateResult.retryAfterSeconds),
          'x-ratelimit-limit': String(rateResult.limit),
          'x-ratelimit-remaining': String(rateResult.remaining),
        },
      );
      return;
    }

    const webRequest = toWebRequest(request, rawBody);
    if (
      await handleBrowserExtension(
        url,
        request,
        rawBody,
        webRequest,
        response,
        id,
      )
    )
      return;
    if (
      (url.pathname === '/api/auth/two-factor/verify-totp' ||
        url.pathname === '/api/auth/two-factor/verify-backup-code') &&
      jsonBody &&
      typeof jsonBody === 'object' &&
      Reflect.get(jsonBody, 'trustDevice') === true
    ) {
      sendProblem(
        response,
        400,
        'trusted_devices_disabled',
        'RSP requires a fresh second factor for privileged actions',
        id,
      );
      return;
    }

    if (
      url.pathname === '/api/auth/two-factor/enable' ||
      url.pathname === '/api/auth/two-factor/disable' ||
      url.pathname === '/api/auth/two-factor/generate-backup-codes'
    ) {
      const session = await auth.api.getSession({
        headers: webRequest.headers,
      });
      const freshness = evaluateFreshSession({
        authenticatedUserId: session?.user.id,
        sessionCreatedAt: session?.session.createdAt,
        now: new Date(),
        freshAgeSeconds: FIVE_MINUTES_SECONDS,
      });
      if (!freshness.allowed) {
        sendProblem(
          response,
          freshness.reason === 'not_authenticated' ? 401 : 403,
          freshness.reason ?? 'reauthentication_required',
          'Managing MFA requires a fresh reauthenticated session',
          id,
        );
        return;
      }
    }

    if (url.pathname === '/api/auth/link-social') {
      const session = await auth.api.getSession({
        headers: webRequest.headers,
      });
      const provider =
        jsonBody && typeof jsonBody === 'object'
          ? Reflect.get(jsonBody, 'provider')
          : undefined;
      const decision = evaluateExplicitLink({
        authenticatedUserId: session?.user.id,
        sessionCreatedAt: session?.session.createdAt,
        now: new Date(),
        freshAgeSeconds: FIVE_MINUTES_SECONDS,
        provider: typeof provider === 'string' ? provider : '',
        currentEmail: session?.user.email ?? '',
      });
      if (!decision.allowed) {
        sendProblem(
          response,
          decision.reason === 'not_authenticated' ? 401 : 403,
          decision.reason ?? 'linking_not_allowed',
          'Connecting a sign-in method requires a fresh reauthenticated session',
          id,
        );
        return;
      }
    }

    const preMfaSession = isMfaVerificationPath(url.pathname)
      ? await auth.api.getSession({ headers: webRequest.headers })
      : null;
    const candidate =
      url.pathname === '/api/auth/two-factor/verify-backup-code' &&
      jsonBody &&
      typeof jsonBody === 'object' &&
      typeof Reflect.get(jsonBody, 'code') === 'string'
        ? (Reflect.get(jsonBody, 'code') as string)
        : undefined;
    const authResponse = await withBackupCodeCandidate(candidate, () =>
      auth.handler(webRequest),
    );
    if (authResponse.ok && isMfaVerificationPath(url.pathname)) {
      if (preMfaSession?.user.twoFactorEnabled === false) {
        await accountLifecycle.recordMfaState(preMfaSession.user.id, true);
      } else {
        await markMfaSession(authResponse);
      }
    }
    await sendWebResponse(response, authResponse, id);
  } catch (error) {
    log('error', 'auth request failed', { requestId: id, error });
    if (!response.headersSent) {
      sendProblem(
        response,
        500,
        'internal_error',
        'Authentication request failed',
        id,
      );
    } else {
      response.end();
    }
  } finally {
    log('info', 'auth request', {
      requestId: id,
      method: request.method,
      path: request.url?.split('?', 1)[0],
      status: response.statusCode,
      durationMs: Math.round((performance.now() - startedAt) * 100) / 100,
    });
  }
}

function isMfaVerificationPath(pathname: string): boolean {
  return (
    pathname === '/api/auth/two-factor/verify-totp' ||
    pathname === '/api/auth/two-factor/verify-backup-code'
  );
}

const shouldListen = process.env.AUTH_DISABLE_LISTEN !== 'true';
const server = shouldListen
  ? createServer((request, response) => {
      void handleRequest(request, response);
    })
  : undefined;

export async function closeAuthServerForTest(): Promise<void> {
  clearInterval(deletionSweepTimer);
  clearInterval(outboxDeliveryTimer);
  if (!server) return;
  await new Promise<void>((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
  });
}

let deletionSweepRunning = false;
async function runDeletionSweep(): Promise<void> {
  if (deletionSweepRunning) return;
  deletionSweepRunning = true;
  try {
    let processed: number;
    do {
      processed = await accountLifecycle.pseudonymizeDue(50);
      if (processed > 0)
        log('info', 'pseudonymized due accounts', { processed });
    } while (processed === 50);
  } catch (error) {
    log('error', 'due account pseudonymization failed', { error });
  } finally {
    deletionSweepRunning = false;
  }
}

let outboxDeliveryRunning = false;
async function runOutboxDelivery(): Promise<void> {
  if (outboxDeliveryRunning) return;
  outboxDeliveryRunning = true;
  try {
    let delivered: number;
    do {
      delivered = await accountLifecycle.flushOutbox(50);
      if (delivered > 0)
        log('info', 'delivered lifecycle outbox', { delivered });
    } while (delivered === 50);
  } catch (error) {
    log('error', 'lifecycle outbox delivery failed', { error });
  } finally {
    outboxDeliveryRunning = false;
  }
}

server?.listen(config.port, config.host, () => {
  log('info', 'auth service listening', {
    host: config.host,
    port: config.port,
  });
  void runDeletionSweep();
  void runOutboxDelivery();
});

const deletionSweepTimer = setInterval(
  () => void runDeletionSweep(),
  60 * 60 * 1_000,
);
deletionSweepTimer.unref();
const outboxDeliveryTimer = setInterval(() => void runOutboxDelivery(), 5_000);
outboxDeliveryTimer.unref();

async function shutdown(signal: string): Promise<void> {
  log('info', 'auth service shutting down', { signal });
  await closeAuthServerForTest();
  await pool.end();
}

process.once('SIGINT', () => void shutdown('SIGINT'));
process.once('SIGTERM', () => void shutdown('SIGTERM'));
