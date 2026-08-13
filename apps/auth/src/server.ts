import { randomUUID } from "node:crypto";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";

import { z } from "zod";

import { accountLifecycle, auth, pool } from "./auth.js";
import { config } from "./config.js";
import { log } from "./logging.js";
import {
  recordOriginRejection,
  recordRateLimitRejection,
  recordResponse,
  renderMetrics,
} from "./metrics.js";
import { withBackupCodeCandidate } from "./security/backup-code-store.js";
import { hasValidServiceToken } from "./security/internal-auth.js";
import { evaluateExplicitLink, evaluateFreshSession } from "./security/linking-policy.js";
import { evaluateBrowserOrigin } from "./security/origin.js";
import {
  authRateLimitKey,
  authRateLimitPolicy,
  FixedWindowRateLimiter,
} from "./security/rate-limit.js";

const MAX_BODY_BYTES = 1024 * 1024;
const FIVE_MINUTES_SECONDS = 5 * 60;
const edgeRateLimiter = new FixedWindowRateLimiter();

const revokeSchema = z.object({
  reason: z.enum([
    "suspension",
    "deletion_requested",
    "password_reset",
    "password_changed",
    "email_changed",
    "provider_linked",
    "mfa_changed",
    "security_admin",
  ]),
});

const stateSchema = z.object({ state: z.enum(["active", "suspended"]) });

const deletionSchema = z.object({
  recoveryUrl: z.url(),
  recoveryDeadline: z.iso.datetime({ offset: true }),
});

function requestId(request: IncomingMessage): string {
  const supplied = request.headers["x-request-id"];
  return typeof supplied === "string" && /^[A-Za-z0-9._-]{1,128}$/.test(supplied)
    ? supplied
    : randomUUID();
}

async function readBody(request: IncomingMessage): Promise<Buffer | undefined> {
  if (request.method === "GET" || request.method === "HEAD") return undefined;
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += buffer.length;
    if (size > MAX_BODY_BYTES) throw new Error("request body too large");
    chunks.push(buffer);
  }
  return chunks.length ? Buffer.concat(chunks) : undefined;
}

function parseJsonBody(body: Buffer | undefined): unknown {
  if (!body?.length) return undefined;
  try {
    return JSON.parse(body.toString("utf8")) as unknown;
  } catch {
    return undefined;
  }
}

function toWebRequest(request: IncomingMessage, body: Buffer | undefined): Request {
  const url = new URL(request.url ?? "/", config.baseUrl);
  const headers = new Headers();
  for (const [name, value] of Object.entries(request.headers)) {
    if (Array.isArray(value)) {
      for (const entry of value) headers.append(name, entry);
    } else if (value !== undefined) {
      headers.set(name, value);
    }
  }
  return new Request(url, {
    method: request.method ?? "GET",
    headers,
    ...(body ? { body: body.toString("utf8") } : {}),
  });
}

function clientIp(request: IncomingMessage): string {
  if (config.trustProxyHeaders) {
    const forwarded = request.headers[config.ipAddressHeader];
    if (typeof forwarded === "string" && forwarded.length <= 128) return forwarded;
  }
  return request.socket.remoteAddress ?? "unknown";
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
    "content-type": "application/json; charset=utf-8",
    "cache-control": "no-store",
    "x-request-id": id,
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
  send(response, status, JSON.stringify({ status, code, detail, requestId: id }), {
    "content-type": "application/problem+json; charset=utf-8",
    "cache-control": "no-store",
    "x-request-id": id,
    ...additionalHeaders,
  });
}

async function sendWebResponse(response: ServerResponse, webResponse: Response, id: string): Promise<void> {
  const headers: Record<string, string | string[]> = {};
  for (const [name, value] of webResponse.headers) {
    if (name.toLowerCase() !== "set-cookie") headers[name] = value;
  }
  const responseHeaders = webResponse.headers as Headers & { getSetCookie?: () => string[] };
  const cookies = responseHeaders.getSetCookie?.();
  if (cookies?.length) headers["set-cookie"] = cookies;
  headers["x-request-id"] = id;
  headers["cache-control"] ??= "no-store";
  send(response, webResponse.status, Buffer.from(await webResponse.arrayBuffer()).toString(), headers);
}

