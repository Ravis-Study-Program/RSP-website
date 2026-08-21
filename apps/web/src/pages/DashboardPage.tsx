import {
  IconArrowRight,
  IconBulb,
  IconCalendarEvent,
  IconClipboardCheck,
  IconUsers,
} from '@tabler/icons-react';
import { Link } from 'react-router-dom';

import {
  useAttempts,
  useCurrentUser,
  useMockInterviews,
  useRecommendation,
  useSeasonEnrollments,
  useSeasonPeople,
  useSeasonWeeks,
  useSeasons,
} from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { ActivityChart } from '@/components/ActivityChart';
import { MetricCard, PersonIdentity, usePageTitle } from '@/components/Common';
import { ErrorState, PageSkeleton } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import { recentActivitySeries } from '@/utils';

export function DashboardPage() {
  usePageTitle('Dashboard');
  const user = useCurrentUser();
  const seasons = useSeasons();
  const eligibleForGlobalPractice = Boolean(
    user.data && (user.data.seasonRoles.length > 0 || user.data.alumni),
  );
  const attempts = useAttempts(eligibleForGlobalPractice);
  const recommendation = useRecommendation(eligibleForGlobalPractice);
  const interviews = useMockInterviews(eligibleForGlobalPractice);
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
      <div className={styles.page}>
        <ErrorState
          onRetry={() => void Promise.all([user.refetch(), seasons.refetch()])}
        />
      </div>
    );

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Workspace</p>
          <h1 className={styles.pageTitle}>
            Welcome, {user.data.name.split(' ')[0]}.
          </h1>
          <p className={styles.lede}>
            {dashboardIntro(role)} Focus on the next useful action; the detail
            is here when you need it.
          </p>
        </div>
        <div className={styles.headerActions}>
          <Link
            className={styles.button}
            to={
              role === 'coordinator'
                ? `/seasons/${currentSeason?.slug}/people`
                : '/practice'
            }
          >
            {role === 'coordinator'
              ? 'Review season roster'
              : 'Continue practice'}{' '}
            <IconArrowRight size={18} aria-hidden="true" />
          </Link>
          <Link className={styles.buttonSecondary} to="/mock-interviews">
            Mock interviews
          </Link>
        </div>
      </header>

      <dl className={styles.contextBar} aria-label="Current workspace summary">
        <div className={styles.contextItem}>
          <dt>Workspace</dt>
          <dd>{activeWorkspace?.label}</dd>
        </div>
        <div className={styles.contextItem}>
          <dt>Season</dt>
          <dd>{currentSeason?.status === 'open' ? 'Open' : 'Graduate'}</dd>
        </div>
        <div className={styles.contextItem}>
          <dt>Timezone</dt>
          <dd>{user.data.timezone.split('/').at(-1)}</dd>
        </div>
      </dl>

      <div className={styles.metricGrid}>
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

      <div className={styles.grid2}>
        <section
          className={styles.section}
          aria-labelledby="dashboard-focus-title"
        >
          <div className={styles.sectionHeader}>
            <h2 id="dashboard-focus-title" className={styles.sectionTitle}>
              {role === 'coordinator'
                ? 'Operations needing attention'
                : role === 'mentor'
                  ? 'Your mentor team'
                  : 'Recommended next problem'}
            </h2>
          </div>
          {role === 'coordinator' ? (
            <div className={styles.panel}>
              <ul className={styles.cleanList}>
                <li className={styles.listRow}>
                  <span>
                    <IconUsers size={18} aria-hidden="true" />{' '}
                    {unassigned.length
                      ? `${unassigned.length} ${unassigned.length === 1 ? 'student is' : 'students are'} unassigned`
                      : 'Every active student is assigned'}
                  </span>
                  <Link to={`/seasons/${currentSeason?.slug}/mentor-teams`}>
                    Mentor teams
                  </Link>
                </li>
                <li className={styles.listRow}>
                  <span>
                    <IconClipboardCheck size={18} aria-hidden="true" />{' '}
                    {activeEnrollments.length} active{' '}
                    {activeEnrollments.length === 1
                      ? 'enrollment'
                      : 'enrollments'}
                  </span>
                  <Link to="/admin/enrollments">Manage</Link>
                </li>
                <li className={styles.listRow}>
                  <span>
                    <IconCalendarEvent size={18} aria-hidden="true" />{' '}
                    {missingWeekResources
                      ? `${missingWeekResources} ${missingWeekResources === 1 ? 'week has' : 'weeks have'} no resource link`
                      : 'All scheduled weeks have resource links'}
                  </span>
                  <Link to="/admin/weeks">Weeks</Link>
                </li>
              </ul>
            </div>
          ) : role === 'mentor' ? (
            <div className={styles.panel}>
              {mentees.length ? (
                <ul className={styles.cleanList}>
                  {mentees.slice(0, 3).map((person) => (
                    <li className={styles.personRow} key={person.id}>
                      <PersonIdentity person={person} />
                      <span className={styles.badgeNeutral}>
                        {person.attempts ?? '—'} attempts
                      </span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className={styles.muted}>
                  No active students are assigned to you in this season.
                </p>
              )}
            </div>
          ) : recommendation.isLoading ? (
            <div className={styles.recommendation} aria-busy="true">
              <span
                className={`${styles.skeleton} ${styles.skeletonRecommendation}`}
              >
                Loading recommendation
              </span>
            </div>
          ) : recommendation.data ? (
            <article className={styles.recommendation}>
              <span className={styles.badge}>
                {recommendation.data.difficulty} ·{' '}
                {recommendation.data.category}
              </span>
              <h3 className={styles.recommendationTitle}>
                {recommendation.data.title}
              </h3>
              <p>{recommendation.data.rationale}</p>
              <div className={styles.buttonRow}>
                <Link className={styles.button} to="/practice">
                  <IconBulb size={18} aria-hidden="true" /> Open recommendation
                </Link>
              </div>
            </article>
          ) : null}
        </section>

        <section className={styles.section} aria-labelledby="activity-title">
          <div className={styles.sectionHeader}>
            <h2 id="activity-title" className={styles.sectionTitle}>
              Your recent activity
            </h2>
          </div>
          <div className={styles.panel}>
            <ActivityChart data={activity} />
          </div>
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
  return 'Your recommendation and recorded practice history are ready.';
}
