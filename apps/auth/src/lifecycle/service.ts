import { createHash, randomBytes, randomUUID } from 'node:crypto';

import type { Pool, PoolClient } from 'pg';

import type { EmailMessage, EmailProvider } from '../email/provider.js';
import { deletionRecoveryEmail } from '../email/templates.js';
import type {
  IdentityLifecycleGateway,
  IdentityLifecycleEvent,
  NewIdentityLifecycleEvent,
  SessionRevocationReason,
} from '../identity/gateway.js';
import { isDeletionRecoverable, nextAccountState } from './policy.js';
import type { AccountState } from '../security/access-policy.js';

interface AccountRow {
  id?: string;
  email: string;
  account_state: AccountState;
  deletion_recovery_deadline: Date | null;
}

interface RecoveryRow extends AccountRow {
  user_id: string;
  expires_at: Date;
}

interface OutboxEntry {
  readonly id: string;
  readonly target: 'identity' | 'email';
  readonly payload: IdentityLifecycleEvent | EmailMessage;
}

export class InvalidDeletionRecoveryTokenError extends Error {
  constructor() {
    super(
      'account deletion recovery token is invalid, expired, or already used',
    );
    this.name = 'InvalidDeletionRecoveryTokenError';
  }
}

export class IdentitySynchronizationPendingError extends Error {
  constructor() {
    super('identity synchronization is pending retry');
    this.name = 'IdentitySynchronizationPendingError';
  }
}

export function hashDeletionRecoveryToken(token: string): string {
  return createHash('sha256').update(token, 'utf8').digest('hex');
}

function addRecoveryToken(url: string, token: string): string {
  const recoveryUrl = new URL(url);
  recoveryUrl.searchParams.set('token', token);
  return recoveryUrl.toString();
}

async function enqueueOutbox(
  client: PoolClient,
  target: OutboxEntry['target'],
  payload: NewIdentityLifecycleEvent | EmailMessage,
  occurredAt: Date,
): Promise<OutboxEntry> {
  const id = randomUUID();
  const durablePayload: OutboxEntry['payload'] =
    target === 'identity'
      ? ({ ...payload, eventId: id } as IdentityLifecycleEvent)
      : (payload as EmailMessage);
  const entry = { id, target, payload: durablePayload } as const;
  await client.query(
    `INSERT INTO lifecycle_outbox
       (id, target, payload, available_at, created_at)
     VALUES ($1, $2, $3::jsonb, $4, $4)`,
    [entry.id, entry.target, JSON.stringify(entry.payload), occurredAt],
  );
  return entry;
}

async function inTransaction<T>(
  pool: Pool,
  operation: (client: PoolClient) => Promise<T>,
): Promise<T> {
  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    const result = await operation(client);
    await client.query('COMMIT');
    return result;
  } catch (error) {
    await client.query('ROLLBACK');
    throw error;
  } finally {
    client.release();
  }
}

async function erasePseudonymizedAccountArtifacts(
  client: PoolClient,
  authUserId: string,
  originalEmail: string,
): Promise<void> {
  await client.query(
    'DELETE FROM account_deletion_recovery_tokens WHERE user_id = $1',
    [authUserId],
  );
  await client.query(
    `DELETE FROM lifecycle_outbox
      WHERE payload->>'authUserId' = $1
         OR (target = 'email' AND lower(payload->>'to') = lower($2))`,
    [authUserId, originalEmail],
  );
}

export class AccountLifecycleService {
  constructor(
    private readonly pool: Pool,
    private readonly emailProvider: EmailProvider,
    private readonly identityGateway: IdentityLifecycleGateway,
    private readonly now: () => Date = () => new Date(),
    private readonly recoveryToken: () => string = () =>
      randomBytes(32).toString('base64url'),
  ) {}

