import { randomBytes } from 'node:crypto';

import { APIError, betterAuth } from 'better-auth';
import { createAuthMiddleware, getSessionFromCtx } from 'better-auth/api';
import { deleteSessionCookie } from 'better-auth/cookies';
import { jwt, twoFactor } from 'better-auth/plugins';
import { Pool } from 'pg';

import { config } from './config.js';
import {
  captureDiscordProfile,
  oauthRequestContext,
} from './identity/discord-profile.js';
import { createEmailProvider } from './email/provider.js';
import { passwordResetEmail, verificationEmail } from './email/templates.js';
import { HttpIdentityLifecycleGateway } from './identity/gateway.js';
import { AccountLifecycleService } from './lifecycle/service.js';
import { log as writeLog } from './logging.js';
import { buildAccessClaims } from './security/access-policy.js';
import { hashedBackupCodeCodec } from './security/backup-code-store.js';

const SEVEN_DAYS_SECONDS = 7 * 24 * 60 * 60;
const ONE_DAY_SECONDS = 24 * 60 * 60;
const FIVE_MINUTES_SECONDS = 5 * 60;
const THIRTY_DAYS_SECONDS = 30 * 24 * 60 * 60;

export const pool = new Pool({
  connectionString: config.databaseUrl,
  max: 10,
  application_name: 'rsp-auth',
  idleTimeoutMillis: 30_000,
  connectionTimeoutMillis: 5_000,
});

const emailProvider = createEmailProvider(config.email);
const identityGateway = new HttpIdentityLifecycleGateway({
  baseUrl: config.identityService.baseUrl,
  token: config.identityService.token,
  timeoutMs: config.identityService.timeoutMs,
});

export const accountLifecycle = new AccountLifecycleService(
  pool,
  emailProvider,
  identityGateway,
);

async function currentSecurityVersion(authUserId: string): Promise<number> {
  const result = await pool.query<{ security_version: number }>(
    'SELECT security_version FROM users WHERE id = $1',
    [authUserId],
  );
  const version = result.rows[0]?.security_version;
  if (!version) throw new Error('auth user security version not found');
  return version;
}

function isMfaCompletionPath(path: string | undefined): boolean {
  return (
    path === '/two-factor/verify-totp' ||
    path === '/two-factor/verify-backup-code'
  );
}

interface VerificationTokenPayload {
  readonly requestType?: string;
  readonly updateTo?: string;
}

function decodeJwtPayload(token: string): VerificationTokenPayload | undefined {
  const encodedPayload = token.split('.')[1];
  if (!encodedPayload) return undefined;
  try {
    const payload: unknown = JSON.parse(
      Buffer.from(encodedPayload, 'base64url').toString('utf8'),
    );
    if (!payload || typeof payload !== 'object') return undefined;
    const requestType = Reflect.get(payload, 'requestType');
    const updateTo = Reflect.get(payload, 'updateTo');
    return {
      ...(typeof requestType === 'string' ? { requestType } : {}),
      ...(typeof updateTo === 'string' ? { updateTo } : {}),
    };
  } catch {
    return undefined;
  }
}

const securitySensitiveReasonByPath = {
  '/change-password': 'password_changed',
  '/two-factor/generate-backup-codes': 'mfa_changed',
  '/unlink-account': 'provider_linked',
} as const;

