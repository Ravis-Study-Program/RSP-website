import { IconAlertTriangle, IconCheck } from '@tabler/icons-react';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useBeforeUnload, useBlocker, useNavigate } from 'react-router-dom';

import { clearAccessToken } from '@/api/client';
import {
  changeEmail,
  changePassword,
  connectPassword,
  enableTwoFactor,
  linkGoogleAccount,
  regenerateBackupCodes,
  requestAccountDeletion,
  signOut,
  verifyTwoFactor,
} from '@/api/authClient';
import { apiRequest } from '@/api/client';
import { adaptCurrentUser } from '@/api/adapters';
import type { Me } from '@/api/generated/models';
import { currentUserOptions, demoMode, useCurrentUser } from '@/api/queries';
import { PageHeader, usePageTitle } from '@/components/Common';
import {
  DirtyFormGuard,
  FormDialog,
  NamedConfirmation,
} from '@/components/Dialogs';
import { TimezoneSelect } from '@/components/TimezoneSelect';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import { difficultyGoal, supportedTimezones } from '@/utils';

export function SettingsPage() {
  usePageTitle('Settings');
  const user = useCurrentUser();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const browserTimezone = useMemo(
    () => Intl.DateTimeFormat().resolvedOptions().timeZone,
    [],
  );
  const [timezone, setTimezone] = useState(browserTimezone);
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [securityMessage, setSecurityMessage] = useState('');
  useEffect(() => {
    if (user.data)
      setTimezone(
        user.data.timezoneConfigured ? user.data.timezone : browserTimezone,
      );
  }, [browserTimezone, user.data]);
  const dirty = Boolean(user.data && timezone !== user.data.timezone);
  const timezoneValid = useMemo(() => {
    try {
      new Intl.DateTimeFormat('en', { timeZone: timezone }).format();
      return true;
    } catch {
      return false;
    }
  }, [timezone]);
  const timezoneOptions = useMemo(
    () => supportedTimezones(browserTimezone, user.data?.timezone ?? ''),
    [browserTimezone, user.data?.timezone],
  );
  const blocker = useBlocker(dirty);

  useBeforeUnload((event) => {
    if (dirty) event.preventDefault();
  });

  if (user.isLoading) return <PageSkeleton label="Loading settings" />;
  if (user.isError || !user.data) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="Settings unavailable"
          message="Your settings could not be loaded. No changes have been made."
          onRetry={() => void user.refetch()}
        />
      </div>
    );
  }

  const saveSettings = async () => {
    if (!dirty || !timezoneValid) return;
    setSaving(true);
    setSaved(false);
    setSaveError('');
    try {
      const updatedUser = demoMode
        ? { ...user.data, timezone, timezoneConfigured: true }
        : adaptCurrentUser(
            await apiRequest<Me>('/me', {
              method: 'PATCH',
              body: JSON.stringify({
                name: user.data.name,
                slug: user.data.slug,
                avatarUrl: user.data.avatarUrl,
                timezone,
              }),
            }),
          );
      queryClient.setQueryData(currentUserOptions.queryKey, updatedUser);
      if (!demoMode)
        await queryClient.invalidateQueries({
          queryKey: currentUserOptions.queryKey,
          exact: true,
        });
      setSaved(true);
    } catch (error) {
      setSaveError(
        error instanceof Error ? error.message : 'Unable to save settings.',
      );
      if (!demoMode)
        await queryClient.invalidateQueries({
          queryKey: currentUserOptions.queryKey,
          exact: true,
        });
    } finally {
      setSaving(false);
    }
  };
  const deleteAccount = async () => {
    try {
      await requestAccountDeletion();
      queryClient.clear();
      navigate('/sign-in', { replace: true });
    } catch (error) {
      setSecurityMessage(
        error instanceof Error
          ? error.message
          : 'Unable to request account deletion.',
      );
    }
  };
  const finishSecurityChange = () => {
    clearAccessToken();
    queryClient.clear();
    navigate('/sign-in?securityChanged=true', { replace: true });
  };
  const endSession = async () => {
    try {
      await signOut();
    } finally {
      queryClient.clear();
      navigate('/sign-in', { replace: true });
    }
  };

  return (
    <div className={styles.page}>
      <DirtyFormGuard blocker={blocker} />
      <PageHeader
        eyebrow="Account"
        title="Settings"
        description="Set your display timezone, view practice goals and manage account security."
        actions={
          <button
            className={styles.button}
            type="button"
            disabled={saving || !dirty || !timezoneValid}
            onClick={() => void saveSettings()}
          >
            {saving ? 'Saving…' : 'Save settings'}
          </button>
        }
      />
      {saved ? (
        <InlineNotice tone="success">
          <IconCheck size={19} aria-hidden="true" /> Settings saved.
        </InlineNotice>
      ) : null}
      {dirty ? (
        <InlineNotice tone="warning">
          You have unsaved settings changes.
        </InlineNotice>
      ) : null}
      {saveError ? (
        <InlineNotice tone="warning">{saveError}</InlineNotice>
      ) : null}
      {securityMessage ? (
        <InlineNotice tone="warning">{securityMessage}</InlineNotice>
      ) : null}
      <div className={styles.grid2}>
        <section className={styles.section} aria-labelledby="preferences-title">
          <div className={styles.sectionHeader}>
            <h2 id="preferences-title" className={styles.sectionTitle}>
              Display and time
            </h2>
          </div>
          <div className={styles.panel}>
            <div className={styles.field}>
              <label htmlFor="timezone">Timezone</label>
              <TimezoneSelect
                id="timezone"
                options={timezoneOptions}
                value={timezone}
                invalid={!timezoneValid}
                describedBy={
                  !timezoneValid ? 'timezone-error' : 'timezone-help'
                }
                getOptionLabel={(zone) =>
                  zone === browserTimezone ? `${zone} (browser detected)` : zone
                }
                onChange={(nextTimezone) => {
                  setTimezone(nextTimezone);
                  setSaved(false);
                }}
              />
              <p id="timezone-help" className={styles.helper}>
                Use an IANA timezone such as Australia/Adelaide. Stored
                timestamps remain UTC.
              </p>
              {!timezoneValid ? (
                <p id="timezone-error" className={styles.fieldError}>
                  Enter a valid IANA timezone.
                </p>
              ) : null}
            </div>
          </div>
        </section>
        <section className={styles.section} aria-labelledby="goals-title">
          <div className={styles.sectionHeader}>
            <h2 id="goals-title" className={styles.sectionTitle}>
              Practice goals
            </h2>
          </div>
          <PracticePreferencesPanel />
        </section>
      </div>
      <section className={styles.section} aria-labelledby="security-title">
        <div className={styles.sectionHeader}>
          <h2 id="security-title" className={styles.sectionTitle}>
            Security
          </h2>
        </div>
        <div className={styles.panel}>
          <div className={styles.listRow}>
            <span className={styles.personIdentity}>
              <span className={styles.personIdentityText}>
                <strong>Two-factor authentication</strong>
                <span>Required for privileged programme roles</span>
              </span>
            </span>
            <span
              className={
                user.data?.mfaVerified
                  ? styles.badgeSuccess
                  : styles.badgeWarning
              }
            >
              {user.data?.mfaVerified
                ? 'Verified this session'
                : 'Step-up required'}
            </span>
            <MfaSetupDialog onComplete={finishSecurityChange} />
          </div>
          <div className={styles.listRow}>
            <span className={styles.personIdentity}>
              <span className={styles.personIdentityText}>
                <strong>Backup codes</strong>
                <span>Single-use codes for account recovery</span>
              </span>
            </span>
            <BackupCodesDialog onComplete={finishSecurityChange} />
          </div>
          <div className={styles.listRow}>
            <span>Connected sign-in methods</span>
            <button
              className={styles.buttonSecondary}
              type="button"
              onClick={() =>
                void linkGoogleAccount().catch((error: unknown) =>
                  setSecurityMessage(
                    error instanceof Error
                      ? error.message
                      : 'Unable to connect Google.',
                  ),
                )
              }
            >
              Connect Google
            </button>
          </div>
          <div className={styles.listRow}>
            <span>
              <strong>Password sign-in method</strong>
              <span className={`${styles.helper} ${styles.helperBlock}`}>
                For Google-only accounts. Connecting requires a fresh session
                and signs you out.
              </span>
            </span>
            <ConnectPasswordDialog
              onConnected={() => {
                queryClient.clear();
                navigate('/sign-in?passwordConnected=true', { replace: true });
              }}
            />
          </div>
          <div className={styles.listRow}>
            <span>
              <strong>Account email</strong>
              <span className={`${styles.helper} ${styles.helperBlock}`}>
                {user.data.email}
              </span>
            </span>
            <ChangeEmailDialog onRequested={finishSecurityChange} />
          </div>
          <div className={styles.listRow}>
            <span>
              <strong>Password</strong>
              <span className={`${styles.helper} ${styles.helperBlock}`}>
                Changing it revokes every session, including this one.
              </span>
            </span>
            <ChangePasswordDialog onChanged={finishSecurityChange} />
          </div>
          <div className={styles.listRow}>
            <span>
              <strong>Current session</strong>
              <span className={`${styles.helper} ${styles.helperBlock}`}>
                End this browser session securely.
              </span>
            </span>
            <button
              className={styles.buttonSecondary}
              type="button"
              onClick={() => void endSession()}
            >
              Sign out
            </button>
          </div>
        </div>
      </section>
      <section className={styles.section} aria-labelledby="danger-title">
        <div className={styles.sectionHeader}>
          <h2 id="danger-title" className={styles.sectionTitle}>
            Account deletion
          </h2>
        </div>
        <div className={styles.panel}>
          <InlineNotice tone="warning">
            <IconAlertTriangle size={19} aria-hidden="true" /> Requesting
            deletion revokes all sessions immediately. A verified recovery link
            can cancel the request during the 30-day grace period.
          </InlineNotice>
          <div className={styles.sectionHeaderSpaced}>
            <NamedConfirmation
              name="DELETE MY ACCOUNT"
              actionLabel="Request account deletion"
              description="After 30 days, credentials and profile contact details are removed or pseudonymised. Historical programme records remain under an opaque ID."
              onConfirm={() => void deleteAccount()}
            />
          </div>
        </div>
      </section>
    </div>
  );
}

