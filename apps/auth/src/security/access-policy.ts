export const ACCOUNT_STATES = [
  'active',
  'suspended',
  'deletion_pending',
  'deleted',
] as const;

export type AccountState = (typeof ACCOUNT_STATES)[number];

export class AccessPolicyError extends Error {
  constructor(
    readonly code:
      | 'email_not_verified'
      | 'account_suspended'
      | 'account_deletion_pending'
      | 'account_deleted',
  ) {
    super(code);
    this.name = 'AccessPolicyError';
  }
}

export interface AuthUserForClaims {
  readonly id: string;
  readonly emailVerified: boolean;
  readonly accountState?: AccountState | null;
  readonly securityVersion?: number | null;
}

export interface AuthSessionForClaims {
  readonly id: string;
  readonly mfaVerifiedAt?: string | Date | null;
}

export interface RspAccessClaims {
  readonly authUserId: string;
  readonly sessionId: string;
  readonly emailVerified: true;
  readonly accountState: 'active';
  readonly securityVersion: number;
  readonly mfaVerified: boolean;
  readonly mfaVerifiedAt?: string;
}

export function isMfaRecent(
  verifiedAt: string | Date | null | undefined,
  now: Date,
  maxAgeSeconds: number,
): boolean {
  if (!verifiedAt) return false;
  const verifiedAtMs = new Date(verifiedAt).getTime();
  if (!Number.isFinite(verifiedAtMs) || verifiedAtMs > now.getTime())
    return false;
  return now.getTime() - verifiedAtMs <= maxAgeSeconds * 1_000;
}

export function buildAccessClaims(options: {
  readonly user: AuthUserForClaims;
  readonly session: AuthSessionForClaims;
  readonly now?: Date;
  readonly mfaMaxAgeSeconds: number;
}): RspAccessClaims {
  if (!options.user.emailVerified)
    throw new AccessPolicyError('email_not_verified');
  const accountState = options.user.accountState ?? 'active';
  if (accountState !== 'active') {
    const errors = {
      suspended: 'account_suspended',
      deletion_pending: 'account_deletion_pending',
      deleted: 'account_deleted',
    } as const;
    throw new AccessPolicyError(errors[accountState]);
  }

  const now = options.now ?? new Date();
  const mfaVerified = isMfaRecent(
    options.session.mfaVerifiedAt,
    now,
    options.mfaMaxAgeSeconds,
  );
  const mfaVerifiedAt = options.session.mfaVerifiedAt
    ? new Date(options.session.mfaVerifiedAt).toISOString()
    : undefined;
  return {
    authUserId: options.user.id,
    sessionId: options.session.id,
    emailVerified: true,
    accountState: 'active',
    securityVersion: options.user.securityVersion ?? 1,
    mfaVerified,
    ...(mfaVerifiedAt ? { mfaVerifiedAt } : {}),
  };
}

export function requiresRecentMfa(
  globalRoles: readonly string[],
  seasonRole?: string,
): boolean {
  return (
    globalRoles.includes('director') ||
    globalRoles.includes('system_admin') ||
    seasonRole === 'coordinator'
  );
}

export function canPerformPrivilegedAction(options: {
  readonly globalRoles: readonly string[];
  readonly seasonRole?: string;
  readonly mfaVerifiedAt?: string | Date | null;
  readonly now: Date;
  readonly maxAgeSeconds: number;
}): boolean {
  if (!requiresRecentMfa(options.globalRoles, options.seasonRole)) return true;
  return isMfaRecent(options.mfaVerifiedAt, options.now, options.maxAgeSeconds);
}
