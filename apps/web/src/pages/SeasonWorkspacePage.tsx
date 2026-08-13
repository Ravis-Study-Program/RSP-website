import { IconCalendarEvent, IconLock, IconUsers } from '@tabler/icons-react';
import { Link, useParams } from 'react-router-dom';

import { useSeasons } from '@/api/queries';
import { ExternalLink, MetricCard, PageHeader, usePageTitle } from '@/components/Common';
import { ErrorState, InlineNotice, PageSkeleton } from '@/components/StatusViews';
import { seasonWeeks } from '@/data/demo';
import styles from '@/styles/App.module.css';
import { formatDate } from '@/utils';

export function SeasonWorkspacePage() {
  const { slug } = useParams();
  const query = useSeasons();
  const season = query.data?.items.find((item) => item.slug === slug);
  usePageTitle(season?.name ?? 'Season');

  if (query.isLoading) return <PageSkeleton label="Loading season" />;
  if (query.isError) return <div className={styles.page}><ErrorState onRetry={() => void query.refetch()} /></div>;
  if (!season) return <div className={styles.page}><ErrorState title="Season not found" message="This season may have moved or you may not have access." /></div>;

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow={season.status === 'open' ? 'Active season' : 'Programme history'}
        title={season.name}
        description={season.summary}
        actions={<span className={season.status === 'open' ? styles.badgeSuccess : styles.badgeNeutral}>{season.status === 'open' ? 'Open' : 'Closed'}</span>}
      />
      {season.status === 'closed' ? <InlineNotice tone="warning"><IconLock size={19} aria-hidden="true" /> This season is read-only. You can still create global practice attempts and mock interviews.</InlineNotice> : null}
      <div className={styles.metricGrid}>
        <MetricCard label="Members" value={season.memberCount} detail="Across all season roles" />
        <MetricCard label="Programme weeks" value={season.weekCount} detail={`${formatDate(season.startsAt)} – ${formatDate(season.endsAt)}`} />
        <MetricCard label="Current focus" value="Week 3" detail="Linked structures" />
      </div>
      <div className={styles.grid2}>
        <section className={styles.section} aria-labelledby="schedule-title">
          <div className={styles.sectionHeader}><h2 id="schedule-title" className={styles.sectionTitle}>Programme schedule</h2><Link to={`/seasons/${season.slug}/practice`}>Open practice</Link></div>
          <div className={styles.panel}>
            <ol className={styles.timeline}>
              {seasonWeeks.map((week) => (
                <li className={styles.timelineItem} key={week.week}>
                  <span className={styles.timelineDot} aria-hidden="true" />
                  <div className={styles.timelineContent}>
                    <h3>Week {week.week}: {week.topic}</h3>
                    <p>{formatDate(week.startsAt)} · {week.resource}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </section>
        <section className={styles.section} aria-labelledby="season-actions-title">
          <div className={styles.sectionHeader}><h2 id="season-actions-title" className={styles.sectionTitle}>Workspace</h2></div>
          <div className={styles.panel}>
            <ul className={styles.resourceList}>
              <li><Link className={styles.resourceLink} to={`/seasons/${season.slug}/people`}><span><IconUsers size={18} aria-hidden="true" /> People directory</span><span aria-hidden="true">→</span></Link></li>
              <li><Link className={styles.resourceLink} to={`/seasons/${season.slug}/mock-interviews`}><span><IconCalendarEvent size={18} aria-hidden="true" /> Mock interviews</span><span aria-hidden="true">→</span></Link></li>
              <li><ExternalLink href="https://leetcode.com/problemset/">LeetCode problem catalogue</ExternalLink></li>
            </ul>
          </div>
        </section>
      </div>
    </div>
  );
}
