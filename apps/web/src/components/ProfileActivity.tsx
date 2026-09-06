import { Tabs } from '@base-ui/react/tabs';
import { useMemo, useState } from 'react';
import {
  filterPractice,
  filterMocks,
  useActivityFilters,
} from '@/activityFilters';
import { PracticeFilters, MockFilters } from '@/components/ActivityFilters';
import type { ColumnDef } from '@tanstack/react-table';
import { useMockInterviews, useUserAttempts, useSeasons } from '@/api/queries';
import { PracticeAnalytics } from '@/components/PracticeAnalytics';
import { ActivityChart } from '@/components/ActivityChart';
import { activitySeries } from '@/activityAnalytics';
import { DataTable } from '@/components/DataTable';
import { MockFeedback } from '@/components/MockFeedback';
import { PersonIdentity } from '@/components/Common';
import { RichTextContent } from '@/components/RichTextContent';
import type { Attempt, MockInterview, Person } from '@/types';
import { formatDateTime, interviewPassed } from '@/utils';
import styles from '@/styles/App.module.css';

const practiceColumns: ColumnDef<Attempt, any>[] = [
  {
    accessorKey: 'problem',
    header: 'Problem',
    cell: ({ row }) =>
      row.original.problemUrl ? (
        <a href={row.original.problemUrl} target="_blank" rel="noreferrer">
          {row.original.problem}
        </a>
      ) : (
        row.original.problem
      ),
  },
  { accessorKey: 'difficulty', header: 'Difficulty' },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    cell: ({ getValue }) => String(getValue()).replaceAll('_', ' '),
  },
  { accessorKey: 'minutes', header: 'Minutes' },
  {
    accessorKey: 'confidence',
    header: 'Confidence',
    cell: ({ getValue }) => (getValue() ? `${getValue()}/5` : '—'),
  },
  {
    accessorKey: 'attemptedAt',
    header: 'Recorded',
    cell: ({ getValue }) => formatDateTime(getValue()),
  },
  {
    accessorKey: 'notes',
    header: 'Notes',
    cell: ({ getValue }) => <RichTextContent html={getValue()} />,
  },
];

export function ProfileActivity({
  person,
  seasonId,
}: {
  person: Person;
  seasonId?: string;
}) {
  const seasons = useSeasons();
  const season = seasons.data?.items.find((s) => s.id === seasonId);
  const [filters, setFilters] = useActivityFilters();
  const attempts = useUserAttempts(person.id, true, seasonId, filters);
  const mocks = useMockInterviews(true, seasonId, undefined, person.id);
  const visibleAttempts = useMemo(
    () => filterPractice(attempts.data?.items ?? [], filters),
    [attempts.data, filters],
  );
  const visibleMocks = useMemo(
    () => filterMocks(mocks.data?.items ?? [], filters),
    [mocks.data, filters],
  );
  const interviewers = [
    ...new Map(
      (mocks.data?.items ?? []).map((m) => [m.interviewer.id, m.interviewer]),
    ).values(),
  ];
  const [tab, setTab] = useState('practice');
  const mockColumns: ColumnDef<MockInterview, any>[] = [
    {
      accessorKey: 'occurredAt',
      header: 'Recorded',
      cell: ({ getValue }) => formatDateTime(getValue()),
    },
    {
      accessorKey: 'interviewee.name',
      header: 'Interviewee',
      cell: ({ row }) => (
        <PersonIdentity person={row.original.interviewee} seasonId={seasonId} />
      ),
    },
    {
      accessorKey: 'interviewer.name',
      header: 'Interviewer',
      cell: ({ row }) => (
        <PersonIdentity person={row.original.interviewer} seasonId={seasonId} />
      ),
    },
    { accessorKey: 'durationMinutes', header: 'Minutes' },
    {
      id: 'result',
      header: 'Result',
      accessorFn: (mock) => (interviewPassed(mock) ? 'Pass' : 'Needs work'),
    },
    {
      id: 'feedback',
      header: 'Feedback',
      cell: ({ row }) => (
        <details>
          <summary>View feedback</summary>
          <MockFeedback mock={row.original} />
        </details>
      ),
    },
  ];
  return (
    <Tabs.Root value={tab} onValueChange={(value) => setTab(String(value))}>
      <Tabs.List className={styles.tabsList} aria-label="Profile activity">
        <Tabs.Tab value="practice" className={styles.tab}>
          Practice
        </Tabs.Tab>
        <Tabs.Tab value="mocks" className={styles.tab}>
          Mock interviews
        </Tabs.Tab>
      </Tabs.List>
      <Tabs.Panel value="practice">
        <PracticeFilters
          seasonId={seasonId}
          filters={filters}
          onChange={setFilters}
        />
        <DataTable
          ariaLabel={`${person.name} practice history`}
          data={visibleAttempts}
          renderSummary={(rows) => (
            <PracticeAnalytics attempts={rows} season={season} />
          )}
          columns={practiceColumns}
          loading={attempts.isLoading}
          error={attempts.isError}
          onRetry={() => void attempts.refetch()}
          emptyTitle="No practice in this period"
          emptyMessage="Recorded practice will appear here."
          getRowId={(a) => a.id}
          renderCard={({ original: a }) => (
            <div>
              <h3>
                {a.problemUrl ? (
                  <a href={a.problemUrl} target="_blank" rel="noreferrer">
                    {a.problem}
                  </a>
                ) : (
                  a.problem
                )}
              </h3>
              <p>
                {a.difficulty} · {a.minutes ?? '—'} min ·{' '}
                {formatDateTime(a.attemptedAt)}
              </p>
              <p>
                {a.outcome.replaceAll('_', ' ')} · Confidence:{' '}
                {a.confidence ? `${a.confidence}/5` : 'Not recorded'}
              </p>
              <RichTextContent html={a.notes} />
            </div>
          )}
        />
      </Tabs.Panel>
      <Tabs.Panel value="mocks">
        <MockFilters
          people={interviewers}
          filters={filters}
          onChange={setFilters}
        />
        <DataTable
          ariaLabel={`${person.name} mock history`}
          data={visibleMocks}
          renderSummary={(rows) => (
            <ActivityChart data={activitySeries([], rows, { season })} />
          )}
          columns={mockColumns}
          loading={mocks.isLoading}
          error={mocks.isError}
          onRetry={() => void mocks.refetch()}
          emptyTitle="No mocks in this period"
          emptyMessage="Received and conducted interviews will appear here."
          getRowId={(m) => m.id}
          renderCard={({ original: m }) => (
            <div>
              <PersonIdentity person={m.interviewee} seasonId={seasonId} />
              <p>Interviewer</p>
              <PersonIdentity person={m.interviewer} seasonId={seasonId} />
              <p>
                {formatDateTime(m.occurredAt)} · {m.durationMinutes} min
              </p>
              <details>
                <summary>View feedback</summary>
                <MockFeedback mock={m} />
              </details>
            </div>
          )}
        />
      </Tabs.Panel>
    </Tabs.Root>
  );
}
