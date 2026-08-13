import { zodResolver } from '@hookform/resolvers/zod';
import { IconArrowUpRight, IconBulb, IconClock, IconPlus, IconX } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { Link, useBeforeUnload } from 'react-router-dom';
import { z } from 'zod';

import { apiRequest } from '@/api/client';
import { demoMode, useAttempts, useRecommendation } from '@/api/queries';
import { ActivityChart } from '@/components/ActivityChart';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import { RichTextEditor } from '@/components/RichTextEditor';
import { ErrorState, InlineNotice } from '@/components/StatusViews';
import { weeklyActivity } from '@/data/demo';
import styles from '@/styles/App.module.css';
import type { Attempt } from '@/types';
import { difficultyGoal, formatDate, outcomeLabel } from '@/utils';

const attemptSchema = z.object({
  problem: z.string().trim().min(2, 'Enter the problem name.'),
  difficulty: z.enum(['Easy', 'Medium', 'Hard']),
  category: z.string().trim().min(2, 'Enter a category.'),
  outcome: z.enum(['independently_solved', 'solved_with_hints', 'not_solved']),
  confidence: z.number().int().min(1).max(5),
  minutes: z.number().int().positive('Time must be greater than zero.').max(600),
  attemptedAt: z.string().min(1),
  notes: z.string().max(20_000),
});

type AttemptFormValues = z.infer<typeof attemptSchema>;

const columns: ColumnDef<Attempt, any>[] = [
  { accessorKey: 'problem', header: 'Problem', cell: ({ row }) => <div><strong><HighlightText text={row.original.problem} /></strong><div className={styles.helper}><HighlightText text={row.original.category} /></div></div> },
  { accessorKey: 'difficulty', header: 'Difficulty', cell: ({ getValue }) => <span className={styles.badge}>{getValue()}</span> },
  { accessorKey: 'outcome', header: 'Outcome', cell: ({ getValue }) => <span className={getValue() === 'independently_solved' ? styles.badgeSuccess : getValue() === 'not_solved' ? styles.badgeDanger : styles.badgeWarning}>{outcomeLabel(getValue())}</span> },
  { accessorKey: 'confidence', header: 'Confidence', cell: ({ getValue }) => getValue() ? `${getValue()}/5` : '—' },
  { accessorKey: 'minutes', header: 'Time', cell: ({ row }) => row.original.minutes ? <span className={row.original.minutes > difficultyGoal(row.original.difficulty) ? styles.badgeWarning : undefined}>{row.original.minutes} min</span> : '—' },
  { accessorKey: 'attemptedAt', header: 'Attempted', cell: ({ getValue }) => formatDate(getValue()) },
];

