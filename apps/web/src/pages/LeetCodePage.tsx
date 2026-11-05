import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import {
  emptyActivityFilters,
  filterPractice,
  useActivityFilters,
} from '@/activityFilters';
import { ProblemSuggestion } from '@/components/ProblemSuggestion';
import { Spoiler } from '@/components/Spoiler';
import { SeasonWeekChoices } from '@/components/ActivityFilters';
import { ActivityPeriodFilter } from '@/components/ActivityPeriodFilter';
import { useActivitySeason, useActivitySeasons } from '@/activityPeriod';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  IconChevronDown,
  IconChevronUp,
  IconDotsVertical,
  IconEdit,
  IconNotes,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useBeforeUnload, useNavigate } from 'react-router-dom';
import { z } from 'zod';

import { adaptAttempt, displayDifficulty } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  Attempt as ApiAttempt,
  AttemptMutation,
  LeetcodeProblem,
} from '@/api/generated/models';
import {
  demoMode,
  demoActivityWeekId,
  useAttempts,
  useCurrentUser,
} from '@/api/queries';
import { PracticeAnalytics } from '@/components/PracticeAnalytics';
import { AppDatePicker } from '@/components/AppDatePicker';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ProblemSelect } from '@/components/ProblemSelect';
import { RichTextEditor } from '@/components/RichTextEditor';
import { RichTextContent } from '@/components/RichTextContent';
import { ErrorState, InlineNotice } from '@/components/StatusViews';
import { cn } from '@/lib/utils';
import { recentDateFilterOptions, withinRecentDays } from '@/tableFilters';
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import type { Attempt, Difficulty } from '@/types';
import {
  calendarDateKey,
  difficultyGoal,
  formatDate,
  outcomeLabel,
  zonedDateTimeToUtc,
} from '@/utils';

const badgeClass = 'rounded-sm font-semibold';
const outcomeTone = {
  success: 'border-transparent bg-success-soft text-success',
  warning: 'border-transparent bg-warning-soft text-warning',
  danger: 'border-transparent bg-destructive-soft text-destructive',
};
const difficultyTone: Record<Difficulty, string> = {
  Easy: outcomeTone.success,
  Medium: outcomeTone.warning,
  Hard: outcomeTone.danger,
};

const attemptSchema = z.object({
  problemId: z.string().min(1, 'Choose a problem.'),
  outcome: z.enum(['independently_solved', 'solved_with_hints', 'not_solved']),
  confidence: z
    .string()
    .refine(
      (value) => value === '' || /^[1-5]$/.test(value),
      'Enter a whole number from 1 to 5, or leave it blank.',
    ),
  minutes: z
    .number()
    .int()
    .positive('Time must be greater than zero.')
    .max(600),
  attemptedAt: z.string().min(1),
  notes: z.string().max(20_000),
});

type AttemptFormValues = z.infer<typeof attemptSchema>;

function attemptFormValues(attempt?: Attempt): AttemptFormValues {
  return attempt
    ? {
        problemId: attempt.problemId,
        outcome: attempt.outcome === 'unknown' ? 'not_solved' : attempt.outcome,
        confidence: attempt.confidence?.toString() ?? '',
        minutes: attempt.minutes ?? 35,
        attemptedAt: attempt.attemptedAt.slice(0, 10),
        notes: attempt.notes,
      }
    : {
        problemId: '',
        outcome: 'independently_solved',
        confidence: '',
        minutes: 35,
        attemptedAt: calendarDateKey(new Date()),
        notes: '',
      };
}

/** Minutes taken, highlighted when within the difficulty's practice goal. */
function AttemptTime({ attempt }: { attempt: Attempt }) {
  return (
    <span
      className={
        attempt.difficulty &&
        attempt.minutes !== null &&
        attempt.minutes <= difficultyGoal(attempt.difficulty)
          ? 'font-semibold text-success'
          : undefined
      }
    >
      {attempt.minutes} min
    </span>
  );
}

const difficultyFilterOptions = (['Easy', 'Medium', 'Hard'] as const).map(
  (value) => ({ label: value, value }),
);
const outcomeFilterOptions = (
  [
    'independently_solved',
    'solved_with_hints',
    'not_solved',
    'unknown',
  ] as const
).map((value) => ({ label: outcomeLabel(value), value }));

