import { StatusBadge } from '@/components/StatusBadge';
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
  useMockInterviewSummary,
  useSeasonEnrollments,
  useSeasonPeople,
  useSeasonWeeks,
  useSeasons,
  useUserParticipation,
} from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import {
  ActivityChart,
  studentWeeklyExpected,
} from '@/components/ActivityChart';
import { MondayUpdateReminder } from '@/components/MondayUpdateReminder';
import {
  MetricCard,
  Page,
  PageSection,
  PersonIdentity,
  usePageTitle,
} from '@/components/Common';
import { LevelProgressSection } from '@/components/LevelProgress';
import { ErrorState, PageSkeleton } from '@/components/StatusViews';
import { isProgramLevel } from '@/levelProgress';
import { recentActivitySeries } from '@/utils';

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
  const interviews = useMockInterviewSummary(
    { mode: 'all', userId: user.data?.id },
    eligibleForGlobalPractice,
  );
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
  const participation = useUserParticipation(
    role === 'student' ? userId : undefined,
  );
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
  const activity = recentActivitySeries(
    attempts.data?.items ?? [],
    interviews.data?.days,
  );
  const recordedLevel = participation.data?.items.find(
    (item) => item.seasonId === currentSeason?.id,
  )?.lastStudentLevel;
  const studentLevel = isProgramLevel(recordedLevel) ? recordedLevel : null;

  if (user.isLoading || seasons.isLoading)
    return <PageSkeleton label="Loading dashboard" />;
  if (user.isError || seasons.isError || !user.data)
    return (
      <Page>
        <ErrorState
          onRetry={() => void Promise.all([user.refetch(), seasons.refetch()])}
        />
      </Page>
    );

  return (
    <Page>
      {role === 'student' &&
      currentSeason?.status === 'open' &&
      !weeks.isError &&
      user.data.seasonRoles.some(
        (membership) =>
          membership.seasonId === currentSeason.id &&
          membership.role === 'student' &&
          membership.state === 'active',
      ) ? (
        <MondayUpdateReminder
          seasonId={currentSeason.id}
          seasonSlug={currentSeason.slug}
          studentId={user.data.id}
          weeks={weeks.data?.items ?? []}
        />
      ) : null}
      <header className="mb-6 flex flex-col items-stretch justify-between gap-5 md:flex-row md:items-start">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
            Welcome, {user.data.name.split(' ')[0]}.
          </h1>
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
            Mock Interviews
          </Link>
        </div>
      </header>

      {/* Everything below the welcome sits in one 24px stack. */}
      <div className="flex flex-col gap-6">
        <div
          className={`grid gap-6 max-[56rem]:grid-cols-1 ${
            ['mentor', 'coordinator', 'director', 'system_admin'].includes(role)
              ? 'grid-cols-3'
              : 'grid-cols-2'
          }`}
        >
          {role === 'coordinator' ||
          role === 'director' ||
          role === 'system_admin' ? (
            <>
              <MetricCard
                label="Active Members"
                value={enrollments.isLoading ? '—' : activeEnrollments.length}
              />
              <MetricCard
                label="Unassigned Students"
                value={seasonPeople.isLoading ? '—' : unassigned.length}
              />
              <MetricCard
                label="Program Week"
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
                label="Assigned Mentees"
                value={seasonPeople.isLoading ? '—' : mentees.length}
              />
              <MetricCard
                label="Interviews Given"
                value={interviews.data?.given ?? '—'}
              />
              <MetricCard
                label="Program Weeks"
                value={weeks.isLoading ? '—' : orderedWeeks.length}
              />
            </>
          ) : (
            <>
              <MetricCard
                label="Leetcodes"
                value={attempts.data?.totalCount ?? '—'}
              />
              <MetricCard
                label="Mock Interviews"
                value={interviews.data?.total ?? '—'}
                labelAside={
                  interviews.data ? (
                    <ReviewNotice
                      pending={Math.max(
                        0,
                        interviews.data.total - interviews.data.reviewed,
                      )}
                      total={interviews.data.total}
                    />
                  ) : null
                }
              />
            </>
          )}
        </div>

        {role === 'student' && studentLevel ? (
          <LevelProgressSection level={studentLevel} />
        ) : null}

        <div
          className={`grid items-stretch gap-6 ${role === 'coordinator' || role === 'mentor' ? 'lg:grid-cols-2' : ''}`}
        >
          {role === 'coordinator' || role === 'mentor' ? (
            <PageSection
              titleId="dashboard-focus-title"
              title={
                role === 'coordinator'
                  ? 'Operations Needing Attention'
                  : 'Your Mentor Team'
              }
            >
              {role === 'coordinator' ? (
                <Card className="flex-1">
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
                          Mentor Teams
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
              ) : (
                <Card className="flex-1">
                  <CardContent>
                    {mentees.length ? (
                      <ul className={listClass}>
                        {mentees.slice(0, 3).map((person) => (
                          <li className={listRowClass} key={person.id}>
                            <PersonIdentity
                              person={person}
                              seasonId={currentSeason?.id}
                            />
                            <StatusBadge>
                              {person.attempts ?? '—'} attempts
                            </StatusBadge>
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
              )}
            </PageSection>
          ) : null}

          <PageSection titleId="activity-title" title="Your Recent Activity">
            <Card className="flex-1">
              <CardContent>
                <ActivityChart
                  data={activity}
                  expected={
                    role === 'student' ? studentWeeklyExpected : undefined
                  }
                />
              </CardContent>
            </Card>
          </PageSection>
        </div>
      </div>
    </Page>
  );
}

/** Green when nothing is waiting, red when every interview is, otherwise amber. */
function ReviewNotice({ pending, total }: { pending: number; total: number }) {
  const label =
    pending === 1 ? '1 review pending' : `${pending} reviews pending`;
  const tone =
    pending === 0 ? 'success' : pending === total ? 'danger' : 'warning';
  return (
    <StatusBadge tone={tone} className="shrink-0">
      {label}
    </StatusBadge>
  );
}
