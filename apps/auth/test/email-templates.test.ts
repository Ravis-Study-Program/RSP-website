import { describe, expect, it } from 'vitest';

import {
  deletionRecoveryEmail,
  invitationEmail,
  roleChangeEmail,
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

it('invitation messages escape names and describe accepting with the invited email', () => {
  const message = invitationEmail(
    'member@example.test',
    '<b>Member</b>',
    'Summer <2026>',
    'student',
    'https://rsp.test/invitations/accept#token=token',
  );
  expect(message.text).toContain('expires in 7 days');
  expect(message.text).toContain('using this email address, then accept');
  expect(message.html).toContain('&lt;b&gt;Member&lt;/b&gt;');
  expect(message.html).not.toContain('<b>');
});

it('role change messages name both roles and escape markup', () => {
  const message = roleChangeEmail(
    'member@example.test',
    '<b>Member</b>',
    'Summer <2026>',
    'mentor',
    'student',
    'https://rsp.test/seasons/summer-2026',
  );
  expect(message.subject).toBe('You are now a mentor in Summer <2026>');
  expect(message.text).toContain('changed from student to mentor');
  expect(message.text).toContain('https://rsp.test/seasons/summer-2026');
  expect(message.html).toContain('&lt;b&gt;Member&lt;/b&gt;');
  expect(message.html).not.toContain('<b>');
});
