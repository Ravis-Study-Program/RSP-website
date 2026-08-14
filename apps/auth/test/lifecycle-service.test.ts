import type { Pool, PoolClient, QueryResult } from 'pg';
import { describe, expect, it, vi } from 'vitest';

import type { EmailProvider } from '../src/email/provider.js';
import type { IdentityLifecycleGateway } from '../src/identity/gateway.js';
import {
  AccountLifecycleService,
  hashDeletionRecoveryToken,
  InvalidDeletionRecoveryTokenError,
} from '../src/lifecycle/service.js';

interface RecordedQuery {
  readonly text: string;
  readonly values?: readonly unknown[];
}

function queryResult<T extends object>(
  rows: T[],
  rowCount = rows.length,
): QueryResult<T> {
  return {
    command: '',
    rowCount,
    oid: 0,
    fields: [],
    rows,
  };
}

function fakeDependencies(
  account?: {
    email: string;
    account_state: 'active' | 'suspended' | 'deletion_pending' | 'deleted';
    deletion_recovery_deadline: Date | null;
  },
  recovery?: {
    user_id: string;
    email: string;
    account_state: 'deletion_pending';
    deletion_recovery_deadline: Date;
    expires_at: Date;
  },
) {
  const queries: RecordedQuery[] = [];
  const client = {
    async query<T extends object>(text: string, values?: readonly unknown[]) {
      queries.push({ text, values });
      if (text.startsWith('SELECT email'))
        return queryResult(account ? [account as T] : []);
      if (text.includes('SELECT t.user_id'))
        return queryResult(recovery ? [recovery as T] : []);
      if (
        text.startsWith('DELETE FROM sessions') &&
        text.includes('RETURNING')
      ) {
        return queryResult([{ id: 'session-1' } as T], 1);
      }
      if (text.includes('RETURNING security_version')) {
        return queryResult([{ security_version: 2 } as T], 1);
      }
      if (text.includes('SET used_at')) return queryResult<T>([], 1);
      return queryResult<T>([]);
    },
    release: vi.fn(),
  };
  const pool = {
    connect: vi.fn(async () => client as unknown as PoolClient),
  } as unknown as Pool;
  const emailProvider: EmailProvider = { send: vi.fn(async () => undefined) };
  const identityGateway: IdentityLifecycleGateway = {
    publish: vi.fn(async () => undefined),
  };
  return { queries, client, pool, emailProvider, identityGateway };
}

