import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit, IconLock, IconMail, IconWorld } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';
import type { Resolver } from 'react-hook-form';
import { useForm } from 'react-hook-form';
import { useParams } from 'react-router-dom';
import { z } from 'zod';

import { adaptCurrentPerson, adaptCurrentUser } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type { Me } from '@/api/generated/models';
import {
  currentUserOptions,
  demoMode,
  useCurrentUser,
  useSeasons,
  useUserAttempts,
  useUserProfile,
} from '@/api/queries';
import { useQueryClient } from '@tanstack/react-query';
import { MetricCard, RoleBadge, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import { TimezoneSelect } from '@/components/TimezoneSelect';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import type { Attempt } from '@/types';
import { formatDateTime, initials, supportedTimezones } from '@/utils';

const profileSchema = z.object({
  name: z.string().trim().min(2, 'Enter your name.').max(120),
  slug: z
    .string()
    .trim()
    .min(3, 'Use at least 3 characters.')
    .max(50)
    .regex(
      /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/,
      'Use lowercase letters, numbers and single hyphens.',
    ),
  timezone: z
    .string()
    .min(1)
    .refine((timezone) => {
      try {
        new Intl.DateTimeFormat(undefined, { timeZone: timezone }).format();
        return true;
      } catch {
        return false;
      }
    }, 'Enter a valid IANA timezone.'),
});
type ProfileValues = z.infer<typeof profileSchema>;

export function ProfilePage() {
  const { slug } = useParams();
  const user = useCurrentUser();
  const isSelf = !slug || slug === user.data?.slug;
  const hasProgrammeAccess = Boolean(
    user.data &&
    (user.data.seasonRoles.length > 0 ||
      user.data.globalRoles.length > 0 ||
      user.data.alumni),
  );
  const seasons = useSeasons(isSelf && hasProgrammeAccess);
  const publicProfile = useUserProfile(isSelf ? undefined : slug);
  const person = isSelf
    ? user.data
      ? adaptCurrentPerson(user.data, seasons.data?.items)
      : undefined
    : (publicProfile.data ?? undefined);
  const canViewMemberHistory = hasProgrammeAccess;
  const attemptHistory = useUserAttempts(person?.id, canViewMemberHistory);
  const showProgrammeDetails = !isSelf || hasProgrammeAccess;
  const [savedName, setSavedName] = useState<string | null>(null);
  const [savedSlug, setSavedSlug] = useState<string | null>(null);
  usePageTitle(person?.name ?? 'Profile');

  if (
    publicProfile.isLoading ||
    user.isLoading ||
    (showProgrammeDetails && seasons.isLoading)
  )
    return <PageSkeleton label="Loading profile" />;
  if (
    publicProfile.isError ||
    user.isError ||
    (showProgrammeDetails && seasons.isError)
  )
    return (
      <div className={styles.page}>
        <ErrorState
          onRetry={() =>
            void Promise.all([
              publicProfile.refetch(),
              user.refetch(),
              seasons.refetch(),
            ])
          }
        />
      </div>
    );
  if (!person)
    return (
      <div className={styles.page}>
        <ErrorState
          title="Profile not found"
          message="This person may no longer be visible to your account."
        />
      </div>
    );

  const displayName = savedName ?? person.name;
  return (
    <div className={styles.page}>
      <header className={styles.profileHeader}>
        <span className={styles.avatar}>{initials(displayName)}</span>
        <div>
          <h1>{displayName}</h1>
          <p>
            @{savedSlug ?? person.slug}
            {person.season ? ` · ${person.season}` : ''}
          </p>
        </div>
      </header>
      <div className={styles.profileActions}>
        {showProgrammeDetails ? (
          <div className={styles.inline}>
            {person.roles.map((role) => (
              <RoleBadge role={role} key={role} />
            ))}
            <span
              className={
                person.status === 'active'
                  ? styles.badgeSuccess
                  : styles.badgeNeutral
              }
            >
              {person.status}
            </span>
          </div>
        ) : null}
        {isSelf && user.data ? (
          <EditProfileDialog
            user={user.data}
            showProgrammeDetails={showProgrammeDetails}
            onSaved={(values) => {
              setSavedName(values.name);
              setSavedSlug(values.slug);
            }}
          />
        ) : null}
      </div>
      {isSelf ? (
        <div className={styles.profileNotice}>
          <InlineNotice tone="success">
            <IconLock size={19} aria-hidden="true" /> Private account details
            below are only visible to you
            {showProgrammeDetails
              ? ' and authorised programme administrators.'
              : '.'}
          </InlineNotice>
        </div>
      ) : null}
      {showProgrammeDetails ? (
        <div className={styles.metricGrid}>
          <MetricCard
            label="Practice attempts"
            value={person.attempts}
            detail="Public programme activity count"
          />
          <MetricCard
            label="Mock interviews"
            value={person.interviews}
            detail="Received and conducted"
          />
          <MetricCard
            label="Last active"
            value={
              person.lastActiveAt
                ? formatDateTime(person.lastActiveAt).split(',')[0]
                : 'Not available'
            }
            detail="Shown to approved members"
          />
        </div>
      ) : null}
      <div
        className={`${styles.grid2} ${!showProgrammeDetails ? styles.profileBasicGrid : ''}`}
      >
        {showProgrammeDetails ? (
          <section className={styles.section}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>Programme participation</h2>
            </div>
            <div className={styles.panel}>
              <ul className={styles.cleanList}>
                <li className={styles.listRow}>
                  <span>Current season</span>
                  <strong>{person.season}</strong>
                </li>
                <li className={styles.listRow}>
                  <span>Role badges</span>
                  <strong>{person.roles.join(', ')}</strong>
                </li>
                <li className={styles.listRow}>
                  <span>Activity visibility</span>
                  <strong>Members and alumni</strong>
                </li>
              </ul>
            </div>
          </section>
        ) : null}
        {isSelf ? (
          <section className={styles.section}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>Private account details</h2>
            </div>
            <div className={styles.panel}>
              <ul className={styles.cleanList}>
                <li className={styles.listRow}>
                  <span className={styles.inline}>
                    <IconMail size={18} aria-hidden="true" /> Email
                  </span>
                  <strong>{user.data?.email}</strong>
                </li>
                <li className={styles.listRow}>
                  <span className={styles.inline}>
                    <IconWorld size={18} aria-hidden="true" /> Timezone
                  </span>
                  <strong>{user.data?.timezone}</strong>
                </li>
                <li className={styles.listRow}>
                  <span>Email verification</span>
                  <span className={styles.badgeSuccess}>Verified</span>
                </li>
              </ul>
            </div>
          </section>
        ) : null}
      </div>
      {showProgrammeDetails ? (
        <section
          className={styles.section}
          aria-labelledby="public-practice-title"
        >
          <div className={styles.sectionHeader}>
            <h2 id="public-practice-title" className={styles.sectionTitle}>
              Public problem history
            </h2>
          </div>
          <DataTable
            ariaLabel={`${displayName} public problem history`}
            data={attemptHistory.data?.items ?? []}
            columns={profileAttemptColumns}
            loading={canViewMemberHistory && attemptHistory.isLoading}
            error={canViewMemberHistory && attemptHistory.isError}
            onRetry={() => void attemptHistory.refetch()}
            emptyTitle="No public attempts"
            emptyMessage="Outcome-known practice will appear here when it is available to your account."
            getRowId={(attempt) => attempt.id}
            renderCard={(row) => (
              <div>
                <h3 className={styles.cardTitle}>{row.original.problem}</h3>
                <p>
                  {row.original.difficulty ?? 'Difficulty unavailable'} ·{' '}
                  {row.original.outcome.replaceAll('_', ' ')}
                </p>
                <p className={styles.helper}>
                  {formatDateTime(row.original.attemptedAt)}
                </p>
              </div>
            )}
          />
        </section>
      ) : null}
      {showProgrammeDetails ? (
        <section
          className={styles.section}
          aria-labelledby="interview-participation-title"
        >
          <div className={styles.sectionHeader}>
            <h2
              id="interview-participation-title"
              className={styles.sectionTitle}
            >
              Mock interview participation
            </h2>
          </div>
          <div className={styles.panel}>
            <p>
              {person.interviews ?? 0} mock{' '}
              {person.interviews === 1 ? 'interview' : 'interviews'} received or
              conducted.
            </p>
            <p className={styles.helper}>
              Detailed private notes and scores remain visible only to
              authorised relationships. The API currently exposes the safe
              aggregate for another member.
            </p>
          </div>
        </section>
      ) : null}
    </div>
  );
}

const profileAttemptColumns: ColumnDef<Attempt, any>[] = [
  { accessorKey: 'problem', header: 'Problem' },
  { accessorKey: 'difficulty', header: 'Difficulty' },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    cell: ({ getValue }) => String(getValue()).replaceAll('_', ' '),
  },
  {
    accessorKey: 'attemptedAt',
    header: 'Attempted',
    cell: ({ getValue }) => formatDateTime(getValue()),
  },
];

