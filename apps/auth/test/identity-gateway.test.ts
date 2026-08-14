import { describe, expect, it, vi } from 'vitest';

import { HttpIdentityLifecycleGateway } from '../src/identity/gateway.js';

describe('identity lifecycle callback', () => {
  it('sends an authenticated, bounded event without exposing the token in errors', async () => {
    const fetchImplementation = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(null, { status: 204 }));
    const gateway = new HttpIdentityLifecycleGateway({
      baseUrl: 'http://api:8081',
      token: 'internal-secret',
      timeoutMs: 1_000,
      fetchImplementation,
    });
    await gateway.publish({
      eventId: 'event-1',
      type: 'sessions_revoked',
      authUserId: 'auth-1',
      reason: 'suspension',
      securityVersion: 2,
      occurredAt: '2026-08-13T00:00:00.000Z',
    });
    expect(fetchImplementation).toHaveBeenCalledOnce();
    const [url, init] = fetchImplementation.mock.calls[0] ?? [];
    expect(url).toBe('http://api:8081/internal/auth/lifecycle-events');
    expect(new Headers(init?.headers).get('authorization')).toBe(
      'Bearer internal-secret',
    );
    expect(init?.body).toContain('"authUserId":"auth-1"');
  });

  it('reports only the downstream status on failure', async () => {
    const gateway = new HttpIdentityLifecycleGateway({
      baseUrl: 'http://api:8081',
      token: 'must-not-leak',
      timeoutMs: 1_000,
      fetchImplementation: vi
        .fn<typeof fetch>()
        .mockResolvedValue(new Response(null, { status: 503 })),
    });
    await expect(
      gateway.publish({
        eventId: 'event-2',
        type: 'email_verified',
        authUserId: 'auth-1',
        securityVersion: 1,
        occurredAt: '2026-08-13T00:00:00.000Z',
      }),
    ).rejects.toThrow('status 503');
    await expect(
      gateway.publish({
        eventId: 'event-2',
        type: 'email_verified',
        authUserId: 'auth-1',
        securityVersion: 1,
        occurredAt: '2026-08-13T00:00:00.000Z',
      }),
    ).rejects.not.toThrow('must-not-leak');
  });

  it('publishes verified MFA configuration for pending role activation', async () => {
    const fetchImplementation = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(null, { status: 204 }));
    const gateway = new HttpIdentityLifecycleGateway({
      baseUrl: 'http://api:8081',
      token: 'internal-secret',
      timeoutMs: 1_000,
      fetchImplementation,
    });
    await gateway.publish({
      eventId: 'event-3',
      type: 'mfa_configured',
      authUserId: 'auth-1',
      securityVersion: 3,
      occurredAt: '2026-08-13T00:00:00.000Z',
    });
    expect(fetchImplementation.mock.calls[0]?.[1]?.body).toContain(
      '"type":"mfa_configured"',
    );
  });

  it('publishes verified email changes without exposing account data elsewhere', async () => {
    const fetchImplementation = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(null, { status: 204 }));
    const gateway = new HttpIdentityLifecycleGateway({
      baseUrl: 'http://api:8081',
      token: 'internal-secret',
      timeoutMs: 1_000,
      fetchImplementation,
    });
    await gateway.publish({
      eventId: 'event-4',
      type: 'email_changed',
      authUserId: 'auth-1',
      email: 'new@example.org',
      emailVerified: true,
      securityVersion: 4,
      occurredAt: '2026-08-13T00:00:00.000Z',
    });
    expect(fetchImplementation.mock.calls[0]?.[1]?.body).toContain(
      '"type":"email_changed"',
    );
    expect(fetchImplementation.mock.calls[0]?.[1]?.body).toContain(
      '"email":"new@example.org"',
    );
  });
});
