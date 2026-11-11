import { config } from '../config.js';
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
  readonly greeting?: string;
  readonly introduction: string;
  readonly action: string;
  readonly url: string;
  readonly expiry: string;
  readonly note?: string;
  readonly senderName?: string;
}): EmailMessage {
  const senderName = options.senderName ?? config.operatorName;
  const safeUrl = escapeHtml(options.url);
  const safeAction = escapeHtml(options.action);
  const safeGreeting = options.greeting ? escapeHtml(options.greeting) : '';
  const safeIntroduction = escapeHtml(options.introduction);
  const safeExpiry = escapeHtml(options.expiry);
  const safeNote = options.note ? escapeHtml(options.note) : '';
  const logoUrl = escapeHtml(new URL('/assets/rsp-logo.png', options.url).href);
  const text = [
    options.greeting,
    options.introduction,
    options.note,
    `${options.action}: ${options.url}`,
    options.expiry,
    `${senderName}, Ravi’s Study Program`,
  ]
    .filter(Boolean)
    .join('\n\n');
  const font =
    "font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;";
  const paragraph = `margin:0 0 16px;${font}font-size:16px;line-height:1.6;color:#1a1a1a;`;
  const html = `<!doctype html>
<html lang="en"><body style="margin:0;padding:0;background:#f4f4f2;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f2;"><tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;background:#ffffff;border:1px solid #e3e3df;border-radius:12px;">
<tr><td style="padding:32px 32px 0;"><img src="${logoUrl}" alt="RSP" width="48" height="48" style="display:block;border:0;border-radius:24px;"></td></tr>
<tr><td style="padding:24px 32px 8px;">
${safeGreeting ? `<p style="${paragraph}font-weight:600;">${safeGreeting}</p>` : ''}
<p style="${paragraph}">${safeIntroduction}</p>
${safeNote ? `<p style="${paragraph}color:#5c5c58;">${safeNote}</p>` : ''}
</td></tr>
<tr><td style="padding:8px 32px 16px;"><a href="${safeUrl}" style="display:inline-block;padding:12px 24px;background:#1a1a1a;color:#ffffff;${font}font-size:16px;font-weight:600;text-decoration:none;border-radius:8px;">${safeAction}</a></td></tr>
<tr><td style="padding:0 32px 24px;"><p style="margin:0;${font}font-size:13px;line-height:1.5;color:#5c5c58;">If the button doesn’t work, copy and paste this link into your browser:<br><a href="${safeUrl}" style="color:#5c5c58;word-break:break-all;">${safeUrl}</a></p></td></tr>
<tr><td style="padding:0 32px 32px;"><p style="margin:0;${font}font-size:14px;line-height:1.5;color:#5c5c58;">${safeExpiry}</p></td></tr>
<tr><td style="padding:20px 32px;border-top:1px solid #e3e3df;background:#fafaf8;border-radius:0 0 12px 12px;">
<p style="margin:0;${font}font-size:14px;line-height:1.4;color:#5c5c58;"><strong style="color:#1a1a1a;">${escapeHtml(senderName)}</strong><br>Ravi’s Study Program</p>
</td></tr>
</table>
</td></tr></table>
</body></html>`;
  return { to: options.to, subject: options.subject, text, html };
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

/** Tells a season member an administrator changed their role there. */
export function roleChangeEmail(
  to: string,
  name: string,
  season: string,
  role: string,
  previousRole: string,
  url: string,
): EmailMessage {
  return linkEmail({
    to,
    subject: `You are now a ${role} in ${season}`,
    introduction: `${name}, your role in ${season} has changed from ${previousRole} to ${role}.`,
    action: `Open ${season}`,
    url,
    expiry: 'If you were not expecting this, contact your season coordinator.',
  });
}

export function invitationEmail(
  to: string,
  name: string,
  season: string,
  role: string,
  url: string,
  inviterName?: string,
): EmailMessage {
  return linkEmail({
    to,
    subject: `Your Invitation to the RSP Website (${season})`,
    senderName: inviterName,
    greeting: `Hi ${name},`,
    introduction: `You've been invited to join the ${season} season of RSP as a ${role}.`,
    note: 'Sign in or create an account using this email address, then accept your invitation.',
    action: 'Accept Invitation',
    url,
    expiry: 'This invitation expires in 7 days and can only be used once.',
  });
}
