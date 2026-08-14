import { describe, expect, it } from 'vitest';

import {
  ACCOUNT_DELETION_RECOVERY_PATH,
  MFA_STEP_UP_BACKUP_PATH,
  MFA_STEP_UP_TOTP_PATH,
  parseUrlEncodedField,
  renderDeletionRecoveryPage,
  renderMfaStepUpPage,
} from '../src/lifecycle/browser.js';

describe('browser lifecycle contracts', () => {
  it('renders a same-origin, explicit deletion cancellation form', () => {
    const html = renderDeletionRecoveryPage('token"><script>alert(1)</script>');
    expect(html).toContain(`action="${ACCOUNT_DELETION_RECOVERY_PATH}"`);
    expect(html).toContain('method="post"');
    expect(html).not.toContain('<script>');
    expect(html).toContain('&lt;script&gt;');
  });

  it('publishes stable OAuth MFA form routes for TOTP and backup codes', () => {
    const html = renderMfaStepUpPage();
    expect(html).toContain(`action="${MFA_STEP_UP_TOTP_PATH}"`);
    expect(html).toContain(`action="${MFA_STEP_UP_BACKUP_PATH}"`);
    expect(html).toContain('autocomplete="one-time-code"');
  });

  it('parses only the named form field', () => {
    expect(
      parseUrlEncodedField(
        Buffer.from('token=safe_value&other=ignored'),
        'token',
      ),
    ).toBe('safe_value');
  });
});
