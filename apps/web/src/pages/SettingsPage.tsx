import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
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
import { difficultyGoal, supportedTimezones } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const sectionClass = 'mt-6 flex flex-col';
const sectionHeaderClass = 'mb-3 flex items-end justify-between gap-4';
const sectionTitleClass = 'text-xl font-semibold tracking-tight';
const listClass = 'm-0 grid list-none gap-3 p-0';
const formClass = 'flex flex-col gap-4';
const dialogActionsClass = 'mt-5 flex justify-end gap-2.5';
const rowListClass = 'divide-y';
const rowClass =
  'flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0';
const rowTextClass = 'block text-sm text-muted-foreground';
const badgeSuccessClass = 'bg-success-soft text-success';
const badgeWarningClass = 'bg-warning-soft text-warning';

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
      <div className={pageClass}>
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
    <div className={pageClass}>
      <DirtyFormGuard blocker={blocker} />
      <PageHeader
        eyebrow="Account"
        title="Settings"
        description="Set your display timezone, view practice goals and manage account security."
        actions={
          <Button
            type="button"
            disabled={saving || !dirty || !timezoneValid}
            onClick={() => void saveSettings()}
          >
            {saving ? 'Saving…' : 'Save settings'}
          </Button>
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
      <div className="grid gap-6 min-[56rem]:grid-cols-2">
        <section className={sectionClass} aria-labelledby="preferences-title">
          <div className={sectionHeaderClass}>
            <h2 id="preferences-title" className={sectionTitleClass}>
              Display and time
            </h2>
          </div>
          <Card className="h-full">
            <CardContent>
              <Field>
                <FieldLabel htmlFor="timezone">Timezone</FieldLabel>
                <TimezoneSelect
                  id="timezone"
                  options={timezoneOptions}
                  value={timezone}
                  invalid={!timezoneValid}
                  describedBy={
                    !timezoneValid ? 'timezone-error' : 'timezone-help'
                  }
                  getOptionLabel={(zone) =>
                    zone === browserTimezone
                      ? `${zone} (browser detected)`
                      : zone
                  }
                  onChange={(nextTimezone) => {
                    setTimezone(nextTimezone);
                    setSaved(false);
                  }}
                />
                <FieldDescription id="timezone-help">
                  Use an IANA timezone such as Australia/Adelaide. Stored
                  timestamps remain UTC.
                </FieldDescription>
                {!timezoneValid ? (
                  <FieldError id="timezone-error">
                    Enter a valid IANA timezone.
                  </FieldError>
                ) : null}
              </Field>
            </CardContent>
          </Card>
        </section>
        <section className={sectionClass} aria-labelledby="goals-title">
          <div className={sectionHeaderClass}>
            <h2 id="goals-title" className={sectionTitleClass}>
              Practice goals
            </h2>
          </div>
          <PracticePreferencesPanel />
        </section>
      </div>
      <section className={sectionClass} aria-labelledby="security-title">
        <div className={sectionHeaderClass}>
          <h2 id="security-title" className={sectionTitleClass}>
            Security
          </h2>
        </div>
        <Card>
          <CardContent className={rowListClass}>
            <div className={rowClass}>
              <span className="grid min-w-0 flex-1">
                <span className="flex flex-wrap items-center gap-2">
                  <strong className="font-semibold">
                    Two-factor authentication
                  </strong>
                  <Badge
                    variant="outline"
                    className={
                      user.data?.mfaVerified
                        ? badgeSuccessClass
                        : badgeWarningClass
                    }
                  >
                    {user.data?.mfaVerified
                      ? 'Verified this session'
                      : 'Step-up required'}
                  </Badge>
                </span>
                <span className={rowTextClass}>
                  Required for privileged programme roles
                </span>
              </span>
              <MfaSetupDialog onComplete={finishSecurityChange} />
            </div>
            <div className={rowClass}>
              <span className="flex min-w-0 items-center gap-2.5">
                <span className="grid min-w-0">
                  <strong className="font-semibold">Backup codes</strong>
                  <span className={rowTextClass}>
                    Single-use codes for account recovery
                  </span>
                </span>
              </span>
              <BackupCodesDialog onComplete={finishSecurityChange} />
            </div>
            <div className={rowClass}>
              <span>Connected sign-in methods</span>
              <Button
                variant="outline"
                size="sm"
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
              </Button>
            </div>
            <div className={rowClass}>
              <span>
                <strong className="font-semibold">
                  Password sign-in method
                </strong>
                <span className={rowTextClass}>
                  For Google-only accounts. Connecting requires a fresh session
                  and signs you out.
                </span>
              </span>
              <ConnectPasswordDialog
                onConnected={() => {
                  queryClient.clear();
                  navigate('/sign-in?passwordConnected=true', {
                    replace: true,
                  });
                }}
              />
            </div>
            <div className={rowClass}>
              <span>
                <strong className="font-semibold">Account email</strong>
                <span className={rowTextClass}>{user.data.email}</span>
              </span>
              <ChangeEmailDialog onRequested={finishSecurityChange} />
            </div>
            <div className={rowClass}>
              <span>
                <strong className="font-semibold">Password</strong>
                <span className={rowTextClass}>
                  Changing it revokes every session, including this one.
                </span>
              </span>
              <ChangePasswordDialog onChanged={finishSecurityChange} />
            </div>
            <div className={rowClass}>
              <span>
                <strong className="font-semibold">Current session</strong>
                <span className={rowTextClass}>
                  End this browser session securely.
                </span>
              </span>
              <Button
                variant="outline"
                size="sm"
                type="button"
                onClick={() => void endSession()}
              >
                Sign out
              </Button>
            </div>
          </CardContent>
        </Card>
      </section>
      <section className={sectionClass} aria-labelledby="danger-title">
        <div className={sectionHeaderClass}>
          <h2 id="danger-title" className={sectionTitleClass}>
            Account deletion
          </h2>
        </div>
        <Alert variant="destructive">
          <IconAlertTriangle size={19} aria-hidden="true" />
          <AlertDescription>
            Requesting deletion revokes all sessions immediately. A verified
            recovery link can cancel the request during the 30-day grace period.
          </AlertDescription>
        </Alert>
        <div className="mt-3 w-fit">
          <NamedConfirmation
            name="DELETE MY ACCOUNT"
            actionLabel="Request account deletion"
            description="After 30 days, credentials and profile contact details are removed or pseudonymised. Historical programme records remain under an opaque ID."
            onConfirm={() => void deleteAccount()}
          />
        </div>
      </section>
    </div>
  );
}

