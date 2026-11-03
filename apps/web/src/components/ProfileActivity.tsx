import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useMemo } from 'react';
import { filterPractice, useActivityFilters } from '@/activityFilters';
import { PracticeFilters, MockFilters } from '@/components/ActivityFilters';
import type { ColumnDef } from '@tanstack/react-table';
import { adaptMockParticipant } from '@/api/adapters';
import {
  useMockInterviewPages,
  useMockInterviewSummary,
  useUserAttempts,
  useSeasons,
} from '@/api/queries';
import { PracticeAnalytics } from '@/components/PracticeAnalytics';
import { ActivityChart } from '@/components/ActivityChart';
import { activitySeries } from '@/activityAnalytics';
import { DataTable } from '@/components/DataTable';
import { MockFeedback } from '@/components/MockFeedback';
import { PersonIdentity } from '@/components/Common';
import {
  RichTextContent,
  sanitizeRichTextForDisplay,
} from '@/components/RichTextContent';
import type { Attempt, MockInterview, Person } from '@/types';
import { formatDateTime, interviewPassed } from '@/utils';

/** Sanitised notes flattened to one line, so table rows stay one height. */
function richTextPreview(html: string) {
  const sanitized = sanitizeRichTextForDisplay(html);
  if (!sanitized) return '';
  const root = new DOMParser().parseFromString(sanitized, 'text/html').body;
  root
    .querySelectorAll('p, li, blockquote, h1, h2, h3, h4, br')
    .forEach((block) => block.append(' '));
  return (root.textContent ?? '').replace(/\s+/g, ' ').trim();
}

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
  { accessorKey: 'difficulty', header: 'Difficulty', meta: { width: 6.5 } },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    meta: { width: 10.5 },
    cell: ({ getValue }) => String(getValue()).replaceAll('_', ' '),
  },
  { accessorKey: 'minutes', header: 'Minutes', meta: { width: 6.5 } },
  {
    accessorKey: 'confidence',
    header: 'Confidence',
    meta: { width: 7 },
    cell: ({ getValue }) => (getValue() ? `${getValue()}/5` : '—'),
  },
  {
    accessorKey: 'attemptedAt',
    header: 'Recorded',
    meta: { width: 11.5 },
    cell: ({ getValue }) => formatDateTime(getValue()),
  },
  {
    accessorKey: 'notes',
    header: 'Notes',
    cell: ({ getValue }) => {
      const preview = richTextPreview(getValue());
      return preview ? (
        <p title={preview}>{preview}</p>
      ) : (
        <span className="text-muted-foreground">No notes recorded.</span>
      );
    },
  },
];

export function ProfileActivity({
  person,
  seasonId,
  tab,
  onTabChange,
}: {
  person: Person;
  seasonId?: string;
  tab: string;
  onTabChange: (tab: string) => void;
}) {
  const seasons = useSeasons();
  const season = seasons.data?.items.find((s) => s.id === seasonId);
  const [filters, setFilters] = useActivityFilters(seasonId);
  // Each tab's data loads only once that tab is shown.
  const attempts = useUserAttempts(
    person.id,
    tab === 'leetcode',
    seasonId,
    filters,
  );
  const mockScope = { mode: 'all', seasonId, userId: person.id } as const;
  const mocks = useMockInterviewPages(
    { ...mockScope, filters },
    tab === 'mocks',
  );
  const mockSummary = useMockInterviewSummary(
    { ...mockScope, filters },
    tab === 'mocks',
  );
  // Unfiltered so selecting an interviewer never removes the other options.
  const mockInterviewers = useMockInterviewSummary(mockScope, tab === 'mocks');
  const visibleAttempts = useMemo(
    () => filterPractice(attempts.data?.items ?? [], filters),
    [attempts.data, filters],
  );
  const visibleMocks = useMemo(
    () => mocks.data?.pages.flatMap((page) => page.items) ?? [],
    [mocks.data],
  );
  const interviewers = useMemo(
    () => (mockInterviewers.data?.interviewers ?? []).map(adaptMockParticipant),
    [mockInterviewers.data],
  );
  const mockColumns: ColumnDef<MockInterview, any>[] = [
    {
      accessorKey: 'occurredAt',
      header: 'Recorded',
      meta: { width: 11.5 },
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
    {
      accessorKey: 'durationMinutes',
      header: 'Minutes',
      meta: { width: 6.5 },
    },
    {
      id: 'result',
      header: 'Result',
      meta: { width: 7.5 },
      accessorFn: (mock) => (interviewPassed(mock) ? 'Pass' : 'Needs work'),
    },
    {
      id: 'feedback',
      header: 'Feedback',
      meta: { wrap: true },
      cell: ({ row }) => (
        <details>
          <summary>View feedback</summary>
          <MockFeedback mock={row.original} />
        </details>
      ),
    },
  ];
  return (
    <Tabs
      className="mt-6"
      value={tab}
      onValueChange={(value) => onTabChange(String(value))}
    >
      <TabsList aria-label="Profile activity">
        <TabsTrigger value="leetcode">LeetCode</TabsTrigger>
        <TabsTrigger value="mocks">Mock interviews</TabsTrigger>
      </TabsList>
      <TabsContent value="leetcode" className="flex flex-col gap-6">
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
      </TabsContent>
      <TabsContent value="mocks" className="flex flex-col gap-6">
        <MockFilters
          seasonId={seasonId}
          people={interviewers}
          filters={filters}
          onChange={setFilters}
        />
        <DataTable
          ariaLabel={`${person.name} mock history`}
          data={visibleMocks}
          renderSummary={() => (
            <ActivityChart
              data={activitySeries([], mockSummary.data?.days, { season })}
            />
          )}
          columns={mockColumns}
          loading={mocks.isLoading}
          error={mocks.isError}
          onRetry={() => void mocks.refetch()}
          serverPagination={{
            totalCount: mocks.data?.pages[0]?.totalCount ?? 0,
            hasMore: Boolean(mocks.hasNextPage),
            loadingMore: mocks.isFetchingNextPage,
            onLoadMore: () => void mocks.fetchNextPage(),
          }}
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
      </TabsContent>
    </Tabs>
  );
}
