import { Alert, AlertDescription } from '@/components/ui/alert';
import { LabelHint } from '@/components/Common';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import { Textarea } from '@/components/ui/textarea';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  IconArrowRight,
  IconDotsVertical,
  IconEdit,
  IconLink,
  IconLock,
  IconLockOpen,
  IconPlus,
  IconTrash,
  IconChevronDown,
} from '@tabler/icons-react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { getSelectedSeasonSlug, selectSeason } from '@/selectedSeason';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
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
import { DeleteEmptySeason } from '@/components/DeleteEmptySeason';
import { FormDialog, DialogCancel } from '@/components/Dialogs';
import type { Page, Season } from '@/types';
import { formatDateTimeInput, zonedDateTimeToUtc } from '@/utils';

const formStack = 'grid gap-4';
const dialogActions = 'mt-5 button-group';

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
    resourcesUrl: optionalHttpsUrl,
  })
  .refine((value) => value.endAt >= value.startAt, {
    path: ['endAt'],
    message: 'The end must be after the start.',
  });

type SeasonFormValues = z.infer<typeof seasonSchema>;

/** A stored time as the local calendar date (YYYY-MM-DD) it falls on. */
function localInputValue(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return formatDateTimeInput(date).slice(0, 10);
}

function defaultValues(season?: Season): SeasonFormValues {
  const start = new Date();
  start.setHours(9, 0, 0, 0);
  const end = new Date(start);
  end.setMonth(end.getMonth() + 3);
  return season
    ? {
        name: season.name,
        slug: season.slug,
        startAt: localInputValue(season.startsAt),
        endAt: localInputValue(season.endsAt),
        resourcesUrl: season.resourcesUrl,
      }
    : {
        name: '',
        slug: '',
        startAt: localInputValue(start.toISOString()),
        endAt: localInputValue(end.toISOString()),
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
  open: controlledOpen,
  onOpenChange,
}: {
  season?: Season;
  trigger?: ReactNode;
  onSaved?: (season: Season) => void;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
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
      // Seasons are whole days: the start of the first to the end of the last.
      startAt: zonedDateTimeToUtc(`${values.startAt}T00:00`),
      endAt: zonedDateTimeToUtc(`${values.endAt}T23:59:59`),
      // Not edited in the app; an edit keeps whatever is stored.
      location: season?.location ?? '',
      imageUrl: season?.imageUrl ?? '',
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
            summary: 'Current program.',
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
      // A new slug changes the season's address: keep the page, and the season
      // remembered as selected, pointing at it.
      if (season && saved.slug !== season.slug) {
        if (getSelectedSeasonSlug() === season.slug) selectSeason(saved.slug);
        const old = `/seasons/${season.slug}`;
        if (
          location.pathname === old ||
          location.pathname.startsWith(`${old}/`)
        )
          navigate(
            `/seasons/${saved.slug}${location.pathname.slice(old.length)}${location.search}${location.hash}`,
            { replace: true },
          );
      }
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
      title={season ? `Edit ${season.name}` : 'Create Season'}
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
      triggerVariant={season ? 'outline' : undefined}
      triggerSize={season ? 'sm' : undefined}
      trigger={
        controlled
          ? undefined
          : (trigger ??
            (season ? (
              <>
                <IconEdit size={17} aria-hidden="true" /> Edit Season
              </>
            ) : (
              <>
                <IconPlus size={18} aria-hidden="true" /> Create Season
              </>
            )))
      }
    >
      <form
        className={formStack}
        onSubmit={(event) => void save(event)}
        noValidate
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField
            id="season-name"
            label="Name"
            error={form.formState.errors.name?.message}
          >
            <Input
              id="season-name"
              autoComplete="off"
              {...form.register('name')}
            />
          </FormField>
          <FormField
            id="season-slug"
            label="URL Slug"
            error={form.formState.errors.slug?.message}
          >
            <Input
              id="season-slug"
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
                value={field.value}
                onChange={field.onChange}
                error={form.formState.errors.endAt?.message}
              />
            )}
          />
          <FormField
            id="season-resources"
            label={
              <>
                Resources URL <LabelHint>optional</LabelHint>
              </>
            }
            error={form.formState.errors.resourcesUrl?.message}
          >
            <Input
              id="season-resources"
              type="url"
              inputMode="url"
              {...form.register('resourcesUrl')}
            />
          </FormField>
        </div>
        {requestError ? (
          <Alert variant="destructive">
            <AlertDescription className="text-destructive">
              {requestError}
            </AlertDescription>
          </Alert>
        ) : null}
        <div className={dialogActions}>
          <DialogCancel />
          <Button type="submit" disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting
              ? 'Saving…'
              : season
                ? 'Save Season'
                : 'Create Season'}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
}