const baseColumns: ColumnDef<Attempt, any>[] = [
  {
    accessorKey: 'problem',
    header: 'Problem',
    cell: ({ row }) => (
      <div>
        <strong>
          {row.original.problemUrl ? (
            <a href={row.original.problemUrl} target="_blank" rel="noreferrer">
              <HighlightText text={row.original.problem} />
            </a>
          ) : (
            <HighlightText text={row.original.problem} />
          )}
        </strong>
        <div className="text-sm text-muted-foreground">
          {row.original.category ? (
            <Spoiler>
              <HighlightText text={row.original.category} />
            </Spoiler>
          ) : (
            'Category unavailable'
          )}
        </div>
      </div>
    ),
  },
  {
    accessorKey: 'difficulty',
    header: 'Difficulty',
    meta: { width: 6.5, filterOptions: difficultyFilterOptions },
    filterFn: 'equalsString',
    cell: ({ getValue }) => (
      <Badge
        variant="outline"
        className={cn(badgeClass, difficultyTone[getValue() as Difficulty])}
      >
        {getValue()}
      </Badge>
    ),
  },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    meta: { width: 10.5, filterOptions: outcomeFilterOptions },
    filterFn: 'equalsString',
    cell: ({ getValue }) => (
      <Badge
        variant="outline"
        className={cn(
          badgeClass,
          getValue() === 'independently_solved'
            ? outcomeTone.success
            : getValue() === 'not_solved'
              ? outcomeTone.danger
              : outcomeTone.warning,
        )}
      >
        {outcomeLabel(getValue())}
      </Badge>
    ),
  },
  {
    accessorKey: 'confidence',
    header: 'Confidence',
    meta: { width: 6.5 },
    cell: ({ getValue }) => (getValue() ? `${getValue()}/5` : '—'),
  },
  {
    accessorKey: 'minutes',
    header: 'Time',
    meta: { width: 5.5 },
    cell: ({ row }) =>
      row.original.minutes ? <AttemptTime attempt={row.original} /> : '—',
  },
  {
    accessorKey: 'attemptedAt',
    header: 'Date Attempted',
    meta: { width: 8.5, filterOptions: recentDateFilterOptions },
    filterFn: withinRecentDays,
    cell: ({ getValue }) => formatDate(getValue()),
  },
];

