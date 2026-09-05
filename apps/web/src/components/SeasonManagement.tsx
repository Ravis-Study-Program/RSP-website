import { zodResolver } from '@hookform/resolvers/zod';
import {
  IconEdit,
  IconLink,
  IconLock,
  IconLockOpen,
  IconPlus,
} from '@tabler/icons-react';
import { useQueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { Controller, useForm, type Resolver } from 'react-hook-form';
import { z } from 'zod';

import { adaptSeason } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  Season as ApiSeason,
  Reasoned,
  SeasonMutation,
  SeasonResourceUpdate,
} from '@/api/generated/models';
import { demoMode, seasonsOptions } from '@/api/queries';
import { AppDatePicker } from '@/components/AppDatePicker';
import { FormDialog } from '@/components/Dialogs';
import styles from '@/styles/App.module.css';
import type { Page, Season } from '@/types';
import { formatDateTimeInput, zonedDateTimeToUtc } from '@/utils';

const optionalHttpsUrl = z
  .string()
  .refine(
    (value) => value === '' || /^https:\/\//i.test(value),
    'Use an HTTPS URL or leave this blank.',
  );
const seasonSchema = z
  .object({
    name: z.string().trim().min(1, 'Enter a season name.'),
    slug: z
      .string()
      .trim()
      .regex(
        /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
        'Use lowercase letters, numbers and hyphens.',
      ),
    startAt: z.string().min(1, 'Choose a start date.'),
    endAt: z.string().min(1, 'Choose an end date.'),
    location: z.string().trim().min(1, 'Enter a location.'),
    imageUrl: optionalHttpsUrl,
    resourcesUrl: optionalHttpsUrl,
  })
  .refine((value) => new Date(value.endAt) > new Date(value.startAt), {
    path: ['endAt'],
    message: 'The end must be after the start.',
  });

type SeasonFormValues = z.infer<typeof seasonSchema>;

function localInputValue(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return formatDateTimeInput(date);
}

function defaultValues(season?: Season): SeasonFormValues {
  const start = new Date();
  start.setHours(9, 0, 0, 0);
  const end = new Date(start);
  end.setMonth(end.getMonth() + 4);
  return season
    ? {
        name: season.name,
        slug: season.slug,
        startAt: localInputValue(season.startsAt),
        endAt: localInputValue(season.endsAt),
        location: season.location,
        imageUrl: season.imageUrl.startsWith('https://') ? season.imageUrl : '',
        resourcesUrl: season.resourcesUrl,
      }
    : {
        name: '',
        slug: '',
        startAt: localInputValue(start.toISOString()),
        endAt: localInputValue(end.toISOString()),
        location: '',
        imageUrl: '',
        resourcesUrl: '',
      };
}

function replaceSeason(page: Page<Season> | undefined, season: Season) {
  if (!page)
    return {
      items: [season],
      totalCount: 1,
      pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
    };
  const exists = page.items.some((item) => item.id === season.id);
  return {
    ...page,
    items: exists
      ? page.items.map((item) => (item.id === season.id ? season : item))
      : [season, ...page.items],
    totalCount: exists ? page.totalCount : page.totalCount + 1,
  };
}