  private async deliver(entry: OutboxEntry): Promise<boolean> {
    try {
      if (entry.target === 'identity') {
        await this.identityGateway.publish(
          entry.payload as IdentityLifecycleEvent,
        );
      } else {
        await this.emailProvider.send(entry.payload as EmailMessage);
      }
      await inTransaction(this.pool, async (client) => {
        await client.query(
          `UPDATE lifecycle_outbox
              SET delivered_at = $2, locked_until = NULL, last_error = NULL
            WHERE id = $1 AND delivered_at IS NULL`,
          [entry.id, this.now()],
        );
      });
      return true;
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message.slice(0, 500)
          : 'delivery failed';
      try {
        await inTransaction(this.pool, async (client) => {
          await client.query(
            `UPDATE lifecycle_outbox
                SET attempt_count = attempt_count + 1,
                    available_at = $2::timestamptz + make_interval(secs => LEAST(3600, 5 * (2 ^ LEAST(attempt_count, 9)))),
                    locked_until = NULL, last_error = $3
              WHERE id = $1 AND delivered_at IS NULL`,
            [entry.id, this.now(), message],
          );
        });
      } catch {
        // The durable row already contains the delivery intent. A later sweep
        // can reclaim an expired lease even if recording this attempt fails.
      }
      return false;
    }
  }

  private async deliverBestEffort(
    entries: readonly OutboxEntry[],
  ): Promise<void> {
    await Promise.all(entries.map((entry) => this.deliver(entry)));
  }

  private async requireIdentityDelivery(entry: OutboxEntry): Promise<void> {
    if (!(await this.deliver(entry)))
      throw new IdentitySynchronizationPendingError();
  }

  async queueIdentityEvent(
    event: NewIdentityLifecycleEvent,
    requireDelivery = false,
  ): Promise<void> {
    const entry = await inTransaction(this.pool, (client) =>
      enqueueOutbox(client, 'identity', event, new Date(event.occurredAt)),
    );
    if (requireDelivery) await this.requireIdentityDelivery(entry);
    else await this.deliverBestEffort([entry]);
  }

