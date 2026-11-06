import { Card, CardContent } from '@/components/ui/card';
import { StatusBadge } from '@/components/StatusBadge';
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
import { useEffect, useMemo, useState, type ReactNode } from 'react';
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
import {
  currentUserOptions,
  demoMode,
  useCurrentUser,
  usePracticeGoals,
  useSignInMethods,
  useUpdatePracticeSettings,
} from '@/api/queries';
import {
  Page,
  PageHeader,
  PageSection,
  usePageTitle,
} from '@/components/Common';
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
import {
  canSetPracticeGoals,
  supportedTimezones,
  type PracticeGoals,
} from '@/utils';
import type { Difficulty } from '@/types';

const listClass = 'm-0 grid list-none gap-3 p-0';
const formClass = 'flex flex-col gap-4';
const dialogActionsClass = 'mt-5 flex justify-end gap-2.5';
const rowListClass = 'divide-y';
const rowClass =
  'flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0';
const rowTextClass = 'block text-sm text-muted-foreground';

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
  const timezoneDirty = Boolean(user.data && timezone !== user.data.timezone);
  // Practice goals apply to everyone; only coordinators and administrators
  // edit them, and they save with the rest of the settings.
  const signInMethods = useSignInMethods();
  const twoFactorEnabled = Boolean(signInMethods.data?.twoFactor);
  // Directors, system admins, coordinators and mentors need two-factor.
  const twoFactorRequired = Boolean(
    signInMethods.data &&
    !twoFactorEnabled &&
    user.data &&
    (user.data.globalRoles.length > 0 ||
      user.data.seasonRoles.some(
        (membership) =>
          membership.state === 'active' &&
          (membership.role === 'coordinator' || membership.role === 'mentor'),
      )),
  );
  const goals = usePracticeGoals();
  const updateGoals = useUpdatePracticeSettings();
  const canEditGoals = canSetPracticeGoals(user.data);
  const [goalDrafts, setGoalDrafts] = useState<GoalDrafts | null>(null);
  const goalsDirty = Boolean(
    canEditGoals &&
    goalDrafts &&
    difficulties.some(
      (difficulty) => goalDrafts[difficulty] !== String(goals[difficulty]),
    ),
  );
  const goalsValid =
    !goalDrafts ||
    difficulties.every((difficulty) => validGoal(goalDrafts[difficulty]));
  const dirty = timezoneDirty || goalsDirty;
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
      <Page>
        <ErrorState
          title="Settings Unavailable"
          message="Your settings could not be loaded. No changes have been made."
          onRetry={() => void user.refetch()}
        />
      </Page>
    );
  }

  const saveSettings = async () => {
    if (!dirty || !timezoneValid || !goalsValid) return;
    setSaving(true);
    setSaved(false);
    setSaveError('');
    try {
      if (goalsDirty && goalDrafts) {
        await updateGoals.mutateAsync({
          easyMinutes: Number(goalDrafts.Easy),
          mediumMinutes: Number(goalDrafts.Medium),
          hardMinutes: Number(goalDrafts.Hard),
        });
        setGoalDrafts(null);
      }
      if (!timezoneDirty) {
        setSaved(true);
        return;
      }
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
    <Page>
      <DirtyFormGuard blocker={blocker} />
      <PageHeader
        title="Settings"
        actions={
          <Button
            type="button"
            disabled={saving || !dirty || !timezoneValid || !goalsValid}
            onClick={() => void saveSettings()}
          >
            {saving ? 'Saving…' : 'Save settings'}
          </Button>
        }
      />
      <div className="flex flex-col gap-6">
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
        {twoFactorRequired ? (
          <InlineNotice tone="warning">
            <IconAlertTriangle size={19} aria-hidden="true" /> Your program role
            needs two-factor authentication. Set it up under Two-factor.
          </InlineNotice>
        ) : null}
        <div className="grid gap-6 min-[56rem]:grid-cols-2">
          <PageSection titleId="preferences-title" title="Display and Time">
            <Card className="h-full">
              <CardContent>
                <Field>
                  <FieldLabel htmlFor="timezone">Timezone</FieldLabel>
                  <TimezoneSelect
                    id="timezone"
                    options={timezoneOptions}
                    value={timezone}
                    invalid={!timezoneValid}
                    describedBy={!timezoneValid ? 'timezone-error' : undefined}
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
                  {!timezoneValid ? (
                    <FieldError id="timezone-error">
                      Enter a valid IANA timezone.
                    </FieldError>
                  ) : null}
                </Field>
              </CardContent>
            </Card>
          </PageSection>
          <PageSection titleId="goals-title" title="Practice Goals">
            <PracticePreferencesPanel
              goals={goals}
              drafts={canEditGoals ? (goalDrafts ?? goalStrings(goals)) : null}
              onChange={(difficulty, value) => {
                setGoalDrafts((drafts) => ({
                  ...(drafts ?? goalStrings(goals)),
                  [difficulty]: value,
                }));
                setSaved(false);
              }}
            />
          </PageSection>
        </div>
        <SecurityGroup title="Sign-In">
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
                {signInMethods.data?.password === false
                  ? 'Add a password to sign in with your email. You’ll then be signed out on all devices.'
                  : 'Changing your password signs you out on all devices.'}
              </span>
            </span>
            {signInMethods.data?.password === false ? (
              <ConnectPasswordDialog
                onConnected={() => {
                  queryClient.clear();
                  navigate('/sign-in?passwordConnected=true', {
                    replace: true,
                  });
                }}
              />
            ) : (
              <ChangePasswordDialog onChanged={finishSecurityChange} />
            )}
          </div>
          <div className={rowClass}>
            <span>
              <strong className="font-semibold">Google sign-in</strong>
              <span className={rowTextClass}>
                Sign in with your Google account.
              </span>
            </span>
            {signInMethods.data?.google ? (
              <Button type="button" disabled>
                <IconCheck size={17} aria-hidden="true" /> Google connected
              </Button>
            ) : (
              <Button
                type="button"
                disabled={!signInMethods.data}
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
            )}
          </div>
        </SecurityGroup>
        <SecurityGroup title="Two-Factor">
          <div className={rowClass}>
            <span className="grid min-w-0 flex-1">
              <span className="flex flex-wrap items-center gap-2">
                <strong className="font-semibold">
                  Two-factor authentication
                </strong>
                {signInMethods.data ? (
                  <StatusBadge tone={twoFactorEnabled ? 'success' : 'danger'}>
                    {twoFactorEnabled ? 'On' : 'Off'}
                  </StatusBadge>
                ) : null}
              </span>
              <span className={rowTextClass}>
                Required for privileged program roles.
              </span>
            </span>
            <MfaSetupDialog onComplete={finishSecurityChange} />
          </div>
          <div className={rowClass}>
            <span className="grid min-w-0">
              <strong className="font-semibold">Backup codes</strong>
              <span className={rowTextClass}>
                {twoFactorEnabled
                  ? 'Single-use codes for when you can’t use your authenticator.'
                  : 'Available once two-factor authentication is on.'}
              </span>
            </span>
            {twoFactorEnabled ? (
              <BackupCodesDialog onComplete={finishSecurityChange} />
            ) : (
              <Button type="button" disabled>
                Regenerate
              </Button>
            )}
          </div>
        </SecurityGroup>
        <SecurityGroup title="Sessions">
          <div className={rowClass}>
            <span>
              <strong className="font-semibold">Current session</strong>
              <span className={rowTextClass}>
                Signing out ends this session only.
              </span>
            </span>
            <Button type="button" onClick={() => void endSession()}>
              Sign out
            </Button>
          </div>
        </SecurityGroup>
        <PageSection titleId="danger-title" title="Account Deletion">
          <Card className="bg-destructive-soft ring-destructive">
            <CardContent className={rowListClass}>
              <div className={rowClass}>
                <span className="grid min-w-0 gap-0.5">
                  <strong className="flex items-center gap-2 font-semibold">
                    <IconAlertTriangle
                      size={18}
                      aria-hidden="true"
                      className="text-destructive"
                    />{' '}
                    Delete your account
                  </strong>
                  <span className={rowTextClass}>
                    Requesting deletion signs you out on all devices. You have
                    30 days to cancel using the recovery link sent to your
                    email.
                  </span>
                </span>
                <NamedConfirmation
                  name="DELETE MY ACCOUNT"
                  actionLabel="Request account deletion"
                  description="After 30 days, your sign-in details and profile contact information are removed or replaced. Program history is kept under an account ID."
                  onConfirm={() => void deleteAccount()}
                />
              </div>
            </CardContent>
          </Card>
        </PageSection>
      </div>
    </Page>
  );
}

