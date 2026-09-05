import { Switch } from '@base-ui/react/switch';
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
import type {
  Me,
  PracticeSettings,
  PracticeSettingsMutation,
} from '@/api/generated/models';
import { patchPracticeSettings } from '@/api/practiceSettingsClient';
import {
  currentUserOptions,
  demoMode,
  practiceSettingsOptions,
  useCurrentUser,
  usePracticeSettings,
} from '@/api/queries';
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
import { supportedTimezones } from '@/utils';

export function SettingsPage() {
  usePageTitle('Settings');
  const user = useCurrentUser();
  const practiceSettings = usePracticeSettings();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const browserTimezone = useMemo(
    () => Intl.DateTimeFormat().resolvedOptions().timeZone,
    [],
  );
  const [timezone, setTimezone] = useState(browserTimezone);
  const [practiceDraft, setPracticeDraft] = useState<PracticeSettings | null>(
    null,
  );
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
  useEffect(() => {
    if (practiceSettings.data)
      setPracticeDraft((current) => current ?? practiceSettings.data);
  }, [practiceSettings.data]);

  const practiceDirty = Boolean(
    practiceDraft &&
    practiceSettings.data &&
    (practiceDraft.easyMinutes !== practiceSettings.data.easyMinutes ||
      practiceDraft.mediumMinutes !== practiceSettings.data.mediumMinutes ||
      practiceDraft.hardMinutes !== practiceSettings.data.hardMinutes),
  );
  const profileDirty = Boolean(user.data && timezone !== user.data.timezone);
  const dirty = profileDirty || practiceDirty;
  const goalsValid = Boolean(
    practiceDraft &&
    [
      practiceDraft.easyMinutes,
      practiceDraft.mediumMinutes,
      practiceDraft.hardMinutes,
    ].every(
      (minutes) => Number.isInteger(minutes) && minutes >= 1 && minutes <= 180,
    ),
  );
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

  if (user.isLoading || practiceSettings.isLoading)
    return <PageSkeleton label="Loading settings" />;
  if (
    user.isError ||
    practiceSettings.isError ||
    !user.data ||
    !practiceDraft
  ) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="Settings unavailable"
          message="Your settings could not be loaded. No changes have been made."
          onRetry={() =>
            void Promise.all([user.refetch(), practiceSettings.refetch()])
          }
        />
      </div>
    );
  }

  const saveSettings = async () => {
    if (!dirty || !goalsValid || !timezoneValid) return;
    setSaving(true);
    setSaved(false);
    setSaveError('');
    try {
      let updatedSettings = practiceDraft;
      if (!demoMode) {
        if (profileDirty) {
          const updatedUser = await apiRequest<Me>('/me', {
            method: 'PATCH',
            body: JSON.stringify({
              name: user.data.name,
              slug: user.data.slug,
              avatarUrl: user.data.avatarUrl,
              timezone,
            }),
          });
          queryClient.setQueryData(
            currentUserOptions.queryKey,
            adaptCurrentUser(updatedUser),
          );
        }
        if (practiceDirty) {
          const mutation: PracticeSettingsMutation = {
            easyMinutes: practiceDraft.easyMinutes,
            mediumMinutes: practiceDraft.mediumMinutes,
            hardMinutes: practiceDraft.hardMinutes,
          };
          updatedSettings = await patchPracticeSettings(mutation);
        }
      } else {
        if (profileDirty)
          queryClient.setQueryData(currentUserOptions.queryKey, {
            ...user.data,
            timezone,
          });
        if (practiceDirty)
          updatedSettings = {
            ...practiceDraft,
          };
      }
      if (practiceDirty) {
        queryClient.setQueryData(
          practiceSettingsOptions.queryKey,
          updatedSettings,
        );
        setPracticeDraft(updatedSettings);
      }
      if (!demoMode && (profileDirty || practiceDirty))
        await queryClient.invalidateQueries({
          queryKey: currentUserOptions.queryKey,
          exact: true,
        });
      setSaved(true);
    } catch (error) {
      const profileNowSaved =
        queryClient.getQueryData(currentUserOptions.queryKey)?.timezone ===
        timezone;
      setSaveError(
        `${profileNowSaved && practiceDirty ? 'Timezone saved, but practice preferences were not saved. ' : ''}${error instanceof Error ? error.message : 'Unable to save settings.'}`,
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
        description="Control your display timezone, practice preferences and account security."
        actions={
          <button
            className={styles.button}
            type="button"
            disabled={saving || !dirty || !goalsValid || !timezoneValid}
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
          <PracticePreferencesPanel
            value={practiceDraft}
            onChange={(next) => {
              setPracticeDraft(next);
              setSaved(false);
            }}
          />
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

export function PracticePreferencesPanel({
  value,
  onChange,
}: {
  value: PracticeSettings;
  onChange: (settings: PracticeSettings) => void;
}) {
  const updateMinutes = (
    field: 'easyMinutes' | 'mediumMinutes' | 'hardMinutes',
    minutes: number,
  ) => onChange({ ...value, [field]: minutes });
  return (
    <div className={styles.panel}>
      <SettingSwitch
        label="Use personal time goals"
        description={
          value.goalsEnabled
            ? 'Enabled by your mentor or programme administrator.'
            : 'An assigned mentor or programme administrator must enable personal goals.'
        }
        checked={value.goalsEnabled}
        disabled
      />
      <div className={`${styles.fieldGrid} ${styles.fieldGridSpaced}`}>
        {(
          [
            ['Easy', 'easyMinutes'],
            ['Medium', 'mediumMinutes'],
            ['Hard', 'hardMinutes'],
          ] as const
        ).map(([difficulty, field]) => {
          const invalid =
            !Number.isInteger(value[field]) ||
            value[field] < 1 ||
            value[field] > 180;
          return (
            <div
              className={`${styles.field} ${
                !value.goalsEnabled ? styles.fieldDisabled : ''
              }`}
              key={difficulty}
            >
              <label htmlFor={`goal-${difficulty}`}>{difficulty} minutes</label>
              <input
                id={`goal-${difficulty}`}
                className={styles.input}
                type="number"
                min="1"
                max="180"
                value={Number.isNaN(value[field]) ? '' : value[field]}
                disabled={!value.goalsEnabled}
                aria-invalid={invalid}
                aria-describedby={
                  invalid ? `goal-${difficulty}-error` : undefined
                }
                onChange={(event) =>
                  updateMinutes(field, event.currentTarget.valueAsNumber)
                }
              />
              {invalid ? (
                <p
                  id={`goal-${difficulty}-error`}
                  className={styles.fieldError}
                >
                  Enter 1 to 180 minutes.
                </p>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function SettingSwitch({
  label,
  description,
  checked,
  onCheckedChange,
  disabled = false,
}: {
  label: string;
  description: string;
  checked: boolean;
  onCheckedChange?: (checked: boolean) => void;
  disabled?: boolean;
}) {
  const descriptionId = `setting-${label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')}-description`;
  return (
    <div
      className={`${styles.switchRow} ${
        disabled && !checked ? styles.switchRowDisabled : ''
      }`}
    >
      <span>
        <strong>{label}</strong>
        <span
          id={descriptionId}
          className={`${styles.helper} ${styles.helperBlock}`}
        >
          {description}
        </span>
      </span>
      <Switch.Root
        className={styles.switchRoot}
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-label={label}
        aria-describedby={descriptionId}
      >
        <span className={styles.switchState} aria-hidden="true">
          {checked ? 'On' : 'Off'}
        </span>
        <Switch.Thumb className={styles.switchThumb} />
      </Switch.Root>
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
