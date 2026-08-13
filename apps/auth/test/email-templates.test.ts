import { describe, expect, it } from "vitest";

import { deletionRecoveryEmail, verificationEmail } from "../src/email/templates.js";

describe("auth email templates", () => {
  it("includes a plain-text fallback and escapes attacker-controlled URL markup", () => {
    const message = verificationEmail(
      "member@example.org",
      'https://rsp.example.org/verify?next=<script>alert("x")</script>',
    );
    expect(message.text).toContain("https://rsp.example.org/verify");
    expect(message.html).not.toContain("<script>");
    expect(message.html).toContain("&lt;script&gt;");
  });

  it("states the exact UTC deletion recovery deadline", () => {
    const message = deletionRecoveryEmail(
      "member@example.org",
      "https://rsp.example.org/recover",
      new Date("2026-09-12T00:00:00.000Z"),
    );
    expect(message.text).toContain("2026-09-12T00:00:00.000Z");
  });
});
