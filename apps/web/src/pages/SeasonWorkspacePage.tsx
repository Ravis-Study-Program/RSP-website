import { IconCalendarEvent, IconLock, IconUsers } from '@tabler/icons-react';
import { Link, Navigate, useParams } from 'react-router-dom';

import {
  useCurrentUser,
  useSeasonEnrollments,
  useSeasonWeeks,
  useSeasons,
} from '@/api/queries';
import {
  ExternalLink,
  MetricCard,
  PageHeader,
  usePageTitle,
} from '@/components/Common';
import {
  SeasonEditorDialog,
  SeasonLifecycleDialog,
  SeasonResourcesDialog,
} from '@/components/SeasonManagement';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import { formatDate } from '@/utils';

export function SeasonWorkspacePage() {
  const { slug } = useParams();
  const query = useSeasons();
  const season = query.data?.items.find((item) => item.slug === slug);
  const user = useCurrentUser();
  const weeks = useSeasonWeeks(season?.id);
  const enrollments = useSeasonEnrollments(season?.id);
  usePageTitle(season?.name ?? 'Season');

  if (query.isLoading || user.isLoading)
    return <PageSkeleton label="Loading season" />;
  if (query.isError || user.isError)
    return (
      <div className={styles.page}>
        <ErrorState
          onRetry={() => void Promise.all([query.refetch(), user.refetch()])}
        />
      </div>
    );
  if (!season)
    return (
      <div className={styles.page}>
        <ErrorState
          title="Season not found"
          message="This season may have moved or you may not have access."
        />
      </div>
    );

  const hasSeasonAccess = Boolean(
    user.data &&
    (user.data.globalRoles.length > 0 ||
      user.data.alumni ||
      user.data.seasonRoles.some(
        (membership) =>
          membership.seasonId === season.id && membership.state === 'active',
      )),
  );
  if (!hasSeasonAccess) return <Navigate to="/forbidden" replace />;

  const isGlobalManager = Boolean(
    user.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const isCoordinator = Boolean(
    user.data?.seasonRoles.some(
      (membership) =>
        membership.seasonId === season.id &&
        membership.role === 'coordinator' &&
        membership.state === 'active',
    ),
  );
  const canEditDefinition = season.status === 'open' && isGlobalManager;
  const canEditResources =
    season.status === 'open' && (isGlobalManager || isCoordinator);
  const canClose =
    season.status === 'open' && (isGlobalManager || isCoordinator);
  const canReopen = season.status === 'closed' && isGlobalManager;
  const now = Date.now();
  const orderedWeeks = [...(weeks.data?.items ?? [])].sort(
    (left, right) => left.number - right.number,
  );
  const currentWeek =
    orderedWeeks.find(
      (week) =>
        new Date(week.startAt).getTime() <= now &&
        now <= new Date(week.endAt).getTime(),
    ) ?? orderedWeeks.find((week) => new Date(week.startAt).getTime() > now);

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow={
          season.status === 'open' ? 'Active season' : 'Programme history'
        }
        title={season.name}
        description={season.summary}
        actions={
          <div className={styles.inline}>
            <span
              className={
                season.status === 'open'
                  ? styles.badgeSuccess
                  : styles.badgeNeutral
              }
            >
              {season.status === 'open' ? 'Open' : 'Closed'}
            </span>
            {canEditDefinition ? <SeasonEditorDialog season={season} /> : null}
            {canEditResources ? (
              <SeasonResourcesDialog season={season} />
            ) : null}
            {canClose ? (
              <SeasonLifecycleDialog season={season} action="close" />
            ) : null}
            {canReopen ? (
              <SeasonLifecycleDialog season={season} action="reopen" />
            ) : null}
          </div>
        }
      />
      {season.status === 'closed' ? (
        <InlineNotice tone="warning">
          <IconLock size={19} aria-hidden="true" /> This season is read-only.
          You can still create global practice attempts and mock interviews.
        </InlineNotice>
      ) : null}
      <div className={styles.metricGrid}>
        <MetricCard
          label="Members"
          value={enrollments.data?.totalCount ?? '—'}
          detail="Across all season roles"
        />
        <MetricCard
          label="Programme weeks"
          value={weeks.data?.totalCount ?? '—'}
          detail={`${formatDate(season.startsAt)} – ${formatDate(season.endsAt)}`}
        />
        <MetricCard
          label="Current focus"
          value={currentWeek ? `Week ${currentWeek.number}` : '—'}
          detail={
            currentWeek
              ? `${formatDate(currentWeek.startAt)} – ${formatDate(currentWeek.endAt)}`
              : 'No current or upcoming week'
          }
        />
      </div>
      <div className={styles.grid2}>
        <section className={styles.section} aria-labelledby="schedule-title">
          <div className={styles.sectionHeader}>
            <h2 id="schedule-title" className={styles.sectionTitle}>
              Programme schedule
            </h2>
            <Link to={`/seasons/${season.slug}/practice`}>Open practice</Link>
          </div>
          <div className={styles.panel}>
            {weeks.isLoading ? (
              <p role="status">Loading programme weeks…</p>
            ) : weeks.isError ? (
              <ErrorState
                title="Schedule unavailable"
                message="The programme schedule could not be loaded."
                onRetry={() => void weeks.refetch()}
              />
            ) : orderedWeeks.length === 0 ? (
              <p className={styles.muted}>
                No weeks have been scheduled for this season.
              </p>
            ) : (
              <ol className={styles.timeline}>
                {orderedWeeks.map((week) => (
                  <li className={styles.timelineItem} key={week.id}>
                    <span className={styles.timelineDot} aria-hidden="true" />
                    <div className={styles.timelineContent}>
                      <h3>Week {week.number}</h3>
                      <p>
                        {formatDate(week.startAt)} – {formatDate(week.endAt)}
                      </p>
                      {week.resourceUrl ? (
                        <ExternalLink href={week.resourceUrl}>
                          Open week resources
                        </ExternalLink>
                      ) : (
                        <span className={styles.helper}>No resource link</span>
                      )}
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </div>
        </section>
        <section
          className={styles.section}
          aria-labelledby="season-actions-title"
        >
          <div className={styles.sectionHeader}>
            <h2 id="season-actions-title" className={styles.sectionTitle}>
              Workspace
            </h2>
          </div>
          <div className={styles.panel}>
            <ul className={styles.resourceList}>
              <li>
                <Link
                  className={styles.resourceLink}
                  to={`/seasons/${season.slug}/people`}
                >
                  <span>
                    <IconUsers size={18} aria-hidden="true" /> People directory
                  </span>
                  <span aria-hidden="true">→</span>
                </Link>
              </li>
              <li>
                <Link
                  className={styles.resourceLink}
                  to={`/seasons/${season.slug}/mock-interviews`}
                >
                  <span>
                    <IconCalendarEvent size={18} aria-hidden="true" /> Mock
                    interviews
                  </span>
                  <span aria-hidden="true">→</span>
                </Link>
              </li>
              {season.resourcesUrl ? (
                <li>
                  <ExternalLink href={season.resourcesUrl}>
                    Season resources
                  </ExternalLink>
                </li>
              ) : null}
              <li>
                <ExternalLink href="https://leetcode.com/problemset/">
                  LeetCode problem catalogue
                </ExternalLink>
              </li>
            </ul>
          </div>
        </section>
      </div>
    </div>
  );
}
