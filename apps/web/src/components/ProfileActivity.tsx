import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useMemo, useState } from 'react';
import { useActivityFilters } from '@/activityFilters';
import { MockFilters } from '@/components/ActivityFilters';
import {
  useCurrentUser,
  useMockInterviewPages,
  useMockInterviewSummary,
  useSeasonPeople,
  useUserAttempts,
  useUserParticipation,
} from '@/api/queries';
import { AttemptTable } from '@/components/AttemptTable';
import { PracticeAnalytics } from '@/components/PracticeAnalytics';
import {
  MockInterviewTable,
  MockTypeField,
  type MockView,
} from '@/pages/MockInterviewsPage';
import type { Attempt, MockInterview, Person } from '@/types';

/**
 * A person's LeetCode attempts and mock interviews, in the same tables as the
 * LeetCode and Mock Interviews pages, read-only. Received and Given are from
 * the person's side.
 */
export function ProfileActivity({
  person,
  tab,
  onTabChange,
}: {
  person: Person;
  tab: string;
  onTabChange: (tab: string) => void;
}) {
  const user = useCurrentUser();
  const isSelf = person.id === user.data?.id;
  const canSeeDetails = useCanSeeActivityDetails(person, isSelf);
  const [urlFilters, setFilters] = useActivityFilters();
  const [view, setView] = useState<MockView>('all');
  // Each tab's data loads only once that tab is shown. Profiles show
  // all-time activity, so there are no season weeks to filter by.
  const attempts = useUserAttempts(person.id, tab === 'leetcode');
  // As on the Mock Interviews page, Received filters by interviewer, Given by
  // interviewee, and All has no people filter.
  const mockFilters = useMemo(
    () => ({
      ...urlFilters,
      difficulties: [],
      categories: [],
      ...(view !== 'received' ? { interviewers: [] } : {}),
      ...(view !== 'given' ? { interviewees: [] } : {}),
      weeks: [],
    }),
    [urlFilters, view],
  );
  const mocks = useMockInterviewPages(
    { mode: view, userId: person.id, filters: mockFilters },
    tab === 'mocks',
  );
  // Only people they have had interviews with, across all time, are offered.
  const participants = useMockInterviewSummary(
    { mode: view, userId: person.id },
    tab === 'mocks',
  );
  const participantOptions =
    (view === 'received'
      ? participants.data?.interviewers
      : view === 'given'
        ? participants.data?.interviewees
        : undefined) ?? [];
  const [updated, setUpdated] = useState<Record<string, MockInterview>>({});
  const [reviewedAttempts, setReviewedAttempts] = useState<
    Record<string, Attempt>
  >({});
  const visibleMocks = useMemo(
    () =>
      (mocks.data?.pages.flatMap((page) => page.items) ?? []).map(
        (mock) => updated[mock.id] ?? mock,
      ),
    [mocks.data, updated],
  );
  // Switches between the two tables, in each table's toolbar.
  const activityTabs = (
    <TabsList
      aria-label="Profile activity"
      className="group-data-[orientation=horizontal]/tabs:h-9"
    >
      <TabsTrigger value="leetcode">LeetCode</TabsTrigger>
      <TabsTrigger value="mocks">Mock Interviews</TabsTrigger>
    </TabsList>
  );
  return (
    <Tabs value={tab} onValueChange={(value) => onTabChange(String(value))}>
      <TabsContent value="leetcode">
        <AttemptTable
          toolbarLead={activityTabs}
          ariaLabel={`${person.name} practice history`}
          data={(attempts.data?.items ?? []).map(
            (attempt) => reviewedAttempts[attempt.id] ?? attempt,
          )}
          showNotes={canSeeDetails}
          canReview={isSelf}
          onReviewed={(attempt) =>
            setReviewedAttempts((items) => ({
              ...items,
              [attempt.id]: attempt,
            }))
          }
          renderSummary={(rows) => <PracticeAnalytics attempts={rows} />}
          loading={attempts.isLoading}
          error={attempts.isError}
          onRetry={() => void attempts.refetch()}
          emptyTitle="No Practice in This Period"
          emptyMessage="Recorded practice will appear here."
        />
      </TabsContent>
      <TabsContent value="mocks">
        <MockInterviewTable
          view={view}
          onViewChange={setView}
          mocks={visibleMocks}
          subjectId={person.id}
          viewerId={user.data?.id}
          onChanged={(changed) => {
            if (changed)
              setUpdated((items) => ({ ...items, [changed.id]: changed }));
            else void mocks.refetch();
          }}
          toolbarLead={activityTabs}
          canViewDetails={(mock) =>
            canSeeDetails ||
            mock.interviewer.id === user.data?.id ||
            mock.interviewee.id === user.data?.id
          }
          ariaLabel={`${person.name} mock history`}
          extraFilters={{
            content: (
              <>
                <MockTypeField view={view} onViewChange={setView} />
                <MockFilters
                  participant={
                    view === 'received' ? 'interviewer' : 'interviewee'
                  }
                  people={participantOptions}
                  filters={mockFilters}
                  onChange={setFilters}
                />
              </>
            ),
            activeCount:
              Number(Boolean(mockFilters.passed)) +
              mockFilters.interviewers.length +
              mockFilters.interviewees.length,
            onClear: () => {
              setView('all');
              setFilters({
                passed: '',
                interviewers: [],
                interviewees: [],
              });
            },
          }}
          loading={mocks.isLoading || user.isLoading}
          error={mocks.isError}
          onRetry={() => void mocks.refetch()}
          serverPagination={{
            totalCount: mocks.data?.pages[0]?.totalCount ?? 0,
            hasMore: Boolean(mocks.hasNextPage),
            loadingMore: mocks.isFetchingNextPage,
            onLoadMore: () => void mocks.fetchNextPage(),
          }}
          emptyTitle="No Mocks in This Period"
          emptyMessage="Received and given interviews will appear here."
        />
      </TabsContent>
    </Tabs>
  );
}

/**
 * Whether the viewer may open this person's LeetCode notes and interview
 * details. Besides the person themselves: Directors and System Admins,
 * coordinators of a season the person was in, and the person's current or
 * past mentors. Everyone else sees only interviews they took part in.
 */
function useCanSeeActivityDetails(person: Person, isSelf: boolean) {
  const viewer = useCurrentUser().data;
  const isAdmin = Boolean(
    viewer?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const needsSeasons = !isSelf && !isAdmin && Boolean(viewer);
  const participation = useUserParticipation(person.id, needsSeasons);
  const personSeasonIds = new Set(
    (participation.data?.items ?? []).map((item) => item.seasonId),
  );
  const staffIn = (role: 'coordinator' | 'mentor') =>
    viewer?.seasonRoles.find(
      (entry) =>
        entry.role === role &&
        (entry.state === 'active' || entry.state === 'completed') &&
        personSeasonIds.has(entry.seasonId),
    );
  const coordinates = Boolean(staffIn('coordinator'));
  // A mentor sees the students they mentor or used to mentor that season.
  const mentorSeasonId =
    needsSeasons && !coordinates ? staffIn('mentor')?.seasonId : undefined;
  const seasonPeople = useSeasonPeople(mentorSeasonId);
  const entry = mentorSeasonId
    ? seasonPeople.data?.items.find((item) => item.id === person.id)
    : undefined;
  const mentors = Boolean(
    viewer &&
    entry &&
    (entry.mentorshipMentorId === viewer.id ||
      (entry.previousMentorIds ?? []).includes(viewer.id)),
  );
  return isSelf || isAdmin || coordinates || mentors;
}
