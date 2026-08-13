import { describe, expect, it } from "vitest";

import { loadConfig } from "../src/config.js";

describe("loadConfig", () => {
  it("provides safe local service defaults", () => {
    const configuration = loadConfig({ NODE_ENV: "test" });
    expect(configuration.baseUrl).toBe("http://localhost:8080");
    expect(configuration.secureCookies).toBe(false);
    expect(configuration.trustedOrigins).toContain("http://localhost:8080");
    expect(configuration.email.provider).toBe("smtp");
  });

  it("requires paired Google credentials", () => {
    expect(() =>
      loadConfig({ NODE_ENV: "test", GOOGLE_CLIENT_ID: "client-only" }),
    ).toThrow(/GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET/);
  });

  it("fails closed on production-only security settings", () => {
    expect(() => loadConfig({ NODE_ENV: "production" })).toThrow();
  });

  it("accepts a complete production configuration", () => {
    const configuration = loadConfig({
      NODE_ENV: "production",
      BETTER_AUTH_URL: "https://rsp.example.org",
      BETTER_AUTH_SECRET: "a".repeat(64),
      DATABASE_URL: "postgresql://rsp:secret@postgres:5432/rsp",
      AUTH_TRUSTED_ORIGINS: "https://rsp.example.org",
      AUTH_COOKIE_SECURE: "true",
      GOOGLE_CLIENT_ID: "google-client",
      GOOGLE_CLIENT_SECRET: "google-secret",
      EMAIL_PROVIDER: "ses",
      EMAIL_FROM: "RSP <noreply@example.org>",
      IDENTITY_SERVICE_TOKEN: "b".repeat(64),
    });
    expect(configuration.secureCookies).toBe(true);
    expect(configuration.google?.clientId).toBe("google-client");
    expect(configuration.email.provider).toBe("ses");
  });

  it("rejects trusted origins containing paths", () => {
    expect(() =>
      loadConfig({ NODE_ENV: "test", AUTH_TRUSTED_ORIGINS: "https://example.org/path" }),
    ).toThrow(/must be an origin/);
  });
});