export function PracticePage() {
  usePageTitle('Practice');
  const attemptsQuery = useAttempts();
  const recommendationQuery = useRecommendation();
  const [localAttempts, setLocalAttempts] = useState<Attempt[]>([]);
  const [dismissed, setDismissed] = useState(false);
  const data = useMemo(() => [...localAttempts, ...(attemptsQuery.data?.items ?? [])], [localAttempts, attemptsQuery.data]);

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Practice"
        title="Problem practice"
        description="Record what happened, not just whether a problem was completed. Outcomes, confidence and time help your next recommendation stay useful."
        actions={<AttemptDialog onCreated={(attempt) => setLocalAttempts((current) => [attempt, ...current])} />}
      />

      <section aria-labelledby="recommendation-title">
        <div className={styles.sectionHeader}><h2 id="recommendation-title" className={styles.sectionTitle}>Current recommendation</h2></div>
        {recommendationQuery.isLoading ? <div className={styles.recommendation} aria-busy="true"><span className={styles.skeleton} style={{ height: '8rem' }}>Loading recommendation</span></div> : recommendationQuery.isError ? <ErrorState onRetry={() => void recommendationQuery.refetch()} /> : dismissed ? (
          <InlineNotice tone="success">Recommendation dismissed. This problem will be excluded for 30 days; refresh to generate another when you are ready.</InlineNotice>
        ) : recommendationQuery.data ? (
          <article className={styles.recommendation}>
            <div className={styles.inline}>
              <span className={styles.badge}>{recommendationQuery.data.difficulty}</span>
              <span className={styles.badge}>{recommendationQuery.data.category}</span>
              <span className={styles.badgeNeutral}><IconClock size={15} aria-hidden="true" /> Goal {recommendationQuery.data.estimatedMinutes} min</span>
            </div>
            <h3 className={styles.recommendationTitle}>{recommendationQuery.data.title}</h3>
            <p>{recommendationQuery.data.rationale}</p>
            <div className={styles.buttonRow}>
              <a className={styles.button} href={recommendationQuery.data.externalUrl} target="_blank" rel="noreferrer">
                Start on LeetCode <IconArrowUpRight size={18} aria-hidden="true" />
              </a>
              <DismissDialog onDismiss={() => setDismissed(true)} />
            </div>
          </article>
        ) : <InlineNotice>Complete an outcome-known attempt to generate a recommendation.</InlineNotice>}
      </section>

      <section className={styles.section} aria-labelledby="attempts-title">
        <div className={styles.sectionHeader}><h2 id="attempts-title" className={styles.sectionTitle}>Attempt history</h2><span className={styles.muted}>{data.length} recorded</span></div>
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
          renderCard={(row) => (
            <div>
              <div className={styles.inline}><span className={styles.badge}>{row.original.difficulty}</span><span className={row.original.outcome === 'independently_solved' ? styles.badgeSuccess : row.original.outcome === 'not_solved' ? styles.badgeDanger : styles.badgeWarning}>{outcomeLabel(row.original.outcome)}</span></div>
              <h3 className={styles.cardTitle} style={{ marginTop: '0.65rem' }}><HighlightText text={row.original.problem} /></h3>
              <p className={styles.helper}>{row.original.category} · {formatDate(row.original.attemptedAt)}</p>
              <p>{row.original.minutes ? `${row.original.minutes} min` : 'No time recorded'} · {row.original.confidence ? `confidence ${row.original.confidence}/5` : 'confidence not recorded'}</p>
            </div>
          )}
        />
      </section>

      <section className={styles.section} aria-labelledby="practice-trend-title">
        <div className={styles.sectionHeader}><h2 id="practice-trend-title" className={styles.sectionTitle}>Practice trend</h2></div>
        <div className={styles.panel}><ActivityChart data={weeklyActivity} /></div>
      </section>
    </div>
  );
}

