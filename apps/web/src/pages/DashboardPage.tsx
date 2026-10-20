import { Badge } from '@/components/ui/badge';
import { buttonVariants } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';

import {
  IconArrowRight,
  IconCalendarEvent,
  IconClipboardCheck,
  IconUsers,
} from '@tabler/icons-react';
import { Link } from 'react-router-dom';

import {
  useAttempts,
  useCurrentUser,
  useMockInterviews,
  useSeasonEnrollments,
  useSeasonPeople,
  useSeasonWeeks,
  useSeasons,
} from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { ActivityChart } from '@/components/ActivityChart';
import { MetricCard, PersonIdentity, usePageTitle } from '@/components/Common';
import { ErrorState, PageSkeleton } from '@/components/StatusViews';
import { recentActivitySeries } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const sectionTitleClass = 'text-xl font-semibold tracking-tight';
const sectionHeaderClass = 'mb-3 flex items-end justify-between gap-4';
const listClass = 'm-0 list-none divide-y p-0';
const listRowClass = 'flex items-center justify-between gap-4 py-3';

export function DashboardPage() {
  usePageTitle('Dashboard');
  const user = useCurrentUser();
  const seasons = useSeasons();
  const eligibleForGlobalPractice = Boolean(
    user.data && (user.data.seasonRoles.length > 0 || user.data.alumni),
  );
  const attempts = useAttempts(eligibleForGlobalPractice);
  // Only the signed-in member's own interviews; never the whole platform.
  const interviews = useMockInterviews({
    enabled: eligibleForGlobalPractice && Boolean(user.data?.id),
    userId: user.data?.id,
  });
  const { activeWorkspace } = useWorkspace();

  const currentSeason =
    seasons.data?.items.find(
      (season) => season.slug === activeWorkspace?.seasonSlug,
    ) ??
    (activeWorkspace?.role === 'director' ||
    activeWorkspace?.role === 'system_admin'
      ? seasons.data?.items.find((season) => season.status === 'open')
      : undefined);
  const seasonPeople = useSeasonPeople(currentSeason?.id, currentSeason?.name);
  const weeks = useSeasonWeeks(currentSeason?.id);
  const enrollments = useSeasonEnrollments(currentSeason?.id);
  const role = activeWorkspace?.role ?? 'graduate';
  const userId = user.data?.id;
  const activeEnrollments =
    enrollments.data?.items.filter((item) => item.state === 'active') ?? [];
  const unassigned =
    seasonPeople.data?.items.filter(
      (person) =>
        person.roles.includes('student') && person.status === 'unassigned',
    ) ?? [];
  const mentees =
    seasonPeople.data?.items.filter(
      (person) =>
        person.mentorshipMentorId === userId &&
        person.enrollmentState === 'active',
    ) ?? [];
  const currentTime = Date.now();
  const orderedWeeks = [...(weeks.data?.items ?? [])].sort(
    (left, right) => left.number - right.number,
  );
  const currentWeek =
    orderedWeeks.find(
      (week) =>
        new Date(week.startAt).getTime() <= currentTime &&
        currentTime <= new Date(week.endAt).getTime(),
    ) ??
    orderedWeeks.find((week) => new Date(week.startAt).getTime() > currentTime);
  const missingWeekResources = orderedWeeks.filter(
    (week) => !week.resourceUrl,
  ).length;
  const ownInterviews =
    interviews.data?.items.filter(
      (interview) =>
        interview.interviewer.id === userId ||
        interview.interviewee.id === userId,
    ) ?? [];
  const givenInterviews =
    interviews.data?.items.filter(
      (interview) => interview.interviewer.id === userId,
    ) ?? [];
  const activity = recentActivitySeries(
    attempts.data?.items ?? [],
    interviews.data?.items ?? [],
    userId,
  );

  if (user.isLoading || seasons.isLoading)
    return <PageSkeleton label="Loading dashboard" />;
  if (user.isError || seasons.isError || !user.data)
    return (
      <div className={pageClass}>
        <ErrorState
          onRetry={() => void Promise.all([user.refetch(), seasons.refetch()])}
        />
      </div>
    );

  return (
    <div className={pageClass}>
      <header className="mb-6 flex flex-col items-stretch justify-between gap-5 md:flex-row md:items-start">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
            Welcome, {user.data.name.split(' ')[0]}.
          </h1>
          <p className="mt-2 max-w-3xl text-base text-muted-foreground">
            {dashboardIntro(role)}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2.5">
          <Link
            to={
              role === 'coordinator'
                ? `/seasons/${currentSeason?.slug}/people`
                : '/leetcode'
            }
            className={buttonVariants({ variant: 'default' })}
          >
            {role === 'coordinator'
              ? 'Review season roster'
              : 'Continue LeetCode'}{' '}
            <IconArrowRight size={18} aria-hidden="true" />
          </Link>
          <Link
            to="/mock-interviews"
            className={buttonVariants({ variant: 'outline' })}
          >
            Mock interviews
          </Link>
        </div>
      </header>

      <dl
        className="mb-6 flex flex-wrap gap-x-6 gap-y-1 text-sm"
        aria-label="Current workspace summary"
      >
        <div className="flex gap-2">
          <dt className="text-muted-foreground">Workspace</dt>
          <dd className="font-semibold">{activeWorkspace?.label}</dd>
        </div>
        <div className="flex gap-2">
          <dt className="text-muted-foreground">Season</dt>
          <dd className="font-semibold">
            {currentSeason?.status === 'open' ? 'Open' : 'Graduate'}
          </dd>
        </div>
        <div className="flex gap-2">
          <dt className="text-muted-foreground">Timezone</dt>
          <dd className="font-semibold">
            {user.data.timezone.split('/').at(-1)}
          </dd>
        </div>
      </dl>

      <div className="mt-4 grid grid-cols-3 gap-4 max-[56rem]:grid-cols-1">
        {role === 'coordinator' ||
        role === 'director' ||
        role === 'system_admin' ? (
          <>
            <MetricCard
              label="Active members"
              value={enrollments.isLoading ? '—' : activeEnrollments.length}
              detail="Students, mentors and coordinators"
            />
            <MetricCard
              label="Unassigned students"
              value={seasonPeople.isLoading ? '—' : unassigned.length}
              detail="Need a mentor team"
            />
            <MetricCard
              label="Programme week"
              value={
                currentWeek
                  ? `${currentWeek.number} of ${orderedWeeks.length}`
                  : '—'
              }
              detail={
                currentWeek
                  ? 'Current or next scheduled week'
                  : 'No current or upcoming week'
              }
            />
          </>
        ) : role === 'mentor' ? (
          <>
            <MetricCard
              label="Assigned mentees"
              value={seasonPeople.isLoading ? '—' : mentees.length}
              detail="Active students in your mentor team"
            />
            <MetricCard
              label="Interviews given"
              value={interviews.isLoading ? '—' : givenInterviews.length}
              detail="Visible interview history"
            />
            <MetricCard
              label="Programme weeks"
              value={weeks.isLoading ? '—' : orderedWeeks.length}
              detail="Published for this season"
            />
          </>
        ) : (
          <>
            <MetricCard
              label="Practice attempts"
              value={attempts.data?.totalCount ?? '—'}
              detail="Your complete history"
            />
            <MetricCard
              label="Independent solves"
              value={
                attempts.data?.items.filter(
                  (item) => item.outcome === 'independently_solved',
                ).length ?? '—'
              }
              detail="Across outcome-known attempts"
            />
            <MetricCard
              label="Mock interviews"
              value={interviews.isLoading ? '—' : ownInterviews.length}
              detail={`${ownInterviews.filter((item) => item.reviewStatus === 'reviewed').length} reviewed`}
            />
          </>
        )}
      </div>

      <div className="mt-8 grid items-stretch gap-6 lg:grid-cols-2">
        <section
          className="mt-6 flex flex-col"
          aria-labelledby="dashboard-focus-title"
        >
          <div className={sectionHeaderClass}>
            <h2 id="dashboard-focus-title" className={sectionTitleClass}>
              {role === 'coordinator'
                ? 'Operations needing attention'
                : role === 'mentor'
                  ? 'Your mentor team'
                  : 'Practice history'}
            </h2>
          </div>
          {role === 'coordinator' ? (
            <Card className="h-full">
              <CardContent>
                <ul className={listClass}>
                  <li className={listRowClass}>
                    <span className="flex items-center gap-2">
                      <IconUsers size={18} aria-hidden="true" />{' '}
                      {unassigned.length
                        ? `${unassigned.length} ${unassigned.length === 1 ? 'student is' : 'students are'} unassigned`
                        : 'Every active student is assigned'}
                    </span>
                    <Link
                      to={`/seasons/${currentSeason?.slug}/mentor-teams`}
                      className={buttonVariants({
                        variant: 'link',
                        size: 'sm',
                      })}
                    >
                      Mentor teams
                    </Link>
                  </li>
                  <li className={listRowClass}>
                    <span className="flex items-center gap-2">
                      <IconClipboardCheck size={18} aria-hidden="true" />{' '}
                      {activeEnrollments.length} active{' '}
                      {activeEnrollments.length === 1
                        ? 'enrollment'
                        : 'enrollments'}
                    </span>
                    <Link
                      to="/admin/enrollments"
                      className={buttonVariants({
                        variant: 'link',
                        size: 'sm',
                      })}
                    >
                      Manage
                    </Link>
                  </li>
                  <li className={listRowClass}>
                    <span className="flex items-center gap-2">
                      <IconCalendarEvent size={18} aria-hidden="true" />{' '}
                      {missingWeekResources
                        ? `${missingWeekResources} ${missingWeekResources === 1 ? 'week has' : 'weeks have'} no resource link`
                        : 'All scheduled weeks have resource links'}
                    </span>
                    <Link
                      to="/admin/weeks"
                      className={buttonVariants({
                        variant: 'link',
                        size: 'sm',
                      })}
                    >
                      Weeks
                    </Link>
                  </li>
                </ul>
              </CardContent>
            </Card>
          ) : role === 'mentor' ? (
            <Card className="h-full">
              <CardContent>
                {mentees.length ? (
                  <ul className={listClass}>
                    {mentees.slice(0, 3).map((person) => (
                      <li className={listRowClass} key={person.id}>
                        <PersonIdentity
                          person={person}
                          seasonId={currentSeason?.id}
                        />
                        <Badge variant="secondary">
                          {person.attempts ?? '—'} attempts
                        </Badge>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-muted-foreground">
                    No active students are assigned to you in this season.
                  </p>
                )}
              </CardContent>
            </Card>
          ) : (
            <Card className="h-full">
              <CardContent>
                <p>
                  Review your recorded problem attempts, outcomes and practice
                  time.
                </p>
                <div className="mt-4 flex flex-wrap gap-2.5">
                  <Link
                    to="/leetcode"
                    className={buttonVariants({ variant: 'default' })}
                  >
                    View LeetCode history
                  </Link>
                </div>
              </CardContent>
            </Card>
          )}
        </section>

        <section
          className="mt-6 flex flex-col"
          aria-labelledby="activity-title"
        >
          <div className={sectionHeaderClass}>
            <h2 id="activity-title" className={sectionTitleClass}>
              Your recent activity
            </h2>
          </div>
          <Card className="h-full">
            <CardContent>
              <ActivityChart data={activity} />
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  );
}

function dashboardIntro(role: string) {
  if (role === 'coordinator')
    return 'Review the current roster, mentor assignments and published weeks.';
  if (role === 'mentor')
    return 'Your assigned mentees and interview activity are gathered here.';
  if (role === 'director' || role === 'system_admin')
    return 'All programme workspaces are available, with clear audit trails for privileged actions.';
  if (role === 'graduate')
    return 'Your cross-season practice and interview history stays with you.';
  if (role === 'former_member')
    return 'Your global practice and mock interview access remains available after a completed season.';
  return 'Your recorded practice and mock interview history are ready.';
}
