import { IconArrowRight, IconBulb, IconCalendarEvent, IconClipboardCheck, IconUsers } from '@tabler/icons-react';
import { Link } from 'react-router-dom';

import { useAttempts, useCurrentUser, usePeople, useRecommendation, useSeasons } from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { ActivityChart } from '@/components/ActivityChart';
import { MetricCard, PersonIdentity, usePageTitle } from '@/components/Common';
import { ErrorState, PageSkeleton } from '@/components/StatusViews';
import { weeklyActivity } from '@/data/demo';
import styles from '@/styles/App.module.css';

export function DashboardPage() {
  usePageTitle('Dashboard');
  const user = useCurrentUser();
  const seasons = useSeasons();
  const attempts = useAttempts();
  const people = usePeople();
  const recommendation = useRecommendation();
  const { activeWorkspace } = useWorkspace();

  if (user.isLoading || seasons.isLoading) return <PageSkeleton label="Loading dashboard" />;
  if (user.isError || seasons.isError || !user.data) return <div className={styles.page}><ErrorState onRetry={() => void Promise.all([user.refetch(), seasons.refetch()])} /></div>;

  const currentSeason = seasons.data?.items.find((season) => season.slug === activeWorkspace?.seasonSlug) ?? seasons.data?.items.find((season) => season.status === 'open');
  const role = activeWorkspace?.role ?? 'graduate';

  return (
    <div className={styles.page}>
      <section className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>Your RSP workspace</p>
          <h1>Good evening, {user.data.name.split(' ')[0]}.</h1>
          <p>{dashboardIntro(role)} Focus on the next useful action; the detail is here when you need it.</p>
          <div className={styles.buttonRow} style={{ marginTop: '1.1rem' }}>
            <Link className={styles.button} to={role === 'coordinator' ? `/seasons/${currentSeason?.slug}/people` : '/practice'}>
              {role === 'coordinator' ? 'Review season roster' : 'Continue practice'} <IconArrowRight size={18} aria-hidden="true" />
            </Link>
            <Link className={styles.buttonSecondary} to="/mock-interviews">Mock interviews</Link>
          </div>
        </div>
        <div className={styles.heroMeta} aria-label="Current season summary">
          <div className={styles.heroMetaRow}><span>Workspace</span><strong>{activeWorkspace?.label}</strong></div>
          <div className={styles.heroMetaRow}><span>Season</span><strong>{currentSeason?.status === 'open' ? 'Open' : 'Graduate'}</strong></div>
          <div className={styles.heroMetaRow}><span>Timezone</span><strong>{user.data.timezone.split('/').at(-1)}</strong></div>
        </div>
      </section>

      <div className={styles.metricGrid}>
        {role === 'coordinator' || role === 'director' || role === 'system_admin' ? (
          <>
            <MetricCard label="Active members" value={currentSeason?.memberCount ?? 0} detail="Students, mentors and coordinators" />
            <MetricCard label="Unassigned students" value={people.data?.items.filter((person) => person.status === 'unassigned').length ?? '—'} detail="Need a mentor team" />
            <MetricCard label="Programme week" value="3 of 12" detail="Trees and traversal begins next" />
          </>
        ) : role === 'mentor' ? (
          <>
            <MetricCard label="Assigned mentees" value="4" detail="All active this season" />
            <MetricCard label="Attempts this week" value="12" detail="Across your mentor team" />
            <MetricCard label="Check-ins due" value="2" detail="Before Sunday evening" />
          </>
        ) : (
          <>
            <MetricCard label="Practice attempts" value={attempts.data?.totalCount ?? '—'} detail="Your complete history" />
            <MetricCard label="Independent solves" value={attempts.data?.items.filter((item) => item.outcome === 'independently_solved').length ?? '—'} detail="Across outcome-known attempts" />
            <MetricCard label="Mock interviews" value="3" detail="Two reviewed, one pending" />
          </>
        )}
      </div>

      <div className={styles.grid2}>
        <section className={styles.section} aria-labelledby="dashboard-focus-title">
          <div className={styles.sectionHeader}><h2 id="dashboard-focus-title" className={styles.sectionTitle}>{role === 'coordinator' ? 'Operations needing attention' : role === 'mentor' ? 'Your mentor team' : 'Recommended next problem'}</h2></div>
          {role === 'coordinator' ? (
            <div className={styles.panel}>
              <ul className={styles.cleanList}>
                <li className={styles.listRow}><span><IconUsers size={18} aria-hidden="true" /> 1 student is unassigned</span><Link to={`/seasons/${currentSeason?.slug}/mentor-teams`}>Assign</Link></li>
                <li className={styles.listRow}><span><IconClipboardCheck size={18} aria-hidden="true" /> 2 enrolments await review</span><Link to="/admin/enrollments">Review</Link></li>
                <li className={styles.listRow}><span><IconCalendarEvent size={18} aria-hidden="true" /> Week 4 resources are incomplete</span><Link to="/admin/weeks">Edit week</Link></li>
              </ul>
            </div>
          ) : role === 'mentor' ? (
            <div className={styles.panel}>
              <ul className={styles.cleanList}>{people.data?.items.filter((person) => person.roles.includes('student')).slice(0, 3).map((person) => <li className={styles.personRow} key={person.id}><PersonIdentity person={person} /><span className={styles.badgeNeutral}>{person.attempts} attempts</span></li>)}</ul>
            </div>
          ) : recommendation.isLoading ? (
            <div className={styles.recommendation} aria-busy="true"><span className={styles.skeleton} style={{ height: '8rem' }}>Loading recommendation</span></div>
          ) : recommendation.data ? (
            <article className={styles.recommendation}>
              <span className={styles.badge}>{recommendation.data.difficulty} · {recommendation.data.category}</span>
              <h3 className={styles.recommendationTitle}>{recommendation.data.title}</h3>
              <p>{recommendation.data.rationale}</p>
              <div className={styles.buttonRow}>
                <Link className={styles.button} to="/practice"><IconBulb size={18} aria-hidden="true" /> Open recommendation</Link>
              </div>
            </article>
          ) : null}
        </section>

        <section className={styles.section} aria-labelledby="activity-title">
          <div className={styles.sectionHeader}><h2 id="activity-title" className={styles.sectionTitle}>{role === 'coordinator' ? 'Season activity' : 'Recent activity'}</h2></div>
          <div className={styles.panel}><ActivityChart data={weeklyActivity} /></div>
        </section>
      </div>
    </div>
  );
}

function dashboardIntro(role: string) {
  if (role === 'coordinator') return 'Your current season is on track, with one mentor assignment requiring attention.';
  if (role === 'mentor') return 'Your mentees have been active this week; two check-ins are ready for follow-up.';
  if (role === 'director' || role === 'system_admin') return 'All programme workspaces are available, with clear audit trails for privileged actions.';
  if (role === 'graduate') return 'Your cross-season practice and interview history stays with you.';
  return 'Your next recommendation is ready and your recent practice is trending upward.';
}
