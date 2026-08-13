import { describe, expect, it } from "vitest";

import { isDeletionRecoverable, nextAccountState } from "../src/lifecycle/policy.js";

describe("account lifecycle policy", () => {
  it("supports suspension and the 30-day deletion progression", () => {
    expect(nextAccountState("active", "suspend")).toBe("suspended");
    expect(nextAccountState("active", "request_deletion")).toBe("deletion_pending");
    expect(nextAccountState("deletion_pending", "cancel_deletion")).toBe("active");
    expect(nextAccountState("deletion_pending", "pseudonymize")).toBe("deleted");
  });

  it("rejects irreversible or semantically invalid transitions", () => {
    expect(() => nextAccountState("deleted", "cancel_deletion")).toThrow(/invalid/);
    expect(() => nextAccountState("active", "pseudonymize")).toThrow(/invalid/);
  });

  it("permits recovery only before the verified grace deadline", () => {
    const now = new Date("2026-08-13T00:00:00Z");
    expect(
      isDeletionRecoverable({
        state: "deletion_pending",
        recoveryDeadline: new Date("2026-08-13T00:00:01Z"),
        now,
      }),
    ).toBe(true);
    expect(
      isDeletionRecoverable({ state: "deletion_pending", recoveryDeadline: now, now }),
    ).toBe(false);
    expect(
      isDeletionRecoverable({
        state: "suspended",
        recoveryDeadline: new Date("2026-09-13T00:00:00Z"),
        now,
      }),
    ).toBe(false);
  });
});