describe('account lifecycle service', () => {
  const now = new Date('2026-08-13T00:00:00.000Z');

  it('revokes every database session, advances the security version, and emits a reason', async () => {
    const dependencies = fakeDependencies();
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await expect(
      service.revokeSessions('auth-1', 'password_changed'),
    ).resolves.toBe(1);
    expect(
      dependencies.queries.map((query) =>
        query.text.trim().split(/\s+/, 2).join(' '),
      ),
    ).toEqual(
      expect.arrayContaining([
        'BEGIN',
        'DELETE FROM',
        'UPDATE users',
        'INSERT INTO',
      ]),
    );
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'sessions_revoked',
        authUserId: 'auth-1',
        reason: 'password_changed',
      }),
    );
  });

  it('publishes the resulting account state after suspending and unsuspending', async () => {
    const suspendedDependencies = fakeDependencies({
      email: 'member@example.org',
      account_state: 'active',
      deletion_recovery_deadline: null,
    });
    const suspendedService = new AccountLifecycleService(
      suspendedDependencies.pool,
      suspendedDependencies.emailProvider,
      suspendedDependencies.identityGateway,
      () => now,
    );
    await suspendedService.setSuspended('auth-1', true);
    expect(suspendedDependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'account_state_changed',
        authUserId: 'auth-1',
        accountState: 'suspended',
        reason: 'suspension',
        securityVersion: 2,
        occurredAt: now.toISOString(),
      }),
    );

    const activeDependencies = fakeDependencies({
      email: 'member@example.org',
      account_state: 'suspended',
      deletion_recovery_deadline: null,
    });
    const activeService = new AccountLifecycleService(
      activeDependencies.pool,
      activeDependencies.emailProvider,
      activeDependencies.identityGateway,
      () => now,
    );
    await activeService.setSuspended('auth-1', false);
    expect(activeDependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'account_state_changed',
        accountState: 'active',
        reason: 'security_admin',
      }),
    );
  });

  it('requests deletion transactionally, revokes sessions, and sends recovery mail', async () => {
    const dependencies = fakeDependencies({
      email: 'member@example.org',
      account_state: 'active',
      deletion_recovery_deadline: null,
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await service.requestDeletion({
      authUserId: 'auth-1',
      recoveryUrl: 'https://rsp.example.org/recover',
      recoveryDeadline: new Date('2026-09-12T00:00:00.000Z'),
    });
    expect(
      dependencies.queries.some((query) =>
        query.text.includes('DELETE FROM sessions'),
      ),
    ).toBe(true);
    expect(dependencies.emailProvider.send).toHaveBeenCalledWith(
      expect.objectContaining({ to: 'member@example.org' }),
    );
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'deletion_requested',
        authUserId: 'auth-1',
        securityVersion: 2,
      }),
    );
  });

  it('stores only a digest of the single-use deletion recovery token', async () => {
    const token = 'A'.repeat(43);
    const dependencies = fakeDependencies({
      email: 'member@example.org',
      account_state: 'active',
      deletion_recovery_deadline: null,
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
      () => token,
    );
    await service.requestDeletion({
      authUserId: 'auth-1',
      recoveryUrl: 'https://rsp.example.org/api/auth/account/deletion/recovery',
      recoveryDeadline: new Date('2026-09-12T00:00:00.000Z'),
    });
    const insert = dependencies.queries.find((query) =>
      query.text.includes('INSERT INTO account_deletion_recovery_tokens'),
    );
    expect(insert?.values).toContain(hashDeletionRecoveryToken(token));
    expect(insert?.values).not.toContain(token);
    expect(dependencies.emailProvider.send).toHaveBeenCalledWith(
      expect.objectContaining({
        text: expect.stringContaining(`token=${token}`),
      }),
    );
  });

  it('atomically consumes a valid recovery token before restoring the account', async () => {
    const token = 'B'.repeat(43);
    const deadline = new Date('2026-09-12T00:00:00.000Z');
    const dependencies = fakeDependencies(undefined, {
      user_id: 'auth-1',
      email: 'member@example.org',
      account_state: 'deletion_pending',
      deletion_recovery_deadline: deadline,
      expires_at: deadline,
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await service.cancelDeletionWithRecoveryToken(token);
    expect(
      dependencies.queries.some((query) => query.text.includes('SET used_at')),
    ).toBe(true);
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'deletion_cancelled',
        securityVersion: 2,
      }),
    );
    await expect(
      service.cancelDeletionWithRecoveryToken('bad token'),
    ).rejects.toBeInstanceOf(InvalidDeletionRecoveryTokenError);
  });

  it('rolls back cancellation after the recovery deadline', async () => {
    const dependencies = fakeDependencies({
      email: 'member@example.org',
      account_state: 'deletion_pending',
      deletion_recovery_deadline: now,
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await expect(service.cancelDeletion('auth-1')).rejects.toThrow(
      'not recoverable',
    );
    expect(dependencies.queries.at(-1)?.text).toBe('ROLLBACK');
    expect(dependencies.identityGateway.publish).not.toHaveBeenCalled();
  });

  it('removes credentials and PII only after the deletion grace period', async () => {
    const dependencies = fakeDependencies({
      email: 'member@example.org',
      account_state: 'deletion_pending',
      deletion_recovery_deadline: new Date('2026-08-12T23:59:59.000Z'),
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await service.pseudonymize('auth-1');
    const sql = dependencies.queries.map((query) => query.text).join('\n');
    expect(sql).toContain('DELETE FROM accounts');
    expect(sql).toContain('DELETE FROM two_factors');
    expect(sql).toContain('DELETE FROM account_deletion_recovery_tokens');
    expect(sql).toContain('DELETE FROM lifecycle_outbox');
    expect(sql).toContain('identifier = $1 OR value = $2');
    expect(sql).toContain("account_state = 'deleted'");
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'auth_pseudonymized' }),
    );
  });

  it('claims due accounts with skip-locked batching for the background sweep', async () => {
    const dependencies = fakeDependencies();
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await expect(service.pseudonymizeDue(50)).resolves.toBe(0);
    expect(
      dependencies.queries.some((query) =>
        query.text.includes('FOR UPDATE SKIP LOCKED'),
      ),
    ).toBe(true);
    await expect(service.pseudonymizeDue(101)).rejects.toBeInstanceOf(
      RangeError,
    );
  });

  it('retains failed callbacks and refuses security-sensitive success until a retry converges', async () => {
    const dependencies = fakeDependencies();
    vi.mocked(dependencies.identityGateway.publish).mockRejectedValueOnce(
      new Error('downstream unavailable'),
    );
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await expect(
      service.revokeSessions('auth-1', 'security_admin'),
    ).rejects.toThrow('identity synchronization is pending retry');
    expect(
      dependencies.queries.some((query) =>
        query.text.includes('INSERT INTO lifecycle_outbox'),
      ),
    ).toBe(true);
    expect(
      dependencies.queries.some((query) =>
        query.text.includes('attempt_count = attempt_count + 1'),
      ),
    ).toBe(true);
    const attemptedEvent = vi.mocked(dependencies.identityGateway.publish).mock
      .calls[0]?.[0];
    expect(attemptedEvent?.eventId).toMatch(/^[0-9a-f-]{36}$/);
    const inserted = dependencies.queries.find((query) =>
      query.text.includes('INSERT INTO lifecycle_outbox'),
    );
    expect(inserted?.values).toContain(attemptedEvent?.eventId);
  });
});
