import { describe, expect, it } from 'vitest';

import {
  deletionRecoveryEmail,
  invitationEmail,
  verificationEmail,
} from '../src/email/templates.js';

describe('auth email templates', () => {
  it('includes a plain-text fallback and escapes attacker-controlled URL markup', () => {
    const message = verificationEmail(
      'member@example.org',
      'https://rsp.example.org/verify?next=<script>alert("x")</script>',
    );
    expect(message.text).toContain('https://rsp.example.org/verify');
    expect(message.html).not.toContain('<script>');
    expect(message.html).toContain('&lt;script&gt;');
  });

  it('states the exact UTC deletion recovery deadline', () => {
    const message = deletionRecoveryEmail(
      'member@example.org',
      'https://rsp.example.org/recover',
      new Date('2026-09-12T00:00:00.000Z'),
    );
    expect(message.text).toContain('2026-09-12T00:00:00.000Z');
  });
});

it('invitation messages escape names and describe verified-email acceptance', () => {
  const message = invitationEmail(
    'member@example.test',
    '<b>Member</b>',
    'Summer <2026>',
    'student',
    'https://rsp.test/invitations/accept#token=token',
  );
  expect(message.text).toContain('expires in 7 days');
  expect(message.text).toContain('verify it');
  expect(message.html).toContain('&lt;b&gt;Member&lt;/b&gt;');
  expect(message.html).not.toContain('<b>');
});
