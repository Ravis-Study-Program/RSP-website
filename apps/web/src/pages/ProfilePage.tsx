import { statusTone } from '@/badgeTones';
import { StatusBadge } from '@/components/StatusBadge';
import { cn } from '@/lib/utils';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit } from '@tabler/icons-react';
import { useMemo, useState } from 'react';
import type { Resolver } from 'react-hook-form';
import { useForm } from 'react-hook-form';
import { useParams } from 'react-router-dom';
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
import {
  MetricCard,
  RoleBadge,
  SeasonBadge,
  StudentLevelBadge,
  usePageTitle,
} from '@/components/Common';
import { ProfileActivity } from '@/components/ProfileActivity';
import { FormDialog } from '@/components/Dialogs';
import { TimezoneSelect } from '@/components/TimezoneSelect';
import { ErrorState, PageSkeleton } from '@/components/StatusViews';
import { formatDateTime, initials, supportedTimezones } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const formClass = 'flex flex-col gap-4';
const inlineClass = 'flex flex-wrap items-center gap-2.5';
const dialogActionsClass = 'mt-5 flex justify-end gap-2.5';

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
  const user = useCurrentUser();
  const isSelf = !slug || slug === user.data?.slug;
  const hasProgramAccess = Boolean(
    user.data &&
    (user.data.seasonRoles.length > 0 ||
      user.data.globalRoles.length > 0 ||
      user.data.alumni),
  );
  const seasons = useSeasons(hasProgramAccess);
  const publicProfile = useUserProfile(isSelf ? undefined : slug);
  const person = isSelf
    ? user.data
      ? adaptCurrentPerson(user.data, seasons.data?.items)
      : undefined
    : (publicProfile.data ?? undefined);
  const participation = useUserParticipation(person?.id, hasProgramAccess);
  // Profiles always show all-time activity.
  const summary = useActivitySummary(hasProgramAccess ? person?.id : undefined);
  const showProgramDetails = !isSelf || hasProgramAccess;
  const [savedName, setSavedName] = useState<string | null>(null);
  const [savedSlug, setSavedSlug] = useState<string | null>(null);
  const [activityTab, setActivityTab] = useState('leetcode');
  usePageTitle(person?.name ?? 'Profile');

  if (
    publicProfile.isLoading ||
    user.isLoading ||
    (showProgramDetails && seasons.isLoading)
  )
    return <PageSkeleton label="Loading profile" />;
  if (
    publicProfile.isError ||
    user.isError ||
    (showProgramDetails && seasons.isError)
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
          title="Profile Not Found"
          message="This person may no longer be visible to your account."
        />
      </div>
    );

  const displayName = savedName ?? person.name;
  // The student level from the most recent season that recorded one.
  const studentLevel = [...(participation.data?.items ?? [])]
    .sort((left, right) => right.startAt.localeCompare(left.startAt))
    .find(
      (entry) => entry.lastStudentLevel !== 'not_applicable',
    )?.lastStudentLevel;
  const seasonsTaken = [
    ...new Map(
      [...(participation.data?.items ?? [])]
        .sort((left, right) => left.startAt.localeCompare(right.startAt))
        .map((entry) => [entry.seasonId, entry]),
    ).values(),
  ];
  return (
    // Every part of the profile sits 24px from the next.
    <div className={cn(pageClass, 'flex flex-col gap-6')}>
      <header className="flex flex-wrap items-center justify-between gap-4 border-b pb-4">
        <div className="flex min-w-0 items-center gap-4">
          <span className="inline-grid size-18 shrink-0 place-items-center rounded-full bg-primary text-xl font-semibold text-primary-foreground">
            {initials(displayName)}
          </span>
          <div className="grid min-w-0 gap-1">
            <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
              {displayName}
            </h1>
            <div className={inlineClass}>
              <p className="text-muted-foreground">
                @{savedSlug ?? person.slug}
              </p>
              {showProgramDetails ? (
                <>
                  {person.roles.map((role) => (
                    <RoleBadge role={role} key={role} />
                  ))}
                  {seasonsTaken.map((entry) => (
                    <SeasonBadge
                      key={entry.seasonId}
                      name={entry.seasonName}
                      startsAt={entry.startAt}
                    />
                  ))}
                  <StudentLevelBadge level={studentLevel} />
                  <StatusBadge tone={statusTone(person.status)}>
                    {statusLabel(person.status)}
                  </StatusBadge>
                </>
              ) : null}
            </div>
          </div>
        </div>
        {isSelf && user.data ? (
          <EditProfileDialog
            user={user.data}
            showProgramDetails={showProgramDetails}
            onSaved={(values) => {
              setSavedName(values.name);
              setSavedSlug(values.slug);
            }}
          />
        ) : null}
      </header>
      {showProgramDetails ? (
        <div className="grid grid-cols-3 gap-6 max-[56rem]:grid-cols-1">
          <MetricCard
            label="Leetcodes"
            value={summary.data?.attemptCount ?? '—'}
          />
          <MetricCard
            label="Mock Interviews"
            value={summary.data?.mockInterviewCount ?? '—'}
          />
          <MetricCard
            label="Last Recorded Activity"
            value={
              summary.data?.lastActivityAt ? (
                formatDateTime(summary.data.lastActivityAt).split(',')[0]
              ) : (
                <span className="text-sm font-normal text-muted-foreground">
                  Not available
                </span>
              )
            }
          />
        </div>
      ) : null}
      {showProgramDetails && summary.isError ? (
        <ErrorState
          title="Activity Totals Unavailable"
          onRetry={() => void summary.refetch()}
        />
      ) : null}
      {showProgramDetails ? (
        <div>
          <ProfileActivity
            person={person}
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
  showProgramDetails,
  onSaved,
}: {
  user: CurrentUser;
  showProgramDetails: boolean;
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
      title="Edit Profile"
      description={
        showProgramDetails
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
        {showProgramDetails ? (
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
