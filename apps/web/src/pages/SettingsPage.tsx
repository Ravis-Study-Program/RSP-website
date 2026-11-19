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
import {
  useBeforeUnload,
  useBlocker,
  useNavigate,
  useSearchParams,
} from 'react-router-dom';

import { clearAccessToken } from '@/api/client';
import {
  changeEmail,
  changePassword,
  connectPassword,
  enableTwoFactor,
  linkGoogleAccount,
  linkDiscordAccount,
  unlinkDiscordAccount,
  socialAuthError,
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
  useWeeklyTargets,
  useSignInMethods,
  useAuthProviders,
  useUpdatePracticeSettings,
} from '@/api/queries';
import {
  Page,
  PageHeader,
  PageSection,
  usePageTitle,
  LabelHint,
} from '@/components/Common';
import {
  DirtyFormGuard,
  FormDialog,
  NamedConfirmation,
  DialogCancel,
} from '@/components/Dialogs';
import { TimezoneSelect } from '@/components/TimezoneSelect';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import {
  canSetPracticeGoals,
  type WeeklyTargets,
  supportedTimezones,
  type PracticeGoals,
} from '@/utils';
import type { Difficulty } from '@/types';

const formClass = 'flex flex-col gap-4';
const dialogActionsClass = 'mt-5 button-group';
// A card's rows share a button column as wide as its widest button
// ("✓ Google Connected"), so the buttons line up down the card.
const rowListClass =
  'grid grid-cols-[minmax(0,1fr)_max-content] gap-x-4 divide-y';