/** A titled section of security rows in its own card. */
function SecurityGroup({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  const id = `${title.toLowerCase().replaceAll(/[^a-z]+/g, '-')}-title`;
  return (
    <PageSection titleId={id} title={title}>
      <Card>
        <CardContent className={rowListClass}>{children}</CardContent>
      </Card>
    </PageSection>
  );
}

const difficulties = ['Easy', 'Medium', 'Hard'] as const;
type GoalDrafts = Record<Difficulty, string>;

function goalStrings(goals: PracticeGoals): GoalDrafts {
  return {
    Easy: String(goals.Easy),
    Medium: String(goals.Medium),
    Hard: String(goals.Hard),
  };
}

/** A goal is a whole number of minutes from 1 to 180. */
function validGoal(value: string) {
  return /^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 180;
}

/**
 * The program's practice goals, one difficulty per column. With `drafts`
 * (for coordinators and administrators) each goal is editable; everyone else
 * sees the same fields read-only.
 */
export function PracticePreferencesPanel({
  goals,
  drafts = null,
  onChange,
}: {
  goals: PracticeGoals;
  drafts?: GoalDrafts | null;
  onChange?: (difficulty: Difficulty, value: string) => void;
}) {
  return (
    <Card className="h-full">
      <CardContent className={formClass}>
        <div className="grid grid-cols-3 gap-3.5 max-[56rem]:grid-cols-1">
          {difficulties.map((difficulty) => {
            const id = `goal-${difficulty.toLowerCase()}`;
            const invalid = drafts ? !validGoal(drafts[difficulty]) : false;
            return (
              <Field key={difficulty}>
                <FieldLabel htmlFor={id}>{difficulty} (minutes)</FieldLabel>
                {drafts ? (
                  <Input
                    id={id}
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={180}
                    value={drafts[difficulty]}
                    aria-invalid={invalid}
                    aria-describedby={invalid ? `${id}-error` : undefined}
                    onChange={(event) =>
                      onChange?.(difficulty, event.target.value)
                    }
                  />
                ) : (
                  <Input
                    id={id}
                    value={goals[difficulty]}
                    readOnly
                    aria-readonly="true"
                    className="text-muted-foreground"
                  />
                )}
                {invalid ? (
                  <FieldError id={`${id}-error`}>
                    Enter 1 to 180 minutes.
                  </FieldError>
                ) : null}
              </Field>
            );
          })}
        </div>
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
      title="Set Up Two-Factor Authentication"
      open={open}
      onOpenChange={setOpen}
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
      title="Regenerate Backup Codes"
      description="This invalidates every existing backup code and requires a fresh sign-in."
      open={open}
      onOpenChange={setOpen}
      trigger="Regenerate"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        {codes.length > 0 ? (
          <>
            <BackupCodeList codes={codes} />
            <FieldDescription>
              You’ve been signed out on all devices. Save these codes before
              continuing.
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
      title="Change Account Email"
      open={open}
      onOpenChange={setOpen}
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
      title="Change Password"
      description="Enter your current password. Changing it signs you out on all devices."
      open={open}
      onOpenChange={setOpen}
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
      title="Set a Password"
      description="Add a password if you currently sign in with Google only. You’ll be signed out on all devices afterward."
      open={open}
      onOpenChange={setOpen}
      trigger="Set password"
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
