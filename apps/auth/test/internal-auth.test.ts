import { describe, expect, it } from "vitest";

import { hasValidServiceToken } from "../src/security/internal-auth.js";

describe("internal service authentication", () => {
  const expected = "a".repeat(32);

  it("accepts only an exact bearer token", () => {
    expect(hasValidServiceToken(`Bearer ${expected}`, expected)).toBe(true);
    expect(hasValidServiceToken(`bearer ${expected}`, expected)).toBe(false);
    expect(hasValidServiceToken(`Bearer ${"b".repeat(32)}`, expected)).toBe(false);
    expect(hasValidServiceToken("Bearer short", expected)).toBe(false);
    expect(hasValidServiceToken(null, expected)).toBe(false);
  });
});
