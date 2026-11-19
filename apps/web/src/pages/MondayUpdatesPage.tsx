import { useSeasonSwitcher } from '@/components/useSeasonSwitcher';
import { Tooltip } from '@/components/ui/tooltip';
import { StatusBadge } from '@/components/StatusBadge';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import type { ColumnDef, Row } from '@tanstack/react-table';
import {
  IconDotsVertical,
  IconEdit,
  IconListDetails,
  IconLock,
  IconLockOpen,
  IconPencil,
  IconTrash,
  IconX,
} from '@tabler/icons-react';
import { useMemo, useState, type FormEvent, type ReactNode } from 'react';
import {
  Link,
  Navigate,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';

import {
  defaultMondayUpdateRange,
  completedMondayUpdateWeeks,
  startedMondayUpdateWeeks,
  mondayUpdateWeeks,
  sameUpdateWeek,
  type MondayUpdateRange,
} from '@/activityDates';
import { apiRequest } from '@/api/client';
import type {
  MondayUpdate,
  MondayUpdateActivity,
  MondayUpdateEditMutation,
  MondayUpdateMark,
  MondayUpdateMarkMutation,
  MondayUpdateMarkRecord,
  MondayUpdateMutation,
  MondayUpdateUnlock,
  MondayUpdateUnlockMutation,
  MondayMentorReport,
  MondayMentorReportStudent,
  MondayWeekMark,
  MondayWeekMarkMutation,
} from '@/api/generated/models';
import {
  demoMode,
  useCurrentUser,
  useMondayMentorReports,
  useMondayUpdates,
  useMondayUpdateRangeActivity,
  useMondayUpdateUnlocks,
  useSeasonMentorships,
  useSeasonPeople,
  useSeasons,
  useSeasonWeeks,
  useWeeklyHours,
  useWeeklyTargets,
  type MondayUpdateFilters,
} from '@/api/queries';
import {
  Page,
  PageHeader,
  PageSection,
  StudentLevelBadge,
  usePageTitle,
  LabelHint,
} from '@/components/Common';
import { DataTable, ExpandRowButton } from '@/components/DataTable';
import {
  FormDialog,
  NamedConfirmation,
  DialogCancel,
} from '@/components/Dialogs';
import { cn } from '@/lib/utils';
import { RichTextContent } from '@/components/RichTextContent';
import {
  pastDue,
  updateStatusTones,
  weekStatus,
  type StudentUpdateStatus,
} from '@/mondayStatus';
import { InfoTip } from '@/components/InfoTip';
import { RichTextEditor } from '@/components/RichTextEditor';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import { studentLevelLabel } from '@/studentLevels';
import type { Person } from '@/types';

type StudentLevel = NonNullable<Person['studentLevel']>;
import {
  formatDate,
  formatDateTime,
  meetsTarget,
  weeklyTargetsFor,
  type WeeklyTargets,
} from '@/utils';
import { MarkBadge } from '@/components/MondayMark';
import { markLabel, markOptions, markTone } from '@/mondayMarks';

type Experience = 'student' | 'mentor' | 'reviewer';

const cardTitleClass = 'text-base font-semibold tracking-tight';
const subTitleClass = 'text-sm font-semibold';
const mutedClass = 'text-sm text-muted-foreground';
const rowClass = 'flex flex-wrap items-center gap-2';
const metaListClass =
  'm-0 grid list-none gap-2 p-0 text-sm text-muted-foreground';
const formClass = 'grid gap-4';

const defaultHours = '0';

/** Hours are 0–168 in half-hour steps; returns null when the input is invalid. */
function parseHours(value: string): number | null {
  if (value.trim() === '') return null;
  const hours = Number(value);
  if (!Number.isFinite(hours) || hours < 0 || hours > 168) return null;
  return Number.isInteger(hours * 2) ? hours : null;
}

const markSelectItems: { label: string; value: MondayUpdateMark | '' }[] = [
  { label: 'Not marked', value: '' },
  ...markOptions,
];

function updateRangeLabel(update: { startDate: string; endDate: string }) {
  return `${formatDate(update.startDate)} – ${formatDate(update.endDate)}`;
}

/** Rich text counts as empty when it carries no text, only markup. */
function hasContent(html: string) {
  return (
    html
      .replace(/<[^>]*>/g, ' ')
      .replace(/&nbsp;/gi, ' ')
      .trim().length > 0
  );
}

function invalidateMondayUpdates(client: QueryClient) {
  return Promise.all([
    client.invalidateQueries({ queryKey: ['monday-updates'] }),
    client.invalidateQueries({ queryKey: ['monday-update-review'] }),
    client.invalidateQueries({ queryKey: ['monday-update-unlocks'] }),
    client.invalidateQueries({ queryKey: ['monday-mentor-reports'] }),
  ]);
}

export function MondayUpdatesPage() {
  const { slug } = useParams();
  const seasons = useSeasons();
  const user = useCurrentUser();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const weeks = useSeasonWeeks(season?.id);
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();

  const actor = user.data;
  const membership = actor?.seasonRoles.find(
    (item) => item.seasonId === season?.id,
  );
  const isGlobalManager = Boolean(
    actor?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  // An admin who also mentors in this season can switch to their own mentees.
  const mentoringHere =
    membership?.role === 'mentor' &&
    (membership.state === 'active' || membership.state === 'completed');
  const canSwitchView = isGlobalManager && mentoringHere;
  const mineView = canSwitchView && searchParams.get('view') === 'mine';
  const viewParam: Record<string, string> = mineView ? { view: 'mine' } : {};
  const experience: Experience | null = mineView
    ? 'mentor'
    : isGlobalManager ||
        (membership?.role === 'coordinator' && membership.state === 'active')
      ? 'reviewer'
      : membership?.role === 'mentor' &&
          (membership.state === 'active' || membership.state === 'completed')
        ? 'mentor'
        : membership?.role === 'student'
          ? 'student'
          : null;

  const availableWeeks =
    experience === 'student'
      ? startedMondayUpdateWeeks(weeks.data?.items ?? [])
      : mondayUpdateWeeks(weeks.data?.items ?? []);
  const completedWeeks = completedMondayUpdateWeeks(weeks.data?.items ?? []);
  // Every week that has started, the current one included, can be written.
  const startedWeeks = startedMondayUpdateWeeks(weeks.data?.items ?? []);
  const selectedWeek =
    availableWeeks.find((week) => week.id === searchParams.get('week')) ??
    completedWeeks.at(-1) ??
    availableWeeks[0];
  const allHistory =
    experience === 'mentor' &&
    (searchParams.get('week') === 'all' || availableWeeks.length === 0);
  const historyRange = { startDate: '1900-01-01', endDate: '9999-12-31' };
  const range = allHistory
    ? historyRange
    : (selectedWeek ?? defaultMondayUpdateRange());
  const queryRange = experience === 'student' ? historyRange : range;

  const mentorships = useSeasonMentorships(
    experience === 'mentor' ? season?.id : undefined,
  );
  const people = useSeasonPeople(
    experience === 'mentor' ? season?.id : undefined,
  );
  const menteeIds = useMemo(() => {
    const mentorId = actor?.id;
    if (!mentorId) return [];
    return [
      ...new Set(
        (mentorships.data?.items ?? [])
          .filter((mentorship) => mentorship.mentorUserId === mentorId)
          .map((mentorship) => mentorship.studentUserId),
      ),
    ];
  }, [mentorships.data, actor?.id]);
  // Every mentee is selected by default; narrowing it unchecks a student.
  // Every mentee is listed; the table's Student filter narrows it.
  const selectedStudentIds = menteeIds;
  const filters: MondayUpdateFilters = {
    startDate: queryRange.startDate,
    endDate: queryRange.endDate,
    studentIds: selectedStudentIds,
  };
  const updates = useMondayUpdates(
    experience === 'student' ||
      (experience === 'mentor' && selectedStudentIds.length > 0),
    season?.id,
    filters,
  );
  // The week mentors send to the coordinator: the chosen ended week.
  const reportWeek =
    !allHistory &&
    selectedWeek &&
    startedWeeks.some((week) => week.id === selectedWeek.id)
      ? selectedWeek
      : undefined;
  const reports = useMondayMentorReports(
    experience === 'reviewer' || experience === 'mentor'
      ? season?.id
      : undefined,
    reportWeek,
    experience === 'mentor' ? actor?.id : undefined,
  );
  const ownReport =
    experience === 'mentor'
      ? reports.data?.find((report) => report.mentor.id === actor?.id)
      : undefined;
  const unlocks = useMondayUpdateUnlocks(
    experience === 'student' || experience === 'mentor'
      ? season?.id
      : undefined,
  );
  usePageTitle('Monday Updates');

  if (seasons.isLoading || user.isLoading || weeks.isLoading)
    return <PageSkeleton label="Loading Monday updates" />;
  if (seasons.isError || user.isError || weeks.isError)
    return (
      <Page>
        <ErrorState
          onRetry={() =>
            void Promise.all([
              seasons.refetch(),
              user.refetch(),
              weeks.refetch(),
            ])
          }
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
  if (!actor || !experience) return <Navigate to="/forbidden" replace />;

  // The list is the actor's own updates; the filter keeps the demo fixtures to
  // the signed-in student as well.
  const ownUpdates = (updates.data?.items ?? []).filter(
    (update) => update.student.id === actor.id,
  );
  const updateFor = (week: MondayUpdateRange) =>
    ownUpdates.find((update) => sameUpdateWeek(update, week));
  // A week past its due date is locked unless a mentor unlocked it.
  const unlockFor = (studentId: string, week: MondayUpdateRange) =>
    (unlocks.data ?? []).find(
      (item) =>
        item.student.id === studentId &&
        item.startDate === week.startDate &&
        item.state !== 'declined',
    );
  const lockedFor = (studentId: string, week: MondayUpdateRange) =>
    pastDue(week) && unlockFor(studentId, week)?.state !== 'approved';
  // Updates belong to program weeks: one row per started week (the current
  // one included), newest
  // first, with its update if there is one.
  const studentRows: StudentUpdateRow[] = startedWeeks
    .map((week) => {
      const update = updateFor(week);
      return {
        id: week.id,
        weekNumber: week.number,
        range: week,
        update,
        status: weekStatus(week, update),
        unlock: unlockFor(actor.id, week),
        locked: lockedFor(actor.id, week),
      } satisfies StudentUpdateRow;
    })
    .reverse();
  // The week to write next: the one a reminder linked to, else the latest
  // completed week without an update.
  const writable = (week: MondayUpdateRange) =>
    !updateFor(week) && !lockedFor(actor.id, week);
  const linkedWeek = startedWeeks.find(
    (week) => week.id === searchParams.get('week') && writable(week),
  );
  const writeWeek = linkedWeek ?? [...startedWeeks].reverse().find(writable);
  const canPost = season.status === 'open' && membership?.state === 'active';
  // Mentors and coordinators pick the season here rather than switching
  // workspace: Directors and System Admins can open any season, everyone else
  // the seasons where they mentor or coordinate.
  const seasonItems =
    experience === 'mentor' || experience === 'reviewer'
      ? (seasons.data?.items ?? [])
          .filter(
            (item) =>
              isGlobalManager ||
              actor.seasonRoles.some(
                (entry) =>
                  entry.seasonId === item.id &&
                  ((entry.role === 'coordinator' && entry.state === 'active') ||
                    (entry.role === 'mentor' &&
                      (entry.state === 'active' ||
                        entry.state === 'completed'))),
              ),
          )
          .map((item) => ({ value: item.slug, label: item.name }))
      : [];
  const goToSeason = (nextSlug: string) =>
    navigate(`/seasons/${encodeURIComponent(nextSlug)}/monday-updates`);
  const weekItems = [
    ...(experience === 'mentor'
      ? [{ value: 'all', label: 'All updates' }]
      : []),
    ...availableWeeks.map((week) => ({
      value: week.id,
      label: `Week ${week.number}: ${updateRangeLabel(week)}${experience === 'student' && ownUpdates.some((update) => sameUpdateWeek(update, week)) ? ' (Submitted)' : ''}`,
    })),
  ];
  const updatesByStudent = new Map<string, MondayUpdate[]>();
  for (const update of updates.data?.items ?? []) {
    const current = updatesByStudent.get(update.student.id);
    if (current) current.push(update);
    else updatesByStudent.set(update.student.id, [update]);
  }
  const weekValue = allHistory ? 'all' : (selectedWeek?.id ?? 'all');
  const programWeeks = mondayUpdateWeeks(weeks.data?.items ?? []);
  const studentNames = new Map<string, string>();
  const studentSlugs = new Map<string, string>();
  const studentLevels = new Map<string, string>();
  for (const person of people.data?.items ?? []) {
    studentNames.set(person.id, person.name);
    studentSlugs.set(person.id, person.slug);
    if (person.studentLevel) studentLevels.set(person.id, person.studentLevel);
  }
  for (const update of updates.data?.items ?? []) {
    if (!studentNames.has(update.student.id))
      studentNames.set(update.student.id, update.student.name);
    if (!studentSlugs.has(update.student.id))
      studentSlugs.set(update.student.id, update.student.slug);
  }
  for (const student of ownReport?.students ?? []) {
    if (!studentNames.has(student.student.id))
      studentNames.set(student.student.id, student.student.name);
    if (!studentSlugs.has(student.student.id))
      studentSlugs.set(student.student.id, student.student.slug);
    if (!studentLevels.has(student.student.id))
      studentLevels.set(student.student.id, student.studentLevel);
  }
  const studentLabel = (studentId: string) =>
    studentNames.get(studentId) ?? studentSlugs.get(studentId) ?? studentId;
  // Each posted update, plus, for one ended week, the students who missed it.
  const missedWeek =
    !allHistory &&
    selectedWeek &&
    startedWeeks.some((week) => week.id === selectedWeek.id)
      ? selectedWeek
      : undefined;
  // The week's students come from the mentor's report when it has loaded,
  // which keeps a student kicked that week; otherwise current mentees.
  const reportStudents = ownReport?.students ?? [];
  const weekStudentIds = [
    ...new Set([
      ...menteeIds,
      ...reportStudents.map((student) => student.student.id),
    ]),
  ];
  const targetsFor = (studentId: string) => {
    const hours = reportStudents.find(
      (student) => student.student.id === studentId,
    )?.targets.hours;
    return hours ? weeklyTargetsFor(hours) : undefined;
  };
  const kickedAt = (studentId: string) =>
    reportStudents.find((student) => student.student.id === studentId)
      ?.kickedAt ?? undefined;
  const mentorRows: MentorUpdateRow[] = [
    ...(updates.data?.items ?? [])
      .filter((update) => weekStudentIds.includes(update.student.id))
      // Updates belong to program weeks; anything else is not shown.
      .flatMap((update) => {
        const week = programWeeks.find((item) => sameUpdateWeek(update, item));
        return week ? [{ update, week }] : [];
      })
      .map(({ update, week }) => {
        return {
          id: update.id,
          student: {
            id: update.student.id,
            name: studentLabel(update.student.id),
            slug: studentSlugs.get(update.student.id),
            level: studentLevels.get(update.student.id),
          },
          weekNumber: week.number,
          range: week,
          update,
          status: weekStatus(week, update),
          locked: lockedFor(update.student.id, week),
          unlock: unlockFor(update.student.id, week),
          targets: targetsFor(update.student.id),
        } satisfies MentorUpdateRow;
      }),
    ...(missedWeek
      ? (reportStudents.length
          ? reportStudents.map((student) => student.student.id)
          : menteeIds
        )
          .filter(
            (studentId) =>
              !(updatesByStudent.get(studentId) ?? []).some((update) =>
                sameUpdateWeek(update, missedWeek),
              ),
          )
          .map(
            (studentId) =>
              ({
                id: `${studentId}:${missedWeek.id}`,
                student: {
                  id: studentId,
                  name: studentLabel(studentId),
                  slug: studentSlugs.get(studentId),
                  level: studentLevels.get(studentId),
                },
                weekNumber: missedWeek.number,
                range: missedWeek,
                status: weekStatus(missedWeek, undefined, kickedAt(studentId)),
                locked: lockedFor(studentId, missedWeek),
                unlock: unlockFor(studentId, missedWeek),
                weekMark: (() => {
                  const student = ownReport?.students.find(
                    (item) => item.student.id === studentId,
                  );
                  return student?.mark
                    ? { mark: student.mark, commentHtml: student.commentHtml }
                    : undefined;
                })(),
              }) satisfies MentorUpdateRow,
          )
      : []),
  ].sort(
    (a, b) =>
      b.range.startDate.localeCompare(a.range.startDate) ||
      a.student.name.localeCompare(b.student.name),
  );

  return (
    <Page>
      <PageHeader
        title="Monday Updates"
        actions={
          experience === 'student' && canPost && writeWeek && updates.data ? (
            <WriteUpdateButton
              key={writeWeek.id}
              seasonId={season.id}
              week={writeWeek}
              student={actor}
              defaultOpen={writeWeek.id === searchParams.get('week')}
            />
          ) : canSwitchView ? (
            <div role="group" aria-label="Updates view" className="flex gap-2">
              <Button
                type="button"
                variant={mineView ? 'default' : 'outline'}
                aria-pressed={mineView}
                onClick={() => setSearchParams({ view: 'mine' })}
              >
                My Students
              </Button>
              <Button
                type="button"
                variant={mineView ? 'outline' : 'default'}
                aria-pressed={!mineView}
                onClick={() => setSearchParams({})}
              >
                All Students
              </Button>
            </div>
          ) : null
        }
      />
      {experience === 'student' ? (
        <>
          {!canPost ? (
            <p className={`${mutedClass} mb-3`}>Updates are read-only.</p>
          ) : null}
          <StudentUpdateTable
            seasonId={season.id}
            rows={studentRows}
            student={actor}
            canPost={canPost}
            loading={updates.isLoading}
            error={updates.isError}
            onRetry={() => void updates.refetch()}
          />
        </>
      ) : experience === 'mentor' ? (
        <div className="flex flex-col gap-6">
          <MentorUpdateTable
            showWeek={allHistory}
            seasonId={season.id}
            mentorId={actor.id}
            rows={mentorRows}
            weekSelect={
              <WeekSelect
                id="monday-week"
                ariaLabel="Program week"
                items={weekItems}
                value={weekValue}
                onChange={(value) =>
                  setSearchParams({ ...viewParam, week: value })
                }
                className="w-64"
              />
            }
            weekFilter={{
              content: (
                <>
                  <Field>
                    <FieldLabel htmlFor="monday-season-filter">
                      Season
                    </FieldLabel>
                    <WeekSelect
                      id="monday-season-filter"
                      items={seasonItems}
                      value={season.slug}
                      onChange={goToSeason}
                      className="w-full"
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="monday-week-filter">Week</FieldLabel>
                    <WeekSelect
                      id="monday-week-filter"
                      items={weekItems}
                      value={weekValue}
                      onChange={(value) =>
                        setSearchParams({ ...viewParam, week: value })
                      }
                      className="w-full"
                    />
                  </Field>
                </>
              ),
              active: searchParams.has('week'),
              onClear: () => setSearchParams(viewParam),
            }}
            loading={updates.isLoading || mentorships.isLoading}
            error={updates.isError || mentorships.isError}
            onRetry={() =>
              void Promise.all([updates.refetch(), mentorships.refetch()])
            }
          />
          <UnlockRequestsTable
            seasonId={season.id}
            requests={(unlocks.data ?? []).filter(
              (item) =>
                item.state === 'pending' && menteeIds.includes(item.student.id),
            )}
            weekNumberOf={(request) =>
              programWeeks.find((week) => sameUpdateWeek(request, week))?.number
            }
          />
        </div>
      ) : (
        <MentorReportsTable
          reports={reports.data ?? []}
          week={reportWeek}
          weekSelect={
            <WeekSelect
              id="monday-week"
              ariaLabel="Program week"
              items={weekItems}
              value={weekValue}
              onChange={(value) =>
                setSearchParams({ ...viewParam, week: value })
              }
              className="w-64"
            />
          }
          seasonFilter={
            <Field>
              <FieldLabel htmlFor="monday-season-filter">Season</FieldLabel>
              <WeekSelect
                id="monday-season-filter"
                items={seasonItems}
                value={season.slug}
                onChange={goToSeason}
                className="w-full"
              />
            </Field>
          }
          loading={reports.isLoading}
          error={reports.isError}
          onRetry={() => void reports.refetch()}
        />
      )}
    </Page>
  );
}

/** One row of the student's table: a completed program week. */
type StudentUpdateRow = {
  id: string;
  weekNumber: number;
  range: MondayUpdateRange;
  update?: MondayUpdate;
  status: StudentUpdateStatus;
  /** Past its due date and not unlocked: it cannot be posted or changed. */
  locked: boolean;
  unlock?: MondayUpdateUnlock;
};

function rowName(row: StudentUpdateRow) {
  return `Week ${row.weekNumber}`;
}

const difficultyDots = [
  { key: 'easy', label: 'Easy', dot: 'bg-success' },
  { key: 'medium', label: 'Medium', dot: 'bg-warning' },
  { key: 'hard', label: 'Hard', dot: 'bg-destructive' },
] as const;

/** Solved LeetCodes as easy · medium · hard counts, each by its colour dot. */
function SolvedByDifficulty({
  activity,
  targets,
}: {
  activity: MondayUpdateActivity;
  targets?: WeeklyTargets;
}) {
  const counts = { easy: 0, medium: 0, hard: 0 };
  for (const attempt of activity.attempts)
    if (attempt.outcome === 'solved' && attempt.difficulty in counts)
      counts[attempt.difficulty as keyof typeof counts] += 1;
  return <SolvedCounts counts={counts} targets={targets} />;
}

/** A week's mock interviews, green once they meet the weekly target. */
function MockCountCell({ count, target }: { count: number; target?: number }) {
  return (
    <span
      className={cn(
        'tabular-nums',
        meetsTarget(count, target) && 'font-semibold text-success',
      )}
      title={target ? `Target: ${target} mock interviews` : undefined}
    >
      {count}
    </span>
  );
}

/** A week's hours, green once they meet the student's weekly hours. */
function HoursCell({ hours, target }: { hours: number; target?: number }) {
  return (
    <span
      className={cn(
        'tabular-nums',
        meetsTarget(hours, target) && 'font-semibold text-success',
      )}
      title={target ? `Target: ${target} hours` : undefined}
    >
      {hours}
    </span>
  );
}

function solvedCount(activity: MondayUpdateActivity) {
  return activity.attempts.filter((attempt) => attempt.outcome === 'solved')
    .length;
}

/**
 * Every program week so far, the current one included, and the update
 * posted for it, newest first.
 * A row expands to the update; writing, editing and deleting live in its menu.
 */
function StudentUpdateTable(props: StudentUpdateTableProps) {
  // A student's own targets colour their counts.
  const targets = useWeeklyTargets();
  return <StudentUpdateTableBody {...props} targets={targets} />;
}

type StudentUpdateTableProps = {
  seasonId: string;
  rows: StudentUpdateRow[];
  student: MondayUpdate['student'];
  canPost: boolean;
  loading: boolean;
  error: boolean;
  onRetry: () => void;
};

function StudentUpdateTableBody({
  seasonId,
  rows,
  student,
  canPost,
  loading,
  error,
  onRetry,
  targets,
}: StudentUpdateTableProps & { targets: WeeklyTargets }) {
  const actions = (row: Row<StudentUpdateRow>) => (
    <StudentUpdateRowActions
      row={row}
      seasonId={seasonId}
      student={student}
      canPost={canPost}
    />
  );
  // Filters are by week and status only, each a dropdown.
  const weekOptions = rows.map((row) => ({
    label: rowName(row),
    value: rowName(row),
  }));
  const columns: ColumnDef<StudentUpdateRow, any>[] = [
    {
      id: 'expand',
      header: 'Details',
      meta: { width: 4.5 },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) =>
        row.original.update ? (
          <ExpandRowButton
            row={row}
            subject={`update for ${rowName(row.original)}`}
          />
        ) : null,
    },
    {
      id: 'week',
      header: 'Week',
      meta: { width: 5.5, filterOptions: weekOptions },
      // Filtered by exact week; sorted by week number.
      accessorFn: (row) => rowName(row),
      filterFn: 'equalsString',
      sortingFn: (a, b) => a.original.weekNumber - b.original.weekNumber,
      cell: ({ row }) => (
        <span className="font-medium">{rowName(row.original)}</span>
      ),
    },
    {
      id: 'dates',
      header: 'Dates',
      meta: { width: 11 },
      accessorFn: (row) => row.range.startDate,
      enableColumnFilter: false,
      cell: ({ row }) => updateRangeLabel(row.original.range),
    },
    {
      id: 'status',
      header: 'Status',
      meta: {
        // Fits the longest pair: Submitted (Late) beside Unlock requested.
        width: 17.5,
        filterOptions: [
          { label: 'Submitted', value: 'Submitted' },
          { label: 'Submitted (Late)', value: 'Submitted (Late)' },
          { label: 'Not submitted', value: 'Not submitted' },
          { label: 'Overdue', value: 'Overdue' },
        ],
      },
      accessorFn: (row) => row.status,
      filterFn: 'equalsString',
      cell: ({ row }) => <UpdateStatus row={row.original} />,
    },
    {
      id: 'hours',
      header: 'Hours',
      meta: { width: 6 },
      accessorFn: (row) => row.update?.hours ?? -1,
      cell: ({ row }) =>
        row.original.update ? (
          <HoursCell hours={row.original.update.hours} target={targets.hours} />
        ) : (
          <Dash />
        ),
    },
    {
      id: 'leetcode',
      header: 'LeetCodes',
      meta: { width: 9 },
      accessorFn: (row) => (row.update ? solvedCount(row.update.activity) : -1),
      cell: ({ row }) =>
        row.original.update ? (
          <SolvedByDifficulty
            activity={row.original.update.activity}
            targets={targets}
          />
        ) : (
          <Dash />
        ),
    },
    {
      id: 'mocks',
      header: 'Mock Interviews',
      meta: { width: 8.5 },
      accessorFn: (row) => row.update?.activity.mocks.length ?? -1,
      cell: ({ row }) =>
        row.original.update ? (
          <MockCountCell
            count={row.original.update.activity.mocks.length}
            target={targets.mocks}
          />
        ) : (
          <Dash />
        ),
    },
    {
      id: 'lastUpdated',
      header: 'Last Updated',
      meta: { width: 10 },
      accessorFn: (row) =>
        row.update ? (row.update.editedAt ?? row.update.createdAt) : '',
      enableColumnFilter: false,
      cell: ({ row }) =>
        row.original.update ? (
          formatDateTime(
            row.original.update.editedAt ?? row.original.update.createdAt,
          )
        ) : (
          <Dash />
        ),
    },
    {
      id: 'actions',
      header: 'Actions',
      meta: { width: 6, align: 'end' },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => actions(row),
    },
  ];
  return (
    <DataTable
      ariaLabel="Your Monday updates"
      data={rows}
      columns={columns}
      loading={loading}
      error={error}
      onRetry={onRetry}
      emptyTitle="No Updates Yet"
      emptyMessage="Updates open after the first program week ends."
      getRowId={(row) => row.id}
      rowCanExpand={(row) => Boolean(row.update)}
      renderExpanded={(row) =>
        row.original.update ? (
          <StudentUpdateDetails update={row.original.update} />
        ) : null
      }
      renderCard={(row) => (
        <div className="grid gap-2.5">
          <div className="flex items-center justify-between gap-3">
            <div className="grid gap-0.5">
              <h3 className={cardTitleClass}>{rowName(row.original)}</h3>
              <span className={mutedClass}>
                {updateRangeLabel(row.original.range)}
              </span>
            </div>
            {actions(row)}
          </div>
          <div className={rowClass}>
            <UpdateStatus row={row.original} />
          </div>
          {row.original.update && row.getIsExpanded() ? (
            <StudentUpdateDetails update={row.original.update} />
          ) : null}
        </div>
      )}
    />
  );
}

function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

/** A posted update: what the student wrote; its hours and activity are in the row. */
function StudentUpdateDetails({ update }: { update: MondayUpdate }) {
  return (
    <div className="grid gap-1">
      <h4 className={detailLabelClass}>Your Update</h4>
      <RichTextContent html={update.contentHtml} empty="No written content." />
    </div>
  );
}

/** A week's menu: show the update, write or edit it, or delete it. */
function StudentUpdateRowActions({
  row,
  seasonId,
  student,
  canPost,
}: {
  row: Row<StudentUpdateRow>;
  seasonId: string;
  student: MondayUpdate['student'];
  canPost: boolean;
}) {
  const queryClient = useQueryClient();
  const { update, range, weekNumber, locked, unlock } = row.original;
  // Past its due date a week is locked: the student asks their mentor to
  // unlock it before writing or changing it.
  const canWrite = canPost && !update && !locked;
  const canChange = canPost && Boolean(update) && !locked;
  const canRequest = canPost && locked && !unlock;
  const [open, setOpen] = useState<'write' | 'delete' | 'unlock' | null>(null);
  const close = (next: boolean) => {
    if (!next) setOpen(null);
  };
  if (!update && !canWrite && !canRequest) return null;
  const label = `Week ${weekNumber}: ${updateRangeLabel(range)}`;
  const deleteUpdate = async () => {
    if (!update) return;
    if (import.meta.env.MODE !== 'production' && demoMode) {
      const { deleteDemoMondayUpdate } = await import('@/data/demo');
      deleteDemoMondayUpdate(update.id);
    } else
      await apiRequest<void>(
        `/seasons/${encodeURIComponent(seasonId)}/monday-updates/${encodeURIComponent(update.id)}`,
        { method: 'DELETE' },
      );
    await invalidateMondayUpdates(queryClient);
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${rowName(row.original)}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {update ? (
            <DropdownMenuItem onClick={row.getToggleExpandedHandler()}>
              <IconListDetails size={17} aria-hidden="true" />
              {row.getIsExpanded() ? 'Hide Details' : 'Show Details'}
            </DropdownMenuItem>
          ) : null}
          {canWrite || canChange ? (
            <DropdownMenuItem onClick={() => setOpen('write')}>
              {update ? (
                <>
                  <IconEdit size={17} aria-hidden="true" /> Edit Update
                </>
              ) : (
                <>
                  <IconPencil size={17} aria-hidden="true" /> Write Update
                </>
              )}
            </DropdownMenuItem>
          ) : null}
          {canRequest ? (
            <DropdownMenuItem onClick={() => setOpen('unlock')}>
              <IconLockOpen size={17} aria-hidden="true" /> Request Unlock
            </DropdownMenuItem>
          ) : null}
          {canChange ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('delete')}
            >
              <IconTrash size={17} aria-hidden="true" /> Delete Update
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {open === 'write' ? (
        <MondayUpdateDialog
          open
          onOpenChange={close}
          seasonId={seasonId}
          range={range}
          weekNumber={weekNumber}
          student={student}
          existing={update}
        />
      ) : null}
      {open === 'unlock' ? (
        <RequestUnlockDialog
          seasonId={seasonId}
          student={student}
          range={range}
          weekNumber={weekNumber}
          onOpenChange={close}
        />
      ) : null}
      {open === 'delete' ? (
        <NamedConfirmation
          open
          onOpenChange={close}
          name={label}
          actionLabel="Delete Update"
          description="This permanently removes the update. The practice and mock activity it covers stays in your history."
          onConfirm={deleteUpdate}
        />
      ) : null}
    </>
  );
}

/** The header's button for the next week to write, opening its dialog. */
function WriteUpdateButton({
  seasonId,
  week,
  student,
  defaultOpen,
}: {
  seasonId: string;
  week: MondayUpdateRange & { number: number };
  student: MondayUpdate['student'];
  defaultOpen: boolean;
}) {
  // A reminder links here with its week, so that week opens straight away.
  const [open, setOpen] = useState(defaultOpen);
  return (
    <>
      <Button type="button" onClick={() => setOpen(true)}>
        <IconPencil aria-hidden="true" /> Write Week {week.number} Update
      </Button>
      {open ? (
        <MondayUpdateDialog
          open
          onOpenChange={setOpen}
          seasonId={seasonId}
          range={week}
          weekNumber={week.number}
          student={student}
        />
      ) : null}
    </>
  );
}

/** What a Monday update can cover, behind an (i) beside its label. */
function WhatToInclude() {
  return (
    <InfoTip label="What to include in your update" size={16}>
      <p className="font-medium text-foreground">What to include</p>
      <ul className="mt-2 list-disc space-y-1 pl-5">
        <li>What you worked on.</li>
        <li>What you learned.</li>
        <li>Any blockers, or help you need.</li>
        <li>What you plan to work on next week.</li>
      </ul>
    </InfoTip>
  );
}

/** Writes a week's update, or edits one already posted, in a dialog. */
function MondayUpdateDialog({
  open,
  onOpenChange,
  ...form
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  seasonId: string;
  range: MondayUpdateRange;
  weekNumber: number;
  student: MondayUpdate['student'];
  existing?: MondayUpdate;
}) {
  const week = `Week ${form.weekNumber}`;
  return (
    <FormDialog
      title={form.existing ? `Edit ${week} Update` : `${week} Update`}
      description={updateRangeLabel(form.range)}
      open={open}
      onOpenChange={onOpenChange}
      contentClassName="sm:max-w-3xl"
    >
      <MondayUpdateForm {...form} onDone={() => onOpenChange(false)} />
    </FormDialog>
  );
}

function MondayUpdateForm({
  seasonId,
  range,
  student,
  existing,
  onDone,
}: {
  seasonId: string;
  range: MondayUpdateRange;
  student: MondayUpdate['student'];
  existing?: MondayUpdate;
  onDone: () => void;
}) {
  const queryClient = useQueryClient();
  const weeklyHours = useWeeklyHours();
  const [content, setContent] = useState(existing?.contentHtml ?? '');
  const [hoursInput, setHoursInput] = useState(
    existing ? String(existing.hours) : defaultHours,
  );
  const [pending, setPending] = useState(false);
  const [attempted, setAttempted] = useState(false);
  const [error, setError] = useState('');
  // Writing is optional; an editor holding only markup saves as empty.
  const contentHtml = hasContent(content) ? content : '';
  const hours = parseHours(hoursInput);
  // Mentors see this range's activity with the update; show it while writing.
  const rangeActivity = useMondayUpdateRangeActivity(
    true,
    seasonId,
    range.startDate,
    range.endDate,
  );

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setAttempted(true);
    setError('');
    if (hours === null) return;
    setPending(true);
    try {
      // A local compile-time guard keeps fixture imports out of production chunks.
      if (import.meta.env.MODE !== 'production' && demoMode) {
        const { editDemoMondayUpdate, saveDemoMondayUpdate } =
          await import('@/data/demo');
        if (existing) editDemoMondayUpdate(existing.id, { contentHtml, hours });
        else
          saveDemoMondayUpdate({
            id: crypto.randomUUID(),
            seasonId,
            student: {
              id: student.id,
              slug: student.slug,
              name: student.name,
              avatarUrl: student.avatarUrl,
            },
            startDate: range.startDate,
            endDate: range.endDate,
            contentHtml,
            hours,
            createdAt: new Date().toISOString(),
            editedAt: null,
            marks: [],
            activity: rangeActivity.data ?? { attempts: [], mocks: [] },
          });
      } else if (existing)
        await apiRequest<MondayUpdate>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-updates/${encodeURIComponent(existing.id)}`,
          {
            method: 'PATCH',
            body: JSON.stringify({
              contentHtml,
              hours,
            } satisfies MondayUpdateEditMutation),
          },
        );
      else
        await apiRequest<MondayUpdate>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-updates`,
          {
            method: 'POST',
            body: JSON.stringify({
              startDate: range.startDate,
              endDate: range.endDate,
              contentHtml,
              hours,
            } satisfies MondayUpdateMutation),
          },
        );
      await invalidateMondayUpdates(queryClient);
      onDone();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to save your update.',
      );
    } finally {
      setPending(false);
    }
  };

  return (
    // The form checks hours itself, so the browser's own validation does not
    // block the submit before its message can show.
    <form
      className={formClass}
      noValidate
      onSubmit={(event) => void submit(event)}
    >
      {pastDue(range) ? (
        <InlineNotice tone="warning">
          {existing
            ? 'Your mentor unlocked this late week. Saving this edit marks it late.'
            : 'Your mentor unlocked this late week, so it will be marked late.'}
        </InlineNotice>
      ) : null}
      {error ? (
        <InlineNotice tone="warning">
          <span role="alert">{error}</span>
        </InlineNotice>
      ) : null}
      <Field className="max-w-sm">
        <FieldLabel htmlFor="monday-update-hours">
          Hours Spent <LabelHint>Expected: ~{weeklyHours}h/week</LabelHint>
        </FieldLabel>
        <Input
          id="monday-update-hours"
          type="number"
          inputMode="decimal"
          min="0"
          max="168"
          step="0.5"
          required
          value={hoursInput}
          onChange={(event) => setHoursInput(event.target.value)}
          aria-invalid={attempted && hours === null}
        />
        {attempted && hours === null ? (
          <FieldError>
            Enter hours between 0 and 168 in steps of 0.5.
          </FieldError>
        ) : null}
      </Field>
      <section aria-label="Activity this week">
        {rangeActivity.isLoading ? (
          <p role="status" className={`${mutedClass} py-2.5`}>
            Loading your LeetCode and mock interviews…
          </p>
        ) : rangeActivity.isError ? (
          <ErrorState
            title="Your Activity Could Not Be Loaded"
            onRetry={() => void rangeActivity.refetch()}
          />
        ) : rangeActivity.data ? (
          <UpdateActivitySummary activity={rangeActivity.data} />
        ) : null}
      </section>
      <RichTextEditor
        id="monday-update-content"
        label="Your Update"
        labelHint={<WhatToInclude />}
        value={content}
        onChange={setContent}
      />
      <div className="button-group">
        <Button type="button" variant="outline" onClick={onDone}>
          Cancel
        </Button>
        <Button type="submit" disabled={pending}>
          {pending
            ? existing
              ? 'Saving…'
              : 'Posting…'
            : existing
              ? 'Save Update'
              : 'Post Update'}
        </Button>
      </div>
    </form>
  );
}