export function LeetCodePage() {
  const navigate = useNavigate();
  usePageTitle('LeetCode');
  const { seasons, season, hasSelection } = useActivitySeason('leetcode');
  const user = useCurrentUser();
  // Practice only belongs to seasons where you were a student.
  const studentSeasons = useActivitySeasons(
    user.data?.id,
    seasons.data?.items ?? [],
    { selectedId: season?.id },
  );
  const [urlFilters, setFilters] = useActivityFilters(season?.id);
  // Only season weeks narrow the table; difficulty and topics are just for
  // suggesting a question, so stale URL parameters must not hide attempts.
  const filters = useMemo(
    () => ({ ...emptyActivityFilters, weeks: urlFilters.weeks }),
    [urlFilters.weeks],
  );
  // Weeks are filtered locally: refetching would remount the table and close
  // its Filters popover while someone is choosing weeks.
  const attemptsQuery = useAttempts(
    !hasSelection || Boolean(season),
    season?.id,
  );
  const [localAttempts, setLocalAttempts] = useState<Attempt[]>([]);
  const [updatedAttempts, setUpdatedAttempts] = useState<
    Record<string, Attempt>
  >({});
  const [removedIds, setRemovedIds] = useState<string[]>([]);
  const [operationMessage, setOperationMessage] = useState('');
  const data = useMemo(
    () =>
      [
        ...new Map(
          [
            ...(attemptsQuery.data?.items ?? []),
            ...localAttempts,
            ...Object.values(updatedAttempts),
          ].map((attempt) => [attempt.id, attempt]),
        ).values(),
      ]
        .sort(
          (left, right) =>
            new Date(right.attemptedAt).getTime() -
            new Date(left.attemptedAt).getTime(),
        )
        .filter((attempt) => !removedIds.includes(attempt.id))
        .map((attempt) => updatedAttempts[attempt.id] ?? attempt)
        .map((attempt) =>
          demoMode
            ? {
                ...attempt,
                weekId: demoActivityWeekId(season?.id, attempt.attemptedAt),
              }
            : attempt,
        )
        .filter(
          (attempt) =>
            !season ||
            (demoMode
              ? attempt.attemptedAt >= season.startsAt &&
                attempt.attemptedAt <= season.endsAt
              : attempt.seasonId === season.id),
        ),
    [localAttempts, attemptsQuery.data, removedIds, updatedAttempts, season],
  );
  const saveAttempt = (attempt: Attempt) => {
    if (attempt.id.startsWith('attempt_local_'))
      setLocalAttempts((items) =>
        items.map((item) => (item.id === attempt.id ? attempt : item)),
      );
    else setUpdatedAttempts((items) => ({ ...items, [attempt.id]: attempt }));
    setOperationMessage(`${attempt.problem} was saved.`);
  };
  const deleteAttempt = async (attempt: Attempt) => {
    setOperationMessage('');
    setRemovedIds((ids) => [...ids, attempt.id]);
    try {
      if (!demoMode && !attempt.id.startsWith('attempt_local_'))
        await apiRequest<void>(
          `/problem-attempts/${encodeURIComponent(attempt.id)}`,
          { method: 'DELETE' },
        );
      setLocalAttempts((items) =>
        items.filter((item) => item.id !== attempt.id),
      );
      setOperationMessage(`${attempt.problem} was deleted.`);
    } catch (error) {
      setRemovedIds((ids) => ids.filter((id) => id !== attempt.id));
      setOperationMessage(
        error instanceof ApiError && error.status === 409
          ? 'That attempt changed elsewhere. Refreshing the latest history.'
          : error instanceof Error
            ? error.message
            : 'Unable to delete the attempt.',
      );
      if (error instanceof ApiError && error.status === 409)
        void attemptsQuery.refetch();
    }
  };
  const columns: ColumnDef<Attempt, any>[] = [
    {
      id: 'expand',
      header: 'Notes',
      meta: { width: 4.5 },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => (
        <Button
          variant="ghost"
          size="icon"
          type="button"
          aria-label={`${row.getIsExpanded() ? 'Hide' : 'Show'} notes for ${row.original.problem}`}
          aria-expanded={row.getIsExpanded()}
          onClick={row.getToggleExpandedHandler()}
        >
          {row.getIsExpanded() ? (
            <IconChevronUp size={18} />
          ) : (
            <IconChevronDown size={18} />
          )}
        </Button>
      ),
    },
    ...baseColumns,
    {
      id: 'actions',
      header: () => <span className="sr-only">Actions</span>,
      meta: { width: 4, align: 'end' },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => (
        <AttemptRowMenu
          attempt={row.original}
          notesExpanded={row.getIsExpanded()}
          onToggleNotes={row.getToggleExpandedHandler()}
          onConflict={() => {
            setOperationMessage(
              'That attempt changed elsewhere. Its latest version has been loaded.',
            );
            return attemptsQuery.refetch();
          }}
          onSaved={saveAttempt}
          onDelete={() => void deleteAttempt(row.original)}
        />
      ),
    },
  ];
  const visibleAttempts = useMemo(
    () => filterPractice(data, filters),
    [data, filters],
  );

  if (seasons.isError || (hasSelection && !seasons.isLoading && !season))
    return (
      <div className="mx-auto w-full max-w-[92rem]">
        <ErrorState
          title="Season unavailable"
          message="The selected season could not be loaded."
          onRetry={() => void seasons.refetch()}
        />
      </div>
    );

  return (
    <div className="mx-auto w-full max-w-[92rem]">
      <PageHeader
        title="LeetCode Practice"
        actions={
          <AttemptDialog
            onSaved={(attempt) => {
              setLocalAttempts((current) => [attempt, ...current]);
              setOperationMessage(`${attempt.problem} was added.`);
            }}
          />
        }
      />
      <div className="flex flex-col gap-6">
        {operationMessage ? (
          <InlineNotice
            tone={
              operationMessage.startsWith('Unable') ||
              operationMessage.includes('changed elsewhere')
                ? 'warning'
                : 'success'
            }
          >
            {operationMessage}
          </InlineNotice>
        ) : null}

        <ProblemSuggestion />
        <section aria-label="Attempt history" className="flex flex-col gap-6">
          <DataTable
            ariaLabel="Problem attempts"
            data={visibleAttempts}
            extraFilters={{
              content: (
                <>
                  <ActivityPeriodFilter
                    label="Season"
                    section="leetcode"
                    season={season}
                    seasons={studentSeasons}
                  />
                  {season && (
                    <SeasonWeekChoices
                      seasonId={season.id}
                      selected={filters.weeks}
                      onChange={(weeks) => setFilters({ weeks })}
                    />
                  )}
                </>
              ),
              activeCount: Number(Boolean(season)) + filters.weeks.length,
              onClear: () => navigate('/leetcode'),
            }}
            renderSummary={(rows) => (
              <PracticeAnalytics
                attempts={rows}
                season={season}
                showActivity={false}
              />
            )}
            columns={columns}
            loading={attemptsQuery.isLoading || seasons.isLoading}
            error={attemptsQuery.isError}
            onRetry={() => void attemptsQuery.refetch()}
            emptyTitle="No attempts recorded"
            getRowId={(attempt) => attempt.id}
            renderExpanded={(row) => (
              <section aria-label={`Notes for ${row.original.problem}`}>
                <RichTextContent html={row.original.notes} />
              </section>
            )}
            renderCard={(row) => (
              <div>
                <div className="flex flex-wrap items-center gap-2.5">
                  <Badge
                    variant="outline"
                    className={cn(
                      badgeClass,
                      row.original.difficulty &&
                        difficultyTone[row.original.difficulty],
                    )}
                  >
                    {row.original.difficulty}
                  </Badge>
                  <Badge
                    variant="outline"
                    className={cn(
                      badgeClass,
                      row.original.outcome === 'independently_solved'
                        ? outcomeTone.success
                        : row.original.outcome === 'not_solved'
                          ? outcomeTone.danger
                          : outcomeTone.warning,
                    )}
                  >
                    {outcomeLabel(row.original.outcome)}
                  </Badge>
                </div>
                <h3 className="mt-3 text-base font-semibold tracking-tight">
                  <a
                    href={row.original.problemUrl}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <HighlightText text={row.original.problem} />
                  </a>
                </h3>
                <p className="text-sm text-muted-foreground">
                  {row.original.category ? (
                    <>
                      <Spoiler>{row.original.category}</Spoiler> ·{' '}
                    </>
                  ) : null}
                  {formatDate(row.original.attemptedAt)}
                </p>
                <p>
                  {row.original.minutes ? (
                    <AttemptTime attempt={row.original} />
                  ) : (
                    'No time recorded'
                  )}{' '}
                  ·{' '}
                  {row.original.confidence
                    ? `confidence ${row.original.confidence}/5`
                    : 'confidence not recorded'}
                </p>
                <RichTextContent html={row.original.notes} />
                <div className="flex flex-wrap items-center gap-2.5">
                  <AttemptDialog
                    attempt={row.original}
                    onConflict={() => {
                      setOperationMessage(
                        'That attempt changed elsewhere. Its latest version has been loaded.',
                      );
                      return attemptsQuery.refetch();
                    }}
                    onSaved={saveAttempt}
                  />
                  <NamedConfirmation
                    name={row.original.problem}
                    actionLabel="Delete attempt"
                    description="This permanently removes the attempt from your practice history."
                    onConfirm={() => void deleteAttempt(row.original)}
                  />
                </div>
              </div>
            )}
          />
        </section>
      </div>
    </div>
  );
}

