import { useActivityFilters, filterPractice } from '@/activityFilters';
import { PracticeFilters } from '@/components/ActivityFilters';
import { adelaideYear } from '@/activityDates';
import { ActivityYearFilter } from '@/components/ActivityYearFilter';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit, IconPlus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useBeforeUnload, useParams } from 'react-router-dom';
import { z } from 'zod';

import { adaptAttempt, displayDifficulty, indexProblems } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  Attempt as ApiAttempt,
  AttemptMutation,
  LeetcodeProblem,
} from '@/api/generated/models';
import {
  demoMode,
  useAttempts,
  useLeetcodeProblems,
  useSeasons,
} from '@/api/queries';
import { PracticeAnalytics } from '@/components/PracticeAnalytics';
import { AppDatePicker } from '@/components/AppDatePicker';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ProblemSelect } from '@/components/ProblemSelect';
import { RichTextEditor } from '@/components/RichTextEditor';
import { RichTextContent } from '@/components/RichTextContent';
import { InlineNotice } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import type { Attempt } from '@/types';
import {
  calendarDateKey,
  difficultyGoal,
  formatDate,
  outcomeLabel,
  zonedDateTimeToUtc,
} from '@/utils';

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
        <div className={styles.helper}>
          <HighlightText
            text={row.original.category ?? 'Category unavailable'}
          />
        </div>
      </div>
    ),
  },
  {
    accessorKey: 'difficulty',
    header: 'Difficulty',
    cell: ({ getValue }) => <span className={styles.badge}>{getValue()}</span>,
  },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    cell: ({ getValue }) => (
      <span
        className={
          getValue() === 'independently_solved'
            ? styles.badgeSuccess
            : getValue() === 'not_solved'
              ? styles.badgeDanger
              : styles.badgeWarning
        }
      >
        {outcomeLabel(getValue())}
      </span>
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
              ? styles.badgeWarning
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

export function PracticePage() {
  usePageTitle('Practice');
  const { slug } = useParams();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const [year, setYear] = useState<number>();
  const [filters, setFilters] = useActivityFilters();
  const attemptsQuery = useAttempts(
    !slug || Boolean(season),
    season?.id,
    year,
    filters,
  );
  const problemsQuery = useLeetcodeProblems();
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
        .filter(
          (attempt) =>
            (!year || adelaideYear(attempt.attemptedAt) === year) &&
            (!season ||
              (demoMode
                ? attempt.attemptedAt >= season.startsAt &&
                  attempt.attemptedAt <= season.endsAt
                : attempt.seasonId === season.id)),
        ),
    [
      localAttempts,
      attemptsQuery.data,
      removedIds,
      updatedAttempts,
      year,
      season,
    ],
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
        <div className={styles.inline}>
          <button
            className={styles.buttonQuiet}
            type="button"
            aria-expanded={row.getIsExpanded()}
            onClick={row.getToggleExpandedHandler()}
          >
            {row.getIsExpanded() ? 'Hide notes' : 'View notes'}
          </button>
          <AttemptDialog
            attempt={row.original}
            problems={problemsQuery.data?.items ?? []}
            loading={problemsQuery.isLoading}
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

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Practice"
        title="Problem practice"
        description="Record what happened, not just whether a problem was completed. Track outcomes, confidence and time across your practice history."
        actions={
          <AttemptDialog
            problems={problemsQuery.data?.items ?? []}
            loading={problemsQuery.isLoading}
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

      <section className={styles.section} aria-labelledby="attempts-title">
        <div className={styles.sectionHeader}>
          <h2 id="attempts-title" className={styles.sectionTitle}>
            Attempt history
          </h2>
          <span className={styles.muted}>
            {visibleAttempts.length} recorded
          </span>
        </div>
        <ActivityYearFilter year={year} onChange={setYear} />
        <PracticeFilters
          seasonId={season?.id}
          filters={filters}
          onChange={setFilters}
        />
        <DataTable
          ariaLabel="Problem attempts"
          data={visibleAttempts}
          renderSummary={(rows) => (
            <PracticeAnalytics attempts={rows} season={season} year={year} />
          )}
          columns={columns}
          loading={attemptsQuery.isLoading}
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
              <div className={styles.inline}>
                <span className={styles.badge}>{row.original.difficulty}</span>
                <span
                  className={
                    row.original.outcome === 'independently_solved'
                      ? styles.badgeSuccess
                      : row.original.outcome === 'not_solved'
                        ? styles.badgeDanger
                        : styles.badgeWarning
                  }
                >
                  {outcomeLabel(row.original.outcome)}
                </span>
              </div>
              <h3 className={`${styles.cardTitle} ${styles.cardTitleSpaced}`}>
                <a
                  href={row.original.problemUrl}
                  target="_blank"
                  rel="noreferrer"
                >
                  <HighlightText text={row.original.problem} />
                </a>
              </h3>
              <p className={styles.helper}>
                {row.original.category} · {formatDate(row.original.attemptedAt)}
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
              <div className={styles.buttonRow}>
                <AttemptDialog
                  attempt={row.original}
                  problems={problemsQuery.data?.items ?? []}
                  loading={problemsQuery.isLoading}
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

function AttemptDialog({
  attempt,
  problems,
  loading,
  onSaved,
  onConflict,
}: {
  attempt?: Attempt;
  problems: LeetcodeProblem[];
  loading: boolean;
  onSaved: (attempt: Attempt) => void;
  onConflict?: () => Promise<unknown> | void;
}) {
  const [open, setOpen] = useState(false);
  const [announcement, setAnnouncement] = useState('');
  const [requestError, setRequestError] = useState('');
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
    const problem = problems.find((item) => item.id === values.problemId);
    if (!problem) return;
    const saved: Attempt = {
      id: attempt?.id ?? `attempt_local_${Date.now()}`,
      problemId: values.problemId,
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
        onSaved(adaptAttempt(apiSaved, indexProblems(problems)));
      } catch (error) {
        if (error instanceof ApiError && error.status === 409) {
          await onConflict?.();
          reset();
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
    if (next) {
      reset(attemptFormValues(attempt));
      setRequestError('');
    } else {
      reset(attemptFormValues(attempt));
      setRequestError('');
    }
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
        <form className={styles.form} onSubmit={onSubmit} noValidate>
          {requestError ? (
            <InlineNotice tone="warning">
              <span role="alert">{requestError}</span>
            </InlineNotice>
          ) : null}
          <div className={styles.field}>
            <label htmlFor="attempt-problem">Problem</label>
            <Controller
              name="problemId"
              control={control}
              render={({ field }) => (
                <ProblemSelect
                  id="attempt-problem"
                  name={field.name}
                  problems={problems}
                  value={field.value}
                  onChange={field.onChange}
                  onBlur={field.onBlur}
                  inputRef={field.ref}
                  disabled={loading || problems.length === 0}
                  invalid={Boolean(errors.problemId)}
                  describedBy={
                    errors.problemId ? 'attempt-problem-error' : undefined
                  }
                />
              )}
            />
            {errors.problemId ? (
              <p id="attempt-problem-error" className={styles.fieldError}>
                {errors.problemId.message}
              </p>
            ) : null}
          </div>
          <div className={styles.fieldGrid}>
            <div className={styles.field}>
              <label htmlFor="attempt-outcome">Outcome</label>
              <select
                id="attempt-outcome"
                className={styles.select}
                {...register('outcome')}
              >
                <option value="independently_solved">
                  Independently solved
                </option>
                <option value="solved_with_hints">Solved with hints</option>
                <option value="not_solved">Not solved</option>
              </select>
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-confidence">
                Confidence (optional, 1–5)
              </label>
              <input
                id="attempt-confidence"
                className={styles.input}
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
                <p id="attempt-confidence-error" className={styles.fieldError}>
                  {errors.confidence.message}
                </p>
              ) : null}
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-minutes">Time taken (minutes)</label>
              <input
                id="attempt-minutes"
                className={styles.input}
                type="number"
                min="1"
                max="600"
                aria-invalid={Boolean(errors.minutes)}
                {...register('minutes', { valueAsNumber: true })}
              />
              {errors.minutes ? (
                <p className={styles.fieldError}>{errors.minutes.message}</p>
              ) : null}
            </div>
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
              type="submit"
              disabled={isSubmitting}
            >
              {isSubmitting
                ? 'Saving…'
                : attempt
                  ? 'Update attempt'
                  : 'Save attempt'}
            </button>
          </div>
        </form>
      </FormDialog>
      <span className={styles.visuallyHidden} role="status">
        {announcement}
      </span>
    </>
  );
}