const authAfterHook = createAuthMiddleware(async (context) => {
  const callbackProvider = context.path?.startsWith('/callback/')
    ? (context.params?.id ?? context.path.split('/')[2])
    : undefined;
  if (callbackProvider === 'google' || callbackProvider === 'discord') {
    const newSession = context.context.newSession;
    if (!newSession?.user.twoFactorEnabled) return;

    deleteSessionCookie(context, true);
    await context.context.internalAdapter.deleteSession(
      newSession.session.token,
    );
    context.context.setNewSession(null);

    const challengeMaxAgeSeconds = 10 * 60;
    const identifier = `2fa-${randomBytes(20).toString('base64url')}`;
    const expiresAt = new Date(Date.now() + challengeMaxAgeSeconds * 1_000);
    await context.context.internalAdapter.createVerificationValue({
      value: newSession.user.id,
      identifier,
      expiresAt,
    });
    await context.context.internalAdapter.createVerificationValue({
      value: '0',
      identifier: `2fa-attempts-${identifier}`,
      expiresAt,
    });
    const challengeCookie = context.context.createAuthCookie('two_factor', {
      maxAge: challengeMaxAgeSeconds,
    });
    await context.setSignedCookie(
      challengeCookie.name,
      identifier,
      context.context.secret,
      challengeCookie.attributes,
    );
    // The app's own two-factor page, the same one password sign-in uses. It
    // verifies with the challenge cookie set above.
    throw context.redirect(`${config.baseUrl}/two-factor`);
  }

  if (
    context.path === '/two-factor/disable' &&
    !(context.context.returned instanceof Error) &&
    context.context.session?.user.twoFactorEnabled === true
  ) {
    await accountLifecycle.recordMfaState(
      context.context.session.user.id,
      false,
    );
    deleteSessionCookie(context, true);
  }

  const reason =
    securitySensitiveReasonByPath[
      context.path as keyof typeof securitySensitiveReasonByPath
    ];
  const authUserId = context.context.session?.user.id;
  if (reason && authUserId && !(context.context.returned instanceof Error)) {
    await accountLifecycle.revokeSessions(authUserId, reason);
  }
});

const socialProviders = {
  ...(config.google
    ? {
        google: {
          clientId: config.google.clientId,
          clientSecret: config.google.clientSecret,
          prompt: 'select_account' as const,
        },
      }
    : {}),
  ...(config.discord
    ? {
        discord: {
          clientId: config.discord.clientId,
          clientSecret: config.discord.clientSecret,
          disableSignUp: true,
          overrideUserInfoOnSignIn: false,
          prompt: 'consent' as const,
        },
      }
    : {}),
};