export function SeasonResourcesDialog({
  season,
  open: controlledOpen,
  onOpenChange,
}: {
  season: Season;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
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
      title={`Edit Resources for ${season.name}`}
      open={open}
      onOpenChange={requestOpenChange}
      triggerVariant="outline"
      triggerSize="sm"
      trigger={
        controlled ? undefined : (
          <>
            <IconLink size={17} aria-hidden="true" /> Edit Resources
          </>
        )
      }
    >
      <div className={formStack}>
        <Field>
          <FieldLabel htmlFor={`season-resources-only-${season.id}`}>
            Season Resources URL
          </FieldLabel>
          <Input
            id={`season-resources-only-${season.id}`}
            type="url"
            inputMode="url"
            value={resourcesUrl}
            onChange={(event) => setResourcesUrl(event.target.value)}
            aria-invalid={Boolean(error)}
          />
        </Field>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription className="text-destructive">
              {error}
            </AlertDescription>
          </Alert>
        ) : null}
        <div className={dialogActions}>
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || !dirty}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save Resources'}
          </Button>
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
  label: ReactNode;
  error?: string;
  children: ReactNode;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {children}
      <FieldError>{error}</FieldError>
    </Field>
  );
}

export function SeasonLifecycleDialog({
  season,
  action,
  onSaved,
  open: controlledOpen,
  onOpenChange,
}: {
  season: Season;
  action: 'close' | 'reopen';
  onSaved?: (season: Season) => void;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const label = action === 'close' ? 'Close Season' : 'Reopen Season';
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
          ? 'Active members will be marked as completed, and this season will become read-only.'
          : 'Only memberships completed when this season was closed will be reactivated.'
      }
      open={open}
      onOpenChange={requestOpenChange}
      triggerVariant={action === 'close' ? 'destructive' : 'outline'}
      triggerSize="sm"
      trigger={
        controlled ? undefined : (
          <>
            <Icon size={17} aria-hidden="true" /> {label}
          </>
        )
      }
    >
      <div className={formStack}>
        <Field>
          <FieldLabel htmlFor={`season-${action}-reason`}>Reason</FieldLabel>
          <Textarea
            id={`season-${action}-reason`}
            required
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`season-${action}-confirm`}>
            Type <strong>{season.name}</strong> to confirm
          </FieldLabel>
          <Input
            id={`season-${action}-confirm`}
            autoComplete="off"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </Field>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription className="text-destructive">
              {error}
            </AlertDescription>
          </Alert>
        ) : null}
        <div className={dialogActions}>
          <DialogCancel />
          <Button
            variant={action === 'close' ? 'destructive' : 'default'}
            type="button"
            disabled={pending || !reason.trim() || confirmation !== season.name}
            onClick={() => void submit()}
          >
            {pending
              ? `${action === 'close' ? 'Closing' : 'Reopening'}…`
              : label}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

/**
 * A season row's actions. Opening is the only action for most members, so
 * that is a plain arrow; anyone who can change the season gets a menu.
 */
/**
 * The season page's one Edit Season button: a menu of the changes this
 * person may make (details, resources, closing or reopening).
 */
