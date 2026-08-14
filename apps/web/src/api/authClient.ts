import { clearAccessToken } from '@/api/client';

export class AuthRequestError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
  ) {
    super(message);
    this.name = 'AuthRequestError';
  }
}

async function readAuthError(response: Response) {
  try {
    const payload = (await response.json()) as {
      detail?: unknown;
      message?: unknown;
      code?: unknown;
    };
    const message =
      typeof payload.detail === 'string'
        ? payload.detail
        : typeof payload.message === 'string'
          ? payload.message
          : 'The authentication request failed.';
    return new AuthRequestError(
      message,
      response.status,
      typeof payload.code === 'string' ? payload.code : undefined,
    );
  } catch {
    return new AuthRequestError(
      'The authentication service returned an unexpected response.',
      response.status,
    );
  }
}

export async function authRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(`/api/auth${path}`, {
    ...init,
    credentials: 'include',
    headers: {
      Accept: 'application/json, application/problem+json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  });
  if (!response.ok) throw await readAuthError(response);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export interface SignInResult {
  twoFactorRedirect?: boolean;
  twoFactorMethods?: string[];
  redirect?: boolean;
  url?: string;
}

export interface BrowserAuthSession {
  session: { id: string; expiresAt?: string };
  user: {
    id: string;
    name: string;
    email: string;
    emailVerified: boolean;
    accountState?: 'active' | 'suspended' | 'deletion_pending' | 'deleted';
  };
}

export async function getBrowserAuthSession(): Promise<BrowserAuthSession | null> {
  return authRequest<BrowserAuthSession | null>('/get-session');
}

export function isEmailVerificationError(error: unknown) {
  if (!(error instanceof AuthRequestError)) return false;
  const normalized = `${error.code ?? ''} ${error.message}`
    .toLowerCase()
    .replaceAll('-', '_')
    .replaceAll(' ', '_');
  return (
    normalized.includes('email_not_verified') ||
    normalized.includes('email_is_not_verified')
  );
}

export function signInWithEmail(email: string, password: string) {
  return authRequest<SignInResult>('/sign-in/email', {
    method: 'POST',
    body: JSON.stringify({
      email,
      password,
      callbackURL: '/dashboard',
      rememberMe: true,
    }),
  });
}

export function signUpWithEmail(name: string, email: string, password: string) {
  return authRequest<{ token: string | null }>('/sign-up/email', {
    method: 'POST',
    body: JSON.stringify({ name, email, password, callbackURL: '/dashboard' }),
  });
}

async function followAuthRedirect(
  path: '/sign-in/social' | '/link-social',
  callbackURL = '/dashboard',
) {
  const result = await authRequest<{ url?: string }>(path, {
    method: 'POST',
    body: JSON.stringify({
      provider: 'google',
      callbackURL,
      errorCallbackURL: '/sign-in?error=google',
      disableRedirect: true,
    }),
  });
  if (!result.url)
    throw new AuthRequestError(
      'Google sign-in did not return a redirect URL.',
      502,
      'missing_redirect',
    );
  window.location.assign(result.url);
}

export function signInWithGoogle() {
  return followAuthRedirect('/sign-in/social');
}

export function linkGoogleAccount() {
  clearAccessToken();
  return followAuthRedirect('/link-social', '/sign-in?securityChanged=true');
}

export function sendVerificationEmail(email: string) {
  return authRequest<{ status: boolean }>('/send-verification-email', {
    method: 'POST',
    body: JSON.stringify({ email, callbackURL: '/sign-in?verified=true' }),
  });
}

export function requestPasswordReset(email: string) {
  return authRequest<{ status: boolean }>('/request-password-reset', {
    method: 'POST',
    body: JSON.stringify({ email, redirectTo: '/reset-password' }),
  });
}

export function resetPassword(token: string, newPassword: string) {
  return authRequest<{ status: boolean }>('/reset-password', {
    method: 'POST',
    body: JSON.stringify({ token, newPassword }),
  });
}

export async function changePassword(
  currentPassword: string,
  newPassword: string,
) {
  const result = await authRequest<{ token?: string }>('/change-password', {
    method: 'POST',
    body: JSON.stringify({
      currentPassword,
      newPassword,
      revokeOtherSessions: true,
    }),
  });
  clearAccessToken();
  return result;
}

export async function changeEmail(newEmail: string) {
  const result = await authRequest<{ status: boolean }>('/change-email', {
    method: 'POST',
    body: JSON.stringify({
      newEmail,
      callbackURL: '/settings?emailChanged=true',
    }),
  });
  clearAccessToken();
  return result;
}

export async function connectPassword(password: string) {
  const result = await authRequest<{
    status: true;
    reauthenticationRequired: true;
  }>('/connect-password', {
    method: 'POST',
    body: JSON.stringify({ password }),
  });
  clearAccessToken();
  return result;
}

export async function signOut() {
  try {
    await authRequest<{ success: boolean }>('/sign-out', { method: 'POST' });
  } finally {
    clearAccessToken();
  }
}

export function verifyTwoFactor(
  code: string,
  method: 'totp' | 'backup-code' = 'totp',
) {
  return authRequest(`/two-factor/verify-${method}`, {
    method: 'POST',
    body: JSON.stringify({ code, trustDevice: false }),
  });
}

export function enableTwoFactor(password?: string) {
  return authRequest<{ totpURI: string; backupCodes: string[] }>(
    '/two-factor/enable',
    {
      method: 'POST',
      body: JSON.stringify(password ? { password } : {}),
    },
  );
}

export function regenerateBackupCodes(password?: string) {
  return authRequest<{ status: true; backupCodes: string[] }>(
    '/two-factor/generate-backup-codes',
    {
      method: 'POST',
      body: JSON.stringify(password ? { password } : {}),
    },
  );
}

export async function requestAccountDeletion() {
  await authRequest<void>('/account/deletion/request', { method: 'POST' });
  clearAccessToken();
}
