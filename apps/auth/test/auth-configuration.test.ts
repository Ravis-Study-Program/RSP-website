import { afterAll, describe, expect, it } from 'vitest';

import { auth, pool } from '../src/auth.js';

afterAll(async () => {
  await pool.end();
});

describe('Better Auth production contract', () => {
  it('uses database-backed rolling seven-day sessions and mandatory email verification', () => {
    expect(auth.options.session?.expiresIn).toBe(7 * 24 * 60 * 60);
    expect(auth.options.session?.updateAge).toBe(24 * 60 * 60);
    expect(auth.options.session?.cookieCache?.enabled).toBe(false);
    expect(auth.options.emailAndPassword?.requireEmailVerification).toBe(true);
    expect(auth.options.emailAndPassword?.revokeSessionsOnPasswordReset).toBe(
      true,
    );
    expect(auth.options.emailVerification?.autoSignInAfterVerification).toBe(
      false,
    );
  });

  it('disables implicit provider linking and different-email merges', () => {
    expect(auth.options.account?.accountLinking).toMatchObject({
      enabled: true,
      disableImplicitLinking: true,
      allowDifferentEmails: false,
      allowUnlinkingAll: false,
      updateUserInfoOnLink: false,
    });
  });

  it('pins TOTP, hashed single-use backups, and five-minute rotating JWTs', () => {
    const plugins = auth.options.plugins ?? [];
    const twoFactorPlugin = plugins.find(
      (plugin) => plugin.id === 'two-factor',
    );
    const jwtPlugin = plugins.find((plugin) => plugin.id === 'jwt');
    expect(twoFactorPlugin).toBeDefined();
    expect(jwtPlugin).toBeDefined();
    expect(twoFactorPlugin?.options).toMatchObject({
      issuer: 'RSP',
      allowPasswordless: true,
      twoFactorTable: 'two_factors',
    });
    expect(jwtPlugin?.options).toMatchObject({
      jwks: {
        rotationInterval: 30 * 24 * 60 * 60,
        gracePeriod: 30 * 24 * 60 * 60,
      },
      jwt: { expirationTime: '5m' },
    });
  });

  it('keeps CSRF/origin checks enabled and disables hard deletion/viewing backup codes', () => {
    expect(auth.options.advanced?.disableCSRFCheck).toBe(false);
    expect(auth.options.advanced?.disableOriginCheck).toBe(false);
    expect(auth.options.disabledPaths).toEqual(
      expect.arrayContaining(['/delete-user', '/two-factor/view-backup-codes']),
    );
  });

  it('checks account state before creating any browser session', () => {
    expect(auth.options.databaseHooks?.session?.create?.before).toBeTypeOf(
      'function',
    );
  });
});
