import { describe, expect, it, vi } from "vitest";

import { HttpIdentityLifecycleGateway } from "../src/identity/gateway.js";

describe("identity lifecycle callback", () => {
  it("sends an authenticated, bounded event without exposing the token in errors", async () => {
    const fetchImplementation = vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 204 }));
    const gateway = new HttpIdentityLifecycleGateway({
      baseUrl: "http://api:8081",
      token: "internal-secret",
      timeoutMs: 1_000,
      fetchImplementation,
    });
    await gateway.publish({
      type: "sessions_revoked",
      authUserId: "auth-1",
      reason: "suspension",
      occurredAt: "2026-08-13T00:00:00.000Z",
    });
    expect(fetchImplementation).toHaveBeenCalledOnce();
    const [url, init] = fetchImplementation.mock.calls[0] ?? [];
    expect(url).toBe("http://api:8081/internal/auth/lifecycle-events");
    expect(new Headers(init?.headers).get("authorization")).toBe("Bearer internal-secret");
    expect(init?.body).toContain('"authUserId":"auth-1"');
  });

  it("reports only the downstream status on failure", async () => {
    const gateway = new HttpIdentityLifecycleGateway({
      baseUrl: "http://api:8081",
      token: "must-not-leak",
      timeoutMs: 1_000,
      fetchImplementation: vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 503 })),
    });
    await expect(
      gateway.publish({
        type: "email_verified",
        authUserId: "auth-1",
        occurredAt: "2026-08-13T00:00:00.000Z",
      }),
    ).rejects.toThrow("status 503");
    await expect(
      gateway.publish({
        type: "email_verified",
        authUserId: "auth-1",
        occurredAt: "2026-08-13T00:00:00.000Z",
      }),
    ).rejects.not.toThrow("must-not-leak");
  });
});
