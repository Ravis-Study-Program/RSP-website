import { zodResolver } from '@hookform/resolvers/zod';
import { Tabs } from '@base-ui/react/tabs';
import { IconChevronDown, IconChevronUp, IconPlus } from '@tabler/icons-react';
import type { ColumnDef, Row } from '@tanstack/react-table';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';

import { demoMode, useCurrentUser, useMockInterviews } from '@/api/queries';
import { AppDatePicker } from '@/components/AppDatePicker';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { RichTextEditor } from '@/components/RichTextEditor';
import { currentPerson, people } from '@/data/demo';
import styles from '@/styles/App.module.css';
import type { MockInterview } from '@/types';
import { formatDateTime, interviewPassed } from '@/utils';

const mockSchema = z.object({
  intervieweeId: z.string().min(1, 'Choose an interviewee.'),
  occurredAt: z.string().min(1, 'Choose a date.'),
  durationMinutes: z.number().int().min(15).max(240),
  technicalTitle: z.string().trim().min(2, 'Describe the technical round.'),
  technicalScore: z.number().int().min(0).max(10),
  technicalNotes: z.string().max(20_000),
  includeBehavioural: z.boolean(),
  behaviouralScore: z.number().int().min(0).max(10),
  behaviouralNotes: z.string().max(20_000),
});
type MockFormValues = z.infer<typeof mockSchema>;

const columns: ColumnDef<MockInterview, any>[] = [
  {
    id: 'expand',
    header: 'Details',
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => <button className={styles.iconButton} type="button" aria-label={`${row.getIsExpanded() ? 'Hide' : 'Show'} details for ${row.original.interviewee.name}`} aria-expanded={row.getIsExpanded()} onClick={row.getToggleExpandedHandler()}>{row.getIsExpanded() ? <IconChevronUp size={18} /> : <IconChevronDown size={18} />}</button>,
  },
  { accessorKey: 'interviewee.name', id: 'interviewee', header: 'Interviewee', cell: ({ row }) => <PersonIdentity person={row.original.interviewee} /> },
  { accessorKey: 'interviewer.name', id: 'interviewer', header: 'Interviewer', cell: ({ row }) => <PersonIdentity person={row.original.interviewer} /> },
  { accessorKey: 'occurredAt', header: 'Date', cell: ({ getValue }) => formatDateTime(getValue()) },
  { accessorKey: 'durationMinutes', header: 'Duration', cell: ({ getValue }) => `${getValue()} min` },
  { id: 'score', header: 'Scores', accessorFn: (row) => row.rounds.reduce((sum, round) => sum + round.score, 0) / row.rounds.length, cell: ({ row }) => <span className={styles.score}>{row.original.rounds.map((round) => round.score).join(' · ')}/10</span> },
  { id: 'result', header: 'Result', accessorFn: interviewPassed, cell: ({ row }) => <span className={interviewPassed(row.original) ? styles.badgeSuccess : styles.badgeDanger}>{interviewPassed(row.original) ? 'Pass' : 'Needs work'}</span> },
  { accessorKey: 'reviewStatus', header: 'Review', cell: ({ getValue }) => <span className={getValue() === 'reviewed' ? styles.badgeSuccess : styles.badgeWarning}>{getValue() === 'reviewed' ? 'Reviewed' : 'Pending'}</span> },
];

type ViewMode = 'received' | 'given' | 'all';

