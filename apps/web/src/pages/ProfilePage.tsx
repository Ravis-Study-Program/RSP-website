import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit, IconLock, IconMail, IconWorld } from '@tabler/icons-react';
import { useMemo, useState } from 'react';
import type { Resolver } from 'react-hook-form';
import { useForm } from 'react-hook-form';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { z } from 'zod';

import { adaptCurrentPerson, adaptCurrentUser } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type { Me } from '@/api/generated/models';
import {
  currentUserOptions,
  demoMode,
  useCurrentUser,
  useSeasons,
  useActivitySummary,
  useUserParticipation,
  useUserProfile,
} from '@/api/queries';
import { useQueryClient } from '@tanstack/react-query';
import { MetricCard, RoleBadge, usePageTitle } from '@/components/Common';
import { ProfileActivity } from '@/components/ProfileActivity';
import { studentLevelLabel } from '@/studentLevels';
import { FormDialog } from '@/components/Dialogs';
import { TimezoneSelect } from '@/components/TimezoneSelect';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import {
  formatDate,
  formatDateTime,
  initials,
  supportedTimezones,
} from '@/utils';

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
  const [searchParams, setSearchParams] = useSearchParams();
  const seasonId = searchParams.get('seasonId') || undefined;
  const user = useCurrentUser();
  const isSelf = !slug || slug === user.data?.slug;
  const hasProgrammeAccess = Boolean(
    user.data &&
    (user.data.seasonRoles.length > 0 ||
      user.data.globalRoles.length > 0 ||
      user.data.alumni),
  );
  const seasons = useSeasons(hasProgrammeAccess);
  const publicProfile = useUserProfile(isSelf ? undefined : slug, seasonId);
  const person = isSelf
    ? user.data
      ? adaptCurrentPerson(user.data, seasons.data?.items)
      : undefined
    : (publicProfile.data ?? undefined);
  const participation = useUserParticipation(
    person?.id,
    hasProgrammeAccess,
    seasonId,
  );
  const summary = useActivitySummary(
    hasProgrammeAccess ? person?.id : undefined,
    hasProgrammeAccess ? seasonId : undefined,
  );
  const selectedSeason = seasons.data?.items.find(
    (season) => season.id === seasonId,
  );
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

  if (seasonId && !seasons.isLoading && !selectedSeason)
    return (
      <div className={styles.page}>
        <ErrorState
          title="Season unavailable"
          message="This season does not exist or is not available to your account."
        />
      </div>
    );
  const displayName = savedName ?? person.name;
  const selectedParticipation = participation.data?.items.find(
    (entry) => entry.seasonId === seasonId,
  );
  const displayedRoles = selectedParticipation
    ? [selectedParticipation.role]
    : person.roles;
  const displayedStatus = selectedParticipation?.state ?? person.status;
  return (
    <div className={styles.page}>
      <header className={styles.profileHeader}>
        <span className={styles.avatar}>{initials(displayName)}</span>
        <div>
          <h1>{displayName}</h1>
          <p>
            @{savedSlug ?? person.slug}
            {showProgrammeDetails
              ? ` · ${selectedSeason?.name ?? 'All time'}`
              : ''}
          </p>
        </div>
      </header>
      {showProgrammeDetails ? (
        <div className={styles.field}>
          <label htmlFor="profile-season">View activity</label>
          <select
            id="profile-season"
            className={styles.select}
            value={seasonId ?? ''}
            onChange={(event) => {
              const next = new URLSearchParams(searchParams);
              if (event.target.value) next.set('seasonId', event.target.value);
              else next.delete('seasonId');
              setSearchParams(next);
            }}
          >
            <option value="">All time</option>
            {(seasons.data?.items ?? [])
              .filter(
                (season) =>
                  season.id === seasonId ||
                  participation.data?.items.some(
                    (entry) => entry.seasonId === season.id,
                  ),
              )
              .map((season) => (
                <option key={season.id} value={season.id}>
                  {season.name}
                </option>
              ))}
          </select>
          <p className={styles.helper}>
            {selectedSeason
              ? `Activity recorded in ${selectedSeason.name}`
              : 'Complete practice and mock history, including activity between seasons.'}
          </p>
        </div>
      ) : null}
      <div className={styles.profileActions}>
        {showProgrammeDetails ? (
          <div className={styles.inline}>
            {displayedRoles.map((role) => (
              <RoleBadge role={role} key={role} />
            ))}
            <span
              className={
                displayedStatus === 'active'
                  ? styles.badgeSuccess
                  : styles.badgeNeutral
              }
            >
              {displayedStatus}
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
            value={summary.data?.attemptCount ?? '—'}
            detail={selectedSeason?.name ?? 'All time'}
          />
          <MetricCard
            label="Mock interviews"
            value={summary.data?.mockInterviewCount ?? '—'}
            detail={
              summary.data
                ? `${summary.data.mocksReceived} received · ${summary.data.mocksConducted} conducted`
                : 'Received and conducted'
            }
          />
          <MetricCard
            label="Last recorded activity"
            value={
              summary.data?.lastActivityAt
                ? formatDateTime(summary.data.lastActivityAt).split(',')[0]
                : 'Not available'
            }
            detail={selectedSeason?.name ?? 'All time'}
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
              {participation.isLoading ? (
                <p role="status">Loading participation…</p>
              ) : participation.isError ? (
                <ErrorState
                  title="Participation unavailable"
                  onRetry={() => void participation.refetch()}
                />
              ) : (
                <ol className={styles.timeline}>
                  {(participation.data?.items ?? []).map((entry) => (
                    <li key={entry.id} className={styles.timelineItem}>
                      <div className={styles.timelineContent}>
                        <Link
                          to={`${isSelf ? '/profile' : `/people/${person.slug}`}?seasonId=${encodeURIComponent(entry.seasonId)}`}
                        >
                          {entry.seasonName}
                        </Link>
                        <p>
                          {formatDate(entry.startAt)} –{' '}
                          {formatDate(entry.endAt)} · {entry.role} ·{' '}
                          {entry.state}
                        </p>
                        {entry.lastStudentLevel !== 'not_applicable' ? (
                          <p>
                            Student level:{' '}
                            {studentLevelLabel(entry.lastStudentLevel)}
                          </p>
                        ) : null}
                        {entry.periods.map((period, index) => (
                          <p className={styles.helper} key={index}>
                            {period.role}: {formatDate(period.startedAt)} –{' '}
                            {period.endedAt
                              ? formatDate(period.endedAt)
                              : 'Season end'}
                          </p>
                        ))}
                      </div>
                    </li>
                  ))}
                  {!participation.data?.items.length ? (
                    <li>No recorded season participation.</li>
                  ) : null}
                </ol>
              )}
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
      {showProgrammeDetails && summary.isError ? (
        <ErrorState
          title="Activity totals unavailable"
          onRetry={() => void summary.refetch()}
        />
      ) : null}
      {showProgrammeDetails ? (
        <ProfileActivity person={person} seasonId={seasonId} />
      ) : null}
    </div>
  );
}

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
