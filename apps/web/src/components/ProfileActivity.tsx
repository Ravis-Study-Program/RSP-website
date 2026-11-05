import { MockInterviewHistory } from '@/components/MockInterviewHistory';
import {
  MockInterviewViewTabs,
  MockInterviewFilterControls,
} from '@/components/MockInterviewFilters';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useMemo, useState } from 'react';
import {
  emptyActivityFilters,
  filterPractice,
  useActivityFilters,
} from '@/activityFilters';
import { SeasonWeekChoices } from '@/components/ActivityFilters';
import { AttemptHistory } from '@/components/AttemptHistory';
import {
  useCurrentUser,
  type MockInterviewMode,
  useMockInterviewPages,
  useMockInterviewSummary,
  useUserAttempts,
  useSeasons,
} from '@/api/queries';
import { PracticeAnalytics } from '@/components/PracticeAnalytics';
import { ActivityChart } from '@/components/ActivityChart';
import { activitySeries } from '@/activityAnalytics';
import type { Person } from '@/types';

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
  const viewer = useCurrentUser();
  const [mockMode, setMockMode] = useState<MockInterviewMode>('all');
  const seasons = useSeasons();
  const season = seasons.data?.items.find((s) => s.id === seasonId);
  const [filters, setFilters] = useActivityFilters(seasonId);
  const practiceFilters = useMemo(
    () => ({ ...emptyActivityFilters, weeks: filters.weeks }),
    [filters.weeks],
  );
  // Each tab's data loads only once that tab is shown.
  const attempts = useUserAttempts(person.id, tab === 'leetcode', seasonId);
  const mockFilters = useMemo(
    () => ({
      ...filters,
      ...(mockMode !== 'received' ? { interviewers: [] } : {}),
      ...(mockMode !== 'given' ? { interviewees: [] } : {}),
    }),
    [mockMode, filters],
  );
  const mockScope = { mode: mockMode, seasonId, userId: person.id };
  const mocks = useMockInterviewPages(
    { ...mockScope, filters: mockFilters },
    tab === 'mocks',
  );
  const mockSummary = useMockInterviewSummary(
    { ...mockScope, filters: mockFilters },
    tab === 'mocks',
  );
  // All-time options stay available when narrowing the visible history.
  const participants = useMockInterviewSummary(
    { mode: mockMode, userId: person.id },
    tab === 'mocks',
  );
  const participantOptions =
    (mockMode === 'received'
      ? participants.data?.interviewers
      : mockMode === 'given'
        ? participants.data?.interviewees
        : undefined) ?? [];
  const visibleAttempts = useMemo(
    () => filterPractice(attempts.data?.items ?? [], practiceFilters),
    [attempts.data, practiceFilters],
  );
  const visibleMocks = useMemo(
    () => mocks.data?.pages.flatMap((page) => page.items) ?? [],
    [mocks.data],
  );
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
        <MockInterviewHistory
          toolbarStart={
            <MockInterviewViewTabs mode={mockMode} onModeChange={setMockMode} />
          }
          seasonId={seasonId}
          subjectId={person.id}
          viewerId={viewer.data?.id}
          mode={mockMode}
          extraFilters={{
            content: (
              <MockInterviewFilterControls
                mode={mockMode}
                onModeChange={setMockMode}
                seasonId={seasonId}
                people={participantOptions}
                filters={mockFilters}
                onChange={setFilters}
              />
            ),
            activeCount:
              Number(Boolean(mockFilters.passed)) +
              mockFilters.interviewers.length +
              mockFilters.interviewees.length +
              mockFilters.weeks.length,
            onClear: () =>
              setFilters({
                passed: '',
                interviewers: [],
                interviewees: [],
                weeks: [],
              }),
          }}
          ariaLabel={`${person.name} mock history`}
          data={visibleMocks}
          renderSummary={() => (
            <ActivityChart
              data={activitySeries([], mockSummary.data?.days, { season })}
            />
          )}
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
        />
      </TabsContent>
    </Tabs>
  );
}
