import { describe, expect, it } from 'vitest';

import {
  deletionRecoveryEmail,
  invitationEmail,
  roleChangeEmail,
  seasonAddedEmail,
  emailChangeConfirmationEmail,
  newEmailVerificationEmail,
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
  expect(message.text).toContain('expires in 14 days');
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

it('invitation messages name the inviter in the footer and repeat the link as text', () => {
  const url = 'https://rsp.test/invitations/accept#token=token';
  const message = invitationEmail(
    'member@example.test',
    'Member',
    'Summer 2026',
    'student',
    url,
    'Ravi <Admin>',
  );
  expect(message.subject).toBe(
    'Your Invitation to the RSP Website (Summer 2026)',
  );
  expect(message.html).toContain('Ravi &lt;Admin&gt;');
  expect(message.html).toContain('If the button doesn’t work');
  const html = message.html ?? '';
  expect(html.indexOf('rsp-logo.png')).toBeLessThan(html.indexOf('Member'));
  expect(message.text).toContain('Ravi <Admin>, Ravi’s Study Program');
});

it('season added messages say no action is needed and link to the season', () => {
  const message = seasonAddedEmail(
    'member@example.test',
    '<b>Member</b>',
    'Summer <2026>',
    'mentor',
    'https://rsp.test/seasons/summer-2026',
    'Ravi',
  );
  expect(message.subject).toBe(
    "You've Been Added to the RSP Website (Summer <2026>)",
  );
  expect(message.text).toContain(
    "You've been added to the Summer <2026> season of RSP as a mentor.",
  );
  expect(message.text).toContain('no further action is required');
  expect(message.text).toContain(
    'Open Summer <2026>: https://rsp.test/seasons/summer-2026',
  );
  expect(message.text).toContain('Ravi, Ravi’s Study Program');
  expect(message.html).toContain('&lt;b&gt;Member&lt;/b&gt;');
  expect(message.html).not.toContain('<b>');
});

it('email change messages name the new address and the step that follows', () => {
  const confirm = emailChangeConfirmationEmail(
    'old@example.test',
    'new@example.test',
    'https://rsp.example/api/auth/verify-email?token=a',
  );
  expect(confirm.to).toBe('old@example.test');
  expect(confirm.subject).toBe('Confirm your RSP email change');
  expect(confirm.text).toContain('to new@example.test');
  expect(confirm.text).toContain('open the link we send to new@example.test');
  const verify = newEmailVerificationEmail(
    'new@example.test',
    'https://rsp.example/api/auth/verify-email?token=b',
  );
  expect(verify.subject).toBe('Verify your new RSP email address');
  expect(verify.text).toContain('finish changing your RSP email');
});
