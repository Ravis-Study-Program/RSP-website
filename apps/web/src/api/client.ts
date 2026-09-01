import type { ErrorResponse } from '@/api/generated/models';

let accessToken: { value: string; expiresAt: number } | null = null;
let accessTokenRequest: Promise<{ value: string; expiresAt: number }> | null =
  null;

export interface AccessTokenClaims {
  exp: number;
  [claim: string]: unknown;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly response: ErrorResponse,
  ) {
    super(response.message);
    this.name = 'ApiError';
  }
}

function invalidTokenError(message: string) {
  return new ApiError(502, {
    code: 'invalid_auth_token',
    message,
    requestId: 'client',
  });
}

export function decodeAccessTokenClaims(token: string): AccessTokenClaims {
  const payload = token.split('.')[1];
  if (!payload)
    throw invalidTokenError(
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
    if (error instanceof ApiError) throw error;
    throw invalidTokenError(
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

  if (!response.ok) throw await errorFromResponse(response);
  const payload = (await response.json()) as { token?: unknown };
  if (typeof payload.token !== 'string' || payload.token.length === 0) {
    throw invalidTokenError(
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
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) throw await errorFromResponse(response);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export async function errorFromResponse(response: Response): Promise<ApiError> {
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    // A non-JSON upstream error is normalised into the API error shape below.
  }
  const partial =
    payload && typeof payload === 'object'
      ? (payload as {
          code?: unknown;
          message?: unknown;
          requestId?: unknown;
          detail?: unknown;
          title?: unknown;
        })
      : {};
  const errorResponse: ErrorResponse = {
    code:
      typeof partial.code === 'string' ? partial.code : 'unexpected_response',
    message:
      typeof partial.message === 'string'
        ? partial.message
        : typeof partial.detail === 'string'
          ? partial.detail
          : typeof partial.title === 'string'
            ? partial.title
            : 'The request failed.',
    requestId:
      typeof partial.requestId === 'string'
        ? partial.requestId
        : (response.headers.get('x-request-id') ?? 'unknown'),
  };
  return new ApiError(response.status, errorResponse);
}
