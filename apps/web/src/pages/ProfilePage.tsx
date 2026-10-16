import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit, IconLock, IconMail, IconWorld } from '@tabler/icons-react';
import { useMemo, useState } from 'react';
import type { Resolver } from 'react-hook-form';
import { useForm } from 'react-hook-form';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { z } from 'zod';

import { adaptCurrentPerson, adaptCurrentUser } from '@/api/adapters';
import type { CurrentUser } from '@/types';
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
import {
  formatDate,
  formatDateTime,
  initials,
  supportedTimezones,
} from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const sectionClass = 'mt-6';
const sectionHeaderClass = 'mb-3 flex items-end justify-between gap-4';
const sectionTitleClass = 'text-xl font-semibold tracking-tight';
const formClass = 'flex flex-col gap-4';
const inlineClass = 'flex flex-wrap items-center gap-2.5';
const dialogActionsClass = 'mt-5 flex justify-end gap-2.5';
const helperClass = 'text-sm text-muted-foreground';
const badgeSuccessClass = 'bg-success-soft text-success';
const badgeNeutralClass = 'bg-muted text-muted-foreground';

function statusLabel(status: string | null | undefined) {
  const text = (status ?? '').replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
}

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
  const [activityTab, setActivityTab] = useState('practice');
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
      <div className={pageClass}>
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
      <div className={pageClass}>
        <ErrorState
          title="Profile not found"
          message="This person may no longer be visible to your account."
        />
      </div>
    );

  if (seasonId && !seasons.isLoading && !selectedSeason)
    return (
      <div className={pageClass}>
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
  const seasonItems = [
    { label: 'All time', value: '' },
    ...(seasons.data?.items ?? [])
      .filter(
        (season) =>
          season.id === seasonId ||
          participation.data?.items.some(
            (entry) => entry.seasonId === season.id,
          ),
      )
      .map((season) => ({ label: season.name, value: season.id })),
  ];
  return (
    <div className={pageClass}>
      <header className="flex items-center gap-4 border-b pb-4">
        <span className="inline-grid size-18 shrink-0 place-items-center rounded-full bg-primary text-xl font-semibold text-primary-foreground">
          {initials(displayName)}
        </span>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
            {displayName}
          </h1>
          <p className="text-muted-foreground">
            @{savedSlug ?? person.slug}
            {showProgrammeDetails
              ? ` · ${selectedSeason?.name ?? 'All time'}`
              : ''}
          </p>
        </div>
      </header>
      {showProgrammeDetails ? (
        <Field className="mt-4">
          <FieldLabel htmlFor="profile-season">View activity</FieldLabel>
          <Select
            items={seasonItems}
            value={seasonId ?? ''}
            onValueChange={(value) => {
              const next = new URLSearchParams(searchParams);
              next.delete('weekId');
              next.delete('year');
              if (value) next.set('seasonId', value);
              else next.delete('seasonId');
              setSearchParams(next);
            }}
          >
            <SelectTrigger id="profile-season" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {seasonItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <FieldDescription>
            {selectedSeason
              ? `Activity recorded in ${selectedSeason.name}`
              : 'Complete practice and mock history, including activity between seasons.'}
          </FieldDescription>
        </Field>
      ) : null}
      <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
        {showProgrammeDetails ? (
          <div className={inlineClass}>
            {displayedRoles.map((role) => (
              <RoleBadge role={role} key={role} />
            ))}
            <Badge
              variant="secondary"
              className={
                displayedStatus === 'active'
                  ? badgeSuccessClass
                  : badgeNeutralClass
              }
            >
              {statusLabel(displayedStatus)}
            </Badge>
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
        <div className="mt-4">
          <InlineNotice>
            <IconLock size={19} aria-hidden="true" /> Private account details
            below are only visible to you
            {showProgrammeDetails
              ? ' and authorised programme administrators.'
              : '.'}
          </InlineNotice>
        </div>
      ) : null}
      {showProgrammeDetails ? (
        <div className="mt-4 grid grid-cols-3 gap-4 max-[56rem]:grid-cols-1">
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
              summary.data?.lastActivityAt ? (
                formatDateTime(summary.data.lastActivityAt).split(',')[0]
              ) : (
                <span className="text-sm font-normal text-muted-foreground">
                  Not available
                </span>
              )
            }
            detail={selectedSeason?.name ?? 'All time'}
          />
        </div>
      ) : null}
      <div
        className={`mt-4 grid items-stretch gap-6 ${showProgrammeDetails && isSelf ? 'min-[56rem]:grid-cols-2' : ''}`}
      >
        {showProgrammeDetails ? (
          <section className={sectionClass}>
            <div className={sectionHeaderClass}>
              <h2 className={sectionTitleClass}>Programme participation</h2>
            </div>
            <Card className="h-full">
              <CardContent>
                {participation.isLoading ? (
                  <p role="status">Loading participation…</p>
                ) : participation.isError ? (
                  <ErrorState
                    title="Participation unavailable"
                    onRetry={() => void participation.refetch()}
                  />
                ) : (
                  <ol className="relative grid gap-0">
                    {(participation.data?.items ?? []).map((entry) => (
                      <li key={entry.id} className="pb-4 last:pb-0">
                        <div className="min-w-0">
                          <Link
                            to={`${isSelf ? '/profile' : `/people/${person.slug}`}?seasonId=${encodeURIComponent(entry.seasonId)}`}
                          >
                            {entry.seasonName}
                          </Link>
                          <p className="text-sm">
                            {formatDate(entry.startAt)} –{' '}
                            {formatDate(entry.endAt)} · {entry.role} ·{' '}
                            {entry.state}
                          </p>
                          {entry.lastStudentLevel !== 'not_applicable' ? (
                            <p className="text-sm">
                              Student level:{' '}
                              {studentLevelLabel(entry.lastStudentLevel)}
                            </p>
                          ) : null}
                          {entry.periods.map((period, index) => (
                            <p className={helperClass} key={index}>
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
              </CardContent>
            </Card>
          </section>
        ) : null}
        {isSelf ? (
          <section className={sectionClass}>
            <div className={sectionHeaderClass}>
              <h2 className={sectionTitleClass}>Private account details</h2>
            </div>
            <Card className="h-full">
              <CardContent>
                <ul className="m-0 list-none divide-y p-0">
                  <li className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
                    <span className={inlineClass}>
                      <IconMail size={18} aria-hidden="true" /> Email
                    </span>
                    <strong className="font-semibold">
                      {user.data?.email}
                    </strong>
                  </li>
                  <li className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
                    <span className={inlineClass}>
                      <IconWorld size={18} aria-hidden="true" /> Timezone
                    </span>
                    <strong className="font-semibold">
                      {user.data?.timezone}
                    </strong>
                  </li>
                  <li className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
                    <span>Email verification</span>
                    <Badge variant="secondary" className={badgeSuccessClass}>
                      Verified
                    </Badge>
                  </li>
                </ul>
              </CardContent>
            </Card>
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
        <div className="mt-6">
          <ProfileActivity
            person={person}
            seasonId={seasonId}
            tab={activityTab}
            onTabChange={setActivityTab}
          />
        </div>
      ) : null}
    </div>
  );
}

function EditProfileDialog({
  user,
  showProgrammeDetails,
  onSaved,
}: {
  user: CurrentUser;
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
      <form className={formClass} noValidate onSubmit={submit}>
        {requestError ? (
          <FieldError role="alert">{requestError}</FieldError>
        ) : null}
        <Field>
          <FieldLabel htmlFor="profile-name">Name</FieldLabel>
          <Input
            id="profile-name"
            aria-invalid={Boolean(errors.name)}
            {...register('name')}
          />
          {errors.name ? <FieldError>{errors.name.message}</FieldError> : null}
        </Field>
        {showProgrammeDetails ? (
          <Field>
            <FieldLabel htmlFor="profile-slug">Profile slug</FieldLabel>
            <div className={inlineClass}>
              <Input
                id="profile-slug"
                autoCapitalize="none"
                autoComplete="off"
                aria-invalid={Boolean(errors.slug || suggestionError)}
                aria-describedby={
                  suggestionError ? 'profile-slug-suggestion-error' : undefined
                }
                {...register('slug')}
              />
              <Button
                variant="outline"
                type="button"
                disabled={isSuggesting}
                onClick={() => void suggestSlug()}
              >
                {isSuggesting ? 'Suggesting…' : 'Suggest'}
              </Button>
            </div>
            {errors.slug ? (
              <FieldError>{errors.slug.message}</FieldError>
            ) : suggestionError ? (
              <FieldError id="profile-slug-suggestion-error" role="alert">
                {suggestionError}
              </FieldError>
            ) : (
              <FieldDescription>
                3–50 lowercase letters, numbers and hyphens.
              </FieldDescription>
            )}
          </Field>
        ) : null}
        <Field>
          <FieldLabel htmlFor="profile-timezone">Timezone</FieldLabel>
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
            <FieldError>{errors.timezone.message}</FieldError>
          ) : (
            <FieldDescription>
              Use an IANA timezone such as Australia/Adelaide.
            </FieldDescription>
          )}
        </Field>
        <div className={dialogActionsClass}>
          <Button
            variant="outline"
            type="button"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Saving…' : 'Save profile'}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
}