export function SeasonEditorDialog({
  season,
  trigger,
  onSaved,
}: {
  season?: Season;
  trigger?: ReactNode;
  onSaved?: (season: Season) => void;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [requestError, setRequestError] = useState('');
  const form = useForm<SeasonFormValues>({
    resolver: zodResolver(seasonSchema as never) as Resolver<SeasonFormValues>,
    defaultValues: defaultValues(season),
  });

  const save = form.handleSubmit(async (values) => {
    setRequestError('');
    const mutation: SeasonMutation = {
      name: values.name.trim(),
      slug: values.slug.trim(),
      startAt: zonedDateTimeToUtc(values.startAt),
      endAt: zonedDateTimeToUtc(values.endAt),
      location: values.location.trim(),
      imageUrl: values.imageUrl.trim(),
      resourcesUrl: values.resourcesUrl.trim(),
    };
    try {
      const saved = demoMode
        ? {
            ...(season ?? {
              id: `season_demo_${Date.now()}`,
              memberCount: 0,
              weekCount: 0,
              status: 'open' as const,
              summary: '',
            }),
            slug: mutation.slug,
            name: mutation.name,
            startsAt: mutation.startAt,
            endsAt: mutation.endAt,
            location: mutation.location,
            imageUrl: mutation.imageUrl,
            resourcesUrl: mutation.resourcesUrl,
            summary: `Current programme at ${mutation.location}.`,
          }
        : adaptSeason(
            await apiRequest<ApiSeason>(
              season ? `/seasons/${encodeURIComponent(season.id)}` : '/seasons',
              {
                method: season ? 'PATCH' : 'POST',
                body: JSON.stringify(mutation),
              },
            ),
          );
      queryClient.setQueryData<Page<Season>>(
        seasonsOptions.queryKey,
        (current) => replaceSeason(current, saved),
      );
      onSaved?.(saved);
      form.reset(defaultValues(saved));
      setOpen(false);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        await queryClient.invalidateQueries({
          queryKey: seasonsOptions.queryKey,
        });
        setRequestError(
          'This season changed elsewhere. The latest version has been fetched; reopen the form before trying again.',
        );
      } else
        setRequestError(
          error instanceof Error ? error.message : 'Unable to save the season.',
        );
    }
  });

  return (
    <FormDialog
      title={season ? `Edit ${season.name}` : 'Create season'}
      description="Set the programme definition. Privileged changes require recent MFA and are audited."
      open={open}
      onOpenChange={(next) => {
        if (
          !next &&
          form.formState.isDirty &&
          !window.confirm('Discard unsaved season changes?')
        )
          return;
        if (next) {
          form.reset(defaultValues(season));
          setRequestError('');
        }
        setOpen(next);
      }}
      trigger={
        trigger ??
        (season ? (
          <>
            <IconEdit size={17} aria-hidden="true" /> Edit season
          </>
        ) : (
          <>
            <IconPlus size={18} aria-hidden="true" /> Create season
          </>
        ))
      }
    >
      <form
        className={styles.form}
        onSubmit={(event) => void save(event)}
        noValidate
      >
        <div className={styles.fieldGrid}>
          <FormField
            id="season-name"
            label="Name"
            error={form.formState.errors.name?.message}
          >
            <input
              id="season-name"
              className={styles.input}
              autoComplete="off"
              {...form.register('name')}
            />
          </FormField>
          <FormField
            id="season-slug"
            label="URL slug"
            error={form.formState.errors.slug?.message}
          >
            <input
              id="season-slug"
              className={styles.input}
              autoCapitalize="none"
              autoComplete="off"
              {...form.register('slug')}
            />
          </FormField>
          <Controller
            control={form.control}
            name="startAt"
            render={({ field }) => (
              <AppDatePicker
                label="Starts"
                granularity="minute"
                value={field.value}
                onChange={field.onChange}
                error={form.formState.errors.startAt?.message}
              />
            )}
          />
          <Controller
            control={form.control}
            name="endAt"
            render={({ field }) => (
              <AppDatePicker
                label="Ends"
                granularity="minute"
                value={field.value}
                onChange={field.onChange}
                error={form.formState.errors.endAt?.message}
              />
            )}
          />
          <FormField
            id="season-location"
            label="Location"
            error={form.formState.errors.location?.message}
          >
            <input
              id="season-location"
              className={styles.input}
              autoComplete="organization"
              {...form.register('location')}
            />
          </FormField>
          <FormField
            id="season-image"
            label="Image URL (optional)"
            error={form.formState.errors.imageUrl?.message}
          >
            <input
              id="season-image"
              className={styles.input}
              type="url"
              inputMode="url"
              {...form.register('imageUrl')}
            />
          </FormField>
          <FormField
            id="season-resources"
            label="Resources URL (optional)"
            error={form.formState.errors.resourcesUrl?.message}
          >
            <input
              id="season-resources"
              className={styles.input}
              type="url"
              inputMode="url"
              {...form.register('resourcesUrl')}
            />
          </FormField>
        </div>
        {requestError ? (
          <p className={styles.fieldError} role="alert">
            {requestError}
          </p>
        ) : null}
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="submit"
            disabled={form.formState.isSubmitting}
          >
            {form.formState.isSubmitting
              ? 'Saving…'
              : season
                ? 'Save season'
                : 'Create season'}
          </button>
        </div>
      </form>
    </FormDialog>
  );
}