/**
 * Hours, LeetCode solved by difficulty and mock interviews for one date range,
 * as mentors see them. Hours are omitted where the student is still entering
 * them.
 */
function UpdateActivitySummary({
  activity,
  hours,
}: {
  activity: MondayUpdateActivity;
  hours?: number;
}) {
  const counts = { easy: 0, medium: 0, hard: 0 };
  for (const attempt of activity.attempts) {
    const solved = attempt.outcome === 'solved';
    if (solved && attempt.difficulty in counts)
      counts[attempt.difficulty as keyof typeof counts] += 1;
  }
  const total = counts.easy + counts.medium + counts.hard;
  const mockCount = activity.mocks.length;
  const difficulties = [
    { key: 'easy', label: 'Easy', count: counts.easy, dot: 'bg-success' },
    { key: 'medium', label: 'Medium', count: counts.medium, dot: 'bg-warning' },
    { key: 'hard', label: 'Hard', count: counts.hard, dot: 'bg-destructive' },
  ] as const;
  const rowClass =
    'grid grid-cols-[9rem_1fr] items-baseline gap-x-4 gap-y-1 border-t border-border py-2.5 first:border-t-0 first:pt-0';
  return (
    <dl className="m-0 grid border-b border-border tabular-nums">
      {hours === undefined ? null : (
        <div className={rowClass}>
          <dt className="text-sm font-semibold">Hours Spent</dt>
          <dd className="m-0 text-sm">
            <strong className="text-base font-semibold">{hours}</strong>
          </dd>
        </div>
      )}
      <div className={rowClass}>
        <dt className="text-sm font-semibold">LeetCode</dt>
        <dd className="m-0 flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <span className="text-sm">
            <strong className="text-base font-semibold">{total}</strong> solved
          </span>
          {difficulties.map((item) => (
            <span
              key={item.key}
              className="flex items-center gap-1.5 text-sm text-muted-foreground"
            >
              <span className={`size-2 rounded-full ${item.dot}`} aria-hidden />
              <span className="font-medium text-foreground">{item.count}</span>
              {item.label}
            </span>
          ))}
        </dd>
      </div>
      <div className={rowClass}>
        <dt className="text-sm font-semibold">Mock Interviews</dt>
        <dd className="m-0 flex flex-wrap items-baseline gap-x-4 text-sm">
          <span>
            <strong className="text-base font-semibold">{mockCount}</strong>{' '}
            {mockCount === 1 ? 'interview' : 'interviews'}
          </span>
        </dd>
      </div>
    </dl>
  );
}

