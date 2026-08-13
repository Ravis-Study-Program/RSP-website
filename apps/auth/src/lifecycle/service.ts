import type { Pool, PoolClient } from "pg";

import type { EmailProvider } from "../email/provider.js";
import { deletionRecoveryEmail } from "../email/templates.js";
import type {
  IdentityLifecycleGateway,
  SessionRevocationReason,
} from "../identity/gateway.js";
import { isDeletionRecoverable, nextAccountState } from "./policy.js";
import type { AccountState } from "../security/access-policy.js";

interface AccountRow {
  email: string;
  account_state: AccountState;
  deletion_recovery_deadline: Date | null;
}

async function inTransaction<T>(pool: Pool, operation: (client: PoolClient) => Promise<T>): Promise<T> {
  const client = await pool.connect();
  try {
    await client.query("BEGIN");
    const result = await operation(client);
    await client.query("COMMIT");
    return result;
  } catch (error) {
    await client.query("ROLLBACK");
    throw error;
  } finally {
    client.release();
  }
}

export class AccountLifecycleService {
  constructor(
    private readonly pool: Pool,
    private readonly emailProvider: EmailProvider,
    private readonly identityGateway: IdentityLifecycleGateway,
    private readonly now: () => Date = () => new Date(),
  ) {}

  async revokeSessions(authUserId: string, reason: SessionRevocationReason): Promise<number> {
    const occurredAt = this.now();
    const revokedCount = await inTransaction(this.pool, async (client) => {
      const deleted = await client.query<{ id: string }>(
        "DELETE FROM sessions WHERE user_id = $1 RETURNING id",
        [authUserId],
      );
      await client.query(
        "UPDATE users SET security_version = security_version + 1, updated_at = $2 WHERE id = $1",
        [authUserId, occurredAt],
      );
      return deleted.rowCount ?? 0;
    });
    await this.identityGateway.publish({
      type: "sessions_revoked",
      authUserId,
      reason,
      occurredAt: occurredAt.toISOString(),
    });
    return revokedCount;
  }

  async setSuspended(authUserId: string, suspended: boolean): Promise<void> {
    const occurredAt = this.now();
    await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        "SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE",
        [authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error("auth user not found");
      if (account.account_state === "deleted" || account.account_state === "deletion_pending") {
        throw new Error(`cannot change suspension for ${account.account_state} account`);
      }
      if (suspended && account.account_state === "active") {
        nextAccountState(account.account_state, "suspend");
      }
      await client.query(
        `UPDATE users
           SET account_state = $2, security_version = security_version + 1, updated_at = $3
         WHERE id = $1`,
        [authUserId, suspended ? "suspended" : "active", occurredAt],
      );
      await client.query("DELETE FROM sessions WHERE user_id = $1", [authUserId]);
    });
    await this.identityGateway.publish({
      type: "sessions_revoked",
      authUserId,
      reason: suspended ? "suspension" : "security_admin",
      occurredAt: occurredAt.toISOString(),
    });
  }

  async requestDeletion(options: {
    readonly authUserId: string;
    readonly recoveryUrl: string;
    readonly recoveryDeadline: Date;
  }): Promise<void> {
    const occurredAt = this.now();
    const email = await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        "SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE",
        [options.authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error("auth user not found");
      nextAccountState(account.account_state, "request_deletion");
      await client.query(
        `UPDATE users
           SET account_state = 'deletion_pending', deletion_requested_at = $2,
               deletion_recovery_deadline = $3, security_version = security_version + 1,
               updated_at = $2
         WHERE id = $1`,
        [options.authUserId, occurredAt, options.recoveryDeadline],
      );
      await client.query("DELETE FROM sessions WHERE user_id = $1", [options.authUserId]);
      return account.email;
    });

    await Promise.all([
      this.emailProvider.send(
        deletionRecoveryEmail(email, options.recoveryUrl, options.recoveryDeadline),
      ),
      this.identityGateway.publish({
        type: "deletion_requested",
        authUserId: options.authUserId,
        recoveryDeadline: options.recoveryDeadline.toISOString(),
        occurredAt: occurredAt.toISOString(),
      }),
    ]);
  }

  async cancelDeletion(authUserId: string): Promise<void> {
    const occurredAt = this.now();
    await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        "SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE",
        [authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error("auth user not found");
      if (
        !isDeletionRecoverable({
          state: account.account_state,
          recoveryDeadline: account.deletion_recovery_deadline,
          now: occurredAt,
        })
      ) {
        throw new Error("account deletion is not recoverable");
      }
      nextAccountState(account.account_state, "cancel_deletion");
      await client.query(
        `UPDATE users
           SET account_state = 'active', deletion_requested_at = NULL,
               deletion_recovery_deadline = NULL, security_version = security_version + 1,
               updated_at = $2
         WHERE id = $1`,
        [authUserId, occurredAt],
      );
    });
    await this.identityGateway.publish({
      type: "deletion_cancelled",
      authUserId,
      occurredAt: occurredAt.toISOString(),
    });
  }

  async pseudonymize(authUserId: string): Promise<void> {
    const occurredAt = this.now();
    await inTransaction(this.pool, async (client) => {
      const result = await client.query<AccountRow>(
        "SELECT email, account_state, deletion_recovery_deadline FROM users WHERE id = $1 FOR UPDATE",
        [authUserId],
      );
      const account = result.rows[0];
      if (!account) throw new Error("auth user not found");
      if (account.account_state !== "deletion_pending") {
        throw new Error("only deletion-pending accounts can be pseudonymized");
      }
      if (!account.deletion_recovery_deadline || account.deletion_recovery_deadline > occurredAt) {
        throw new Error("account deletion grace period has not elapsed");
      }
      nextAccountState(account.account_state, "pseudonymize");
      await client.query("DELETE FROM sessions WHERE user_id = $1", [authUserId]);
      await client.query("DELETE FROM accounts WHERE user_id = $1", [authUserId]);
      await client.query("DELETE FROM two_factors WHERE user_id = $1", [authUserId]);
      await client.query("DELETE FROM verifications WHERE identifier = $1", [account.email]);
      await client.query(
        `UPDATE users
           SET name = 'Deleted member', email = $2, email_verified = FALSE, image = NULL,
               account_state = 'deleted', deletion_requested_at = NULL,
               deletion_recovery_deadline = NULL, two_factor_enabled = FALSE,
               security_version = security_version + 1, updated_at = $3
         WHERE id = $1`,
        [authUserId, `deleted+${authUserId}@invalid.rsp.local`, occurredAt],
      );
    });
    await this.identityGateway.publish({
      type: "auth_pseudonymized",
      authUserId,
      occurredAt: occurredAt.toISOString(),
    });
  }
}
