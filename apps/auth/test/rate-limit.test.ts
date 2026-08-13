import { describe, expect, it } from "vitest";

import {
  authRateLimitKey,
  authRateLimitPolicy,
  FixedWindowRateLimiter,
} from "../src/security/rate-limit.js";

describe("auth rate limiting", () => {
  it("enforces a fixed window and resets without extending it", () => {
    let currentTime = 1_000;
    const limiter = new FixedWindowRateLimiter(() => currentTime);
    expect(limiter.consume("account", { max: 2, windowSeconds: 60 }).allowed).toBe(true);
    expect(limiter.consume("account", { max: 2, windowSeconds: 60 }).allowed).toBe(true);
    const rejected = limiter.consume("account", { max: 2, windowSeconds: 60 });
    expect(rejected.allowed).toBe(false);
    expect(rejected.retryAfterSeconds).toBe(60);
    currentTime += 60_000;
    expect(limiter.consume("account", { max: 2, windowSeconds: 60 }).allowed).toBe(true);
  });

  it("uses five-per-minute limits for authentication and verification", () => {
    expect(authRateLimitPolicy("/api/auth/sign-in/email")).toEqual({ max: 5, windowSeconds: 60 });
    expect(authRateLimitPolicy("/api/auth/two-factor/verify-totp")).toEqual({
      max: 5,
      windowSeconds: 60,
    });
    expect(authRateLimitPolicy("/api/auth/token").max).toBe(20);
  });

  it("keys account attempts without retaining raw email or cookies", () => {
    const key = authRateLimitKey({
      pathname: "/api/auth/sign-in/email",
      ipAddress: "192.0.2.1",
      cookieHeader: "rsp-auth.session=secret-cookie",
      requestBody: { email: "Member@Example.org" },
      secret: "x".repeat(32),
    });
    expect(key).not.toContain("member@example.org");
    expect(key).not.toContain("secret-cookie");
    expect(key).toMatch(/^\/api\/auth\/sign-in\/email:/);
  });
});
