import { useCallback, useMemo } from 'react';
import { Navigate, useLocation, useParams } from 'react-router-dom';

import {
  completedMondayUpdateWeeks,
  programTimezone,
  sameUpdateWeek,
} from '@/activityDates';
import type { MondayUpdate } from '@/api/generated/models';
import {
  useCurrentUser,
  useMondayUpdates,
  useSeasonEnrollments,
  useSeasons,
  useSeasonPeople,
  useSeasonWeeks,
} from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { PeopleTable, type PersonLatestMark } from '@/components/PeopleTable';
import { PersonRowActions } from '@/components/PersonRowActions';
import { pickLatestMark } from '@/mondayMarks';
import { weekStatus } from '@/mondayStatus';
import type { Person } from '@/types';
import { calendarDateKey } from '@/utils';
import { useSeasonSwitcher } from '@/components/useSeasonSwitcher';

function isKicked(person: Person) {
  return (person.enrollmentState ?? person.status) === 'kicked';
}

/** A mentor's own students, even for an admin who could see everyone. */
export function MyStudentsPage() {
  return <MenteesPage scope="mine" />;
}

export function MenteesPage({ scope }: { scope?: 'mine' } = {}) {
  const { slug } = useParams();
  const location = useLocation();
  const seasons = useSeasons();
  const currentUser = useCurrentUser();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  // A closed season shows its students as they ended up; a mentor looks at an
  // earlier season by choosing it at the top left.
  const showPrevious = season?.status === 'closed';
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
  // The seasons this person has mentored in, newest first, to switch between.
  const mentoredSeasonIds = new Set(
    currentUser.data?.seasonRoles
      .filter((membership) => membership.role === 'mentor')
      .map((membership) => membership.seasonId) ?? [],
  );
  const seasonSwitcher = useSeasonSwitcher(
    location.pathname.split('/').pop() ?? 'my-students',
    undefined,
    (seasons.data?.items ?? []).filter((item) =>
      mentoredSeasonIds.has(item.id),
    ),
  );
  // The row menu is the People one, with the same rules; correcting an
  // enrollment needs the enrollment itself.
  const viewer = currentUser.data;
  const correctsEnrollments = Boolean(
    viewer?.globalRoles.length ||
    viewer?.seasonRoles.some(
      (membership) =>
        membership.seasonId === season?.id &&
        membership.role === 'coordinator' &&
        membership.state === 'active',
    ),
  );
  const enrollments = useSeasonEnrollments(
    season && correctsEnrollments ? season.id : undefined,
  );
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
        toolbarLead={seasonSwitcher}
        ariaLabel={canManageEveryStudent ? 'All Students' : 'Assigned students'}
        data={mentees}
        seasonId={season?.id}
        latestMark={showLatestMark ? latestMarkState : undefined}
        showThisWeek={showLatestMark && Boolean(dueWeek)}
        renderActions={(person) => (
          <PersonRowActions
            viewer={viewer}
            person={person}
            season={season}
            seasons={seasons.data?.items ?? []}
            enrollment={enrollments.data?.items.find(
              (item) => item.id === person.enrollmentId,
            )}
            onChanged={() =>
              void Promise.all([query.refetch(), enrollments.refetch()])
            }
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
