import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useMemo } from 'react';
import {
  emptyActivityFilters,
  filterPractice,
  useActivityFilters,
} from '@/activityFilters';
import { SeasonWeekChoices, MockFilters } from '@/components/ActivityFilters';
import { AttemptHistory } from '@/components/AttemptHistory';
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
import type { MockInterview, Person } from '@/types';
import { formatDate, interviewPassed } from '@/utils';

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
  const practiceFilters = useMemo(
    () => ({ ...emptyActivityFilters, weeks: filters.weeks }),
    [filters.weeks],
  );
  // Each tab's data loads only once that tab is shown.
  const attempts = useUserAttempts(person.id, tab === 'leetcode', seasonId);
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
    () => filterPractice(attempts.data?.items ?? [], practiceFilters),
    [attempts.data, practiceFilters],
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
      meta: { width: 7.5 },
      // Mocks are dated, not timed.
      cell: ({ getValue }) => formatDate(getValue()),
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
      accessorFn: (mock) => (interviewPassed(mock) ? 'Pass' : 'Fail'),
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
        <TabsTrigger value="mocks">Mock Interviews</TabsTrigger>
      </TabsList>
      <TabsContent value="leetcode" className="flex flex-col gap-6">
        <AttemptHistory
          ariaLabel={`${person.name} practice history`}
          data={visibleAttempts}
          renderSummary={(rows) => (
            <PracticeAnalytics attempts={rows} season={season} />
          )}
          loading={attempts.isLoading}
          error={attempts.isError}
          onRetry={() => void attempts.refetch()}
          emptyTitle="No practice in this period"
          emptyMessage="Recorded practice will appear here."
          extraFilters={{
            content: seasonId ? (
              <SeasonWeekChoices
                seasonId={seasonId}
                selected={practiceFilters.weeks}
                onChange={(weeks) => setFilters({ weeks })}
              />
            ) : null,
            activeCount: practiceFilters.weeks.length,
            onClear: () => setFilters({ weeks: [] }),
          }}
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
                {formatDate(m.occurredAt)} · {m.durationMinutes} min
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
