import { Badge } from '@/components/ui/badge';
import { buttonVariants } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  IconArrowRight,
  IconCalendarEvent,
  IconExternalLink,
  IconLock,
  IconNotebook,
  IconUsers,
} from '@tabler/icons-react';
import { Link, Navigate, useParams } from 'react-router-dom';

import {
  useActivitySummary,
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
import { formatDate } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const sectionClass = 'mt-6';
const sectionHeaderClass = 'mb-3 flex items-end justify-between gap-4';
const sectionTitleClass = 'text-xl font-semibold tracking-tight';
const inlineClass = 'flex flex-wrap items-center gap-2.5';
const helperClass = 'text-sm text-muted-foreground';
const statusBadgeClass = 'h-5 rounded-4xl px-2 text-xs font-medium';
const badgeSuccessClass = 'bg-success-soft text-success';
const badgeNeutralClass = 'bg-muted text-muted-foreground';
const metricGridClass =
  'mt-4 grid grid-cols-2 gap-3 min-[56rem]:grid-cols-3 min-[56rem]:gap-4';
const resourceLinkClass =
  'flex min-h-11 items-center justify-between gap-4 py-3 font-medium text-foreground hover:underline';

export function SeasonWorkspacePage() {
  const { slug } = useParams();
  const query = useSeasons();
  const season = query.data?.items.find((item) => item.slug === slug);
  const user = useCurrentUser();
  const weeks = useSeasonWeeks(season?.id);
  const enrollments = useSeasonEnrollments(season?.id);
  const activity = useActivitySummary(undefined, season?.id);
  usePageTitle(season?.name ?? 'Season');

  if (query.isLoading || user.isLoading)
    return <PageSkeleton label="Loading season" />;
  if (query.isError || user.isError)
    return (
      <div className={pageClass}>
        <ErrorState
          onRetry={() => void Promise.all([query.refetch(), user.refetch()])}
        />
      </div>
    );
  if (!season)
    return (
      <div className={pageClass}>
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
            membership.state === 'active' ||
            (membership.seasonId === season.id &&
              membership.state === 'completed'),
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
    <div className={pageClass}>
      <PageHeader
        eyebrow={
          season.status === 'open' ? 'Active season' : 'Programme history'
        }
        title={season.name}
        description={season.summary}
        actions={
          <div className={inlineClass}>
            <Badge
              variant="secondary"
              className={`${statusBadgeClass} ${
                season.status === 'open' ? badgeSuccessClass : badgeNeutralClass
              }`}
            >
              {season.status === 'open' ? 'Open' : 'Closed'}
            </Badge>
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
        <div className="mt-4">
          <InlineNotice tone="warning">
            <IconLock size={19} aria-hidden="true" /> This season is read-only.
            You can still create global practice attempts and mock interviews.
          </InlineNotice>
        </div>
      ) : null}
      <div className={metricGridClass}>
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
      <div className={metricGridClass}>
        <MetricCard
          label="Practice attempts"
          value={activity.data?.attemptCount ?? '—'}
          detail="Recorded in this season"
        />
        <MetricCard
          label="Mock interviews"
          value={activity.data?.mockInterviewCount ?? '—'}
          detail="Each interview counted once"
        />
        <MetricCard
          label="Last recorded activity"
          value={
            activity.data?.lastActivityAt
              ? formatDate(activity.data.lastActivityAt)
              : '—'
          }
          detail="Within this season"
        />
      </div>
      {activity.isError ? (
        <ErrorState
          title="Activity totals unavailable"
          onRetry={() => void activity.refetch()}
        />
      ) : null}
      <div className="mt-4 grid items-start gap-6 min-[56rem]:grid-cols-2">
        <section className={sectionClass} aria-labelledby="schedule-title">
          <div className="mb-3 flex items-center justify-between gap-4">
            <h2 id="schedule-title" className={sectionTitleClass}>
              Programme schedule
            </h2>
            <Link
              to={`/seasons/${season.slug}/practice`}
              className={buttonVariants({ variant: 'outline', size: 'sm' })}
            >
              Open practice
            </Link>
          </div>
          <Card>
            <CardContent>
              {weeks.isLoading ? (
                <p role="status">Loading programme weeks…</p>
              ) : weeks.isError ? (
                <ErrorState
                  title="Schedule unavailable"
                  message="The programme schedule could not be loaded."
                  onRetry={() => void weeks.refetch()}
                />
              ) : orderedWeeks.length === 0 ? (
                <p className={helperClass}>
                  No weeks have been scheduled for this season.
                </p>
              ) : (
                <ol className="m-0 grid list-none gap-0 p-0">
                  {orderedWeeks.map((week) => (
                    <li
                      className="relative grid grid-cols-[2.2rem_1fr] gap-3 pb-4 last:pb-0 before:absolute before:top-4 before:bottom-[-0.2rem] before:left-[0.45rem] before:w-0.5 before:bg-border before:content-[''] last:before:hidden"
                      key={week.id}
                    >
                      <span
                        className="z-10 mt-1 size-3.5 rounded-sm bg-primary"
                        aria-hidden="true"
                      />
                      <div className="min-w-0">
                        <h3 className="text-base font-semibold">
                          Week {week.number}
                        </h3>
                        <p className="text-sm">
                          {formatDate(week.startAt)} – {formatDate(week.endAt)}
                        </p>
                        {week.resourceUrl ? (
                          <ExternalLink href={week.resourceUrl}>
                            Open week resources
                          </ExternalLink>
                        ) : (
                          <span className={helperClass}>No resource link</span>
                        )}
                      </div>
                    </li>
                  ))}
                </ol>
              )}
            </CardContent>
          </Card>
        </section>
        <section
          className={sectionClass}
          aria-labelledby="season-actions-title"
        >
          <div className={sectionHeaderClass}>
            <h2 id="season-actions-title" className={sectionTitleClass}>
              Workspace
            </h2>
          </div>
          <Card>
            <CardContent>
              <ul className="m-0 list-none divide-y p-0">
                <li>
                  <Link
                    className={resourceLinkClass}
                    to={`/seasons/${season.slug}/people`}
                  >
                    <span className={inlineClass}>
                      <IconUsers size={18} aria-hidden="true" /> People
                      directory
                    </span>
                    <IconArrowRight size={17} aria-hidden="true" />
                  </Link>
                </li>
                <li>
                  <Link
                    className={resourceLinkClass}
                    to={`/seasons/${season.slug}/mock-interviews`}
                  >
                    <span className={inlineClass}>
                      <IconCalendarEvent size={18} aria-hidden="true" /> Mock
                      interviews
                    </span>
                    <IconArrowRight size={17} aria-hidden="true" />
                  </Link>
                </li>
                {isGlobalManager ? (
                  <li>
                    <Link
                      className={resourceLinkClass}
                      to={`/seasons/${season.slug}/monday-updates`}
                    >
                      <span className={inlineClass}>
                        <IconNotebook size={18} aria-hidden="true" /> Monday
                        updates
                      </span>
                      <IconArrowRight size={17} aria-hidden="true" />
                    </Link>
                  </li>
                ) : null}
                {season.resourcesUrl ? (
                  <li>
                    <a
                      className={resourceLinkClass}
                      href={season.resourcesUrl}
                      target="_blank"
                      rel="noreferrer"
                    >
                      <span className={inlineClass}>Season resources</span>
                      <IconExternalLink size={17} aria-hidden="true" />
                    </a>
                  </li>
                ) : null}
                <li>
                  <a
                    className={resourceLinkClass}
                    href="https://leetcode.com/problemset/"
                    target="_blank"
                    rel="noreferrer"
                  >
                    <span className={inlineClass}>
                      LeetCode problem catalogue
                    </span>
                    <IconExternalLink size={17} aria-hidden="true" />
                  </a>
                </li>
              </ul>
            </CardContent>
          </Card>
        </section>
      </div>
    </div>
  );
}