function AttemptDialog({ onCreated }: { onCreated: (attempt: Attempt) => void }) {
  const [open, setOpen] = useState(false);
  const [announcement, setAnnouncement] = useState('');
  const { register, control, handleSubmit, reset, formState: { errors, isDirty, isSubmitting } } = useForm<AttemptFormValues>({
    resolver: zodResolver(attemptSchema),
    defaultValues: {
      problem: '',
      difficulty: 'Medium',
      category: '',
      outcome: 'independently_solved',
      confidence: 3,
      minutes: 35,
      attemptedAt: new Date().toISOString().slice(0, 10),
      notes: '',
    },
  });

  useBeforeUnload((event) => {
    if (isDirty) event.preventDefault();
  });

  const onSubmit = handleSubmit(async (values) => {
    const attemptedAt = `${values.attemptedAt}T00:00:00.000Z`;
    const created: Attempt = {
      id: `attempt_local_${Date.now()}`,
      ...values,
      attemptedAt,
      revision: 1,
    };
    if (!demoMode) {
      const apiCreated = await apiRequest<Attempt>('/problem-attempts', { method: 'POST', body: JSON.stringify({ ...values, attemptedAt, revision: 0 }) });
      onCreated(apiCreated);
    } else onCreated(created);
    setAnnouncement(`${values.problem} was added to your attempt history.`);
    reset();
    setOpen(false);
  });

  const requestOpenChange = (next: boolean) => {
    if (!next && isDirty && !window.confirm('Discard your unsaved attempt?')) return;
    if (!next) reset();
    setOpen(next);
  };

  return (
    <>
      <FormDialog
        title="Log problem attempt"
        description="Record enough detail for your progress history and next recommendation."
        open={open}
        onOpenChange={requestOpenChange}
        trigger={<><IconPlus size={18} aria-hidden="true" /> Log attempt</>}
      >
        <form className={styles.form} onSubmit={onSubmit} noValidate>
          <div className={styles.field}>
            <label htmlFor="attempt-problem">Problem</label>
            <input id="attempt-problem" className={styles.input} aria-invalid={Boolean(errors.problem)} aria-describedby={errors.problem ? 'attempt-problem-error' : undefined} {...register('problem')} />
            {errors.problem ? <p id="attempt-problem-error" className={styles.fieldError}>{errors.problem.message}</p> : null}
          </div>
          <div className={styles.fieldGrid}>
            <div className={styles.field}>
              <label htmlFor="attempt-difficulty">Difficulty</label>
              <select id="attempt-difficulty" className={styles.select} {...register('difficulty')}><option>Easy</option><option>Medium</option><option>Hard</option></select>
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-category">Category</label>
              <input id="attempt-category" className={styles.input} aria-invalid={Boolean(errors.category)} {...register('category')} />
              {errors.category ? <p className={styles.fieldError}>{errors.category.message}</p> : null}
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-outcome">Outcome</label>
              <select id="attempt-outcome" className={styles.select} {...register('outcome')}>
                <option value="independently_solved">Independently solved</option>
                <option value="solved_with_hints">Solved with hints</option>
                <option value="not_solved">Not solved</option>
              </select>
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-confidence">Confidence (1–5)</label>
              <input id="attempt-confidence" className={styles.input} type="number" min="1" max="5" {...register('confidence', { valueAsNumber: true })} />
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-minutes">Time taken (minutes)</label>
              <input id="attempt-minutes" className={styles.input} type="number" min="1" max="600" aria-invalid={Boolean(errors.minutes)} {...register('minutes', { valueAsNumber: true })} />
              {errors.minutes ? <p className={styles.fieldError}>{errors.minutes.message}</p> : null}
            </div>
            <div className={styles.field}>
              <label htmlFor="attempt-date">Attempt date</label>
              <input id="attempt-date" className={styles.input} type="date" {...register('attemptedAt')} />
            </div>
          </div>
          <Controller control={control} name="notes" render={({ field }) => <RichTextEditor id="attempt-notes" label="Notes" value={field.value} onChange={field.onChange} error={errors.notes?.message} />} />
          <div className={styles.dialogActions}>
            <button className={styles.buttonSecondary} type="button" onClick={() => requestOpenChange(false)}>Cancel</button>
            <button className={styles.button} type="submit" disabled={isSubmitting}>{isSubmitting ? 'Saving…' : 'Save attempt'}</button>
          </div>
        </form>
      </FormDialog>
      <span className={styles.visuallyHidden} role="status">{announcement}</span>
    </>
  );
}

function DismissDialog({ onDismiss }: { onDismiss: () => void }) {
  const [reason, setReason] = useState('');
  return (
    <FormDialog title="Dismiss recommendation" description="This problem will not be recommended again for 30 days." trigger={<><IconX size={18} aria-hidden="true" /> Dismiss</>}>
      <div className={styles.field}>
        <label htmlFor="dismiss-reason">Reason (optional)</label>
        <textarea id="dismiss-reason" className={styles.textarea} value={reason} onChange={(event) => setReason(event.target.value)} placeholder="For example: already completed elsewhere" />
      </div>
      <div className={styles.dialogActions}><button className={styles.button} type="button" onClick={onDismiss}>Dismiss for 30 days</button></div>
    </FormDialog>
  );
}
