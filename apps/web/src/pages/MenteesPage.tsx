import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import { useCallback, useMemo, useState } from 'react';
import { Navigate, useParams } from 'react-router-dom';

import {
  completedMondayUpdateWeeks,
  programTimezone,
  sameUpdateWeek,
} from '@/activityDates';
import type { MondayUpdate } from '@/api/generated/models';
import {
  useCurrentUser,
  useMondayUpdates,
  useSeasons,
  useSeasonPeople,
  useSeasonWeeks,
} from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import {
  PeopleTable,
  StudentRowActions,
  type PersonLatestMark,
} from '@/components/PeopleTable';
import { pickLatestMark } from '@/mondayMarks';
import { weekStatus } from '@/mondayStatus';
import { canSetStudentLevel } from '@/studentLevels';
import { RemovalDialog } from '@/components/RemovalDialog';
import type { Person } from '@/types';
import { calendarDateKey } from '@/utils';

function isKicked(person: Person) {
  return (person.enrollmentState ?? person.status) === 'kicked';
}

/** A mentor's own students, even for an admin who could see everyone. */
export function MyStudentsPage() {
  return <MenteesPage scope="mine" />;
}

export function MenteesPage({ scope }: { scope?: 'mine' } = {}) {
  const { slug } = useParams();
  const seasons = useSeasons();
  const currentUser = useCurrentUser();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  const [includePrevious, setIncludePrevious] = useState(false);
  const showPrevious = includePrevious || season?.status === 'closed';
  const canManageEveryStudent =
    scope !== 'mine' &&
    Boolean(
      currentUser.data?.globalRoles.length ||
      currentUser.data?.seasonRoles.some(
        (membership) =>
          membership.seasonId === season?.id &&
          membership.role === 'coordinator' &&
          (membership.state === 'active' || membership.state === 'completed'),
      ),
    );
  usePageTitle(canManageEveryStudent ? 'All Students' : 'My Students');
  const canChangeLevel = canSetStudentLevel(currentUser.data, season);
  const mentees = useMemo(
    () =>
      (query.data?.items ?? []).filter((person) => {
        const student = person.seasonRole
          ? person.seasonRole === 'student'
          : person.roles.includes('student');
        const activeStudent =
          student &&
          (person.enrollmentState
            ? person.enrollmentState === 'active'
            : person.status === 'active');
        // A kicked student stays listed, with the status Kicked, for whoever
        // mentored them.
        const kicked = student && isKicked(person);
        const assigned = person.mentorshipMentorId === currentUser.data?.id;
        const previouslyAssigned =
          person.previousMentorIds?.includes(currentUser.data?.id ?? '') ??
          false;
        if (showPrevious)
          return canManageEveryStudent
            ? person.seasonRole === 'student'
            : assigned || previouslyAssigned;
        return (
          (activeStudent && (canManageEveryStudent || assigned)) ||
          (kicked && (canManageEveryStudent || previouslyAssigned))
        );
      }),
    [canManageEveryStudent, currentUser.data?.id, query.data, showPrevious],
  );
  // Coordinators and admins have no marks of their own; the column is
  // mentor-only and covers the mentor's current mentees for the whole season.
  const showLatestMark = Boolean(currentUser.data) && !canManageEveryStudent;
  const currentMenteeIds = useMemo(
    () =>
      (query.data?.items ?? [])
        .filter(
          (person) =>
            person.mentorshipMentorId === currentUser.data?.id ||
            (isKicked(person) &&
              (person.previousMentorIds ?? []).includes(
                currentUser.data?.id ?? '',
              )),
        )
        .map((person) => person.id),
    [currentUser.data?.id, query.data],
  );
  const seasonUpdateFilters = useMemo(
    () => ({
      startDate: season
        ? calendarDateKey(season.startsAt, programTimezone)
        : '',
      endDate: season ? calendarDateKey(season.endsAt, programTimezone) : '',
      studentIds: currentMenteeIds,
    }),
    [currentMenteeIds, season],
  );
  const seasonUpdates = useMondayUpdates(
    showLatestMark && Boolean(season) && currentMenteeIds.length > 0,
    season?.id,
    seasonUpdateFilters,
  );
  const latestMarks = useMemo(() => {
    const byStudent = new Map<string, MondayUpdate[]>();
    for (const update of seasonUpdates.data?.items ?? [])
      byStudent.set(update.student.id, [
        ...(byStudent.get(update.student.id) ?? []),
        update,
      ]);
    return new Map(
      [...byStudent].map(([studentId, updates]) => [
        studentId,
        pickLatestMark(updates),
      ]),
    );
  }, [seasonUpdates.data]);
  // This week is the latest ended program week, whose update is due now;
  // once the season is over there is no such week.
  const weeks = useSeasonWeeks(showLatestMark ? season?.id : undefined);
  const seasonRunning = Boolean(
    season &&
    season.status === 'open' &&
    calendarDateKey(new Date(), programTimezone) <=
      calendarDateKey(season.endsAt, programTimezone),
  );
  const dueWeek = seasonRunning
    ? completedMondayUpdateWeeks(weeks.data?.items ?? []).at(-1)
    : undefined;
  const latestMarkState = useCallback(
    (person: Person): PersonLatestMark => {
      if (!currentMenteeIds.includes(person.id)) return 'not-mentee';
      if (seasonUpdates.isLoading) return 'loading';
      if (seasonUpdates.isError) return 'error';
      return {
        latest: latestMarks.get(person.id),
        thisWeek: dueWeek
          ? weekStatus(
              dueWeek,
              (seasonUpdates.data?.items ?? []).find(
                (update) =>
                  update.student.id === person.id &&
                  sameUpdateWeek(update, dueWeek),
              ),
              isKicked(person) ? 'kicked' : undefined,
            )
          : undefined,
      };
    },
    [
      currentMenteeIds,
      dueWeek,
      latestMarks,
      seasonUpdates.data,
      seasonUpdates.isError,
      seasonUpdates.isLoading,
    ],
  );
  // Coordinators and admins manage every student from People.
  if (canManageEveryStudent && slug)
    return <Navigate to={`/seasons/${slug}/people`} replace />;
  return (
    <Page>
      <PageHeader
        title={canManageEveryStudent ? 'All Students' : 'My Students'}
      />
      <PeopleTable
        variant="students"
        toolbarStart={
          <div className="flex h-9 items-center gap-2">
            <Checkbox
              id="include-previous-students"
              checked={showPrevious}
              disabled={season?.status === 'closed'}
              onCheckedChange={(checked) => setIncludePrevious(checked)}
            />
            <Label htmlFor="include-previous-students" className="font-normal">
              Include previous students
            </Label>
          </div>
        }
        ariaLabel={canManageEveryStudent ? 'All Students' : 'Assigned students'}
        data={mentees}
        seasonId={season?.id}
        latestMark={showLatestMark ? latestMarkState : undefined}
        showThisWeek={showLatestMark && Boolean(dueWeek)}
        renderActions={(person) => (
          <StudentRowActions
            person={person}
            seasonId={season?.id}
            canSetLevel={
              canChangeLevel &&
              Boolean(season) &&
              person.enrollmentState === 'active'
            }
            canRemove={
              canChangeLevel &&
              person.enrollmentState === 'active' &&
              (canManageEveryStudent ||
                person.mentorshipMentorId === currentUser.data?.id)
            }
            onLevelChanged={() => void query.refetch()}
            renderRemoveDialog={(dialog) => (
              <RemovalDialog
                {...dialog}
                seasonId={season?.id}
                person={person}
                onRemoved={() => void query.refetch()}
              />
            )}
          />
        )}
        loading={query.isLoading || seasons.isLoading || currentUser.isLoading}
        error={query.isError || seasons.isError || currentUser.isError}
        onRetry={() =>
          void Promise.all([
            query.refetch(),
            seasons.refetch(),
            currentUser.refetch(),
          ])
        }
        emptyTitle="No Students in This View"
        emptyMessage="A Coordinator can assign students to your mentor team."
      />
    </Page>
  );
}