function EditProfileDialog({
  user,
  showProgrammeDetails,
  onSaved,
}: {
  user: NonNullable<ReturnType<typeof useCurrentUser>['data']>;
  showProgrammeDetails: boolean;
  onSaved: (values: ProfileValues) => void;
}) {
  const [open, setOpen] = useState(false);
  const [requestError, setRequestError] = useState('');
  const [suggestionError, setSuggestionError] = useState('');
  const [isSuggesting, setIsSuggesting] = useState(false);
  const queryClient = useQueryClient();
  const timezoneOptions = useMemo(
    () => supportedTimezones(user.timezone),
    [user.timezone],
  );
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema as never) as Resolver<ProfileValues>,
    defaultValues: {
      name: user.name,
      slug: user.slug,
      timezone: user.timezone,
    },
  });
  const timezoneField = register('timezone');
  const selectedTimezone = watch('timezone');
  const submit = handleSubmit(async (values) => {
    setRequestError('');
    try {
      if (!demoMode) {
        const updated = adaptCurrentUser(
          await apiRequest<Me>('/me', {
            method: 'PATCH',
            body: JSON.stringify({
              name: values.name,
              slug: values.slug,
              avatarUrl: user.avatarUrl,
              timezone: values.timezone,
            }),
          }),
        );
        queryClient.setQueryData(currentUserOptions.queryKey, updated);
      } else {
        queryClient.setQueryData(currentUserOptions.queryKey, {
          ...user,
          name: values.name,
          slug: values.slug,
          timezone: values.timezone,
        });
      }
      onSaved(values);
      reset(values);
      setOpen(false);
    } catch (error) {
      setRequestError(
        error instanceof ApiError && error.status === 409
          ? 'That profile slug is already in use, or this profile changed elsewhere. Choose another slug or reopen the form.'
          : error instanceof Error
            ? error.message
            : 'Unable to update your profile.',
      );
    }
  });
  const onOpenChange = (next: boolean) => {
    if (!next && isDirty && !window.confirm('Discard your profile changes?'))
      return;
    if (next) setSuggestionError('');
    setOpen(next);
  };
  const suggestSlug = async () => {
    setSuggestionError('');
    setIsSuggesting(true);
    try {
      const suggestion = demoMode
        ? { slug: `member-${crypto.randomUUID().slice(0, 8)}` }
        : await apiRequest<{ slug: string }>('/me/slug-suggestion');
      setValue('slug', suggestion.slug, {
        shouldDirty: true,
        shouldValidate: true,
      });
    } catch (error) {
      setSuggestionError(
        error instanceof Error
          ? error.message
          : 'Unable to suggest a profile slug.',
      );
    } finally {
      setIsSuggesting(false);
    }
  };
  return (
    <FormDialog
      title="Edit profile"
      description={
        showProgrammeDetails
          ? 'Your name and slug are visible to approved active members and alumni.'
          : 'Set the name shown on your account.'
      }
      open={open}
      onOpenChange={onOpenChange}
      trigger={
        <>
          <IconEdit size={17} aria-hidden="true" /> Edit profile
        </>
      }
    >
      <form className={styles.form} noValidate onSubmit={submit}>
        {requestError ? (
          <p className={styles.fieldError} role="alert">
            {requestError}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="profile-name">Name</label>
          <input
            id="profile-name"
            className={styles.input}
            aria-invalid={Boolean(errors.name)}
            {...register('name')}
          />
          {errors.name ? (
            <p className={styles.fieldError}>{errors.name.message}</p>
          ) : null}
        </div>
        {showProgrammeDetails ? (
          <div className={styles.field}>
            <label htmlFor="profile-slug">Profile slug</label>
            <div className={styles.inline}>
              <input
                id="profile-slug"
                className={styles.input}
                autoCapitalize="none"
                autoComplete="off"
                aria-invalid={Boolean(errors.slug || suggestionError)}
                aria-describedby={
                  suggestionError ? 'profile-slug-suggestion-error' : undefined
                }
                {...register('slug')}
              />
              <button
                className={styles.buttonSecondary}
                type="button"
                disabled={isSuggesting}
                onClick={() => void suggestSlug()}
              >
                {isSuggesting ? 'Suggesting…' : 'Suggest'}
              </button>
            </div>
            {errors.slug ? (
              <p className={styles.fieldError}>{errors.slug.message}</p>
            ) : suggestionError ? (
              <p
                id="profile-slug-suggestion-error"
                className={styles.fieldError}
                role="alert"
              >
                {suggestionError}
              </p>
            ) : (
              <p className={styles.helper}>
                3–50 lowercase letters, numbers and hyphens.
              </p>
            )}
          </div>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="profile-timezone">Timezone</label>
          <TimezoneSelect
            id="profile-timezone"
            name={timezoneField.name}
            options={timezoneOptions}
            value={selectedTimezone}
            invalid={Boolean(errors.timezone)}
            inputRef={timezoneField.ref}
            onBlur={timezoneField.onBlur}
            onChange={(timezone) =>
              setValue('timezone', timezone, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
          />
          {errors.timezone ? (
            <p className={styles.fieldError}>{errors.timezone.message}</p>
          ) : (
            <p className={styles.helper}>
              Use an IANA timezone such as Australia/Adelaide.
            </p>
          )}
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting ? 'Saving…' : 'Save profile'}
          </button>
        </div>
      </form>
    </FormDialog>
  );
}