export function MockInterviewsPage() {
  usePageTitle('Mock interviews');
  const query = useMockInterviews();
  const user = useCurrentUser();
  const [mode, setMode] = useState<ViewMode>('received');
  const [created, setCreated] = useState<MockInterview[]>([]);
  const [removedIds, setRemovedIds] = useState<string[]>([]);
  const all = useMemo(() => [...created, ...(query.data?.items ?? [])].filter((mock) => !removedIds.includes(mock.id)), [created, query.data, removedIds]);
  const filtered = useMemo(() => {
    if (mode === 'received') return all.filter((mock) => mock.interviewee.id === user.data?.id);
    if (mode === 'given') return all.filter((mock) => mock.interviewer.id === user.data?.id);
    return all;
  }, [all, mode, user.data?.id]);

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Interview preparation"
        title="Mock interviews"
        description="Keep rounds, scores and feedback together. Interview details are versioned, and only the interviewee controls their review."
        actions={<NewMockDialog onCreated={(mock) => setCreated((items) => [mock, ...items])} />}
      />
      <Tabs.Root value={mode} onValueChange={(value) => setMode(value as ViewMode)}>
        <Tabs.List className={styles.tabsList} aria-label="Mock interview view">
          <Tabs.Tab className={styles.tab} value="received">Received</Tabs.Tab>
          <Tabs.Tab className={styles.tab} value="given">Given</Tabs.Tab>
          <Tabs.Tab className={styles.tab} value="all">All available</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value={mode}>
          <DataTable
            ariaLabel={`${mode} mock interviews`}
            data={filtered}
            columns={columns}
            loading={query.isLoading || user.isLoading}
            error={query.isError || user.isError}
            onRetry={() => void Promise.all([query.refetch(), user.refetch()])}
            emptyTitle={`No ${mode} mock interviews`}
            emptyMessage={mode === 'received' ? 'Interviews where you are the interviewee will appear here.' : mode === 'given' ? 'Interviews you conduct will appear here.' : 'Create the first eligible mock interview.'}
            getRowId={(mock) => mock.id}
            renderExpanded={(row) => <MockDetails row={row} currentUserId={user.data?.id} onDelete={() => setRemovedIds((ids) => [...ids, row.original.id])} />}
            renderCard={(row) => (
              <div>
                <div className={styles.inline}><span className={interviewPassed(row.original) ? styles.badgeSuccess : styles.badgeDanger}>{interviewPassed(row.original) ? 'Pass' : 'Needs work'}</span><span className={row.original.reviewStatus === 'reviewed' ? styles.badgeSuccess : styles.badgeWarning}>{row.original.reviewStatus}</span></div>
                <h3 className={styles.cardTitle} style={{ marginTop: '0.7rem' }}>{row.original.interviewee.name}</h3>
                <p className={styles.helper}>with {row.original.interviewer.name} · {formatDateTime(row.original.occurredAt)}</p>
                <p>{row.original.durationMinutes} minutes · {row.original.rounds.length} {row.original.rounds.length === 1 ? 'round' : 'rounds'}</p>
                <button className={styles.buttonSecondary} type="button" aria-expanded={row.getIsExpanded()} onClick={row.getToggleExpandedHandler()}>View details</button>
                {row.getIsExpanded() ? <div style={{ marginTop: '0.75rem' }}><MockDetails row={row} currentUserId={user.data?.id} onDelete={() => setRemovedIds((ids) => [...ids, row.original.id])} /></div> : null}
              </div>
            )}
          />
        </Tabs.Panel>
      </Tabs.Root>
    </div>
  );
}

function MockDetails({ row, currentUserId, onDelete }: { row: Row<MockInterview>; currentUserId?: string; onDelete: () => void }) {
  const mock = row.original;
  const canEditDetails = mock.interviewer.id === currentUserId;
  const canReview = mock.interviewee.id === currentUserId;
  return (
    <div className={styles.grid2}>
      <div>
        <h3 className={styles.cardTitle}>Rounds and scores</h3>
        <div className={styles.rounds} style={{ marginTop: '0.7rem' }}>
          {mock.rounds.map((round) => <article className={styles.round} key={round.id}><div className={styles.roundHeader}><strong>{round.title}</strong><span className={`${styles.score} ${round.score >= 5 ? styles.pass : styles.fail}`}>{round.score}/10</span></div><span className={styles.badgeNeutral}>{round.type}</span><p>{round.notes}</p></article>)}
        </div>
      </div>
      <div>
        <h3 className={styles.cardTitle}>Interviewee review</h3>
        <p>{mock.reviewComments ?? 'A review has not been submitted.'}</p>
        <div className={styles.buttonRow}>
          {canReview ? <button className={styles.buttonSecondary} type="button">{mock.reviewStatus === 'reviewed' ? 'Edit my review' : 'Add my review'}</button> : null}
          {canEditDetails ? <button className={styles.buttonSecondary} type="button">Edit details</button> : null}
          {canEditDetails ? <NamedConfirmation name={mock.interviewee.name} actionLabel="Delete interview" description="The interview will be soft-deleted. Its immutable version history and audit record will remain." onConfirm={onDelete} /> : null}
        </div>
      </div>
    </div>
  );
}