export function PracticePreferencesPanel() {
  return (
    <Card className="h-full">
      <CardContent className={formClass}>
        <p className="text-muted-foreground">
          Time goals are fixed for everyone and apply automatically.
        </p>
        <dl className="grid grid-cols-2 gap-3.5 max-[56rem]:grid-cols-1">
          {(['Easy', 'Medium', 'Hard'] as const).map((difficulty) => (
            <div key={difficulty}>
              <dt className="text-sm font-semibold">{difficulty}</dt>
              <dd className="text-sm text-muted-foreground">
                {difficultyGoal(difficulty)} minutes
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

function BackupCodeList({ codes }: { codes: string[] }) {
  return (
    <div>
      <p>Store these codes somewhere private. They will not be shown again.</p>
      <ul className={listClass}>
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
      triggerVariant="outline"
      triggerSize="sm"
      trigger="Set up"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        {!totpURI ? (
          <>
            <Field>
              <FieldLabel htmlFor="mfa-password">Current password</FieldLabel>
              <Input
                id="mfa-password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
              <FieldDescription>
                Google-only accounts must connect a password sign-in method
                before setting up TOTP.
              </FieldDescription>
            </Field>
            <Button
              type="button"
              disabled={pending || password.length < 12}
              onClick={() => void begin()}
            >
              {pending ? 'Starting…' : 'Begin setup'}
            </Button>
          </>
        ) : (
          <>
            <Field>
              <FieldLabel htmlFor="mfa-uri">Authenticator setup URI</FieldLabel>
              <Textarea id="mfa-uri" readOnly value={totpURI} />
            </Field>
            <BackupCodeList codes={backupCodes} />
            <Field>
              <FieldLabel htmlFor="mfa-code">Authenticator code</FieldLabel>
              <Input
                id="mfa-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
              />
            </Field>
            <Button
              type="button"
              disabled={pending || code.length < 6}
              onClick={() => void verify()}
            >
              {pending ? 'Verifying…' : 'Verify and finish'}
            </Button>
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
      triggerVariant="outline"
      triggerSize="sm"
      trigger="Regenerate"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        {codes.length > 0 ? (
          <>
            <BackupCodeList codes={codes} />
            <FieldDescription>
              All sessions have been revoked. Confirm only after storing the
              codes.
            </FieldDescription>
            <Button type="button" onClick={onComplete}>
              I stored the codes — sign in again
            </Button>
          </>
        ) : (
          <>
            <Field>
              <FieldLabel htmlFor="backup-password">
                Current password
              </FieldLabel>
              <Input
                id="backup-password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </Field>
            <Button
              type="button"
              disabled={pending || password.length < 12}
              onClick={() => void regenerate()}
            >
              {pending ? 'Regenerating…' : 'Regenerate codes'}
            </Button>
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
      triggerVariant="outline"
      triggerSize="sm"
      trigger="Change email"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor="new-account-email">New email</FieldLabel>
          <Input
            id="new-account-email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </Field>
        <div className={dialogActionsClass}>
          <Button
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Requesting…' : 'Send verification'}
          </Button>
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
      triggerVariant="outline"
      triggerSize="sm"
      trigger="Change password"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor="current-account-password">
            Current password
          </FieldLabel>
          <Input
            id="current-account-password"
            type="password"
            autoComplete="current-password"
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="new-account-password">New password</FieldLabel>
          <Input
            id="new-account-password"
            type="password"
            autoComplete="new-password"
            value={newPassword}
            onChange={(event) => setNewPassword(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="confirm-account-password">
            Confirm new password
          </FieldLabel>
          <Input
            id="confirm-account-password"
            type="password"
            autoComplete="new-password"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </Field>
        <div className={dialogActionsClass}>
          <Button
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Changing…' : 'Change password'}
          </Button>
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
      triggerVariant="outline"
      triggerSize="sm"
      trigger="Connect password"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor="connect-password">New password</FieldLabel>
          <Input
            id="connect-password"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="connect-password-confirmation">
            Confirm new password
          </FieldLabel>
          <Input
            id="connect-password-confirmation"
            type="password"
            autoComplete="new-password"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </Field>
        <div className={dialogActionsClass}>
          <Button
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Connecting…' : 'Connect and sign out'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}