export function SeasonEditMenu({
  season,
  canEditDefinition,
  canEditResources,
  canClose,
  canReopen,
  canDelete = false,
  onDeleted,
}: {
  season: Season;
  canEditDefinition: boolean;
  canEditResources: boolean;
  canClose: boolean;
  canReopen: boolean;
  canDelete?: boolean;
  /** Called once the season has been deleted. */
  onDeleted?: () => void;
}) {
  const [open, setOpen] = useState<
    'details' | 'resources' | 'close' | 'reopen' | 'delete' | null
  >(null);
  const close = (next: boolean) => {
    if (!next) setOpen(null);
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button className="min-w-66 justify-center" />}
        >
          <IconEdit size={17} aria-hidden="true" /> Edit Season
          <IconChevronDown size={16} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {canEditDefinition ? (
            <DropdownMenuItem onClick={() => setOpen('details')}>
              <IconEdit size={17} aria-hidden="true" /> Edit Details
            </DropdownMenuItem>
          ) : null}
          {canEditResources ? (
            <DropdownMenuItem onClick={() => setOpen('resources')}>
              <IconLink size={17} aria-hidden="true" /> Edit Resources
            </DropdownMenuItem>
          ) : null}
          {canReopen ? (
            <DropdownMenuItem onClick={() => setOpen('reopen')}>
              <IconLockOpen size={17} aria-hidden="true" /> Reopen Season
            </DropdownMenuItem>
          ) : null}
          {canClose ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('close')}
            >
              <IconLock size={17} aria-hidden="true" /> Close Season
            </DropdownMenuItem>
          ) : null}
          {canDelete ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('delete')}
            >
              <IconTrash size={17} aria-hidden="true" /> Delete Season
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {open === 'details' ? (
        <SeasonEditorDialog season={season} open onOpenChange={close} />
      ) : null}
      {open === 'resources' ? (
        <SeasonResourcesDialog season={season} open onOpenChange={close} />
      ) : null}
      {open === 'delete' ? (
        <DeleteEmptySeason
          season={season}
          open
          onOpenChange={close}
          onDeleted={onDeleted}
        />
      ) : null}
      {open === 'close' || open === 'reopen' ? (
        <SeasonLifecycleDialog
          season={season}
          action={open}
          open
          onOpenChange={close}
        />
      ) : null}
    </>
  );
}

export function SeasonRowMenu({
  season,
  canEdit = false,
  canClose = false,
  canReopen = false,
  canDelete = false,
}: {
  season: Season;
  canEdit?: boolean;
  canClose?: boolean;
  canReopen?: boolean;
  canDelete?: boolean;
}) {
  const [open, setOpen] = useState<'edit' | 'lifecycle' | 'delete' | null>(
    null,
  );
  const close = (next: boolean) => {
    if (!next) setOpen(null);
  };
  const lifecycle = season.status === 'open' ? 'close' : 'reopen';
  const canChangeLifecycle = lifecycle === 'close' ? canClose : canReopen;
  if (!canEdit && !canChangeLifecycle && !canDelete)
    return (
      <Link
        to={`/seasons/${season.slug}`}
        className="inline-flex size-9 items-center justify-center rounded-md text-foreground hover:bg-muted"
        aria-label={`Open ${season.name}`}
      >
        <IconArrowRight size={18} aria-hidden="true" />
      </Link>
    );
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${season.name}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {canEdit ? (
            <DropdownMenuItem onClick={() => setOpen('edit')}>
              <IconEdit size={17} aria-hidden="true" /> Edit
            </DropdownMenuItem>
          ) : null}
          {canChangeLifecycle ? (
            <DropdownMenuItem
              variant={lifecycle === 'close' ? 'destructive' : undefined}
              onClick={() => setOpen('lifecycle')}
            >
              {lifecycle === 'close' ? (
                <IconLock size={17} aria-hidden="true" />
              ) : (
                <IconLockOpen size={17} aria-hidden="true" />
              )}
              {lifecycle === 'close' ? 'Close' : 'Reopen'}
            </DropdownMenuItem>
          ) : null}
          {canDelete ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('delete')}
            >
              <IconTrash size={17} aria-hidden="true" /> Delete
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {open === 'edit' ? (
        <SeasonEditorDialog season={season} open onOpenChange={close} />
      ) : null}
      {open === 'lifecycle' ? (
        <SeasonLifecycleDialog
          season={season}
          action={lifecycle}
          open
          onOpenChange={close}
        />
      ) : null}
      {open === 'delete' ? (
        <DeleteEmptySeason season={season} open onOpenChange={close} />
      ) : null}
    </>
  );
}
