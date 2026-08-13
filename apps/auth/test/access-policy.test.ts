import { describe, expect, it } from "vitest";

import {
  AccessPolicyError,
  buildAccessClaims,
  canPerformPrivilegedAction,
  isMfaRecent,
  requiresRecentMfa,
} from "../src/security/access-policy.js";

const now = new Date("2026-08-13T12:00:00.000Z");

describe("access-token policy", () => {
  it("emits only stable auth/session and recent MFA claims", () => {
    const claims = buildAccessClaims({
      user: { id: "auth-1", emailVerified: true, accountState: "active", securityVersion: 3 },
      session: { id: "session-1", mfaVerifiedAt: "2026-08-13T11:58:00.000Z" },
      now,
      mfaMaxAgeSeconds: 300,
    });
    expect(claims).toEqual({
      authUserId: "auth-1",
      sessionId: "session-1",
      emailVerified: true,
      accountState: "active",
      securityVersion: 3,
      mfaVerified: true,
      mfaVerifiedAt: "2026-08-13T11:58:00.000Z",
    });
    expect(claims).not.toHaveProperty("email");
  });

  it.each([
    [false, "active", "email_not_verified"],
    [true, "suspended", "account_suspended"],
    [true, "deletion_pending", "account_deletion_pending"],
    [true, "deleted", "account_deleted"],
  ] as const)("rejects emailVerified=%s state=%s", (emailVerified, accountState, code) => {
    expect(() =>
      buildAccessClaims({
        user: { id: "auth-1", emailVerified, accountState },
        session: { id: "session-1" },
        now,
        mfaMaxAgeSeconds: 300,
      }),
    ).toThrow(new AccessPolicyError(code));
  });

  it("rejects future and expired MFA timestamps", () => {
    expect(isMfaRecent("2026-08-13T12:00:01.000Z", now, 300)).toBe(false);
    expect(isMfaRecent("2026-08-13T11:54:59.000Z", now, 300)).toBe(false);
    expect(isMfaRecent("2026-08-13T11:55:00.000Z", now, 300)).toBe(true);
  });

  it("requires MFA for every privileged role scope", () => {
    expect(requiresRecentMfa(["director"])).toBe(true);
    expect(requiresRecentMfa(["system_admin"])).toBe(true);
    expect(requiresRecentMfa([], "coordinator")).toBe(true);
    expect(requiresRecentMfa([], "mentor")).toBe(false);
    expect(
      canPerformPrivilegedAction({
        globalRoles: ["director"],
        mfaVerifiedAt: null,
        now,
        maxAgeSeconds: 300,
      }),
    ).toBe(false);
  });
});
