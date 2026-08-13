import { describe, expect, it } from "vitest";

import {
  hashedBackupCodeCodec,
  isHashedBackupCodePayload,
  withBackupCodeCandidate,
} from "../src/security/backup-code-store.js";

describe("hashed Better Auth backup-code storage", () => {
  it("stores only salted hashes and reveals only a matching request candidate", async () => {
    const first = "ABCDE-1234567";
    const second = "FGHIJ-7654321";
    const stored = await hashedBackupCodeCodec.encrypt(JSON.stringify([first, second]));
    expect(isHashedBackupCodePayload(stored)).toBe(true);
    expect(stored).not.toContain(first);
    expect(stored).not.toContain(second);

    const decoded = await withBackupCodeCandidate(first, () =>
      hashedBackupCodeCodec.decrypt(stored),
    );
    const values = JSON.parse(decoded) as string[];
    expect(values).toContain(first);
    expect(values).not.toContain(second);

    const afterUse = await hashedBackupCodeCodec.encrypt(
      JSON.stringify(values.filter((value) => value !== first)),
    );
    const replay = await withBackupCodeCandidate(first, () =>
      hashedBackupCodeCodec.decrypt(afterUse),
    );
    expect(JSON.parse(replay)).not.toContain(first);

    const remaining = await withBackupCodeCandidate(second, () =>
      hashedBackupCodeCodec.decrypt(afterUse),
    );
    expect(JSON.parse(remaining)).toContain(second);
  });

  it("does not reveal hashes as usable codes outside verification context", async () => {
    const stored = await hashedBackupCodeCodec.encrypt(JSON.stringify(["ABCDE-1234567"]));
    const decoded = JSON.parse(await hashedBackupCodeCodec.decrypt(stored)) as string[];
    expect(decoded[0]).toMatch(/^s1\$/);
    expect(decoded[0]).not.toBe("ABCDE-1234567");
  });
});