/** Notes, edit and delete actions for a table row, behind a kebab menu. */
function AttemptRowMenu({
  attempt,
  notesExpanded,
  onToggleNotes,
  onSaved,
  onConflict,
  onDelete,
}: {
  attempt: Attempt;
  notesExpanded: boolean;
  onToggleNotes: () => void;
  onSaved: (attempt: Attempt) => void;
  onConflict: () => Promise<unknown> | void;
  onDelete: () => void;
}) {
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${attempt.problem}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={onToggleNotes}>
            <IconNotes size={17} aria-hidden="true" />
            {notesExpanded ? 'Hide notes' : 'View notes'}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setEditOpen(true)}>
            <IconEdit size={17} aria-hidden="true" /> Edit
          </DropdownMenuItem>
          <DropdownMenuItem
            variant="destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <IconTrash size={17} aria-hidden="true" /> Delete attempt
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <AttemptDialog
        attempt={attempt}
        open={editOpen}
        onOpenChange={setEditOpen}
        onConflict={onConflict}
        onSaved={onSaved}
      />
      <NamedConfirmation
        name={attempt.problem}
        actionLabel="Delete attempt"
        description="This permanently removes the attempt from your practice history."
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onConfirm={onDelete}
      />
    </>
  );
}

const outcomeItems: {
  label: string;
  value: AttemptFormValues['outcome'];
}[] = [
  { label: 'Solved', value: 'independently_solved' },
  { label: 'Solved with hints', value: 'solved_with_hints' },
  { label: 'Not solved', value: 'not_solved' },
];