function NewMockDialog({ onCreated }: { onCreated: (mock: MockInterview) => void }) {
  const [open, setOpen] = useState(false);
  const { register, control, watch, handleSubmit, reset, formState: { errors, isDirty, isSubmitting } } = useForm<MockFormValues>({
    resolver: zodResolver(mockSchema),
    defaultValues: {
      intervieweeId: people.find((person) => person.roles.includes('student'))?.id ?? '',
      occurredAt: new Date().toISOString().slice(0, 10),
      durationMinutes: 60,
      technicalTitle: '',
      technicalScore: 5,
      technicalNotes: '',
      includeBehavioural: true,
      behaviouralScore: 5,
      behaviouralNotes: '',
    },
  });
  const includeBehavioural = watch('includeBehavioural');
  const onSubmit = handleSubmit(async (values) => {
    const interviewee = people.find((person) => person.id === values.intervieweeId) ?? people[0];
    const mock: MockInterview = {
      id: `mock_local_${Date.now()}`,
      interviewee,
      interviewer: currentPerson,
      season: 'Semester 2, 2026',
      occurredAt: `${values.occurredAt}T03:30:00.000Z`,
      durationMinutes: values.durationMinutes,
      rounds: [
        { id: `round_technical_${Date.now()}`, type: 'technical', title: values.technicalTitle, score: values.technicalScore, notes: values.technicalNotes },
        ...(values.includeBehavioural ? [{ id: `round_behavioural_${Date.now()}`, type: 'behavioural' as const, title: 'Behavioural round', score: values.behaviouralScore, notes: values.behaviouralNotes }] : []),
      ],
      reviewStatus: 'pending',
      revision: 1,
    };
    if (!demoMode) {
      // The generated API client replaces this local composition after contract generation.
      await Promise.resolve();
    }
    onCreated(mock);
    reset();
    setOpen(false);
  });
  const requestOpen = (next: boolean) => {
    if (!next && isDirty && !window.confirm('Discard your unsaved interview?')) return;
    if (!next) reset();
    setOpen(next);
  };

  return (
    <FormDialog title="Record mock interview" description="You are recorded as the interviewer. Participant identities cannot be changed after creation without an audited correction." open={open} onOpenChange={requestOpen} trigger={<><IconPlus size={18} aria-hidden="true" /> New interview</>}>
      <form className={styles.form} noValidate onSubmit={onSubmit}>
        <div className={styles.field}>
          <label htmlFor="mock-interviewee">Interviewee</label>
          <select id="mock-interviewee" className={styles.select} {...register('intervieweeId')}>
            {people.filter((person) => person.id !== currentPerson.id && person.status !== 'unassigned').map((person) => <option key={person.id} value={person.id}>{person.name} · {person.roles.join(', ')}</option>)}
          </select>
          {errors.intervieweeId ? <p className={styles.fieldError}>{errors.intervieweeId.message}</p> : null}
        </div>
        <div className={styles.fieldGrid}>
          <Controller control={control} name="occurredAt" render={({ field }) => <AppDatePicker label="Interview date" value={field.value} onChange={field.onChange} error={errors.occurredAt?.message} />} />
          <div className={styles.field}>
            <label htmlFor="mock-duration">Duration (minutes)</label>
            <input id="mock-duration" className={styles.input} type="number" min="15" max="240" {...register('durationMinutes', { valueAsNumber: true })} />
          </div>
        </div>
        <fieldset className={styles.form} style={{ border: 0, padding: 0 }}>
          <legend className={styles.legend}>Technical round</legend>
          <div className={styles.fieldGrid}>
            <div className={styles.field}><label htmlFor="technical-title">Problem or topic</label><input id="technical-title" className={styles.input} aria-invalid={Boolean(errors.technicalTitle)} {...register('technicalTitle')} />{errors.technicalTitle ? <p className={styles.fieldError}>{errors.technicalTitle.message}</p> : null}</div>
            <div className={styles.field}><label htmlFor="technical-score">Score (0–10)</label><input id="technical-score" className={styles.input} type="number" min="0" max="10" {...register('technicalScore', { valueAsNumber: true })} /></div>
          </div>
          <Controller control={control} name="technicalNotes" render={({ field }) => <RichTextEditor id="technical-notes" label="Technical notes" value={field.value} onChange={field.onChange} />} />
        </fieldset>
        <label className={styles.checkLabel}><input type="checkbox" {...register('includeBehavioural')} /> Include a behavioural round</label>
        {includeBehavioural ? (
          <fieldset className={styles.form} style={{ border: 0, padding: 0 }}>
            <legend className={styles.legend}>Behavioural round</legend>
            <div className={styles.field}><label htmlFor="behavioural-score">Score (0–10)</label><input id="behavioural-score" className={styles.input} type="number" min="0" max="10" {...register('behaviouralScore', { valueAsNumber: true })} /></div>
            <Controller control={control} name="behaviouralNotes" render={({ field }) => <RichTextEditor id="behavioural-notes" label="Behavioural notes" value={field.value} onChange={field.onChange} />} />
          </fieldset>
        ) : null}
        <div className={styles.dialogActions}><button className={styles.buttonSecondary} type="button" onClick={() => requestOpen(false)}>Cancel</button><button className={styles.button} type="submit" disabled={isSubmitting}>{isSubmitting ? 'Saving…' : 'Save interview'}</button></div>
      </form>
    </FormDialog>
  );
}