function internalUserRoute(pathname: string): { authUserId: string; action: string } | undefined {
  const match = /^\/internal\/auth\/users\/([^/]+)\/(revoke-sessions|state|deletion|deletion-cancel|pseudonymize)$/.exec(
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
  if (request.method !== "POST") {
    sendProblem(response, 405, "method_not_allowed", "POST is required", id, { allow: "POST" });
    return;
  }
  if (!hasValidServiceToken(request.headers.authorization ?? null, config.identityService.token)) {
    sendProblem(response, 401, "invalid_service_token", "Internal authentication failed", id);
    return;
  }
  if (!route.authUserId || route.authUserId.length > 200) {
    sendProblem(response, 400, "invalid_auth_user_id", "Invalid auth user ID", id);
    return;
  }

  switch (route.action) {
    case "revoke-sessions": {
      const parsed = revokeSchema.safeParse(body);
      if (!parsed.success) {
        sendProblem(response, 400, "invalid_request", "A valid revocation reason is required", id);
        return;
      }
      const revoked = await accountLifecycle.revokeSessions(route.authUserId, parsed.data.reason);
      sendJson(response, 200, { revoked }, id);
      return;
    }
    case "state": {
      const parsed = stateSchema.safeParse(body);
      if (!parsed.success) {
        sendProblem(response, 400, "invalid_request", "State must be active or suspended", id);
        return;
      }
      await accountLifecycle.setSuspended(route.authUserId, parsed.data.state === "suspended");
      send(response, 204, undefined, { "x-request-id": id });
      return;
    }
    case "deletion": {
      const parsed = deletionSchema.safeParse(body);
      if (!parsed.success) {
        sendProblem(response, 400, "invalid_request", "Invalid deletion recovery details", id);
        return;
      }
      const recoveryOrigin = new URL(parsed.data.recoveryUrl).origin;
      if (!config.trustedOrigins.includes(recoveryOrigin)) {
        sendProblem(response, 400, "invalid_recovery_url", "Recovery URL origin is not trusted", id);
        return;
      }
      const deadline = new Date(parsed.data.recoveryDeadline);
      const duration = deadline.getTime() - Date.now();
      if (duration < 29 * 24 * 60 * 60 * 1_000 || duration > 31 * 24 * 60 * 60 * 1_000) {
        sendProblem(response, 400, "invalid_recovery_deadline", "Recovery deadline must be 30 days", id);
        return;
      }
      await accountLifecycle.requestDeletion({
        authUserId: route.authUserId,
        recoveryUrl: parsed.data.recoveryUrl,
        recoveryDeadline: deadline,
      });
      send(response, 204, undefined, { "x-request-id": id });
      return;
    }
    case "deletion-cancel":
      await accountLifecycle.cancelDeletion(route.authUserId);
      send(response, 204, undefined, { "x-request-id": id });
      return;
    case "pseudonymize":
      await accountLifecycle.pseudonymize(route.authUserId);
      send(response, 204, undefined, { "x-request-id": id });
      return;
    default:
      sendProblem(response, 404, "not_found", "Internal endpoint not found", id);
  }
}

async function handleRequest(request: IncomingMessage, response: ServerResponse): Promise<void> {
  const id = requestId(request);
  const startedAt = performance.now();
  try {
    const url = new URL(request.url ?? "/", config.baseUrl);
    if (url.pathname === "/health/live") {
      sendJson(response, 200, { status: "live" }, id);
      return;
    }
    if (url.pathname === "/health/ready") {
      await pool.query("SELECT 1");
      sendJson(response, 200, { status: "ready" }, id);
      return;
    }
    if (url.pathname === "/metrics") {
      send(response, 200, renderMetrics(), {
        "content-type": "text/plain; version=0.0.4; charset=utf-8",
        "x-request-id": id,
      });
      return;
    }

    const rawBody = await readBody(request);
    const jsonBody = parseJsonBody(rawBody);
    const internalRoute = internalUserRoute(url.pathname);
    if (internalRoute) {
      await handleInternal(internalRoute, request, jsonBody, response, id);
      return;
    }
    if (!url.pathname.startsWith("/api/auth/")) {
      sendProblem(response, 404, "not_found", "Endpoint not found", id);
      return;
    }

    const origin = evaluateBrowserOrigin(
      request.method ?? "GET",
      request.headers.origin ?? null,
      config.trustedOrigins,
    );
    if (!origin.allowed) {
      recordOriginRejection();
      sendProblem(response, 403, "untrusted_origin", "Request origin is not trusted", id);
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
      sendProblem(response, 429, "rate_limited", "Too many authentication requests", id, {
        "retry-after": String(rateResult.retryAfterSeconds),
        "x-ratelimit-limit": String(rateResult.limit),
        "x-ratelimit-remaining": String(rateResult.remaining),
      });
      return;
    }

    const webRequest = toWebRequest(request, rawBody);
    if (
      (url.pathname === "/api/auth/two-factor/verify-totp" ||
        url.pathname === "/api/auth/two-factor/verify-backup-code") &&
      jsonBody &&
      typeof jsonBody === "object" &&
      Reflect.get(jsonBody, "trustDevice") === true
    ) {
      sendProblem(
        response,
        400,
        "trusted_devices_disabled",
        "RSP requires a fresh second factor for privileged actions",
        id,
      );
      return;
    }

    if (
      url.pathname === "/api/auth/two-factor/enable" ||
      url.pathname === "/api/auth/two-factor/disable" ||
      url.pathname === "/api/auth/two-factor/generate-backup-codes"
    ) {
      const session = await auth.api.getSession({ headers: webRequest.headers });
      const freshness = evaluateFreshSession({
        authenticatedUserId: session?.user.id,
        sessionCreatedAt: session?.session.createdAt,
        now: new Date(),
        freshAgeSeconds: FIVE_MINUTES_SECONDS,
      });
      if (!freshness.allowed) {
        sendProblem(
          response,
          freshness.reason === "not_authenticated" ? 401 : 403,
          freshness.reason ?? "reauthentication_required",
          "Managing MFA requires a fresh reauthenticated session",
          id,
        );
        return;
      }
    }

    if (url.pathname === "/api/auth/link-social") {
      const session = await auth.api.getSession({ headers: webRequest.headers });
      const provider =
        jsonBody && typeof jsonBody === "object" ? Reflect.get(jsonBody, "provider") : undefined;
      const decision = evaluateExplicitLink({
        authenticatedUserId: session?.user.id,
        sessionCreatedAt: session?.session.createdAt,
        now: new Date(),
        freshAgeSeconds: FIVE_MINUTES_SECONDS,
        provider: typeof provider === "string" ? provider : "",
        currentEmail: session?.user.email ?? "",
      });
      if (!decision.allowed) {
        sendProblem(
          response,
          decision.reason === "not_authenticated" ? 401 : 403,
          decision.reason ?? "linking_not_allowed",
          "Connecting a sign-in method requires a fresh reauthenticated session",
          id,
        );
        return;
      }
    }

    const candidate =
      url.pathname === "/api/auth/two-factor/verify-backup-code" &&
      jsonBody &&
      typeof jsonBody === "object" &&
      typeof Reflect.get(jsonBody, "code") === "string"
        ? (Reflect.get(jsonBody, "code") as string)
        : undefined;
    const authResponse = await withBackupCodeCandidate(candidate, () => auth.handler(webRequest));
    await sendWebResponse(response, authResponse, id);
  } catch (error) {
    log("error", "auth request failed", { requestId: id, error });
    if (!response.headersSent) {
      sendProblem(response, 500, "internal_error", "Authentication request failed", id);
    } else {
      response.end();
    }
  } finally {
    log("info", "auth request", {
      requestId: id,
      method: request.method,
      path: request.url?.split("?", 1)[0],
      status: response.statusCode,
      durationMs: Math.round((performance.now() - startedAt) * 100) / 100,
    });
  }
}

const server = createServer((request, response) => {
  void handleRequest(request, response);
});

server.listen(config.port, config.host, () => {
  log("info", "auth service listening", { host: config.host, port: config.port });
});

async function shutdown(signal: string): Promise<void> {
  log("info", "auth service shutting down", { signal });
  server.close();
  await pool.end();
}

process.once("SIGINT", () => void shutdown("SIGINT"));
process.once("SIGTERM", () => void shutdown("SIGTERM"));
