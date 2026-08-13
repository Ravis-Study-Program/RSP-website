import type { Pool, PoolClient, QueryResult } from "pg";
import { describe, expect, it, vi } from "vitest";

import type { EmailProvider } from "../src/email/provider.js";
import type { IdentityLifecycleGateway } from "../src/identity/gateway.js";
import { AccountLifecycleService } from "../src/lifecycle/service.js";

interface RecordedQuery {
  readonly text: string;
  readonly values?: readonly unknown[];
}

function queryResult<T extends object>(rows: T[], rowCount = rows.length): QueryResult<T> {
  return {
    command: "",
    rowCount,
    oid: 0,
    fields: [],
    rows,
  };
}

function fakeDependencies(account?: {
  email: string;
  account_state: "active" | "suspended" | "deletion_pending" | "deleted";
  deletion_recovery_deadline: Date | null;
}) {
  const queries: RecordedQuery[] = [];
  const client = {
    async query<T extends object>(text: string, values?: readonly unknown[]) {
      queries.push({ text, values });
      if (text.startsWith("SELECT email")) return queryResult(account ? [account as T] : []);
      if (text.startsWith("DELETE FROM sessions") && text.includes("RETURNING")) {
        return queryResult([{ id: "session-1" } as T], 1);
      }
      return queryResult<T>([]);
    },
    release: vi.fn(),
  };
  const pool = { connect: vi.fn(async () => client as unknown as PoolClient) } as unknown as Pool;
  const emailProvider: EmailProvider = { send: vi.fn(async () => undefined) };
  const identityGateway: IdentityLifecycleGateway = { publish: vi.fn(async () => undefined) };
  return { queries, client, pool, emailProvider, identityGateway };
}

describe("account lifecycle service", () => {
  const now = new Date("2026-08-13T00:00:00.000Z");

  it("revokes every database session, advances the security version, and emits a reason", async () => {
    const dependencies = fakeDependencies();
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await expect(service.revokeSessions("auth-1", "password_changed")).resolves.toBe(1);
    expect(dependencies.queries.map((query) => query.text.trim().split(/\s+/, 2).join(" "))).toEqual([
      "BEGIN",
      "DELETE FROM",
      "UPDATE users",
      "COMMIT",
    ]);
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "sessions_revoked",
        authUserId: "auth-1",
        reason: "password_changed",
      }),
    );
  });

  it("requests deletion transactionally, revokes sessions, and sends recovery mail", async () => {
    const dependencies = fakeDependencies({
      email: "member@example.org",
      account_state: "active",
      deletion_recovery_deadline: null,
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await service.requestDeletion({
      authUserId: "auth-1",
      recoveryUrl: "https://rsp.example.org/recover",
      recoveryDeadline: new Date("2026-09-12T00:00:00.000Z"),
    });
    expect(dependencies.queries.some((query) => query.text.includes("DELETE FROM sessions"))).toBe(true);
    expect(dependencies.emailProvider.send).toHaveBeenCalledWith(
      expect.objectContaining({ to: "member@example.org" }),
    );
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({ type: "deletion_requested", authUserId: "auth-1" }),
    );
  });

  it("rolls back cancellation after the recovery deadline", async () => {
    const dependencies = fakeDependencies({
      email: "member@example.org",
      account_state: "deletion_pending",
      deletion_recovery_deadline: now,
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await expect(service.cancelDeletion("auth-1")).rejects.toThrow("not recoverable");
    expect(dependencies.queries.at(-1)?.text).toBe("ROLLBACK");
    expect(dependencies.identityGateway.publish).not.toHaveBeenCalled();
  });

  it("removes credentials and PII only after the deletion grace period", async () => {
    const dependencies = fakeDependencies({
      email: "member@example.org",
      account_state: "deletion_pending",
      deletion_recovery_deadline: new Date("2026-08-12T23:59:59.000Z"),
    });
    const service = new AccountLifecycleService(
      dependencies.pool,
      dependencies.emailProvider,
      dependencies.identityGateway,
      () => now,
    );
    await service.pseudonymize("auth-1");
    const sql = dependencies.queries.map((query) => query.text).join("\n");
    expect(sql).toContain("DELETE FROM accounts");
    expect(sql).toContain("DELETE FROM two_factors");
    expect(sql).toContain("account_state = 'deleted'");
    expect(dependencies.identityGateway.publish).toHaveBeenCalledWith(
      expect.objectContaining({ type: "auth_pseudonymized" }),
    );
  });
});
