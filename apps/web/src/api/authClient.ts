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
    twoFactorEnabled?: boolean;
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

/**
 * Signs up. With the token of an invitation sent to this email, the account
 * starts verified: opening the invitation already proved the inbox.
 */
export function signUpWithEmail(
  name: string,
  email: string,
  password: string,
  invitationToken?: string,
) {
  return authRequest<{
    token: string | null;
    user?: { emailVerified?: boolean };
  }>('/sign-up/email', {
    method: 'POST',
    body: JSON.stringify({
      name,
      email,
      password,
      callbackURL: '/dashboard',
      ...(invitationToken ? { invitationToken } : {}),
    }),
  });
}

async function followAuthRedirect(
  path: '/sign-in/social' | '/link-social',
  provider: 'google' | 'discord',
  callbackURL = '/dashboard',
) {
  const result = await authRequest<{ url?: string }>(path, {
    method: 'POST',
    body: JSON.stringify({
      provider,
      callbackURL,
      errorCallbackURL: `${path === '/link-social' ? '/settings' : '/sign-in'}?provider=${provider}`,
      disableRedirect: true,
    }),
  });
  if (!result.url)
    throw new AuthRequestError(
      'Sign-in did not return a redirect URL.',
      502,
      'missing_redirect',
    );
  window.location.assign(result.url);
}

export function signInWithGoogle() {
  return followAuthRedirect('/sign-in/social', 'google');
}

/** Which sign-in methods the account has connected. */
export async function listSignInMethods() {
  const [accounts, session] = await Promise.all([
    authRequest<{ id: string; providerId: string }[]>('/list-accounts'),
    getBrowserAuthSession(),
  ]);
  return {
    discordAccountId:
      accounts.find((account) => account.providerId === 'discord')?.id ?? null,
    google: accounts.some((account) => account.providerId === 'google'),
    password: accounts.some((account) => account.providerId === 'credential'),
    twoFactor: Boolean(session?.user.twoFactorEnabled),
  };
}

export function linkGoogleAccount() {
  clearAccessToken();
  return followAuthRedirect(
    '/link-social',
    'google',
    '/sign-in?securityChanged=true',
  );
}

export function signInWithDiscord() {
  return followAuthRedirect('/sign-in/social', 'discord');
}

export function getAuthProviders() {
  return authRequest<{ discord: boolean }>('/providers');
}

/**
 * Asks the auth service to check the member's Discord profile picture. It
 * checks Discord at most once a day and says whether the picture changed.
 */
export function refreshDiscordAvatar() {
  return authRequest<{
    result: 'skipped' | 'unchanged' | 'updated' | 'unavailable';
  }>('/discord/avatar/refresh', { method: 'POST', body: '{}' });
}

export function linkDiscordAccount() {
  clearAccessToken();
  return followAuthRedirect(
    '/link-social',
    'discord',
    '/sign-in?discordConnected=true',
  );
}

export async function unlinkDiscordAccount(accountId: string) {
  await authRequest('/unlink-account', {
    method: 'POST',
    body: JSON.stringify({ accountId }),
  });
  clearAccessToken();
}

export function socialAuthError(params: URLSearchParams): string {
  const error = params.get('error');
  if (!error) return '';
  const discord = params.get('provider') === 'discord';
  if (
    discord &&
    [
      'signup_disabled',
      'account_not_linked',
      'primary_sign_in_required',
    ].includes(error)
  ) {
    return 'Sign in with email or Google, then connect Discord in Settings.';
  }
  if (error === 'reauthentication_required')
    return 'Sign in again with email or Google, then reconnect from Settings.';
  if (error === 'email_does_not_match')
    return 'Use a Google account with the same email as your RSP account.';
  if (
    error === 'account_already_linked_to_different_user' ||
    error === 'account_already_linked'
  )
    return 'This sign-in account is already connected to another RSP account.';
  if (
    discord &&
    [
      'email_not_found',
      'email_not_verified',
      'unable_to_link_account',
    ].includes(error)
  )
    return 'Add and verify an email address in Discord, then try connecting again.';
  if (error === 'discord_already_connected')
    return 'Disconnect your current Discord account before connecting another.';
  return `${discord ? 'Discord' : 'Google'} sign-in could not be completed. Please try again.`;
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