export function SeasonResourcesDialog({ season }: { season: Season }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [resourcesUrl, setResourcesUrl] = useState(season.resourcesUrl);
  const [baseline, setBaseline] = useState(season.resourcesUrl);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty = resourcesUrl !== baseline;

  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved season resource changes?')
    )
      return;
    if (next) {
      setResourcesUrl(season.resourcesUrl);
      setBaseline(season.resourcesUrl);
      setError('');
    }
    setOpen(next);
  };

  const save = async () => {
    const trimmedUrl = resourcesUrl.trim();
    if (!/^https:\/\/[^\s]+$/i.test(trimmedUrl)) {
      setError('Enter a complete HTTPS resource URL.');
      return;
    }
    setPending(true);
    setError('');
    try {
      const saved = demoMode
        ? { ...season, resourcesUrl: trimmedUrl }
        : adaptSeason(
            await apiRequest<ApiSeason>(
              `/seasons/${encodeURIComponent(season.id)}/resources`,
              {
                method: 'PATCH',
                body: JSON.stringify({
                  resourcesUrl: trimmedUrl,
                } satisfies SeasonResourceUpdate),
              },
            ),
          );
      queryClient.setQueryData<Page<Season>>(
        seasonsOptions.queryKey,
        (current) => replaceSeason(current, saved),
      );
      setBaseline(saved.resourcesUrl);
      setResourcesUrl(saved.resourcesUrl);
      setOpen(false);
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        await queryClient.invalidateQueries({
          queryKey: seasonsOptions.queryKey,
        });
        setError(
          'This season changed elsewhere. The latest version has been fetched; reopen this form before trying again.',
        );
      } else {
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to save the season resources.',
        );
      }
    } finally {
      setPending(false);
    }
  };

  return (
    <FormDialog
      title={`Edit resources for ${season.name}`}
      description="Coordinators can update only this resource link; season dates and identity remain Director-managed."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        <>
          <IconLink size={17} aria-hidden="true" /> Edit resources
        </>
      }
    >
      <div className={styles.form}>
        <div className={styles.field}>
          <label htmlFor={`season-resources-only-${season.id}`}>
            Season resources URL
          </label>
          <input
            id={`season-resources-only-${season.id}`}
            className={styles.input}
            type="url"
            inputMode="url"
            value={resourcesUrl}
            onChange={(event) => setResourcesUrl(event.target.value)}
            aria-invalid={Boolean(error)}
          />
        </div>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
            type="button"
            disabled={pending || !dirty}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save resources'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function FormField({
  id,
  label,
  error,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.field}>
      <label htmlFor={id}>{label}</label>
      {children}
      {error ? (
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}

export function SeasonLifecycleDialog({
  season,
  action,
  onSaved,
}: {
  season: Season;
  action: 'close' | 'reopen';
  onSaved?: (season: Season) => void;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const label = action === 'close' ? 'Close season' : 'Reopen season';
  const Icon = action === 'close' ? IconLock : IconLockOpen;
  const dirty = reason.length > 0 || confirmation.length > 0;
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm(`Discard the unsaved ${action} reason?`)
    )
      return;
    if (next) {
      setReason('');
      setConfirmation('');
      setError('');
    }
    setOpen(next);
  };
  const submit = async () => {
    setPending(true);
    setError('');
    const body: Reasoned = {
      reason: reason.trim(),
    };
    try {
      const saved = demoMode
        ? {
            ...season,
            status:
              action === 'close' ? ('closed' as const) : ('open' as const),
          }
        : adaptSeason(
            await apiRequest<ApiSeason>(
              `/seasons/${encodeURIComponent(season.id)}/${action}`,
              { method: 'POST', body: JSON.stringify(body) },
            ),
          );
      queryClient.setQueryData<Page<Season>>(
        seasonsOptions.queryKey,
        (current) => replaceSeason(current, saved),
      );
      onSaved?.(saved);
      setOpen(false);
      setReason('');
      setConfirmation('');
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        await queryClient.invalidateQueries({
          queryKey: seasonsOptions.queryKey,
        });
        setError(
          `This season changed elsewhere. The latest version has been fetched; reopen before trying to ${action} it.`,
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : `Unable to ${action} the season.`,
        );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`${label}: ${season.name}`}
      description={
        action === 'close'
          ? 'Active enrollments will be completed and season-scoped writes locked.'
          : 'Only enrollments completed by the close event will be restored.'
      }
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        <>
          <Icon size={17} aria-hidden="true" /> {label}
        </>
      }
    >
      <div className={styles.form}>
        <div className={styles.field}>
          <label htmlFor={`season-${action}-reason`}>Reason</label>
          <textarea
            id={`season-${action}-reason`}
            className={styles.textarea}
            required
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor={`season-${action}-confirm`}>
            Type <strong>{season.name}</strong> to confirm
          </label>
          <input
            id={`season-${action}-confirm`}
            className={styles.input}
            autoComplete="off"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.dialogActions}>
          <button
            className={action === 'close' ? styles.buttonDanger : styles.button}
            type="button"
            disabled={
              pending ||
              reason.trim().length < 5 ||
              confirmation !== season.name
            }
            onClick={() => void submit()}
          >
            {pending
              ? `${action === 'close' ? 'Closing' : 'Reopening'}…`
              : label}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}
