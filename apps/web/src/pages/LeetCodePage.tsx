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
import { Input } from '@/components/ui/input';
import { useActivityFilters, filterPractice } from '@/activityFilters';
import { ProblemSuggestion } from '@/components/ProblemSuggestion';
import { Spoiler } from '@/components/Spoiler';
import { PracticeFilters } from '@/components/ActivityFilters';
import { ActivityPeriodFilter } from '@/components/ActivityPeriodFilter';
import { useActivitySeason } from '@/activityPeriod';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit, IconPlus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useBeforeUnload } from 'react-router-dom';
import { z } from 'zod';

import { adaptAttempt, displayDifficulty } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  Attempt as ApiAttempt,
  AttemptMutation,
  LeetcodeProblem,
} from '@/api/generated/models';
import { demoMode, demoActivityWeekId, useAttempts } from '@/api/queries';
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
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import type { Attempt } from '@/types';
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
    cell: ({ getValue }) => (
      <Badge variant="secondary" className={badgeClass}>
        {getValue()}
      </Badge>
    ),
  },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
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
    cell: ({ getValue }) => (getValue() ? `${getValue()}/5` : '—'),
  },
  {
    accessorKey: 'minutes',
    header: 'Time',
    cell: ({ row }) =>
      row.original.minutes ? (
        <span
          className={
            row.original.difficulty &&
            row.original.minutes > difficultyGoal(row.original.difficulty)
              ? 'font-semibold text-warning'
              : undefined
          }
        >
          {row.original.minutes} min
        </span>
      ) : (
        '—'
      ),
  },
  {
    accessorKey: 'attemptedAt',
    header: 'Attempted',
    cell: ({ getValue }) => formatDate(getValue()),
  },
];

export function LeetCodePage() {
  usePageTitle('LeetCode');
  const { seasons, season, hasSelection } = useActivitySeason();
  const [filters, setFilters] = useActivityFilters(season?.id);
  const attemptsQuery = useAttempts(
    !hasSelection || Boolean(season),
    season?.id,
    filters,
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
    ...baseColumns,
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => (
        <div className="flex flex-wrap items-center gap-2.5">
          <Button
            variant="ghost"
            type="button"
            aria-expanded={row.getIsExpanded()}
            onClick={row.getToggleExpandedHandler()}
          >
            {row.getIsExpanded() ? 'Hide notes' : 'View notes'}
          </Button>
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
        eyebrow="LeetCode"
        title="LeetCode practice"
        description="Record what happened, not just whether a problem was completed. Track outcomes, confidence and time across your practice history."
        actions={
          <AttemptDialog
            onSaved={(attempt) => {
              setLocalAttempts((current) => [attempt, ...current]);
              setOperationMessage(`${attempt.problem} was added.`);
            }}
          />
        }
      />
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

      <section className="mt-6" aria-labelledby="attempts-title">
        <div className="mb-3 flex items-end justify-between gap-4">
          <h2
            id="attempts-title"
            className="text-xl font-semibold tracking-tight"
          >
            Attempt history
          </h2>
          <span className="text-sm text-muted-foreground">
            {visibleAttempts.length} recorded
          </span>
        </div>
        <ActivityPeriodFilter
          section="leetcode"
          season={season}
          seasons={seasons.data?.items ?? []}
        />
        <PracticeFilters
          seasonId={season?.id}
          filters={filters}
          onChange={setFilters}
        />
        <ProblemSuggestion filters={filters} />
        <DataTable
          ariaLabel="Problem attempts"
          data={visibleAttempts}
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
          emptyMessage="Log your first problem attempt to begin building a useful practice history."
          getRowId={(attempt) => attempt.id}
          renderExpanded={(row) => (
            <section aria-label={`Notes for ${row.original.problem}`}>
              <RichTextContent html={row.original.notes} />
            </section>
          )}
          renderCard={(row) => (
            <div>
              <div className="flex flex-wrap items-center gap-2.5">
                <Badge variant="secondary" className={badgeClass}>
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
                {row.original.minutes
                  ? `${row.original.minutes} min`
                  : 'No time recorded'}{' '}
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
  );
}

const outcomeItems: {
  label: string;
  value: AttemptFormValues['outcome'];
}[] = [
  { label: 'Independently solved', value: 'independently_solved' },
  { label: 'Solved with hints', value: 'solved_with_hints' },
  { label: 'Not solved', value: 'not_solved' },
];

function AttemptDialog({
  attempt,
  onSaved,
  onConflict,
}: {
  attempt?: Attempt;
  onSaved: (attempt: Attempt) => void;
  onConflict?: () => Promise<unknown> | void;
}) {
  const [open, setOpen] = useState(false);
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
        description="Record enough detail to make your progress history useful."
        open={open}
        onOpenChange={requestOpenChange}
        trigger={
          attempt ? (
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
                Confidence (optional, 1–5)
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
