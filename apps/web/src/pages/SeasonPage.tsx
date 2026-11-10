import { StatusBadge } from '@/components/StatusBadge';
import { isBlankRichText } from '@/richText';
import { Card, CardContent } from '@/components/ui/card';
import {
  IconArrowRight,
  IconCode,
  IconCircleCheckFilled,
  IconExternalLink,
  IconLock,
  IconMessage2,
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
  MetricCard,
  Page,
  PageHeader,
  PageSection,
  usePageTitle,
} from '@/components/Common';
import { SeasonEditMenu } from '@/components/SeasonManagement';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import { seasonAccess } from '@/seasonAccess';
import { formatDate } from '@/utils';

const inlineClass = 'flex flex-wrap items-center gap-2.5';
const helperClass = 'text-sm text-muted-foreground';
// Panels sit 1.5rem apart, as everywhere else (see DESIGN.md).
const metricGridClass = 'grid grid-cols-2 gap-6';
const resourceLinkClass =
  'flex min-h-11 items-center justify-between gap-4 py-3 font-medium text-foreground hover:underline';

export function SeasonPage() {
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
      <Page>
        <ErrorState
          onRetry={() => void Promise.all([query.refetch(), user.refetch()])}
        />
      </Page>
    );
  if (!season)
    return (
      <Page>
        <ErrorState
          title="Season Not Found"
          message="This season may have moved or you may not have access."
        />
      </Page>
    );

  const { hasSeasonAccess, isGlobalManager, isCoordinator } = seasonAccess(
    user.data,
    season.id,
  );
  if (!hasSeasonAccess) return <Navigate to="/forbidden" replace />;
  const canEditDefinition = season.status === 'open' && isGlobalManager;
  const canEditResources =
    season.status === 'open' && (isGlobalManager || isCoordinator);
  const canClose =
    season.status === 'open' && (isGlobalManager || isCoordinator);
  const canReopen = season.status === 'closed' && isGlobalManager;
  const membership = user.data?.seasonRoles.find(
    (entry) => entry.seasonId === season.id,
  );
  const retainedMembership =
    membership?.state === 'active' || membership?.state === 'completed';
  const canViewStudents =
    isGlobalManager ||
    (retainedMembership &&
      (membership?.role === 'mentor' || membership?.role === 'coordinator'));
  const canViewUpdates =
    isGlobalManager ||
    isCoordinator ||
    (retainedMembership &&
      (membership?.role === 'student' || membership?.role === 'mentor'));
  const now = Date.now();
  const orderedWeeks = [...(weeks.data?.items ?? [])].sort(
    (left, right) => left.number - right.number,
  );
  const currentWeek = orderedWeeks.find(
    (week) =>
      new Date(week.startAt).getTime() <= now &&
      now <= new Date(week.endAt).getTime(),
  );

  return (
    <Page>
      <PageHeader
        title={season.name}
        status={
          <StatusBadge
            tone={season.status === 'open' ? 'success' : 'muted'}
            className="h-7 px-2.5 text-sm"
          >
            {season.status === 'open' ? 'Open' : 'Closed'}
          </StatusBadge>
        }
        actions={
          canEditDefinition || canEditResources || canClose || canReopen ? (
            <SeasonEditMenu
              season={season}
              canEditDefinition={canEditDefinition}
              canEditResources={canEditResources}
              canClose={canClose}
              canReopen={canReopen}
            />
          ) : undefined
        }
      />
      <div className="flex flex-col gap-6">
        {season.status === 'closed' ? (
          <div>
            <InlineNotice tone="warning">
              <IconLock size={19} aria-hidden="true" /> This season is
              read-only. You can still log LeetCode attempts and mock interviews
              outside this season.
            </InlineNotice>
          </div>
        ) : null}
        <div className={metricGridClass}>
          <MetricCard
            label="Students"
            value={
              enrollments.data?.items.filter(
                (member) => member.role === 'student',
              ).length ?? '—'
            }
          />
          <MetricCard
            label="Program Weeks"
            value={weeks.data?.totalCount ?? '—'}
          />
        </div>
        <div className={metricGridClass}>
          <MetricCard
            label="Leetcodes"
            value={activity.data?.attemptCount ?? '—'}
          />
          <MetricCard
            label="Mock Interviews"
            value={activity.data?.mockInterviewCount ?? '—'}
          />
        </div>
        {activity.isError ? (
          <ErrorState
            title="Activity Totals Unavailable"
            onRetry={() => void activity.refetch()}
          />
        ) : null}
        <div className="grid items-start gap-6 min-[56rem]:grid-cols-2">
          <PageSection titleId="schedule-title" title="Program Schedule">
            <Card>
              <CardContent>
                {weeks.isLoading ? (
                  <p role="status">Loading program weeks…</p>
                ) : weeks.isError ? (
                  <ErrorState
                    title="Schedule Unavailable"
                    message="The program schedule could not be loaded."
                    onRetry={() => void weeks.refetch()}
                  />
                ) : orderedWeeks.length === 0 ? (
                  <p className={helperClass}>
                    No weeks have been scheduled for this season.
                  </p>
                ) : (
                  <ol className="m-0 grid list-none gap-0 p-0">
                    {orderedWeeks.map((week) => {
                      const done = new Date(week.endAt).getTime() < now;
                      const current = week.id === currentWeek?.id;
                      return (
                        <li
                          className="relative grid grid-cols-[2.2rem_1fr] gap-3 pb-4 last:pb-0 before:absolute before:top-4 before:bottom-[-0.2rem] before:left-[0.45rem] before:w-0.5 before:bg-border before:content-[''] last:before:hidden"
                          key={week.id}
                          aria-current={
                            week.id === currentWeek?.id ? 'date' : undefined
                          }
                        >
                          <WeekMarker done={done} current={current} />
                          <div className="min-w-0">
                            <div className="flex flex-wrap items-center gap-2">
                              <h3 className="text-base font-semibold">
                                Week {week.number}
                              </h3>
                            </div>
                            <p className="text-sm">
                              {formatDate(week.startAt)} –{' '}
                              {formatDate(week.endAt)}
                            </p>
                            {!isBlankRichText(week.resourcesHtml) ? (
                              <Link
                                className="mt-1.5 inline-flex items-center gap-1.5 text-sm font-medium text-foreground underline-offset-4 hover:underline"
                                to={`/seasons/${season.slug}/weeks/${week.number}`}
                              >
                                Week Resources
                                <IconArrowRight size={15} aria-hidden="true" />
                              </Link>
                            ) : canEditResources ? (
                              <Link
                                className="mt-1.5 inline-flex items-center gap-1.5 text-sm font-medium text-foreground underline-offset-4 hover:underline"
                                to={`/seasons/${season.slug}/weeks/${week.number}`}
                              >
                                Add Resources
                                <IconArrowRight size={15} aria-hidden="true" />
                              </Link>
                            ) : (
                              <span className={helperClass}>
                                No resources yet
                              </span>
                            )}
                          </div>
                        </li>
                      );
                    })}
                  </ol>
                )}
              </CardContent>
            </Card>
          </PageSection>
          <PageSection titleId="season-actions-title" title="Season Links">
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
                        Directory
                      </span>
                      <IconArrowRight size={17} aria-hidden="true" />
                    </Link>
                  </li>
                  <li>
                    <Link
                      className={resourceLinkClass}
                      to={`/seasons/${season.slug}/leetcode`}
                    >
                      <span className={inlineClass}>
                        <IconCode size={18} aria-hidden="true" /> LeetCode
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
                        <IconMessage2 size={18} aria-hidden="true" /> Mock
                        Interviews
                      </span>
                      <IconArrowRight size={17} aria-hidden="true" />
                    </Link>
                  </li>
                  {canViewUpdates ? (
                    <li>
                      <Link
                        className={resourceLinkClass}
                        to={`/seasons/${season.slug}/monday-updates`}
                      >
                        <span className={inlineClass}>
                          <IconNotebook size={18} aria-hidden="true" /> Monday
                          Updates
                        </span>
                        <IconArrowRight size={17} aria-hidden="true" />
                      </Link>
                    </li>
                  ) : null}
                  {canViewStudents &&
                  !isGlobalManager &&
                  membership?.role === 'mentor' ? (
                    <li>
                      <Link
                        className={resourceLinkClass}
                        to={`/seasons/${season.slug}/mentees`}
                      >
                        <span className={inlineClass}>
                          <IconUsers size={18} aria-hidden="true" /> My Students
                        </span>
                        <IconArrowRight size={17} aria-hidden="true" />
                      </Link>
                    </li>
                  ) : null}
                  {isGlobalManager &&
                  membership?.role === 'mentor' &&
                  retainedMembership ? (
                    <li>
                      <Link
                        className={resourceLinkClass}
                        to={`/seasons/${season.slug}/my-students`}
                      >
                        <span className={inlineClass}>
                          <IconUsers size={18} aria-hidden="true" /> My Students
                        </span>
                        <IconArrowRight size={17} aria-hidden="true" />
                      </Link>
                    </li>
                  ) : null}
                  {isGlobalManager || isCoordinator ? (
                    <li>
                      <Link
                        className={resourceLinkClass}
                        to={`/seasons/${season.slug}/mentor-teams`}
                      >
                        <span className={inlineClass}>
                          <IconUsers size={18} aria-hidden="true" /> Mentor
                          Teams
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
                        <span className={inlineClass}>Season Resources</span>
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
                        LeetCode Problem Catalogue
                      </span>
                      <IconExternalLink size={17} aria-hidden="true" />
                    </a>
                  </li>
                </ul>
              </CardContent>
            </Card>
          </PageSection>
        </div>
      </div>
    </Page>
  );
}

/** Done weeks get a check, the current week a filled dot, future weeks an empty circle. */
function WeekMarker({ done, current }: { done: boolean; current: boolean }) {
  const label = done ? 'Completed' : current ? 'In progress' : 'Upcoming';
  const base = 'z-10 mt-0.5 size-4 shrink-0';
  return (
    <span className="flex">
      <span className="sr-only">{label}. </span>
      {done ? (
        <IconCircleCheckFilled className={base} aria-hidden="true" />
      ) : (
        <span
          className={`${base} rounded-full border-2 ${current ? 'border-success bg-success shadow-[inset_0_0_0_2px_var(--card)]' : 'border-foreground bg-card'}`}
          aria-hidden="true"
        />
      )}
    </span>
  );
}
