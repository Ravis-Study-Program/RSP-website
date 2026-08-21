import { zodResolver } from '@hookform/resolvers/zod';
import {
  IconArrowUpRight,
  IconClock,
  IconEdit,
  IconPlus,
  IconX,
} from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useBeforeUnload } from 'react-router-dom';
import { z } from 'zod';

import { adaptAttempt, displayDifficulty, indexProblems } from '@/api/adapters';
import { ApiProblem, apiRequest } from '@/api/client';
import type {
  Attempt as ApiAttempt,
  AttemptMutation,
  LeetcodeProblem,
} from '@/api/generated/models';
import {
  demoMode,
  useAttempts,
  useLeetcodeProblems,
  useRecommendation,
} from '@/api/queries';
import { ActivityChart } from '@/components/ActivityChart';
import { AppDatePicker } from '@/components/AppDatePicker';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { RichTextEditor } from '@/components/RichTextEditor';
import { RichTextContent } from '@/components/RichTextContent';
import { ErrorState, InlineNotice } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import type { Attempt } from '@/types';
import {
  calendarDateKey,
  difficultyGoal,
  formatDate,
  outcomeLabel,
  recentActivitySeries,
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
          <HighlightText text={row.original.problem} />
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
  const attemptsQuery = useAttempts();
  const recommendationQuery = useRecommendation();
  const problemsQuery = useLeetcodeProblems();
  const [localAttempts, setLocalAttempts] = useState<Attempt[]>([]);
  const [updatedAttempts, setUpdatedAttempts] = useState<
    Record<string, Attempt>
  >({});
  const [removedIds, setRemovedIds] = useState<string[]>([]);
  const [operationMessage, setOperationMessage] = useState('');
  const [dismissed, setDismissed] = useState(false);
  const data = useMemo(
    () =>
      [...localAttempts, ...(attemptsQuery.data?.items ?? [])]
        .filter((attempt) => !removedIds.includes(attempt.id))
        .map((attempt) => updatedAttempts[attempt.id] ?? attempt),
    [localAttempts, attemptsQuery.data, removedIds, updatedAttempts],
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
          `/problem-attempts/${encodeURIComponent(attempt.id)}?revision=${attempt.revision}`,
          { method: 'DELETE' },
        );
      setLocalAttempts((items) =>
        items.filter((item) => item.id !== attempt.id),
      );
      setOperationMessage(`${attempt.problem} was deleted.`);
    } catch (error) {
      setRemovedIds((ids) => ids.filter((id) => id !== attempt.id));
      setOperationMessage(
        error instanceof ApiProblem && error.problem.status === 409
          ? 'That attempt changed elsewhere. Refreshing the latest history.'
          : error instanceof Error
            ? error.message
            : 'Unable to delete the attempt.',
      );
      if (error instanceof ApiProblem && error.problem.status === 409)
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
            description="This permanently removes the attempt. Your recommendation may change."
            onConfirm={() => void deleteAttempt(row.original)}
          />
        </div>
      ),
    },
  ];
  const activity = useMemo(() => recentActivitySeries(data), [data]);

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Practice"
        title="Problem practice"
        description="Record what happened, not just whether a problem was completed. Outcomes, confidence and time help your next recommendation stay useful."
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

      <section aria-labelledby="recommendation-title">
        <div className={styles.sectionHeader}>
          <h2 id="recommendation-title" className={styles.sectionTitle}>
            Current recommendation
          </h2>
        </div>
        {recommendationQuery.isLoading ? (
          <div className={styles.recommendation} aria-busy="true">
            <span
              className={`${styles.skeleton} ${styles.skeletonRecommendation}`}
            >
              Loading recommendation
            </span>
          </div>
        ) : recommendationQuery.isError ? (
          <ErrorState onRetry={() => void recommendationQuery.refetch()} />
        ) : dismissed ? (
          <InlineNotice tone="success">
            Recommendation dismissed. This problem will be excluded for 30 days;
            refresh to generate another when you are ready.
          </InlineNotice>
        ) : recommendationQuery.data ? (
          <article className={styles.recommendation}>
            <div className={styles.inline}>
              <span className={styles.badge}>
                {recommendationQuery.data.difficulty}
              </span>
              <span className={styles.badge}>
                {recommendationQuery.data.category}
              </span>
              <span className={styles.badgeNeutral}>
                <IconClock size={15} aria-hidden="true" /> Goal{' '}
                {recommendationQuery.data.estimatedMinutes} min
              </span>
            </div>
            <h3 className={styles.recommendationTitle}>
              {recommendationQuery.data.title}
            </h3>
            <p>{recommendationQuery.data.rationale}</p>
            <div className={styles.buttonRow}>
              <a
                className={styles.button}
                href={recommendationQuery.data.externalUrl}
                target="_blank"
                rel="noreferrer"
              >
                Start on LeetCode{' '}
                <IconArrowUpRight size={18} aria-hidden="true" />
              </a>
              <DismissDialog onDismiss={() => setDismissed(true)} />
            </div>
          </article>
        ) : (
          <InlineNotice>
            Complete an outcome-known attempt to generate a recommendation.
          </InlineNotice>
        )}
      </section>

      <section className={styles.section} aria-labelledby="attempts-title">
        <div className={styles.sectionHeader}>
          <h2 id="attempts-title" className={styles.sectionTitle}>
            Attempt history
          </h2>
          <span className={styles.muted}>{data.length} recorded</span>
        </div>
        <DataTable
          ariaLabel="Problem attempts"
          data={data}
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
                <HighlightText text={row.original.problem} />
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
                  description="This permanently removes the attempt. Your recommendation may change."
                  onConfirm={() => void deleteAttempt(row.original)}
                />
              </div>
            </div>
          )}
        />
      </section>

      <section
        className={styles.section}
        aria-labelledby="practice-trend-title"
      >
        <div className={styles.sectionHeader}>
          <h2 id="practice-trend-title" className={styles.sectionTitle}>
            Practice trend
          </h2>
        </div>
        <div className={styles.panel}>
          <ActivityChart data={activity} />
        </div>
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
      difficulty: displayDifficulty(problem.difficulty),
      category: problem.categories[0] ?? null,
      outcome: values.outcome,
      confidence,
      minutes: values.minutes,
      notes: values.notes,
      attemptedAt,
      revision: (attempt?.revision ?? 0) + 1,
    };
    if (!demoMode) {
      try {
        const mutation: AttemptMutation = {
          ...values,
          confidence,
          attemptedAt,
          ...(attempt ? { revision: attempt.revision } : {}),
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
        if (error instanceof ApiProblem && error.problem.status === 409) {
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
        description="Record enough detail for your progress history and next recommendation."
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
            <select
              id="attempt-problem"
              className={styles.select}
              disabled={loading || problems.length === 0}
              aria-invalid={Boolean(errors.problemId)}
              aria-describedby={
                errors.problemId ? 'attempt-problem-error' : undefined
              }
              {...register('problemId')}
            >
              <option value="">
                {loading ? 'Loading problems…' : 'Choose a problem'}
              </option>
              {problems.map((problem) => (
                <option key={problem.id} value={problem.id}>
                  {problem.number}. {problem.title} ·{' '}
                  {displayDifficulty(problem.difficulty)}
                </option>
              ))}
            </select>
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

function DismissDialog({ onDismiss }: { onDismiss: () => void }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty = reason.length > 0;
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard your recommendation dismissal reason?')
    )
      return;
    if (next) {
      setReason('');
      setError('');
    }
    setOpen(next);
  };
  const dismiss = async () => {
    setPending(true);
    setError('');
    try {
      if (!demoMode)
        await apiRequest<void>('/recommendations/current/dismiss', {
          method: 'POST',
          body: JSON.stringify({ reason: reason || undefined }),
        });
      onDismiss();
      setReason('');
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to dismiss this recommendation.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Dismiss recommendation"
      description="This problem will not be recommended again for 30 days."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        <>
          <IconX size={18} aria-hidden="true" /> Dismiss
        </>
      }
    >
      <div className={styles.field}>
        <label htmlFor="dismiss-reason">Reason (optional)</label>
        <textarea
          id="dismiss-reason"
          className={styles.textarea}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder="For example: already completed elsewhere"
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
          disabled={pending}
          onClick={() => requestOpenChange(false)}
        >
          Cancel
        </button>
        <button
          className={styles.button}
          type="button"
          disabled={pending}
          onClick={() => void dismiss()}
        >
          {pending ? 'Dismissing…' : 'Dismiss for 30 days'}
        </button>
      </div>
    </FormDialog>
  );
}
