export const ACCOUNT_DELETION_REQUEST_PATH =
  '/api/auth/account/deletion/request';
export const ACCOUNT_DELETION_RECOVERY_PATH =
  '/api/auth/account/deletion/recovery';
export const MFA_STEP_UP_PATH = '/api/auth/mfa/step-up';
export const MFA_STEP_UP_TOTP_PATH = '/api/auth/mfa/step-up/totp';
export const MFA_STEP_UP_BACKUP_PATH = '/api/auth/mfa/step-up/backup-code';

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function page(title: string, body: string): string {
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${escapeHtml(title)} · RSP</title><style>body{font:1rem/1.5 system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;color:#20202a;background:#faf9ff}main{background:white;border:1px solid #d8d4e5;border-radius:1rem;padding:2rem}label{display:block;margin:1rem 0 .35rem}input{box-sizing:border-box;width:100%;font:inherit;padding:.7rem;border:1px solid #777;border-radius:.5rem}button{margin-top:1rem;font:inherit;font-weight:700;padding:.7rem 1rem;border:0;border-radius:.5rem;color:white;background:#6941c6}button:focus-visible,input:focus-visible{outline:3px solid #00a7c4;outline-offset:2px}.error{color:#a1122a}</style></head><body><main>${body}</main></body></html>`;
}

export function renderDeletionRecoveryPage(
  token: string,
  error?: string,
): string {
  const safeToken = escapeHtml(token);
  return page(
    'Recover account',
    `<h1>Cancel account deletion</h1><p>Confirm that you want to keep your RSP account. This recovery link can be used once.</p>${error ? `<p class="error" role="alert">${escapeHtml(error)}</p>` : ''}<form method="post" action="${ACCOUNT_DELETION_RECOVERY_PATH}"><input type="hidden" name="token" value="${safeToken}"><button type="submit">Keep my account</button></form>`,
  );
}

export function renderDeletionRecoveredPage(): string {
  return page(
    'Account recovered',
    '<h1>Your account is active again</h1><p>The deletion request was cancelled. For security, sign in again to continue.</p><p><a href="/">Return to RSP</a></p>',
  );
}

export function renderMfaStepUpPage(error?: string): string {
  return page(
    'Two-factor verification',
    `<h1>Complete two-factor verification</h1><p>Your Google sign-in is waiting for a second factor. Enter an authenticator code or a one-time backup code.</p>${error ? `<p class="error" role="alert">${escapeHtml(error)}</p>` : ''}<form method="post" action="${MFA_STEP_UP_TOTP_PATH}"><label for="totp">Authenticator code</label><input id="totp" name="code" inputmode="numeric" autocomplete="one-time-code" required minlength="6" maxlength="8"><button type="submit">Verify code</button></form><form method="post" action="${MFA_STEP_UP_BACKUP_PATH}"><label for="backup">Backup code</label><input id="backup" name="code" autocomplete="one-time-code" required minlength="8" maxlength="128"><button type="submit">Use backup code</button></form>`,
  );
}

export function parseUrlEncodedField(
  body: Buffer | undefined,
  field: string,
): string {
  if (!body?.length) return '';
  return new URLSearchParams(body.toString('utf8')).get(field)?.trim() ?? '';
}