/** The program week picker, beside a table's search or in its filters. */
function WeekSelect({
  id,
  ariaLabel,
  items,
  value,
  onChange,
  className,
}: {
  id: string;
  ariaLabel?: string;
  items: { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
  className?: string;
}) {
  return (
    <Select
      items={items}
      value={value}
      onValueChange={(next) => {
        if (next) onChange(next);
      }}
    >
      <SelectTrigger id={id} aria-label={ariaLabel} className={className}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}

/** A week's status, with its unlock request or unlock beside it. */
function UpdateStatus({
  row,
}: {
  row: {
    status: StudentUpdateStatus;
    locked: boolean;
    unlock?: MondayUpdateUnlock;
    update?: MondayUpdate;
  };
}) {
  return (
    <span className="flex items-center gap-2 whitespace-nowrap">
      <StatusBadge tone={updateStatusTones[row.status]}>
        {row.status}
      </StatusBadge>
      {row.unlock?.state === 'pending' ? (
        <StatusBadge tone="info">Unlock requested</StatusBadge>
      ) : row.locked && row.status !== 'Kicked' ? (
        <Tooltip content="This Monday update is locked. Request an unlock from this row’s menu.">
          <button
            type="button"
            aria-label="Locked. Request an unlock from this row’s menu."
            className="inline-flex cursor-default rounded-sm text-muted-foreground"
          >
            <IconLock size={15} aria-hidden="true" />
          </button>
        </Tooltip>
      ) : null}
    </span>
  );
}

/** Asks the student's mentor to unlock a week past its due date. */
function RequestUnlockDialog({
  seasonId,
  student,
  range,
  weekNumber,
  onOpenChange,
}: {
  seasonId: string;
  student: MondayUpdate['student'];
  range: MondayUpdateRange;
  weekNumber: number;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [reason, setReason] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const submit = async () => {
    setPending(true);
    setError('');
    try {
      if (import.meta.env.MODE !== 'production' && demoMode) {
        const { saveDemoMondayUpdateUnlock } = await import('@/data/demo');
        saveDemoMondayUpdateUnlock({
          id: crypto.randomUUID(),
          seasonId,
          student: { ...student, avatarUrl: student.avatarUrl ?? null },
          startDate: range.startDate,
          endDate: range.endDate,
          reason: reason.trim(),
          state: 'pending',
          requestedAt: new Date().toISOString(),
          decidedAt: null,
          decidedBy: null,
        });
      } else
        await apiRequest<MondayUpdateUnlock>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-update-unlocks`,
          {
            method: 'POST',
            body: JSON.stringify({
              startDate: range.startDate,
              endDate: range.endDate,
              reason: reason.trim(),
            } satisfies MondayUpdateUnlockMutation),
          },
        );
      await invalidateMondayUpdates(queryClient);
      onOpenChange(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to send your request.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`Request Unlock · Week ${weekNumber}`}
      open
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next);
      }}
    >
      <div className="grid gap-4">
        <Field>
          <Textarea
            id="unlock-reason"
            aria-label="Reason"
            placeholder="Why is this update late?"
            value={reason}
            maxLength={1000}
            onChange={(event) => setReason(event.target.value)}
          />
        </Field>
        {error ? (
          <InlineNotice tone="warning">
            <span role="alert">{error}</span>
          </InlineNotice>
        ) : null}
        <div className="button-group">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || !reason.trim()}
            onClick={() => void submit()}
          >
            {pending ? 'Sending…' : 'Send Request'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

/** One row of the mentor table: a student's update, or a week they missed. */
type MentorUpdateRow = {
  id: string;
  student: { id: string; name: string; slug?: string; level?: string };
  weekNumber?: number;
  range: MondayUpdateRange;
  update?: MondayUpdate;
  status: StudentUpdateStatus;
  /** Past its due date and not unlocked: it cannot be posted or changed. */
  locked: boolean;
  unlock?: MondayUpdateUnlock;
  /** The mentor's mark on a week with no update. */
  weekMark?: { mark: MondayUpdateMark; commentHtml?: string | null };
  /** The student's weekly targets, from the mentor's report. */
  targets?: WeeklyTargets;
};

/**
 * A mentor's students' updates, one row each, newest week first. With a
 * week selected, students who have not posted for it get a row too. A row
 * expands to the update, its marks and its edit history; marking lives in
 * the row menu.
 */
function MentorUpdateTable({
  showWeek,
  seasonId,
  mentorId,
  rows,
  weekSelect,
  weekFilter,
  loading,
  error,
  onRetry,
}: {
  /** The Week and Dates columns, only for All updates. */
  showWeek: boolean;
  seasonId: string;
  mentorId: string;
  rows: MentorUpdateRow[];
  /** The program week picker, before the search. */
  weekSelect: ReactNode;
  /** The same picker for the Filters popover, with its active state. */
  weekFilter: { content: ReactNode; active: boolean; onClear: () => void };
  loading: boolean;
  error: boolean;
  onRetry: () => void;
}) {
  const seasonSwitcher = useSeasonSwitcher('monday-updates');
  const ownMark = (row: MentorUpdateRow) =>
    row.update
      ? row.update.marks.find((record) => record.mentor.id === mentorId)
      : row.weekMark;
  const optionsOf = (values: string[]) =>
    [...new Set(values)]
      .sort((a, b) => a.localeCompare(b))
      .map((value) => ({ label: value, value }));
  const columns: ColumnDef<MentorUpdateRow, any>[] = [
    {
      id: 'expand',
      header: 'Details',
      meta: { width: 4.5 },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) =>
        row.original.update ? (
          <ExpandRowButton
            row={row}
            subject={`update from ${row.original.student.name} for ${mentorRowWeek(row.original)}`}
          />
        ) : null,
    },
    {
      id: 'student',
      header: 'Student',
      meta: {
        width: 11,
        filterOptions: optionsOf(rows.map((row) => row.student.name)),
      },
      accessorFn: (row) => row.student.name,
      filterFn: 'equalsString',
      cell: ({ row }) =>
        row.original.student.slug ? (
          <Link
            to={`/people/${row.original.student.slug}`}
            className="font-medium underline-offset-4 hover:underline"
          >
            {row.original.student.name}
          </Link>
        ) : (
          <span className="font-medium">{row.original.student.name}</span>
        ),
    },
    {
      id: 'level',
      header: 'Level',
      meta: { width: 7.5 },
      accessorFn: (row) => studentLevelLabel(row.student.level as StudentLevel),
      cell: ({ row }) =>
        row.original.student.level &&
        row.original.student.level !== 'not_applicable' ? (
          <StudentLevelBadge
            level={row.original.student.level as StudentLevel}
          />
        ) : (
          <Dash />
        ),
    },
    ...(showWeek
      ? ([
          {
            id: 'week',
            header: 'Week',
            meta: { width: 5.5 },
            accessorFn: (row) => row.weekNumber ?? 0,
            enableColumnFilter: false,
            cell: ({ row }) => mentorRowWeek(row.original),
          },
          {
            id: 'dates',
            header: 'Dates',
            meta: { width: 11 },
            accessorFn: (row) => row.range.startDate,
            enableColumnFilter: false,
            cell: ({ row }) => updateRangeLabel(row.original.range),
          },
        ] satisfies ColumnDef<MentorUpdateRow, any>[])
      : []),
    {
      id: 'status',
      header: 'Status',
      meta: {
        // Fits the longest pair: Submitted (Late) beside Unlock requested.
        width: 17.5,
        filterOptions: optionsOf(rows.map((row) => row.status)),
      },
      accessorFn: (row) => row.status,
      filterFn: 'equalsString',
      cell: ({ row }) => <UpdateStatus row={row.original} />,
    },
    {
      id: 'hours',
      header: 'Hours',
      meta: { width: 6 },
      accessorFn: (row) => row.update?.hours ?? -1,
      cell: ({ row }) =>
        row.original.update ? (
          <HoursCell
            hours={row.original.update.hours}
            target={row.original.targets?.hours}
          />
        ) : (
          <Dash />
        ),
    },
    {
      id: 'leetcode',
      header: 'LeetCodes',
      meta: { width: 9 },
      accessorFn: (row) => (row.update ? solvedCount(row.update.activity) : -1),
      cell: ({ row }) =>
        row.original.update ? (
          <SolvedByDifficulty
            activity={row.original.update.activity}
            targets={row.original.targets}
          />
        ) : (
          <Dash />
        ),
    },
    {
      id: 'mocks',
      header: 'Mock Interviews',
      meta: { width: 8.5 },
      accessorFn: (row) => row.update?.activity.mocks.length ?? -1,
      cell: ({ row }) =>
        row.original.update ? (
          <MockCountCell
            count={row.original.update.activity.mocks.length}
            target={row.original.targets?.mocks}
          />
        ) : (
          <Dash />
        ),
    },
    {
      id: 'mark',
      header: 'Mark',
      meta: {
        width: 8,
        filterOptions: [
          { label: 'Not marked', value: 'Not marked' },
          ...markOptions.map((option) => ({
            label: option.label,
            value: option.label,
          })),
        ],
      },
      accessorFn: (row) => {
        const mark = ownMark(row);
        return mark ? markLabel(mark.mark) : 'Not marked';
      },
      filterFn: 'equalsString',
      cell: ({ row }) => {
        const mark = ownMark(row.original);
        return mark ? (
          <MarkBadge mark={mark.mark} />
        ) : (
          <StatusBadge tone="muted">Not marked</StatusBadge>
        );
      },
    },
    {
      id: 'actions',
      header: 'Actions',
      meta: { width: 6, align: 'end' },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => (
        <MentorUpdateRowActions
          row={row}
          seasonId={seasonId}
          mentorId={mentorId}
        />
      ),
    },
  ];
  return (
    <DataTable
      ariaLabel="Student Monday updates"
      data={rows}
      columns={columns}
      loading={loading}
      error={error}
      onRetry={onRetry}
      toolbarLead={
        <>
          {seasonSwitcher}
          {weekSelect}
        </>
      }
      extraFilters={{
        content: weekFilter.content,
        activeCount: Number(weekFilter.active),
        onClear: weekFilter.onClear,
      }}
      emptyTitle="No Updates in This Week"
      emptyMessage="A coordinator can assign students to your mentor team."
      getRowId={(row) => row.id}
      rowCanExpand={(row) => Boolean(row.update)}
      renderExpanded={(row) =>
        row.original.update ? (
          <MentorUpdateDetails
            update={row.original.update}
            mentorId={mentorId}
          />
        ) : null
      }
      renderCard={(row) => (
        <div className="grid gap-2.5">
          <div className="flex items-center justify-between gap-3">
            <div className="grid gap-0.5">
              <h3 className={cardTitleClass}>{row.original.student.name}</h3>
              <span className={mutedClass}>
                {mentorRowWeek(row.original)}:{' '}
                {updateRangeLabel(row.original.range)}
              </span>
            </div>
            <MentorUpdateRowActions
              row={row}
              seasonId={seasonId}
              mentorId={mentorId}
            />
          </div>
          <div className={rowClass}>
            <UpdateStatus row={row.original} />
            {row.original.update ? (
              ownMark(row.original) ? (
                <MarkBadge mark={ownMark(row.original)!.mark} />
              ) : (
                <StatusBadge tone="muted">Not marked</StatusBadge>
              )
            ) : null}
          </div>
          {row.original.update && row.getIsExpanded() ? (
            <MentorUpdateDetails
              update={row.original.update}
              mentorId={mentorId}
            />
          ) : null}
        </div>
      )}
    />
  );
}

function mentorRowWeek(row: MentorUpdateRow) {
  return row.weekNumber
    ? `Week ${row.weekNumber}`
    : updateRangeLabel(row.range);
}

// Labels inside an expanded row, the same in every Monday update table.
const detailLabelClass = 'text-xs font-semibold text-muted-foreground';

/**
 * A posted update as its mentor reads it: what the student wrote, then the
 * mentor's notes and any other mentors' marks. Hours and activity are in
 * the row, as on the coordinator's table.
 */
function MentorUpdateDetails({
  update,
  mentorId,
}: {
  update: MondayUpdate;
  mentorId: string;
}) {
  const ownMark = update.marks.find((record) => record.mentor.id === mentorId);
  const otherMarks = update.marks.filter(
    (record) => record.mentor.id !== mentorId,
  );
  return (
    <div className="grid gap-4">
      <div className="grid gap-1">
        <h4 className={detailLabelClass}>Student Update</h4>
        <RichTextContent
          html={update.contentHtml}
          empty="No written content."
        />
      </div>
      <div className="grid gap-1 border-t border-border pt-3">
        <h4 className={detailLabelClass}>Mentor Notes</h4>
        <RichTextContent html={ownMark?.commentHtml} empty="No notes." />
      </div>
      {otherMarks.length ? (
        <div className="grid gap-2 border-t border-border pt-4">
          <h4 className={subTitleClass}>Other mentor marks</h4>
          <ul className={metaListClass}>
            {otherMarks.map((record) => (
              <li key={record.id}>
                <strong className="font-semibold text-foreground">
                  {record.mentor.name}
                </strong>{' '}
                <MarkBadge mark={record.mark} />
                <RichTextContent
                  html={record.commentHtml}
                  empty="No comment."
                />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

/** A mentor row's menu: show the update and add or edit your mark. */
function MentorUpdateRowActions({
  row,
  seasonId,
  mentorId,
}: {
  row: Row<MentorUpdateRow>;
  seasonId: string;
  mentorId: string;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const { update, locked, unlock, student, range } = row.original;
  // A late week, posted or not, can be unlocked for the student.
  const canUnlock = locked && Boolean(row.original.weekNumber);
  const canRelock = unlock?.state === 'approved';
  if (!update && !canUnlock && !row.original.weekNumber) return null;
  const ownMark = update
    ? update.marks.find((record) => record.mentor.id === mentorId)
    : row.original.weekMark;
  const unlockWeek = () =>
    void unlockMondayUpdate(seasonId, student, range, row.original.unlock).then(
      () => invalidateMondayUpdates(queryClient),
    );
  const relock = () => {
    if (!unlock) return;
    void relockMondayUpdate(seasonId, unlock).then(() =>
      invalidateMondayUpdates(queryClient),
    );
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${student.name}'s ${mentorRowWeek(row.original)} update`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {update ? (
            <DropdownMenuItem onClick={row.getToggleExpandedHandler()}>
              <IconListDetails size={17} aria-hidden="true" />
              {row.getIsExpanded() ? 'Hide Details' : 'Show Details'}
            </DropdownMenuItem>
          ) : null}
          {update || row.original.weekNumber ? (
            <DropdownMenuItem onClick={() => setOpen(true)}>
              <IconEdit size={17} aria-hidden="true" /> Edit Notes
            </DropdownMenuItem>
          ) : null}
          {canUnlock ? (
            <DropdownMenuItem onClick={unlockWeek}>
              <IconLockOpen size={17} aria-hidden="true" /> Unlock Update
            </DropdownMenuItem>
          ) : null}
          {canRelock ? (
            <DropdownMenuItem onClick={relock}>
              <IconLock size={17} aria-hidden="true" /> Lock Update
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {open ? (
        <MarkDialog
          seasonId={seasonId}
          student={student}
          range={range}
          updateId={update?.id}
          mentorId={mentorId}
          current={ownMark ?? undefined}
          onOpenChange={setOpen}
        />
      ) : null}
    </>
  );
}

/**
 * Unlocks a student's late week as their mentor, approving their request
 * if they made one.
 */
async function unlockMondayUpdate(
  seasonId: string,
  student: { id: string; name: string; slug?: string },
  range: MondayUpdateRange,
  request?: MondayUpdateUnlock,
) {
  if (import.meta.env.MODE !== 'production' && demoMode) {
    const { saveDemoMondayUpdateUnlock } = await import('@/data/demo');
    saveDemoMondayUpdateUnlock({
      id: request?.id ?? crypto.randomUUID(),
      seasonId,
      student: {
        id: student.id,
        name: student.name,
        slug: student.slug ?? student.id,
        avatarUrl: null,
      },
      startDate: range.startDate,
      endDate: range.endDate,
      reason: request?.reason ?? 'Unlocked by a mentor.',
      state: 'approved',
      requestedAt: request?.requestedAt ?? new Date().toISOString(),
      decidedAt: new Date().toISOString(),
      decidedBy: null,
    });
    return;
  }
  await apiRequest<MondayUpdateUnlock>(
    `/seasons/${encodeURIComponent(seasonId)}/monday-update-unlocks`,
    {
      method: 'POST',
      body: JSON.stringify({
        startDate: range.startDate,
        endDate: range.endDate,
        studentId: student.id,
      } satisfies MondayUpdateUnlockMutation),
    },
  );
}

/** Relocks a student's late week as their mentor. */
async function relockMondayUpdate(
  seasonId: string,
  unlock: MondayUpdateUnlock,
) {
  if (import.meta.env.MODE !== 'production' && demoMode) {
    const { saveDemoMondayUpdateUnlock } = await import('@/data/demo');
    saveDemoMondayUpdateUnlock({
      ...unlock,
      state: 'declined',
      decidedAt: new Date().toISOString(),
    });
    return;
  }
  await apiRequest<MondayUpdateUnlock>(
    `/seasons/${encodeURIComponent(seasonId)}/monday-update-unlocks/${encodeURIComponent(unlock.id)}/relock`,
    { method: 'POST' },
  );
}

/** Declines a student's request to unlock a late week. */
async function declineMondayUpdateUnlock(
  seasonId: string,
  request: MondayUpdateUnlock,
) {
  if (import.meta.env.MODE !== 'production' && demoMode) {
    const { saveDemoMondayUpdateUnlock } = await import('@/data/demo');
    saveDemoMondayUpdateUnlock({
      ...request,
      state: 'declined',
      decidedAt: new Date().toISOString(),
    });
    return;
  }
  await apiRequest<MondayUpdateUnlock>(
    `/seasons/${encodeURIComponent(seasonId)}/monday-update-unlocks/${encodeURIComponent(request.id)}/decision`,
    { method: 'POST', body: JSON.stringify({ decision: 'declined' }) },
  );
}

/**
 * Students' pending requests to unlock late weeks, for their mentor to
 * approve or decline. Draws nothing when there are none.
 */
function UnlockRequestsTable({
  seasonId,
  requests,
  weekNumberOf,
}: {
  seasonId: string;
  requests: MondayUpdateUnlock[];
  weekNumberOf: (request: MondayUpdateUnlock) => number | undefined;
}) {
  const queryClient = useQueryClient();
  if (!requests.length) return null;
  const week = (request: MondayUpdateUnlock) => {
    const number = weekNumberOf(request);
    return number ? `Week ${number}` : updateRangeLabel(request);
  };
  const decide = (request: MondayUpdateUnlock, approve: boolean) =>
    void (
      approve
        ? unlockMondayUpdate(seasonId, request.student, request, request)
        : declineMondayUpdateUnlock(seasonId, request)
    ).then(() => invalidateMondayUpdates(queryClient));
  const actions = (request: MondayUpdateUnlock) => (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="icon" />}
        aria-label={`More actions for ${request.student.name}'s unlock request`}
      >
        <IconDotsVertical size={18} aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem onClick={() => decide(request, true)}>
          <IconLockOpen size={17} aria-hidden="true" /> Approve and Unlock
        </DropdownMenuItem>
        <DropdownMenuItem
          variant="destructive"
          onClick={() => decide(request, false)}
        >
          <IconX size={17} aria-hidden="true" /> Decline
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
  const columns: ColumnDef<MondayUpdateUnlock, any>[] = [
    {
      id: 'student',
      header: 'Student',
      meta: { width: 11 },
      accessorFn: (request) => request.student.name,
      cell: ({ row }) => (
        <span className="font-medium">{row.original.student.name}</span>
      ),
    },
    {
      id: 'week',
      header: 'Week',
      meta: { width: 6 },
      accessorFn: (request) => request.startDate,
      enableColumnFilter: false,
      cell: ({ row }) => week(row.original),
    },
    { accessorKey: 'reason', header: 'Reason', meta: { width: 24 } },
    {
      accessorKey: 'requestedAt',
      header: 'Requested',
      meta: { width: 10 },
      enableColumnFilter: false,
      cell: ({ getValue }) => formatDateTime(getValue()),
    },
    {
      id: 'actions',
      header: 'Actions',
      meta: { width: 6, align: 'end' },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => actions(row.original),
    },
  ];
  return (
    <PageSection titleId="monday-unlock-requests-title" title="Unlock Requests">
      <DataTable
        ariaLabel="Unlock requests"
        data={requests}
        columns={columns}
        emptyTitle="No Unlock Requests"
        getRowId={(request) => request.id}
        renderCard={(row) => (
          <div className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <h3 className={cardTitleClass}>
                {row.original.student.name} · {week(row.original)}
              </h3>
              {actions(row.original)}
            </div>
            <p className="text-sm">{row.original.reason}</p>
            <span className={mutedClass}>
              Requested {formatDateTime(row.original.requestedAt)}
            </span>
          </div>
        )}
      />
    </PageSection>
  );
}

/** Adds or edits the signed-in mentor's mark and comment on an update. */
function MarkDialog({
  seasonId,
  student,
  range,
  updateId,
  mentorId,
  current,
  onOpenChange,
}: {
  seasonId: string;
  student: { id: string; name: string };
  range: MondayUpdateRange;
  /** The posted update to mark; without one, the week itself is marked. */
  updateId?: string;
  mentorId: string;
  current?: { mark: MondayUpdateMark; commentHtml?: string | null };
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [mark, setMark] = useState<MondayUpdateMark | ''>(current?.mark ?? '');
  const [comment, setComment] = useState(current?.commentHtml ?? '');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const label = updateRangeLabel(range);
  const update = {
    id: updateId ?? `${student.id}:${range.startDate}`,
    student,
  };
  // "Not marked" clears the mentor's mark; there is nothing to save when the
  // choice matches the saved one.
  const unchanged =
    mark === (current?.mark ?? '') && comment === (current?.commentHtml ?? '');
  const save = async () => {
    setPending(true);
    setError('');
    try {
      if (import.meta.env.MODE !== 'production' && demoMode) {
        if (!updateId) {
          const { clearDemoWeekMark, saveDemoWeekMark } =
            await import('@/data/demo');
          if (!mark)
            clearDemoWeekMark(seasonId, student.id, mentorId, range.startDate);
          else
            saveDemoWeekMark({
              seasonId,
              studentId: student.id,
              mentorId,
              startDate: range.startDate,
              endDate: range.endDate,
              mark,
              commentHtml: comment,
              updatedAt: new Date().toISOString(),
            });
        }
      } else if (updateId)
        await apiRequest<MondayUpdateMarkRecord>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-updates/${encodeURIComponent(updateId)}/mark`,
          {
            method: 'PUT',
            body: JSON.stringify({
              mark,
              commentHtml: comment,
            } satisfies MondayUpdateMarkMutation),
          },
        );
      else
        await apiRequest<MondayWeekMark>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-week-marks`,
          {
            method: 'PUT',
            body: JSON.stringify({
              studentId: student.id,
              startDate: range.startDate,
              endDate: range.endDate,
              mark,
              commentHtml: comment,
            } satisfies MondayWeekMarkMutation),
          },
        );
      await invalidateMondayUpdates(queryClient);
      onOpenChange(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to save your mark.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`Edit Notes · ${update.student.name}`}
      description={label}
      open
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next);
      }}
    >
      <div className="grid gap-4">
        <Field>
          <FieldLabel htmlFor={`monday-mark-${update.id}`}>
            Mark for {update.student.name}
          </FieldLabel>
          <Select
            items={markSelectItems}
            value={mark}
            onValueChange={(value) =>
              setMark((value ?? '') as MondayUpdateMark | '')
            }
          >
            <SelectTrigger id={`monday-mark-${update.id}`} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {markSelectItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    <span
                      className={`size-2 shrink-0 self-center rounded-full ${
                        item.value
                          ? markTone[item.value].dot
                          : 'border border-border-strong'
                      }`}
                      aria-hidden
                    />
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <RichTextEditor
          id={`monday-mark-comment-${update.id}`}
          label={`Comment for ${update.student.name}`}
          value={comment}
          onChange={setComment}
        />
        {error ? (
          <InlineNotice tone="warning">
            <span role="alert">{error}</span>
          </InlineNotice>
        ) : null}
        <div className="button-group">
          <DialogCancel />
          <Button
            type="button"
            disabled={pending || unchanged || (!mark && !current)}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save Mark'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

/** A week's marks as a row of coloured dots and counts, strongest first. */
function MarkCounts({ students }: { students: MondayMentorReportStudent[] }) {
  return (
    <span className="flex items-center gap-3 tabular-nums">
      {markOptions.map((option) => {
        const count = students.filter(
          (item) => item.mark === option.value,
        ).length;
        return (
          <span
            key={option.value}
            className="flex items-center gap-1.5"
            aria-label={`${count} ${option.label}`}
            title={option.label}
          >
            <span
              className={`size-2 rounded-full ${markTone[option.value].dot}`}
              aria-hidden
            />
            {count}
          </span>
        );
      })}
    </span>
  );
}

/**
 * The coordinator's view: one row per mentor for the week, with whether they
 * sent it and a summary. A row expands to each student's week and the
 * mentor's notes, never the student's own writing.
 */
function MentorReportsTable({
  reports,
  week,
  weekSelect,
  seasonFilter,
  loading,
  error,
  onRetry,
}: {
  reports: MondayMentorReport[];
  week?: MondayUpdateRange;
  weekSelect: ReactNode;
  seasonFilter: ReactNode;
  loading: boolean;
  error: boolean;
  onRetry: () => void;
}) {
  const seasonSwitcher = useSeasonSwitcher('monday-updates');
  const statusOf = (student: MondayMentorReportStudent) =>
    week
      ? weekStatus(
          week,
          student.postedAt
            ? { createdAt: student.postedAt, editedAt: student.editedAt }
            : undefined,
          student.kickedAt,
        )
      : undefined;
  const columns: ColumnDef<MondayMentorReport, any>[] = [
    {
      id: 'expand',
      header: 'Details',
      meta: { width: 4.5 },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) =>
        row.original.students.length ? (
          <ExpandRowButton
            row={row}
            subject={`students for ${row.original.mentor.name}`}
          />
        ) : null,
    },
    {
      id: 'mentor',
      header: 'Mentor',
      meta: { width: 11 },
      accessorFn: (report) => report.mentor.name,
      cell: ({ row }) => (
        <Link
          to={`/people/${row.original.mentor.slug}`}
          className="font-medium underline-offset-4 hover:underline"
        >
          {row.original.mentor.name}
        </Link>
      ),
    },
    {
      id: 'marked',
      header: 'Marked',
      meta: { width: 6 },
      accessorFn: (report) =>
        report.students.filter((student) => student.mark).length,
      cell: ({ row, getValue }) =>
        `${getValue()} of ${row.original.students.length}`,
    },
    {
      id: 'posted',
      header: 'Posted',
      meta: { width: 6 },
      accessorFn: (report) =>
        report.students.filter((student) => student.postedAt).length,
      cell: ({ row, getValue }) =>
        `${getValue()} of ${row.original.students.length}`,
    },
    {
      id: 'late',
      header: 'Late',
      meta: { width: 5 },
      accessorFn: (report) =>
        report.students.filter((student) => {
          const status = statusOf(student);
          return status === 'Submitted (Late)' || status === 'Overdue';
        }).length,
    },
    {
      id: 'marks',
      header: 'Marks',
      meta: { width: 13 },
      enableSorting: false,
      cell: ({ row }) => <MarkCounts students={row.original.students} />,
    },
  ];
  // Like the mock interview rounds: a bordered sub-table, one row per
  // student, its cells divided by hairlines, and the mentor's notes, which
  // can run long, along the bottom of the row. Headings show on wide
  // screens; stacked rows label each cell instead.
  const studentRowClass =
    'grid gap-3 p-4 min-[56rem]:grid-cols-[minmax(0,1.4fr)_repeat(6,minmax(0,1fr))] min-[56rem]:gap-x-4';
  const cellClass =
    'grid content-start gap-1 min-[56rem]:border-l min-[56rem]:pl-4';
  // Sub-table headings match the table's own column headings.
  const headingClass = 'text-xs font-semibold text-muted-foreground';
  const cellHeadingClass =
    'text-xs font-semibold text-muted-foreground min-[56rem]:sr-only';
  const students = (report: MondayMentorReport) => (
    <div className="overflow-hidden rounded-lg border bg-card">
      <section
        aria-label={`Students of ${report.mentor.name}`}
        className="min-w-0"
      >
        <div
          className={cn(
            studentRowClass,
            'hidden border-b py-3 min-[56rem]:grid',
          )}
        >
          <h2 className={headingClass}>Student</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Level</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Status</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Hours</h2>
          <h2 className={cn(headingClass, 'pl-4')}>LeetCodes</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Mocks</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Mark</h2>
        </div>
        <ol className="m-0 list-none divide-y p-0 text-sm">
          {report.students.map((student) => {
            const status = statusOf(student);
            return (
              <li key={student.student.id} className={studentRowClass}>
                <Link
                  to={`/people/${student.student.slug}`}
                  className="self-start font-semibold underline-offset-4 hover:underline"
                >
                  {student.student.name}
                </Link>
                <div className={cellClass}>
                  <h3 className={cellHeadingClass}>Level</h3>
                  {student.studentLevel &&
                  student.studentLevel !== 'not_applicable' ? (
                    <span>
                      <StudentLevelBadge
                        level={student.studentLevel as StudentLevel}
                      />
                    </span>
                  ) : (
                    <Dash />
                  )}
                </div>
                <div className={cellClass}>
                  <h3 className={cellHeadingClass}>Status</h3>
                  {status ? (
                    <span>
                      <StatusBadge tone={updateStatusTones[status]}>
                        {status}
                      </StatusBadge>
                    </span>
                  ) : (
                    <Dash />
                  )}
                </div>
                <div className={cn(cellClass, 'tabular-nums')}>
                  <h3 className={cellHeadingClass}>Hours</h3>
                  {student.hours === null ? (
                    <Dash />
                  ) : (
                    <HoursCell
                      hours={student.hours}
                      target={student.targets.hours}
                    />
                  )}
                </div>
                <div className={cellClass}>
                  <h3 className={cellHeadingClass}>LeetCodes</h3>
                  <SolvedCounts
                    counts={student.leetcodeCounts}
                    targets={weeklyTargetsFor(student.targets.hours)}
                  />
                </div>
                <div className={cn(cellClass, 'tabular-nums')}>
                  <h3 className={cellHeadingClass}>Mocks</h3>
                  <MockCountCell
                    count={student.mockCount}
                    target={weeklyTargetsFor(student.targets.hours).mocks}
                  />
                </div>
                <div className={cellClass}>
                  <h3 className={cellHeadingClass}>Mark</h3>
                  <span>
                    {student.mark ? (
                      <MarkBadge mark={student.mark} />
                    ) : (
                      <StatusBadge tone="muted">Not marked</StatusBadge>
                    )}
                  </span>
                </div>
                <div className="grid gap-1 border-t border-border pt-3 min-[56rem]:col-span-full">
                  <h3 className="text-xs font-semibold text-muted-foreground">
                    Mentor Notes
                  </h3>
                  <RichTextContent
                    html={student.commentHtml}
                    empty="No notes."
                  />
                </div>
              </li>
            );
          })}
        </ol>
      </section>
    </div>
  );
  return (
    <DataTable
      ariaLabel="Mentor Monday updates"
      data={reports}
      columns={columns}
      loading={loading}
      error={error}
      onRetry={onRetry}
      toolbarLead={
        <>
          {seasonSwitcher}
          {weekSelect}
        </>
      }
      extraFilters={{
        content: seasonFilter,
        // A season is always chosen, so it never counts as an extra filter.
        activeCount: 0,
        onClear: () => {},
      }}
      emptyTitle="No Mentors in This Week"
      emptyMessage="Mentors appear here once students are assigned to them."
      getRowId={(report) => report.mentor.id}
      renderExpanded={(row) => students(row.original)}
      renderCard={(row) => (
        <div className="grid gap-2.5">
          <div className="flex items-center justify-between gap-3">
            <h3 className={cardTitleClass}>{row.original.mentor.name}</h3>
            <span className={mutedClass}>
              {row.original.students.filter((student) => student.mark).length}{' '}
              of {row.original.students.length} marked
            </span>
          </div>
          <MarkCounts students={row.original.students} />
          {students(row.original)}
        </div>
      )}
    />
  );
}

/** Solved counts by difficulty, each by its colour dot. */
/**
 * Solved counts by difficulty, each by its colour dot; a count turns green
 * once it meets the student's weekly target for that difficulty.
 */
function SolvedCounts({
  counts,
  targets,
}: {
  counts: { easy: number; medium: number; hard: number };
  targets?: WeeklyTargets;
}) {
  return (
    <span className="flex items-center gap-3 tabular-nums">
      {difficultyDots.map((item) => {
        const met = meetsTarget(counts[item.key], targets?.[item.key]);
        return (
          <span
            key={item.key}
            className={cn(
              'flex items-center gap-1.5',
              met && 'font-semibold text-success',
            )}
            aria-label={`${counts[item.key]} ${item.label}${met ? ', target met' : ''}`}
            title={
              targets?.[item.key]
                ? `Target: ${targets[item.key]} ${item.label}`
                : undefined
            }
          >
            <span className={`size-2 rounded-full ${item.dot}`} aria-hidden />
            {counts[item.key]}
          </span>
        );
      })}
    </span>
  );
}
