import type { EmailMessage } from './provider.js';

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function linkEmail(options: {
  readonly to: string;
  readonly subject: string;
  readonly introduction: string;
  readonly action: string;
  readonly url: string;
  readonly expiry: string;
}): EmailMessage {
  const safeUrl = escapeHtml(options.url);
  const safeAction = escapeHtml(options.action);
  const safeIntroduction = escapeHtml(options.introduction);
  const safeExpiry = escapeHtml(options.expiry);
  return {
    to: options.to,
    subject: options.subject,
    text: `${options.introduction}\n\n${options.action}: ${options.url}\n\n${options.expiry}`,
    html: `<p>${safeIntroduction}</p><p><a href="${safeUrl}">${safeAction}</a></p><p>${safeExpiry}</p>`,
  };
}

export function verificationEmail(to: string, url: string): EmailMessage {
  return linkEmail({
    to,
    subject: 'Verify your RSP email',
    introduction: 'Verify this email address before accessing RSP.',
    action: 'Verify email',
    url,
    expiry:
      'This link expires in one hour. If you did not create an account, ignore this email.',
  });
}

export function passwordResetEmail(to: string, url: string): EmailMessage {
  return linkEmail({
    to,
    subject: 'Reset your RSP password',
    introduction: 'A password reset was requested for your RSP account.',
    action: 'Reset password',
    url,
    expiry:
      'This link expires in one hour. If you did not request it, ignore this email.',
  });
}

export function deletionRecoveryEmail(
  to: string,
  url: string,
  recoveryDeadline: Date,
): EmailMessage {
  return linkEmail({
    to,
    subject: 'Recover your RSP account',
    introduction:
      'Your RSP account is scheduled for deletion and all sessions were revoked.',
    action: 'Cancel account deletion',
    url,
    expiry: `Recovery is available until ${recoveryDeadline.toISOString()}.`,
  });
}