export function PracticePreferencesPanel() {
  return (
    <div className={styles.panel}>
      <p className={styles.helper}>
        Time goals are fixed for everyone and apply automatically.
      </p>
      <dl className={styles.fieldGrid}>
        {(['Easy', 'Medium', 'Hard'] as const).map((difficulty) => (
          <div key={difficulty}>
            <dt>{difficulty}</dt>
            <dd>{difficultyGoal(difficulty)} minutes</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function BackupCodeList({ codes }: { codes: string[] }) {
  return (
    <div>
      <p>Store these codes somewhere private. They will not be shown again.</p>
      <ul className={styles.cleanList}>
        {codes.map((code) => (
          <li key={code}>
            <code>{code}</code>
          </li>
        ))}
      </ul>
    </div>
  );
}

function MfaSetupDialog({ onComplete }: { onComplete: () => void }) {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [totpURI, setTotpURI] = useState('');
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const begin = async () => {
    setPending(true);
    setError('');
    try {
      const result = await enableTwoFactor(password || undefined);
      setTotpURI(result.totpURI);
      setBackupCodes(result.backupCodes);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to start two-factor setup.',
      );
    } finally {
      setPending(false);
    }
  };
  const verify = async () => {
    setPending(true);
    setError('');
    try {
      await verifyTwoFactor(code.trim());
      clearAccessToken();
      onComplete();
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'The authenticator code was not accepted.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Set up two-factor authentication"
      description="Reauthenticate, add the TOTP URI to your authenticator, then verify a code."
      open={open}
      onOpenChange={setOpen}
      trigger="Set up"
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        {!totpURI ? (
          <>
            <div className={styles.field}>
              <label htmlFor="mfa-password">Current password</label>
              <input
                id="mfa-password"
                className={styles.input}
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
              <p className={styles.helper}>
                Google-only accounts must connect a password sign-in method
                before setting up TOTP.
              </p>
            </div>
            <button
              className={styles.button}
              type="button"
              disabled={pending || password.length < 12}
              onClick={() => void begin()}
            >
              {pending ? 'Starting…' : 'Begin setup'}
            </button>
          </>
        ) : (
          <>
            <div className={styles.field}>
              <label htmlFor="mfa-uri">Authenticator setup URI</label>
              <textarea
                id="mfa-uri"
                className={styles.textarea}
                readOnly
                value={totpURI}
              />
            </div>
            <BackupCodeList codes={backupCodes} />
            <div className={styles.field}>
              <label htmlFor="mfa-code">Authenticator code</label>
              <input
                id="mfa-code"
                className={styles.input}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
              />
            </div>
            <button
              className={styles.button}
              type="button"
              disabled={pending || code.length < 6}
              onClick={() => void verify()}
            >
              {pending ? 'Verifying…' : 'Verify and finish'}
            </button>
          </>
        )}
      </div>
    </FormDialog>
  );
}

function BackupCodesDialog({ onComplete }: { onComplete: () => void }) {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [codes, setCodes] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const regenerate = async () => {
    setPending(true);
    setError('');
    try {
      const result = await regenerateBackupCodes(password || undefined);
      setCodes(result.backupCodes);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to regenerate backup codes.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Regenerate backup codes"
      description="This invalidates every existing backup code and requires a fresh sign-in."
      open={open}
      onOpenChange={setOpen}
      trigger="Regenerate"
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        {codes.length > 0 ? (
          <>
            <BackupCodeList codes={codes} />
            <p className={styles.helper}>
              All sessions have been revoked. Confirm only after storing the
              codes.
            </p>
            <button
              className={styles.button}
              type="button"
              onClick={onComplete}
            >
              I stored the codes — sign in again
            </button>
          </>
        ) : (
          <>
            <div className={styles.field}>
              <label htmlFor="backup-password">Current password</label>
              <input
                id="backup-password"
                className={styles.input}
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </div>
            <button
              className={styles.button}
              type="button"
              disabled={pending || password.length < 12}
              onClick={() => void regenerate()}
            >
              {pending ? 'Regenerating…' : 'Regenerate codes'}
            </button>
          </>
        )}
      </div>
    </FormDialog>
  );
}

function ChangeEmailDialog({ onRequested }: { onRequested: () => void }) {
  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async () => {
    if (!/^\S+@\S+\.\S+$/.test(email))
      return setError('Enter a valid email address.');
    setPending(true);
    setError('');
    try {
      if (!demoMode) await changeEmail(email.trim());
      onRequested();
      setOpen(false);
      setEmail('');
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to request the email change.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Change account email"
      description="This security-sensitive action requires a fresh session. Your new address must be verified."
      open={open}
      onOpenChange={setOpen}
      trigger="Change email"
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="new-account-email">New email</label>
          <input
            id="new-account-email"
            className={styles.input}
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Requesting…' : 'Send verification'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function ChangePasswordDialog({ onChanged }: { onChanged: () => void }) {
  const [open, setOpen] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async () => {
    if (!currentPassword) return setError('Enter your current password.');
    if (newPassword.length < 12) return setError('Use at least 12 characters.');
    if (newPassword !== confirmation)
      return setError('The new passwords do not match.');
    setPending(true);
    setError('');
    try {
      if (!demoMode) await changePassword(currentPassword, newPassword);
      onChanged();
      setOpen(false);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmation('');
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to change the password.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Change password"
      description="Confirm your current password. Every session, including this one, is revoked after the change."
      open={open}
      onOpenChange={setOpen}
      trigger="Change password"
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="current-account-password">Current password</label>
          <input
            id="current-account-password"
            className={styles.input}
            type="password"
            autoComplete="current-password"
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="new-account-password">New password</label>
          <input
            id="new-account-password"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            value={newPassword}
            onChange={(event) => setNewPassword(event.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="confirm-account-password">Confirm new password</label>
          <input
            id="confirm-account-password"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Changing…' : 'Change password'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function ConnectPasswordDialog({ onConnected }: { onConnected: () => void }) {
  const [open, setOpen] = useState(false);
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async () => {
    if (password.length < 12 || password.length > 128)
      return setError('Use 12 to 128 characters.');
    if (password !== confirmation)
      return setError('The passwords do not match.');
    setPending(true);
    setError('');
    try {
      if (!demoMode) await connectPassword(password);
      onConnected();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to connect password sign-in.',
      );
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Connect password sign-in"
      description="Use this only if you currently sign in with Google. All sessions are revoked after the method is connected."
      open={open}
      onOpenChange={setOpen}
      trigger="Connect password"
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="connect-password">New password</label>
          <input
            id="connect-password"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="connect-password-confirmation">
            Confirm new password
          </label>
          <input
            id="connect-password-confirmation"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Connecting…' : 'Connect and sign out'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}