export const auth = betterAuth({
  appName: 'RSP',
  baseURL: config.baseUrl,
  basePath: '/api/auth',
  secret: config.secret,
  database: pool,
  trustedOrigins: [...config.trustedOrigins],
  // Stored OAuth tokens stay server-side; only the avatar refresh uses them.
  disabledPaths: [
    '/delete-user',
    '/two-factor/view-backup-codes',
    '/get-access-token',
    '/refresh-token',
  ],
  advanced: {
    cookiePrefix: 'rsp-auth',
    useSecureCookies: config.secureCookies,
    disableCSRFCheck: false,
    disableOriginCheck: false,
    trustedProxyHeaders: config.trustProxyHeaders,
    ipAddress: {
      ipAddressHeaders: config.trustProxyHeaders
        ? [config.ipAddressHeader]
        : [],
      ipv6Subnet: 64,
    },
    defaultCookieAttributes: {
      httpOnly: true,
      sameSite: 'lax',
      secure: config.secureCookies,
      path: '/',
    },
  },
  user: {
    async validateUserInfo({ user, source }, context) {
      const provider = source.oauth?.providerId;
      if (!provider) return;
      if (source.action === 'link-account') {
        const session = await getSessionFromCtx(context);
        if (
          !session ||
          session.user.id !== user.id ||
          Date.now() - new Date(session.session.createdAt).getTime() >
            FIVE_MINUTES_SECONDS * 1000
        ) {
          return { error: 'reauthentication_required' };
        }
        context.context.session = session;
        const local = await pool.query<{
          email: string;
          email_verified: boolean;
          account_state: string;
        }>(
          'SELECT email, email_verified, account_state FROM users WHERE id = $1',
          [user.id],
        );
        const member = local.rows[0];
        if (!member?.email_verified || member.account_state !== 'active') {
          return {
            error: 'email_not_verified',
            errorDescription:
              'Sign in with your verified RSP email or Google first.',
          };
        }
        if (
          provider === 'google' &&
          user.email?.toLowerCase() !== member.email.toLowerCase()
        ) {
          return { error: 'email_does_not_match' };
        }
        if (provider === 'discord') {
          const methods = await pool.query<{
            provider_id: string;
            account_id: string;
          }>(
            'SELECT provider_id, account_id FROM accounts WHERE user_id = $1',
            [user.id],
          );
          if (
            !methods.rows.some(
              (account) =>
                account.provider_id === 'credential' ||
                account.provider_id === 'google',
            )
          ) {
            return { error: 'primary_sign_in_required' };
          }
          if (
            methods.rows.some(
              (account) =>
                account.provider_id === 'discord' &&
                account.account_id !== source.oauth?.profile?.id,
            )
          ) {
            return {
              error: 'discord_already_connected',
              errorDescription:
                'Disconnect your current Discord account first.',
            };
          }
        }
        const requestState = oauthRequestContext.getStore();
        const accountId =
          provider === 'discord'
            ? source.oauth?.profile?.id
            : source.oauth?.profile?.sub;
        if (requestState && user.id && typeof accountId === 'string') {
          requestState.link = {
            authUserId: user.id,
            providerId: provider,
            accountId,
          };
        }
      }
      if (provider === 'discord') {
        if (source.action === 'create-user')
          return { error: 'signup_disabled' };
        if (user.id && source.oauth?.profile) {
          try {
            captureDiscordProfile(user.id, source.oauth.profile);
          } catch {
            writeLog('warn', 'Discord avatar could not be read');
          }
        }
      }
    },
    modelName: 'users',
    fields: {
      createdAt: 'created_at',
      updatedAt: 'updated_at',
      emailVerified: 'email_verified',
    },
    additionalFields: {
      accountState: {
        type: ['active', 'suspended', 'deletion_pending', 'deleted'],
        fieldName: 'account_state',
        required: true,
        defaultValue: 'active',
        input: false,
        returned: false,
      },
      securityVersion: {
        type: 'number',
        fieldName: 'security_version',
        required: true,
        defaultValue: 1,
        input: false,
        returned: false,
      },
      deletionRequestedAt: {
        type: 'date',
        fieldName: 'deletion_requested_at',
        required: false,
        input: false,
        returned: false,
      },
      deletionRecoveryDeadline: {
        type: 'date',
        fieldName: 'deletion_recovery_deadline',
        required: false,
        input: false,
        returned: false,
      },
    },
    changeEmail: {
      enabled: true,
      updateEmailWithoutVerification: false,
      async sendChangeEmailConfirmation({ user, url }) {
        await emailProvider.send({
          ...verificationEmail(user.email, url),
          subject: 'Confirm your RSP email change',
        });
      },
    },
    deleteUser: { enabled: false },
  },
  session: {
    modelName: 'sessions',
    fields: {
      createdAt: 'created_at',
      updatedAt: 'updated_at',
      userId: 'user_id',
      expiresAt: 'expires_at',
      ipAddress: 'ip_address',
      userAgent: 'user_agent',
    },
    expiresIn: SEVEN_DAYS_SECONDS,
    updateAge: ONE_DAY_SECONDS,
    freshAge: FIVE_MINUTES_SECONDS,
    cookieCache: { enabled: false },
    additionalFields: {
      mfaVerifiedAt: {
        type: 'date',
        fieldName: 'mfa_verified_at',
        required: false,
        input: false,
        returned: false,
      },
    },
  },
  account: {
    modelName: 'accounts',
    fields: {
      createdAt: 'created_at',
      updatedAt: 'updated_at',
      userId: 'user_id',
      providerId: 'provider_id',
      accountId: 'account_id',
      accessToken: 'access_token',
      refreshToken: 'refresh_token',
      idToken: 'id_token',
      accessTokenExpiresAt: 'access_token_expires_at',
      refreshTokenExpiresAt: 'refresh_token_expires_at',
    },
    updateAccountOnSignIn: true,
    encryptOAuthTokens: true,
    storeStateStrategy: 'database',
    storeAccountCookie: false,
    accountLinking: {
      enabled: true,
      disableImplicitLinking: true,
      allowDifferentEmails: true,
      allowUnlinkingAll: false,
      updateUserInfoOnLink: false,
      trustedProviders: [],
    },
  },
  verification: {
    modelName: 'verifications',
    fields: {
      createdAt: 'created_at',
      updatedAt: 'updated_at',
      expiresAt: 'expires_at',
    },
    storeIdentifier: 'hashed',
    storeInDatabase: true,
  },
  emailAndPassword: {
    enabled: true,
    requireEmailVerification: true,
    autoSignIn: false,
    minPasswordLength: 12,
    maxPasswordLength: 128,
    resetPasswordTokenExpiresIn: 60 * 60,
    revokeSessionsOnPasswordReset: true,
    async sendResetPassword({ user, url }) {
      await emailProvider.send(passwordResetEmail(user.email, url));
    },
    async onPasswordReset({ user }) {
      await accountLifecycle.revokeSessions(user.id, 'password_reset');
    },
  },
  emailVerification: {
    sendOnSignUp: true,
    autoSignInAfterVerification: false,
    expiresIn: 60 * 60,
    async sendVerificationEmail({ user, url }) {
      // Signed up from an invitation to this address: already verified.
      if (user.emailVerified) return;
      await emailProvider.send(verificationEmail(user.email, url));
    },
    async afterEmailVerification(user, request) {
      const token = request
        ? new URL(request.url).searchParams.get('token')
        : null;
      const tokenPayload = token ? decodeJwtPayload(token) : undefined;
      const emailChangeCompleted =
        tokenPayload?.requestType === 'change-email-verification' &&
        tokenPayload.updateTo === user.email;
      if (emailChangeCompleted) {
        await accountLifecycle.revokeSessions(user.id, 'email_changed');
        await accountLifecycle.queueIdentityEvent(
          {
            type: 'email_changed',
            authUserId: user.id,
            email: user.email,
            emailVerified: true,
            securityVersion: await currentSecurityVersion(user.id),
            occurredAt: new Date().toISOString(),
          },
          true,
        );
        return;
      }
      await accountLifecycle.queueIdentityEvent({
        type: 'email_verified',
        authUserId: user.id,
        securityVersion: await currentSecurityVersion(user.id),
        occurredAt: new Date().toISOString(),
      });
    },
  },
  socialProviders,
  plugins: [
    twoFactor({
      issuer: 'RSP',
      allowPasswordless: true,
      twoFactorTable: 'two_factors',
      twoFactorCookieMaxAge: 10 * 60,
      trustDeviceMaxAge: THIRTY_DAYS_SECONDS,
      backupCodeOptions: {
        amount: 10,
        length: 12,
        storeBackupCodes: hashedBackupCodeCodec,
      },
      accountLockout: {
        enabled: true,
        maxFailedAttempts: 5,
        durationSeconds: 15 * 60,
      },
      schema: {
        user: { fields: { twoFactorEnabled: 'two_factor_enabled' } },
        twoFactor: {
          modelName: 'two_factors',
          fields: {
            userId: 'user_id',
            backupCodes: 'backup_codes',
            failedVerificationCount: 'failed_verification_count',
            lockedUntil: 'locked_until',
          },
        },
      },
    }),
    jwt({
      jwks: {
        keyPairConfig: { alg: 'EdDSA', crv: 'Ed25519' },
        rotationInterval: THIRTY_DAYS_SECONDS,
        gracePeriod: THIRTY_DAYS_SECONDS,
      },
      jwt: {
        issuer: config.baseUrl,
        audience: config.apiAudience,
        expirationTime: '5m',
        getSubject: ({ user }) => user.id,
        async definePayload({ user, session }) {
          // Better Auth deliberately removes fields marked `returned: false`
          // from the public session object. Fetch the authoritative private
          // access state here rather than either exposing it to the browser or
          // silently signing a JWT with the field defaults.
          const result = await pool.query<{
            email_verified: boolean;
            account_state:
              'active' | 'suspended' | 'deletion_pending' | 'deleted';
            security_version: number;
            mfa_verified_at: Date | null;
          }>(
            `SELECT users.email_verified, users.account_state, users.security_version,
                    sessions.mfa_verified_at
               FROM sessions
               JOIN users ON users.id = sessions.user_id
              WHERE sessions.id = $1 AND users.id = $2`,
            [session.id, user.id],
          );
          const access = result.rows[0];
          if (!access) throw new Error('access-token session no longer exists');
          return buildAccessClaims({
            user: {
              id: user.id,
              emailVerified: access.email_verified,
              accountState: access.account_state,
              securityVersion: access.security_version,
            },
            session: {
              id: session.id,
              mfaVerifiedAt: access.mfa_verified_at,
            },
            mfaMaxAgeSeconds: config.mfaMaxAgeSeconds,
          });
        },
      },
      schema: {
        jwks: {
          modelName: 'jwks',
          fields: {
            publicKey: 'public_key',
            privateKey: 'private_key',
            createdAt: 'created_at',
            expiresAt: 'expires_at',
          },
        },
      },
    }),
  ],
  rateLimit: {
    enabled: true,
    storage: 'database',
    modelName: 'rate_limits',
    fields: {
      lastRequest: 'last_request',
    },
    window: 60,
    max: 30,
    customRules: {
      '/sign-in/email': { window: 60, max: 5 },
      '/sign-up/email': { window: 60, max: 5 },
      '/send-verification-email': { window: 60, max: 5 },
      '/request-password-reset': { window: 60, max: 5 },
      '/forget-password': { window: 60, max: 5 },
      '/reset-password': { window: 60, max: 5 },
      '/two-factor/*': { window: 60, max: 5 },
      '/token': { window: 60, max: 20 },
    },
  },
  databaseHooks: {
    user: {
      create: {
        // Signing up from an invitation link proves the person reads the
        // invited inbox, so an account for that address starts verified.
        async before(user, context) {
          const invitationToken = (
            context?.body as { invitationToken?: unknown } | undefined
          )?.invitationToken;
          if (
            context?.path !== '/sign-up/email' ||
            typeof invitationToken !== 'string' ||
            invitationToken.length !== 43
          )
            return;
          if (
            await identityGateway.invitationSentTo(invitationToken, user.email)
          )
            return { data: { ...user, emailVerified: true } };
        },
        async after(user) {
          await accountLifecycle.queueIdentityEvent({
            type: 'auth_user_created',
            authUserId: user.id,
            email: user.email,
            name: user.name,
            emailVerified: user.emailVerified,
            securityVersion: await currentSecurityVersion(user.id),
            occurredAt: new Date().toISOString(),
          });
        },
      },
    },
    session: {
      create: {
        async before(session, context) {
          const state = await pool.query<{ account_state: string }>(
            'SELECT account_state FROM users WHERE id = $1',
            [session.userId],
          );
          if (state.rows[0]?.account_state !== 'active') {
            throw new APIError('FORBIDDEN', {
              message: 'Account is unavailable',
            });
          }
          return {
            data: {
              ...session,
              mfaVerifiedAt: isMfaCompletionPath(context?.path)
                ? new Date()
                : null,
            },
          };
        },
      },
    },
  },
  hooks: {
    after: authAfterHook,
  },
  logger: {
    level: config.nodeEnvironment === 'production' ? 'info' : 'debug',
    log(level, message, ...args) {
      writeLog(level, message, { details: args });
    },
  },
});

export type Auth = typeof auth;