function AttemptDialog({
  attempt,
  onSaved,
  onConflict,
  open: controlledOpen,
  onOpenChange,
}: {
  attempt?: Attempt;
  onSaved: (attempt: Attempt) => void;
  onConflict?: () => Promise<unknown> | void;
  /** When provided, the dialog is opened externally and renders no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
  const [announcement, setAnnouncement] = useState('');
  const [requestError, setRequestError] = useState('');
  const [selectedProblem, setSelectedProblem] =
    useState<LeetcodeProblem | null>(attempt?.catalogProblem ?? null);
  const {
    register,
    control,
    handleSubmit,
    reset,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<AttemptFormValues>({
    resolver: zodResolver(
      attemptSchema as never,
    ) as Resolver<AttemptFormValues>,
    defaultValues: attemptFormValues(attempt),
  });

  useBeforeUnload((event) => {
    if (isDirty) event.preventDefault();
  });

  const onSubmit = handleSubmit(async (values) => {
    setRequestError('');
    const attemptedAt = zonedDateTimeToUtc(`${values.attemptedAt}T12:00`);
    const confidence =
      values.confidence === '' ? null : Number(values.confidence);
    const problem = selectedProblem;
    if (!problem || problem.id !== values.problemId) return;
    const saved: Attempt = {
      id: attempt?.id ?? `attempt_local_${Date.now()}`,
      problemId: values.problemId,
      catalogProblem: problem,
      problem: problem.title,
      problemUrl: problem.link,
      difficulty: displayDifficulty(problem.difficulty),
      category: problem.categories.join(', ') || null,
      categories: problem.categories,
      outcome: values.outcome,
      confidence,
      minutes: values.minutes,
      notes: values.notes,
      attemptedAt,
    };
    if (!demoMode) {
      try {
        const mutation: AttemptMutation = {
          ...values,
          confidence,
          attemptedAt,
        };
        const apiSaved = await apiRequest<ApiAttempt>(
          attempt
            ? `/problem-attempts/${encodeURIComponent(attempt.id)}`
            : '/problem-attempts',
          {
            method: attempt ? 'PATCH' : 'POST',
            body: JSON.stringify(mutation),
          },
        );
        onSaved(adaptAttempt(apiSaved));
      } catch (error) {
        if (error instanceof ApiError && error.status === 409) {
          await onConflict?.();
          reset();
          setSelectedProblem(attempt?.catalogProblem ?? null);
          setRequestError(
            'This attempt changed elsewhere. Its latest version has been fetched; close and reopen the editor before trying again.',
          );
        } else {
          setRequestError(
            error instanceof Error
              ? error.message
              : 'Unable to save this attempt.',
          );
        }
        return;
      }
    } else onSaved(saved);
    setAnnouncement(
      `${problem.title} was ${attempt ? 'updated' : 'added to your attempt history'}.`,
    );
    reset();
    setOpen(false);
  });

  const requestOpenChange = (next: boolean) => {
    if (!next && isDirty && !window.confirm('Discard your unsaved attempt?'))
      return;
    reset(attemptFormValues(attempt));
    setSelectedProblem(attempt?.catalogProblem ?? null);
    setRequestError('');
    setOpen(next);
  };

  return (
    <>
      <FormDialog
        title={attempt ? `Edit ${attempt.problem}` : 'Log problem attempt'}
        open={open}
        onOpenChange={requestOpenChange}
        trigger={
          controlled ? undefined : attempt ? (
            <>
              <IconEdit size={17} aria-hidden="true" /> Edit
            </>
          ) : (
            <>
              <IconPlus size={18} aria-hidden="true" /> Log attempt
            </>
          )
        }
      >
        <form className="grid gap-4" onSubmit={onSubmit} noValidate>
          {requestError ? (
            <InlineNotice tone="warning">
              <span role="alert">{requestError}</span>
            </InlineNotice>
          ) : null}
          <Field>
            <FieldLabel htmlFor="attempt-problem">Problem</FieldLabel>
            <Controller
              name="problemId"
              control={control}
              render={({ field }) => (
                <ProblemSelect
                  id="attempt-problem"
                  name={field.name}
                  value={selectedProblem}
                  onChange={(problem) => {
                    setSelectedProblem(problem);
                    field.onChange(problem?.id ?? '');
                  }}
                  onBlur={field.onBlur}
                  inputRef={field.ref}
                  invalid={Boolean(errors.problemId)}
                  describedBy={
                    errors.problemId ? 'attempt-problem-error' : undefined
                  }
                />
              )}
            />
            {errors.problemId ? (
              <FieldError id="attempt-problem-error">
                {errors.problemId.message}
              </FieldError>
            ) : null}
          </Field>
          <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
            <Field>
              <FieldLabel htmlFor="attempt-outcome">Outcome</FieldLabel>
              <Controller
                name="outcome"
                control={control}
                render={({ field }) => (
                  <Select
                    items={outcomeItems}
                    name={field.name}
                    value={field.value}
                    onValueChange={(value) => {
                      if (value) field.onChange(value);
                    }}
                  >
                    <SelectTrigger id="attempt-outcome" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {outcomeItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="attempt-confidence">
                Confidence (1–5)
              </FieldLabel>
              <Input
                id="attempt-confidence"
                type="number"
                min="1"
                max="5"
                placeholder="Optional"
                aria-invalid={Boolean(errors.confidence)}
                aria-describedby={
                  errors.confidence ? 'attempt-confidence-error' : undefined
                }
                {...register('confidence')}
              />
              {errors.confidence ? (
                <FieldError id="attempt-confidence-error">
                  {errors.confidence.message}
                </FieldError>
              ) : null}
            </Field>
            <Field>
              <FieldLabel htmlFor="attempt-minutes">
                Time taken (minutes)
              </FieldLabel>
              <Input
                id="attempt-minutes"
                type="number"
                min="1"
                max="600"
                aria-invalid={Boolean(errors.minutes)}
                {...register('minutes', { valueAsNumber: true })}
              />
              {errors.minutes ? (
                <FieldError>{errors.minutes.message}</FieldError>
              ) : null}
            </Field>
            <Controller
              control={control}
              name="attemptedAt"
              render={({ field }) => (
                <AppDatePicker
                  label="Attempt date"
                  value={field.value}
                  onChange={field.onChange}
                  error={errors.attemptedAt?.message}
                />
              )}
            />
          </div>
          <Controller
            control={control}
            name="notes"
            render={({ field }) => (
              <RichTextEditor
                id="attempt-notes"
                label="Notes"
                value={field.value}
                onChange={field.onChange}
                error={errors.notes?.message}
              />
            )}
          />
          <div className="mt-5 flex justify-end gap-2.5">
            <Button
              variant="outline"
              type="button"
              onClick={() => requestOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting
                ? 'Saving…'
                : attempt
                  ? 'Update attempt'
                  : 'Save attempt'}
            </Button>
          </div>
        </form>
      </FormDialog>
      <span className="sr-only" role="status">
        {announcement}
      </span>
    </>
  );
}