const rowClass =
  'col-span-2 grid grid-cols-subgrid items-center py-3 first:pt-0 last:pb-0 [&>button]:justify-self-stretch';
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
  const [searchParams] = useSearchParams();
  const [securityMessage, setSecurityMessage] = useState(
    socialAuthError(searchParams),
  );
  const [discordPending, setDiscordPending] = useState(false);
  const providers = useAuthProviders();
  useEffect(() => {
    if (user.data)
      setTimezone(
        user.data.timezoneConfigured ? user.data.timezone : browserTimezone,
      );
  }, [browserTimezone, user.data]);
  // A new account has no timezone of its own yet: the browser's is filled in
  // for it, and saved automatically, so that is the starting point, not a change.
  const timezoneDirty = Boolean(
    user.data &&
    timezone !==
      (user.data.timezoneConfigured ? user.data.timezone : browserTimezone),
  );
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
  const weekly = useWeeklyTargets();
  const [weeklyDrafts, setWeeklyDrafts] = useState<WeeklyDrafts | null>(null);
  const hoursDirty = Boolean(
    canEditGoals &&
    weeklyDrafts &&
    weeklyKeys.some((key) => weeklyDrafts[key] !== String(weekly[key])),
  );
  const goalsValid =
    (!goalDrafts ||
      difficulties.every((difficulty) => validGoal(goalDrafts[difficulty]))) &&
    (!weeklyDrafts ||
      weeklyKeys.every((key) => validWeekly(key, weeklyDrafts[key])));
  const dirty = timezoneDirty || goalsDirty || hoursDirty;
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
      if (goalsDirty || hoursDirty) {
        const nextGoals = goalDrafts ?? goalStrings(goals);
        await updateGoals.mutateAsync({
          easyMinutes: Number(nextGoals.Easy),
          mediumMinutes: Number(nextGoals.Medium),
          hardMinutes: Number(nextGoals.Hard),
          weeklyHours: Number(weeklyDrafts?.hours ?? weekly.hours),
        });
        setGoalDrafts(null);
        setWeeklyDrafts(null);
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
  /** Signs out after a change that revoked every session, saying which. */
  const finishSecurityChange = (
    change: 'securityChanged' | 'discordDisconnected' = 'securityChanged',
  ) => {
    clearAccessToken();
    queryClient.clear();
    navigate(`/sign-in?${change}=true`, { replace: true });
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
            {saving ? 'Saving…' : 'Save Settings'}
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
        {/* Three columns from 70rem: Practice Goals takes the middle, wider
            one for its three fields. */}
        <div className="grid gap-6 min-[56rem]:grid-cols-2 min-[70rem]:grid-cols-[1fr_2fr_1fr]">
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
          <PageSection titleId="weekly-title" title="Weekly Requirements">
            <WeeklyRequirementsPanel
              targets={weekly}
              drafts={
                canEditGoals ? (weeklyDrafts ?? weeklyStrings(weekly)) : null
              }
              onChange={(key, value) => {
                setWeeklyDrafts((drafts) => ({
                  ...(drafts ?? weeklyStrings(weekly)),
                  [key]: value,
                }));
                setSaved(false);
              }}
            />
          </PageSection>
        </div>
        <SecurityGroup title="Sign-In">
          <div className={rowClass}>
            <span>
              <strong className="font-semibold">Account Email</strong>
              <span className={rowTextClass}>{user.data.email}</span>
            </span>
            <ChangeEmailDialog onRequested={() => finishSecurityChange()} />
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
              <ChangePasswordDialog onChanged={() => finishSecurityChange()} />
            )}
          </div>
          <div className={rowClass}>
            <span>
              <strong className="font-semibold">Google Sign-In</strong>
              <span className={rowTextClass}>
                {signInMethods.data?.google
                  ? 'Sign in with your Google account.'
                  : 'Sign in with your Google account. Connecting signs you out on all devices.'}
              </span>
            </span>
            {signInMethods.data?.google ? (
              <Button type="button" disabled>
                <IconCheck size={17} aria-hidden="true" /> Google Connected
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
          {providers.data?.discord || signInMethods.data?.discordAccountId ? (
            <div className={rowClass}>
              <span>
                <strong className="font-semibold">Discord Sign-In</strong>
                <span className={rowTextClass}>
                  {signInMethods.data?.discordAccountId
                    ? 'Sign in with your Discord account. Your Discord profile picture is shown on this site and kept up to date. Disconnecting signs you out on all devices.'
                    : 'Sign in with your Discord account. Your Discord profile picture will be shown on this site. Connecting signs you out on all devices.'}
                </span>
              </span>
              <Button
                type="button"
                disabled={!signInMethods.data || discordPending}
                onClick={() => {
                  setDiscordPending(true);
                  setSecurityMessage('');
                  const accountId = signInMethods.data?.discordAccountId;
                  const action = accountId
                    ? unlinkDiscordAccount(accountId).then(() =>
                        finishSecurityChange('discordDisconnected'),
                      )
                    : linkDiscordAccount();
                  void action.catch((error: unknown) => {
                    setSecurityMessage(
                      error instanceof Error
                        ? error.message
                        : 'Unable to update Discord connection.',
                    );
                    setDiscordPending(false);
                  });
                }}
              >
                {discordPending
                  ? 'Please Wait…'
                  : signInMethods.data?.discordAccountId
                    ? 'Disconnect Discord'
                    : 'Connect Discord'}
              </Button>
            </div>
          ) : null}
        </SecurityGroup>
        <SecurityGroup title="Two-Factor">
          <div className={rowClass}>
            <span className="grid min-w-0 flex-1">
              <span className="flex flex-wrap items-center gap-2">
                <strong className="font-semibold">
                  Two-Factor Authentication
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
            <MfaSetupDialog onComplete={() => finishSecurityChange()} />
          </div>
          <div className={rowClass}>
            <span className="grid min-w-0">
              <strong className="font-semibold">Backup Codes</strong>
              <span className={rowTextClass}>
                {twoFactorEnabled
                  ? 'Single-use codes for when you can’t use your authenticator.'
                  : 'Available once two-factor authentication is on.'}
              </span>
            </span>
            {twoFactorEnabled ? (
              <BackupCodesDialog onComplete={() => finishSecurityChange()} />
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
              <strong className="font-semibold">Sign Out</strong>
              <span className={rowTextClass}>
                Only signs you out of this session.
              </span>
            </span>
            <Button type="button" onClick={() => void endSession()}>
              Sign Out
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
                    Delete Your Account
                  </strong>
                  <span className={rowTextClass}>
                    Requesting deletion signs you out on all devices. You have
                    30 days to cancel using the recovery link sent to your
                    email.
                  </span>
                </span>
                <NamedConfirmation
                  name="DELETE MY ACCOUNT"
                  actionLabel="Request Account Deletion"
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
                <FieldLabel htmlFor={id}>
                  {difficulty} <LabelHint>minutes</LabelHint>
                </FieldLabel>
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
                    disabled
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

// Only weekly hours are set here; LeetCode and mock targets are fixed for now.
const weeklyKeys = ['hours'] as const;
type WeeklyKey = (typeof weeklyKeys)[number];
type WeeklyDrafts = Record<WeeklyKey, string>;

function weeklyStrings(targets: WeeklyTargets): WeeklyDrafts {
  return { hours: String(targets.hours) };
}

/** Weekly hours are 1 to 168; each weekly LeetCode target 0 to 50. */
function validWeekly(key: WeeklyKey, value: string) {
  const [min, max] = key === 'hours' ? [1, 168] : [0, 50];
  return /^\d+$/.test(value) && Number(value) >= min && Number(value) <= max;
}

const weeklyFields: { key: WeeklyKey; label: string; error: string }[] = [
  { key: 'hours', label: 'Hours per Week', error: 'Enter 1 to 168 hours.' },
];

/**
 * What the program expects of each student every week: hours, and LeetCode
 * problems solved by difficulty. With `drafts` (for coordinators and
 * administrators) it is editable; everyone else sees it read-only. A
 * student's own targets, set by their mentor, replace these.
 */
function WeeklyRequirementsPanel({
  targets,
  drafts,
  onChange,
}: {
  targets: WeeklyTargets;
  drafts: WeeklyDrafts | null;
  onChange: (key: WeeklyKey, value: string) => void;
}) {
  return (
    <Card className="h-full">
      <CardContent className="grid gap-3.5">
        {weeklyFields.map(({ key, label, error }) => {
          const id = `weekly-${key}`;
          const invalid = drafts !== null && !validWeekly(key, drafts[key]);
          return (
            <Field key={key}>
              <FieldLabel htmlFor={id}>{label}</FieldLabel>
              {drafts !== null ? (
                <Input
                  id={id}
                  type="number"
                  inputMode="numeric"
                  min={key === 'hours' ? 1 : 0}
                  max={key === 'hours' ? 168 : 50}
                  value={drafts[key]}
                  aria-invalid={invalid}
                  aria-describedby={invalid ? `${id}-error` : undefined}
                  onChange={(event) => onChange(key, event.target.value)}
                />
              ) : (
                <Input
                  id={id}
                  value={targets[key]}
                  disabled
                  className="text-muted-foreground"
                />
              )}
              {invalid ? (
                <FieldError id={`${id}-error`}>{error}</FieldError>
              ) : null}
            </Field>
          );
        })}
      </CardContent>
    </Card>
  );
}

function BackupCodeList({ codes }: { codes: string[] }) {
  return (
    <div>
      <p>Store these codes somewhere private. They will not be shown again.</p>
      <ul className="m-0 mt-2 grid list-none grid-cols-2 gap-x-6 gap-y-2 p-0">
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
      trigger="Set Up"
      contentClassName="sm:max-w-2xl"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        {!totpURI ? (
          <>
            <Field>
              <FieldLabel htmlFor="mfa-password">Current Password</FieldLabel>
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
            <div className="button-group">
              <DialogCancel />
              <Button
                type="button"
                disabled={pending || password.length < 12}
                onClick={() => void begin()}
              >
                {pending ? 'Starting…' : 'Begin Setup'}
              </Button>
            </div>
          </>
        ) : (
          <>
            <Field>
              <FieldLabel htmlFor="mfa-uri">Authenticator Setup URI</FieldLabel>
              <Textarea
                id="mfa-uri"
                readOnly
                value={totpURI}
                className="min-w-0 break-all"
              />
            </Field>
            <BackupCodeList codes={backupCodes} />
            <Field>
              <FieldLabel htmlFor="mfa-code">Authenticator Code</FieldLabel>
              <Input
                id="mfa-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
              />
            </Field>
            <div className="button-group">
              <DialogCancel />
              <Button
                type="button"
                disabled={pending || code.length < 6}
                onClick={() => void verify()}
              >
                {pending ? 'Verifying…' : 'Verify and Finish'}
              </Button>
            </div>
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
              I Stored the Codes — Sign In Again
            </Button>
          </>
        ) : (
          <>
            <Field>
              <FieldLabel htmlFor="backup-password">
                Current Password
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
              {pending ? 'Regenerating…' : 'Regenerate Codes'}
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
      trigger="Change Email"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor="new-account-email">New Email</FieldLabel>
          <Input
            id="new-account-email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </Field>
        <div className={dialogActionsClass}>
          <DialogCancel />
          <Button
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Requesting…' : 'Send Verification'}
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
      trigger="Change Password"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor="current-account-password">
            Current Password
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
          <FieldLabel htmlFor="new-account-password">New Password</FieldLabel>
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
            Confirm New Password
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
          <DialogCancel />
          <Button
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Changing…' : 'Change Password'}
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
      trigger="Set Password"
    >
      <div className={formClass}>
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor="connect-password">New Password</FieldLabel>
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
            Confirm New Password
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
          <DialogCancel />
          <Button
            type="button"
            disabled={pending}
            onClick={() => void submit()}
          >
            {pending ? 'Connecting…' : 'Connect and Sign Out'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}