  async flushOutbox(limit = 50): Promise<number> {
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) {
      throw new RangeError('outbox batch limit must be between 1 and 100');
    }
    const entries = await inTransaction(this.pool, async (client) => {
      const result = await client.query<{
        id: string;
        target: OutboxEntry['target'];
        payload: OutboxEntry['payload'];
      }>(
        `WITH claimed AS (
           SELECT id
             FROM lifecycle_outbox
            WHERE delivered_at IS NULL AND available_at <= $1
              AND (locked_until IS NULL OR locked_until <= $1)
            ORDER BY available_at, created_at, id
            LIMIT $2
            FOR UPDATE SKIP LOCKED
         )
         UPDATE lifecycle_outbox AS outbox
            SET locked_until = $1::timestamptz + interval '1 minute'
           FROM claimed
          WHERE outbox.id = claimed.id
         RETURNING outbox.id, outbox.target, outbox.payload`,
        [this.now(), limit],
      );
      return result.rows;
    });
    let delivered = 0;
    for (const entry of entries) {
      if (await this.deliver(entry)) delivered += 1;
    }
    return delivered;
  }

  async revokeSessions(
    authUserId: string,
    reason: SessionRevocationReason,
  ): Promise<number> {
    const occurredAt = this.now();
    const result = await inTransaction(this.pool, async (client) => {
      const deleted = await client.query<{ id: string }>(
        'DELETE FROM sessions WHERE user_id = $1 RETURNING id',
        [authUserId],
      );
      const updated = await client.query<{ security_version: number }>(
        'UPDATE users SET security_version = security_version + 1, updated_at = $2 WHERE id = $1 RETURNING security_version',
        [authUserId, occurredAt],
      );
      const securityVersion = updated.rows[0]?.security_version;
      if (!securityVersion) throw new Error('auth user not found');
      const event = {
        type: 'sessions_revoked',
        authUserId,
        reason,
        securityVersion,
        occurredAt: occurredAt.toISOString(),
      } as const;
      return {
        revokedCount: deleted.rowCount ?? 0,
        outbox: await enqueueOutbox(client, 'identity', event, occurredAt),
      };
    });
    await this.requireIdentityDelivery(result.outbox);
    return result.revokedCount;
  }

  async recordMfaState(authUserId: string, configured: boolean): Promise<void> {
    const occurredAt = this.now();
    const outbox = await inTransaction(this.pool, async (client) => {
      await client.query('DELETE FROM sessions WHERE user_id = $1', [
        authUserId,
      ]);
      const updated = await client.query<{ security_version: number }>(
        'UPDATE users SET security_version = security_version + 1, updated_at = $2 WHERE id = $1 RETURNING security_version',
        [authUserId, occurredAt],
      );
      const securityVersion = updated.rows[0]?.security_version;
      if (!securityVersion) throw new Error('auth user not found');
      return enqueueOutbox(
        client,
        'identity',
        {
          type: configured ? 'mfa_configured' : 'mfa_disabled',
          authUserId,
          securityVersion,
          occurredAt: occurredAt.toISOString(),
        },
        occurredAt,
      );
    });
    await this.requireIdentityDelivery(outbox);
  }

  async setSuspended(
    authUserId: string,
    suspended: boolean,
    options: { readonly reason?: string; readonly actorUserId?: string } = {},
  ): Promise<void> {
    const occurredAt = this.now();
    const outbox = await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        'SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE',
        [authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error('auth user not found');
      if (
        account.account_state === 'deleted' ||
        account.account_state === 'deletion_pending'
      ) {
        throw new Error(
          `cannot change suspension for ${account.account_state} account`,
        );
      }
      if (suspended && account.account_state === 'active') {
        nextAccountState(account.account_state, 'suspend');
      }
      const updated = await client.query<{ security_version: number }>(
        `UPDATE users
           SET account_state = $2, security_version = security_version + 1, updated_at = $3
         WHERE id = $1
         RETURNING security_version`,
        [authUserId, suspended ? 'suspended' : 'active', occurredAt],
      );
      await client.query('DELETE FROM sessions WHERE user_id = $1', [
        authUserId,
      ]);
      const version = updated.rows[0]?.security_version;
      if (!version) throw new Error('auth user not found');
      return enqueueOutbox(
        client,
        'identity',
        {
          type: 'account_state_changed',
          authUserId,
          accountState: suspended ? 'suspended' : 'active',
          reason:
            options.reason ?? (suspended ? 'suspension' : 'security_admin'),
          ...(options.actorUserId ? { actorUserId: options.actorUserId } : {}),
          securityVersion: version,
          occurredAt: occurredAt.toISOString(),
        },
        occurredAt,
      );
    });
    await this.requireIdentityDelivery(outbox);
  }

  async requestDeletion(options: {
    readonly authUserId: string;
    readonly recoveryUrl: string;
    readonly recoveryDeadline: Date;
  }): Promise<void> {
    const occurredAt = this.now();
    const token = this.recoveryToken();
    const tokenHash = hashDeletionRecoveryToken(token);
    const result = await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        'SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE',
        [options.authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error('auth user not found');
      nextAccountState(account.account_state, 'request_deletion');
      await client.query(
        `UPDATE account_deletion_recovery_tokens
            SET revoked_at = $2
          WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL`,
        [options.authUserId, occurredAt],
      );
      await client.query(
        `INSERT INTO account_deletion_recovery_tokens
           (id, user_id, token_hash, requested_at, expires_at)
         VALUES ($1, $2, $3, $4, $5)`,
        [
          randomUUID(),
          options.authUserId,
          tokenHash,
          occurredAt,
          options.recoveryDeadline,
        ],
      );
      const updated = await client.query<{ security_version: number }>(
        `UPDATE users
           SET account_state = 'deletion_pending', deletion_requested_at = $2,
               deletion_recovery_deadline = $3, security_version = security_version + 1,
               updated_at = $2
         WHERE id = $1
         RETURNING security_version`,
        [options.authUserId, occurredAt, options.recoveryDeadline],
      );
      await client.query('DELETE FROM sessions WHERE user_id = $1', [
        options.authUserId,
      ]);
      const securityVersion = updated.rows[0]?.security_version;
      if (!securityVersion) throw new Error('auth user not found');
      const recoveryMessage = deletionRecoveryEmail(
        account.email,
        addRecoveryToken(options.recoveryUrl, token),
        options.recoveryDeadline,
      );
      const identityEvent = {
        type: 'deletion_requested',
        authUserId: options.authUserId,
        recoveryDeadline: options.recoveryDeadline.toISOString(),
        securityVersion,
        occurredAt: occurredAt.toISOString(),
      } as const;
      return Promise.all([
        enqueueOutbox(client, 'email', recoveryMessage, occurredAt),
        enqueueOutbox(client, 'identity', identityEvent, occurredAt),
      ]);
    });
    const identity = result.find((entry) => entry.target === 'identity');
    const email = result.find((entry) => entry.target === 'email');
    if (!identity) throw new Error('deletion lifecycle event was not queued');
    // Email delivery is retried independently. The security-sensitive request
    // only succeeds after the API has mirrored the new account/security state.
    if (email) await this.deliverBestEffort([email]);
    await this.requireIdentityDelivery(identity);
  }

  async cancelDeletion(authUserId: string): Promise<void> {
    const occurredAt = this.now();
    const outbox = await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        'SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE',
        [authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error('auth user not found');
      if (
        !isDeletionRecoverable({
          state: account.account_state,
          recoveryDeadline: account.deletion_recovery_deadline,
          now: occurredAt,
        })
      ) {
        throw new Error('account deletion is not recoverable');
      }
      nextAccountState(account.account_state, 'cancel_deletion');
      const updated = await client.query<{ security_version: number }>(
        `UPDATE users
           SET account_state = 'active', deletion_requested_at = NULL,
               deletion_recovery_deadline = NULL, security_version = security_version + 1,
               updated_at = $2
         WHERE id = $1
         RETURNING security_version`,
        [authUserId, occurredAt],
      );
      await client.query(
        `UPDATE account_deletion_recovery_tokens
            SET revoked_at = $2
          WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL`,
        [authUserId, occurredAt],
      );
      const version = updated.rows[0]?.security_version;
      if (!version) throw new Error('auth user not found');
      return enqueueOutbox(
        client,
        'identity',
        {
          type: 'deletion_cancelled',
          authUserId,
          securityVersion: version,
          occurredAt: occurredAt.toISOString(),
        },
        occurredAt,
      );
    });
    await this.requireIdentityDelivery(outbox);
  }

  async cancelDeletionWithRecoveryToken(token: string): Promise<void> {
    if (!/^[A-Za-z0-9_-]{32,128}$/.test(token)) {
      throw new InvalidDeletionRecoveryTokenError();
    }
    const occurredAt = this.now();
    const tokenHash = hashDeletionRecoveryToken(token);
    const result = await inTransaction(this.pool, async (client) => {
      const selected = await client.query<RecoveryRow>(
        `SELECT t.user_id, t.expires_at, u.email, u.account_state,
                u.deletion_recovery_deadline
           FROM account_deletion_recovery_tokens AS t
           JOIN users AS u ON u.id = t.user_id
          WHERE t.token_hash = $1 AND t.used_at IS NULL AND t.revoked_at IS NULL
          FOR UPDATE OF t, u`,
        [tokenHash],
      );
      const recovery = selected.rows[0];
      if (
        !recovery ||
        recovery.expires_at.getTime() <= occurredAt.getTime() ||
        !isDeletionRecoverable({
          state: recovery.account_state,
          recoveryDeadline: recovery.deletion_recovery_deadline,
          now: occurredAt,
        })
      ) {
        throw new InvalidDeletionRecoveryTokenError();
      }
      const consumed = await client.query(
        `UPDATE account_deletion_recovery_tokens
            SET used_at = $2
          WHERE token_hash = $1 AND used_at IS NULL AND revoked_at IS NULL`,
        [tokenHash, occurredAt],
      );
      if (consumed.rowCount !== 1)
        throw new InvalidDeletionRecoveryTokenError();
      const updated = await client.query<{ security_version: number }>(
        `UPDATE users
            SET account_state = 'active', deletion_requested_at = NULL,
                deletion_recovery_deadline = NULL, security_version = security_version + 1,
                updated_at = $2
          WHERE id = $1 AND account_state = 'deletion_pending'
          RETURNING security_version`,
        [recovery.user_id, occurredAt],
      );
      const securityVersion = updated.rows[0]?.security_version;
      if (!securityVersion) throw new InvalidDeletionRecoveryTokenError();
      return enqueueOutbox(
        client,
        'identity',
        {
          type: 'deletion_cancelled',
          authUserId: recovery.user_id,
          securityVersion,
          occurredAt: occurredAt.toISOString(),
        },
        occurredAt,
      );
    });
    await this.requireIdentityDelivery(result);
  }

  async pseudonymize(authUserId: string): Promise<void> {
    const occurredAt = this.now();
    const outbox = await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        'SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE',
        [authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error('auth user not found');
      if (account.account_state !== 'deletion_pending') {
        throw new Error('only deletion-pending accounts can be pseudonymized');
      }
      if (
        !account.deletion_recovery_deadline ||
        account.deletion_recovery_deadline > occurredAt
      ) {
        throw new Error('account deletion grace period has not elapsed');
      }
      nextAccountState(account.account_state, 'pseudonymize');
      await client.query('DELETE FROM sessions WHERE user_id = $1', [
        authUserId,
      ]);
      await client.query('DELETE FROM accounts WHERE user_id = $1', [
        authUserId,
      ]);
      await client.query('DELETE FROM two_factors WHERE user_id = $1', [
        authUserId,
      ]);
      await client.query(
        'DELETE FROM verifications WHERE identifier = $1 OR value = $2',
        [account.email, authUserId],
      );
      await erasePseudonymizedAccountArtifacts(
        client,
        authUserId,
        account.email,
      );
      const updated = await client.query<{ security_version: number }>(
        `UPDATE users
           SET name = 'Deleted member', email = $2, email_verified = FALSE, image = NULL,
               account_state = 'deleted', deletion_requested_at = NULL,
               deletion_recovery_deadline = NULL, two_factor_enabled = FALSE,
               security_version = security_version + 1, updated_at = $3
         WHERE id = $1
         RETURNING security_version`,
        [authUserId, `deleted+${authUserId}@invalid.rsp.local`, occurredAt],
      );
      const version = updated.rows[0]?.security_version;
      if (!version) throw new Error('auth user not found');
      return enqueueOutbox(
        client,
        'identity',
        {
          type: 'auth_pseudonymized',
          authUserId,
          securityVersion: version,
          occurredAt: occurredAt.toISOString(),
        },
        occurredAt,
      );
    });
    await this.requireIdentityDelivery(outbox);
  }

  async pseudonymizeDue(limit = 50): Promise<number> {
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) {
      throw new RangeError(
        'pseudonymization batch limit must be between 1 and 100',
      );
    }
    const occurredAt = this.now();
    const completed = await inTransaction(this.pool, async (client) => {
      const due = await client.query<AccountRow & { id: string }>(
        `SELECT id, email, account_state, deletion_recovery_deadline
           FROM users
          WHERE account_state = 'deletion_pending'
            AND deletion_recovery_deadline <= $1
          ORDER BY deletion_recovery_deadline, id
          LIMIT $2
          FOR UPDATE SKIP LOCKED`,
        [occurredAt, limit],
      );
      const events: OutboxEntry[] = [];
      for (const account of due.rows) {
        await client.query('DELETE FROM sessions WHERE user_id = $1', [
          account.id,
        ]);
        await client.query('DELETE FROM accounts WHERE user_id = $1', [
          account.id,
        ]);
        await client.query('DELETE FROM two_factors WHERE user_id = $1', [
          account.id,
        ]);
        await client.query(
          'DELETE FROM verifications WHERE identifier = $1 OR value = $2',
          [account.email, account.id],
        );
        await erasePseudonymizedAccountArtifacts(
          client,
          account.id,
          account.email,
        );
        const updated = await client.query<{ security_version: number }>(
          `UPDATE users
              SET name = 'Deleted member', email = $2, email_verified = FALSE, image = NULL,
                  account_state = 'deleted', deletion_requested_at = NULL,
                  deletion_recovery_deadline = NULL, two_factor_enabled = FALSE,
                  security_version = security_version + 1, updated_at = $3
            WHERE id = $1 AND account_state = 'deletion_pending'
            RETURNING security_version`,
          [account.id, `deleted+${account.id}@invalid.rsp.local`, occurredAt],
        );
        const securityVersion = updated.rows[0]?.security_version;
        if (securityVersion) {
          events.push(
            await enqueueOutbox(
              client,
              'identity',
              {
                type: 'auth_pseudonymized',
                authUserId: account.id,
                securityVersion,
                occurredAt: occurredAt.toISOString(),
              },
              occurredAt,
            ),
          );
        }
      }
      return events;
    });
    await this.deliverBestEffort(completed);
    return completed.length;
  }
}
