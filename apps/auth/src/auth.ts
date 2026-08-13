import { randomBytes } from "node:crypto";

import { betterAuth } from "better-auth";
import { createAuthMiddleware } from "better-auth/api";
import { deleteSessionCookie } from "better-auth/cookies";
import { jwt, twoFactor } from "better-auth/plugins";
import { Pool } from "pg";

import { config } from "./config.js";
import { createEmailProvider } from "./email/provider.js";
import { passwordResetEmail, verificationEmail } from "./email/templates.js";
import { HttpIdentityLifecycleGateway } from "./identity/gateway.js";
import { AccountLifecycleService } from "./lifecycle/service.js";
import { log as writeLog } from "./logging.js";
import { buildAccessClaims } from "./security/access-policy.js";
import { hashedBackupCodeCodec } from "./security/backup-code-store.js";

const SEVEN_DAYS_SECONDS = 7 * 24 * 60 * 60;
const ONE_DAY_SECONDS = 24 * 60 * 60;
const FIVE_MINUTES_SECONDS = 5 * 60;
const THIRTY_DAYS_SECONDS = 30 * 24 * 60 * 60;

export const pool = new Pool({
  connectionString: config.databaseUrl,
  max: 10,
  application_name: "rsp-auth",
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

function isMfaCompletionPath(path: string | undefined): boolean {
  return path === "/two-factor/verify-totp" || path === "/two-factor/verify-backup-code";
}

const securitySensitiveReasonByPath = {
  "/change-password": "password_changed",
  "/change-email": "email_changed",
  "/two-factor/disable": "mfa_changed",
  "/two-factor/generate-backup-codes": "mfa_changed",
  "/unlink-account": "provider_linked",
} as const;

const authAfterHook = createAuthMiddleware(async (context) => {
  if (context.path === "/callback/google") {
    const newSession = context.context.newSession;
    if (!newSession?.user.twoFactorEnabled) return;

    deleteSessionCookie(context, true);
    await context.context.internalAdapter.deleteSession(newSession.session.token);
    context.context.setNewSession(null);

    const challengeMaxAgeSeconds = 10 * 60;
    const identifier = `2fa-${randomBytes(20).toString("base64url")}`;
    const expiresAt = new Date(Date.now() + challengeMaxAgeSeconds * 1_000);
    await context.context.internalAdapter.createVerificationValue({
      value: newSession.user.id,
      identifier,
      expiresAt,
    });
    await context.context.internalAdapter.createVerificationValue({
      value: "0",
      identifier: `2fa-attempts-${identifier}`,
      expiresAt,
    });
    const challengeCookie = context.context.createAuthCookie("two_factor", {
      maxAge: challengeMaxAgeSeconds,
    });
    await context.setSignedCookie(
      challengeCookie.name,
      identifier,
      context.context.secret,
      challengeCookie.attributes,
    );
    throw context.redirect(`${config.baseUrl}/two-factor?source=google`);
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

const socialProviders = config.google
  ? {
      google: {
        clientId: config.google.clientId,
        clientSecret: config.google.clientSecret,
        prompt: "select_account" as const,
      },
    }
  : {};

export const auth = betterAuth({
  appName: "RSP",
  baseURL: config.baseUrl,
  basePath: "/api/auth",
  secret: config.secret,
  database: pool,
  trustedOrigins: [...config.trustedOrigins],
  disabledPaths: ["/delete-user", "/two-factor/view-backup-codes"],
  advanced: {
    cookiePrefix: "rsp-auth",
    useSecureCookies: config.secureCookies,
    disableCSRFCheck: false,
    disableOriginCheck: false,
    trustedProxyHeaders: config.trustProxyHeaders,
    ipAddress: {
      ipAddressHeaders: config.trustProxyHeaders ? [config.ipAddressHeader] : [],
      ipv6Subnet: 64,
    },
    defaultCookieAttributes: {
      httpOnly: true,
      sameSite: "lax",
      secure: config.secureCookies,
      path: "/",
    },
  },
  user: {
    modelName: "users",
    fields: {
      createdAt: "created_at",
      updatedAt: "updated_at",
      emailVerified: "email_verified",
    },
    additionalFields: {
      accountState: {
        type: ["active", "suspended", "deletion_pending", "deleted"],
        fieldName: "account_state",
        required: true,
        defaultValue: "active",
        input: false,
        returned: false,
      },
      securityVersion: {
        type: "number",
        fieldName: "security_version",
        required: true,
        defaultValue: 1,
        input: false,
        returned: false,
      },
      deletionRequestedAt: {
        type: "date",
        fieldName: "deletion_requested_at",
        required: false,
        input: false,
        returned: false,
      },
      deletionRecoveryDeadline: {
        type: "date",
        fieldName: "deletion_recovery_deadline",
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
          subject: "Confirm your RSP email change",
        });
      },
    },
    deleteUser: { enabled: false },
  },
  session: {
    modelName: "sessions",
    fields: {
      createdAt: "created_at",
      updatedAt: "updated_at",
      userId: "user_id",
      expiresAt: "expires_at",
      ipAddress: "ip_address",
      userAgent: "user_agent",
    },
    expiresIn: SEVEN_DAYS_SECONDS,
    updateAge: ONE_DAY_SECONDS,
    freshAge: FIVE_MINUTES_SECONDS,
    cookieCache: { enabled: false },
    additionalFields: {
      mfaVerifiedAt: {
        type: "date",
        fieldName: "mfa_verified_at",
        required: false,
        input: false,
        returned: false,
      },
    },
  },
  account: {
    modelName: "accounts",
    fields: {
      createdAt: "created_at",
      updatedAt: "updated_at",
      userId: "user_id",
      providerId: "provider_id",
      accountId: "account_id",
      accessToken: "access_token",
      refreshToken: "refresh_token",
      idToken: "id_token",
      accessTokenExpiresAt: "access_token_expires_at",
      refreshTokenExpiresAt: "refresh_token_expires_at",
    },
    updateAccountOnSignIn: true,
    encryptOAuthTokens: true,
    storeStateStrategy: "database",
    storeAccountCookie: false,
    accountLinking: {
      enabled: true,
      disableImplicitLinking: true,
      allowDifferentEmails: false,
      allowUnlinkingAll: false,
      updateUserInfoOnLink: false,
      trustedProviders: [],
    },
  },
  verification: {
    modelName: "verifications",
    fields: {
      createdAt: "created_at",
      updatedAt: "updated_at",
      expiresAt: "expires_at",
    },
    storeIdentifier: "hashed",
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
      await accountLifecycle.revokeSessions(user.id, "password_reset");
    },
  },
  emailVerification: {
    sendOnSignUp: true,
    autoSignInAfterVerification: false,
    expiresIn: 60 * 60,
    async sendVerificationEmail({ user, url }) {
      await emailProvider.send(verificationEmail(user.email, url));
    },
    async afterEmailVerification(user) {
      await identityGateway.publish({
        type: "email_verified",
        authUserId: user.id,
        occurredAt: new Date().toISOString(),
      });
    },
  },
  socialProviders,
  plugins: [
    twoFactor({
      issuer: "RSP",
      allowPasswordless: true,
      twoFactorTable: "two_factors",
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
        user: { fields: { twoFactorEnabled: "two_factor_enabled" } },
        twoFactor: {
          modelName: "two_factors",
          fields: {
            userId: "user_id",
            backupCodes: "backup_codes",
            failedVerificationCount: "failed_verification_count",
            lockedUntil: "locked_until",
          },
        },
      },
    }),
    jwt({
      jwks: {
        keyPairConfig: { alg: "EdDSA", crv: "Ed25519" },
        rotationInterval: THIRTY_DAYS_SECONDS,
        gracePeriod: THIRTY_DAYS_SECONDS,
      },
      jwt: {
        issuer: config.baseUrl,
        audience: config.apiAudience,
        expirationTime: "5m",
        getSubject: ({ user }) => user.id,
        definePayload: ({ user, session }) =>
          buildAccessClaims({
            user: {
              id: user.id,
              emailVerified: user.emailVerified,
              accountState: user.accountState,
              securityVersion: user.securityVersion,
            },
            session: {
              id: session.id,
              mfaVerifiedAt: session.mfaVerifiedAt,
            },
            mfaMaxAgeSeconds: config.mfaMaxAgeSeconds,
          }),
      },
      schema: {
        jwks: {
          modelName: "jwks",
          fields: {
            publicKey: "public_key",
            privateKey: "private_key",
            createdAt: "created_at",
            expiresAt: "expires_at",
          },
        },
      },
    }),
  ],
  rateLimit: {
    enabled: true,
    storage: "database",
    modelName: "rate_limits",
    fields: {
      lastRequest: "last_request",
    },
    window: 60,
    max: 30,
    customRules: {
      "/sign-in/email": { window: 60, max: 5 },
      "/sign-up/email": { window: 60, max: 5 },
      "/send-verification-email": { window: 60, max: 5 },
      "/request-password-reset": { window: 60, max: 5 },
      "/forget-password": { window: 60, max: 5 },
      "/reset-password": { window: 60, max: 5 },
      "/two-factor/*": { window: 60, max: 5 },
      "/token": { window: 60, max: 20 },
    },
  },
  databaseHooks: {
    user: {
      create: {
        async after(user) {
          await identityGateway.publish({
            type: "auth_user_created",
            authUserId: user.id,
            email: user.email,
            emailVerified: user.emailVerified,
            occurredAt: new Date().toISOString(),
          });
        },
      },
    },
    account: {
      create: {
        async after(account, context) {
          const linkingSessionUserId = context?.context.session?.user.id;
          if (
            context?.path === "/callback/google" &&
            account.providerId === "google" &&
            linkingSessionUserId === account.userId
          ) {
            await accountLifecycle.revokeSessions(account.userId, "provider_linked");
          }
        },
      },
    },
    session: {
      create: {
        async before(session, context) {
          return {
            data: {
              ...session,
              mfaVerifiedAt: isMfaCompletionPath(context?.path) ? new Date() : null,
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
    level: config.nodeEnvironment === "production" ? "info" : "debug",
    log(level, message, ...args) {
      writeLog(level, message, { details: args });
    },
  },
});

export type Auth = typeof auth;
