import type { ProblemDetails } from '@/types';

let accessToken: { value: string; expiresAt: number } | null = null;
let accessTokenRequest: Promise<{ value: string; expiresAt: number }> | null =
  null;

export interface AccessTokenClaims {
  exp: number;
  [claim: string]: unknown;
}

export class ApiProblem extends Error {
  constructor(public readonly problem: ProblemDetails) {
    super(problem.detail ?? problem.title);
    this.name = 'ApiProblem';
  }
}

function invalidTokenProblem(detail: string) {
  return new ApiProblem({
    type: 'about:blank',
    title: 'Invalid authentication response',
    status: 502,
    detail,
    code: 'invalid_auth_token',
    requestId: 'client',
  });
}

export function decodeAccessTokenClaims(token: string): AccessTokenClaims {
  const payload = token.split('.')[1];
  if (!payload)
    throw invalidTokenProblem(
      'The authentication service returned a malformed access token.',
    );

  try {
    const normalized = payload.replaceAll('-', '+').replaceAll('_', '/');
    const decoded = JSON.parse(
      atob(normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')),
    ) as unknown;
    if (
      !decoded ||
      typeof decoded !== 'object' ||
      typeof (decoded as { exp?: unknown }).exp !== 'number' ||
      !Number.isFinite((decoded as { exp: number }).exp)
    ) {
      throw new Error('Missing exp claim');
    }
    return decoded as AccessTokenClaims;
  } catch (error) {
    if (error instanceof ApiProblem) throw error;
    throw invalidTokenProblem(
      'The authentication service returned an access token without a valid expiry.',
    );
  }
}

async function requestAccessToken() {
  const response = await fetch('/api/auth/token', {
    method: 'GET',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  });

  if (!response.ok) throw await problemFromResponse(response);
  const payload = (await response.json()) as { token?: unknown };
  if (typeof payload.token !== 'string' || payload.token.length === 0) {
    throw invalidTokenProblem(
      'The authentication service response did not include an access token.',
    );
  }
  return {
    value: payload.token,
    expiresAt: decodeAccessTokenClaims(payload.token).exp * 1000,
  };
}

export async function getAccessToken(): Promise<string> {
  if (accessToken && accessToken.expiresAt > Date.now() + 15_000)
    return accessToken.value;

  accessTokenRequest ??= requestAccessToken();
  try {
    accessToken = await accessTokenRequest;
    return accessToken.value;
  } finally {
    accessTokenRequest = null;
  }
}

export async function getAccessTokenClaims(): Promise<AccessTokenClaims> {
  return decodeAccessTokenClaims(await getAccessToken());
}

export function clearAccessToken() {
  accessToken = null;
  accessTokenRequest = null;
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const token = await getAccessToken();
  const response = await fetch(`/api/v2${path}`, {
    ...init,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json, application/problem+json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) throw await problemFromResponse(response);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export async function problemFromResponse(
  response: Response,
): Promise<ApiProblem> {
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    // A non-JSON upstream error is normalised into Problem Details below.
  }
  const partial =
    payload && typeof payload === 'object'
      ? (payload as Partial<ProblemDetails> & { message?: unknown })
      : {};
  const problem: ProblemDetails = {
    type: typeof partial.type === 'string' ? partial.type : 'about:blank',
    title: typeof partial.title === 'string' ? partial.title : 'Request failed',
    status:
      typeof partial.status === 'number' ? partial.status : response.status,
    ...(typeof partial.detail === 'string'
      ? { detail: partial.detail }
      : typeof partial.message === 'string'
        ? { detail: partial.message }
        : {}),
    ...(typeof partial.instance === 'string'
      ? { instance: partial.instance }
      : {}),
    code:
      typeof partial.code === 'string' ? partial.code : 'unexpected_response',
    requestId:
      typeof partial.requestId === 'string'
        ? partial.requestId
        : (response.headers.get('x-request-id') ?? 'unknown'),
    ...(Array.isArray(partial.errors) ? { errors: partial.errors } : {}),
  };
  return new ApiProblem(problem);
}
