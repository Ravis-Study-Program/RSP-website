import { describe, expect, it } from 'vitest';

import {
  evaluateExplicitLink,
  evaluateFreshSession,
} from '../src/security/linking-policy.js';

const baseline = {
  authenticatedUserId: 'auth-user',
  sessionCreatedAt: '2026-08-13T11:58:00.000Z',
  now: new Date('2026-08-13T12:00:00.000Z'),
  freshAgeSeconds: 300,
  provider: 'google',
  currentEmail: 'member@example.org',
};

describe('explicit account linking', () => {
  it('allows Google from a reauthenticated fresh session', () => {
    expect(evaluateExplicitLink(baseline)).toEqual({ allowed: true });
  });

  it('rejects implicit/stale and unauthenticated linking', () => {
    expect(
      evaluateExplicitLink({ ...baseline, authenticatedUserId: undefined })
        .reason,
    ).toBe('not_authenticated');
    expect(
      evaluateExplicitLink({
        ...baseline,
        sessionCreatedAt: '2026-08-13T11:54:59.000Z',
      }).reason,
    ).toBe('reauthentication_required');
  });

  it('reuses the same fresh-session rule for MFA and backup-code management', () => {
    expect(evaluateFreshSession(baseline).allowed).toBe(true);
    expect(
      evaluateFreshSession({
        ...baseline,
        sessionCreatedAt: '2026-08-13T11:00:00.000Z',
      }).reason,
    ).toBe('reauthentication_required');
  });

  it('allows no provider other than Google', () => {
    expect(
      evaluateExplicitLink({ ...baseline, provider: 'github' }).reason,
    ).toBe('unsupported_provider');
  });

  it('never links different emails', () => {
    expect(
      evaluateExplicitLink({
        ...baseline,
        providerEmail: 'attacker@example.org',
      }).reason,
    ).toBe('different_email');
    expect(
      evaluateExplicitLink({ ...baseline, providerEmail: 'MEMBER@example.org' })
        .allowed,
    ).toBe(true);
  });
});
