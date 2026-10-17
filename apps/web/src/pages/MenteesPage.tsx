import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldLabel } from '@/components/ui/field';
import { IconUserMinus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';

import { programmeTimezone } from '@/activityDates';
import { apiRequest } from '@/api/client';
import type { Enrollment, MondayUpdate } from '@/api/generated/models';
import {
  demoMode,
  useCurrentUser,
  useMondayUpdates,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { LatestMarkCell } from '@/components/MondayMark';
import { latestMarkRank, pickLatestMark, type LatestMark } from '@/mondayMarks';
import { StudentLevelDialog } from '@/components/StudentLevelDialog';
import { canSetStudentLevel, studentLevelLabel } from '@/studentLevels';
import { FormDialog } from '@/components/Dialogs';
import type { Person } from '@/types';
import { calendarDateKey, formatDateTime } from '@/utils';

/** A row's latest-mark cell: a status, or the mentee's latest update. */
type MenteeMark =
  | 'not-mentee'
  | 'loading'
  | 'error'
  | { latest: LatestMark | undefined };

const menteeMarkText = {
  'not-mentee': '—',
  loading: 'Loading…',
  error: 'Unavailable',
} as const;

function MenteeMarkCell({ state }: { state: MenteeMark }) {
  if (typeof state === 'string')
    return (
      <span className="text-sm text-muted-foreground">
        {menteeMarkText[state]}
      </span>
    );
  return <LatestMarkCell latest={state.latest} noUpdateText="No updates yet" />;
}

export function MenteesPage() {
  usePageTitle('My students');
  const { slug } = useParams();
  const seasons = useSeasons();
  const currentUser = useCurrentUser();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  const [includePrevious, setIncludePrevious] = useState(false);
  const showPrevious = includePrevious || season?.status === 'closed';
  const [removed, setRemoved] = useState<string[]>([]);
  const canManageEveryStudent = Boolean(
    currentUser.data?.globalRoles.length ||
      currentUser.data?.seasonRoles.some(
        (membership) =>
          membership.seasonId === season?.id &&
          membership.role === 'coordinator' &&
          (membership.state === 'active' || membership.state === 'completed'),
      ),
  );
  const canChangeLevel = canSetStudentLevel(currentUser.data, season);
  const mentees = useMemo(
    () =>
      (query.data?.items ?? []).filter((person) => {
        const activeStudent =
          (person.seasonRole
            ? person.seasonRole === 'student'
            : person.roles.includes('student')) &&
          (person.enrollmentState
            ? person.enrollmentState === 'active'
            : person.status === 'active');
        const assigned = person.mentorshipMentorId === currentUser.data?.id;
        const previouslyAssigned =
          person.previousMentorIds?.includes(currentUser.data?.id ?? '') ??
          false;
        if (showPrevious)
          return canManageEveryStudent
            ? person.seasonRole === 'student'
            : assigned || previouslyAssigned;
        return (
          activeStudent &&
          (canManageEveryStudent || assigned) &&
          !removed.includes(person.id)
        );
      }),
    [
      canManageEveryStudent,
      currentUser.data?.id,
      query.data,
      removed,
      showPrevious,
    ],
  );
  // Coordinators and admins have no marks of their own; the column is
  // mentor-only and covers the mentor's current mentees for the whole season.
  const showLatestMark = Boolean(currentUser.data) && !canManageEveryStudent;
  const currentMenteeIds = useMemo(
    () =>
      (query.data?.items ?? [])
        .filter((person) => person.mentorshipMentorId === currentUser.data?.id)
        .map((person) => person.id),
    [currentUser.data?.id, query.data],
  );
  const seasonUpdateFilters = useMemo(
    () => ({
      startDate: season
        ? calendarDateKey(season.startsAt, programmeTimezone)
        : '',
      endDate: season ? calendarDateKey(season.endsAt, programmeTimezone) : '',
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
  const latestMarkState = useCallback(
    (person: Person): MenteeMark => {
      if (!currentMenteeIds.includes(person.id)) return 'not-mentee';
      if (seasonUpdates.isLoading) return 'loading';
      if (seasonUpdates.isError) return 'error';
      return { latest: latestMarks.get(person.id) };
    },
    [
      currentMenteeIds,
      latestMarks,
      seasonUpdates.isError,
      seasonUpdates.isLoading,
    ],
  );
  const columns = useMemo<ColumnDef<Person, any>[]>(
    () => [
      {
        accessorKey: 'name',
        header: 'Student',
        cell: ({ row }) => (
          <PersonIdentity
            seasonId={season?.id}
            person={row.original}
            privateView
          />
        ),
      },
      {
        accessorKey: 'studentLevel',
        header: 'Level',
        cell: ({ row }) => studentLevelLabel(row.original.studentLevel),
      },
      { accessorKey: 'attempts', header: 'Attempts' },
      { accessorKey: 'interviews', header: 'Mock interviews' },
      {
        accessorKey: 'lastActiveAt',
        header: 'Last recorded activity',
        cell: ({ getValue }) => (getValue() ? formatDateTime(getValue()) : '—'),
      },
      ...(showLatestMark
        ? [
            {
              id: 'latestMark',
              header: 'Latest Monday mark',
              accessorFn: (person: Person) => {
                const state = latestMarkState(person);
                return typeof state === 'string'
                  ? 7
                  : latestMarkRank(state.latest);
              },
              cell: ({ row }: { row: { original: Person } }) => (
                <MenteeMarkCell state={latestMarkState(row.original)} />
              ),
            } satisfies ColumnDef<Person, number>,
          ]
        : []),
      {
        id: 'actions',
        header: 'Actions',
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <div className="flex flex-wrap items-center gap-2">
            {canChangeLevel &&
            season &&
            row.original.enrollmentState === 'active' ? (
              <StudentLevelDialog
                seasonId={season.id}
                person={row.original}
                onChanged={() => void query.refetch()}
              />
            ) : null}
            {canChangeLevel &&
            row.original.enrollmentState === 'active' &&
            (canManageEveryStudent ||
              row.original.mentorshipMentorId === currentUser.data?.id) ? (
              <RemovalDialog
                seasonId={season?.id}
                person={row.original}
                onRemoved={() => setRemoved((ids) => [...ids, row.original.id])}
              />
            ) : null}
          </div>
        ),
      },
    ],
    [
      canChangeLevel,
      canManageEveryStudent,
      currentUser.data?.id,
      latestMarkState,
      query,
      season,
      setRemoved,
      showLatestMark,
    ],
  );

  return (
    <div className="mx-auto w-full max-w-[92rem]">
      <PageHeader
        eyebrow="Mentoring"
        title={canManageEveryStudent ? 'All students' : 'My students'}
        description={
          canManageEveryStudent
            ? 'Review students in this season.'
            : 'Review students assigned to you. Open People to browse every member in this season.'
        }
      />
      <div className="mt-2 mb-4 flex min-h-6 items-center gap-2">
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
      <DataTable
        ariaLabel={canManageEveryStudent ? 'All students' : 'Assigned students'}
        data={mentees}
        columns={columns}
        loading={query.isLoading || seasons.isLoading || currentUser.isLoading}
        error={query.isError || seasons.isError || currentUser.isError}
        onRetry={() =>
          void Promise.all([
            query.refetch(),
            seasons.refetch(),
            currentUser.refetch(),
          ])
        }
        emptyTitle="No students in this view"
        emptyMessage="A Coordinator can assign students to your mentor team."
        getRowId={(person) => person.id}
        renderCard={(row) => (
          <div>
            <PersonIdentity
              seasonId={season?.id}
              person={row.original}
              privateView
            />
            <p>
              {row.original.attempts ?? '—'} attempts ·{' '}
              {row.original.interviews ?? '—'} interviews
            </p>
            {showLatestMark ? (
              <MenteeMarkCell state={latestMarkState(row.original)} />
            ) : null}
            <div className="flex flex-wrap items-center gap-2">
              {canChangeLevel &&
              season &&
              row.original.enrollmentState === 'active' ? (
                <StudentLevelDialog
                  seasonId={season.id}
                  person={row.original}
                  onChanged={() => void query.refetch()}
                />
              ) : null}
              {canChangeLevel &&
              row.original.enrollmentState === 'active' &&
              (canManageEveryStudent ||
                row.original.mentorshipMentorId === currentUser.data?.id) ? (
                <RemovalDialog
                  seasonId={season?.id}
                  person={row.original}
                  onRemoved={() =>
                    setRemoved((ids) => [...ids, row.original.id])
                  }
                />
              ) : null}
            </div>
          </div>
        )}
      />
    </div>
  );
}

function RemovalDialog({
  seasonId,
  person,
  onRemoved,
}: {
  seasonId?: string;
  person: Person;
  onRemoved: () => void;
}) {
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const remove = async () => {
    if (demoMode) {
      onRemoved();
      return;
    }
    if (!seasonId || !person.enrollmentId) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body = {
          reason: reason.trim(),
        };
        await apiRequest<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(person.enrollmentId)}/remove`,
          { method: 'POST', body: JSON.stringify(body) },
        );
      }
      onRemoved();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to remove this member.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      triggerVariant="destructive"
      triggerSize="sm"
      title={`Remove ${person.name}?`}
      description="Removal immediately revokes season resources and directory access. The reason and actor are stored in an immutable audit event."
      trigger={
        <>
          <IconUserMinus size={17} aria-hidden="true" /> Remove
        </>
      }
    >
      <Field>
        <FieldLabel htmlFor={`reason-${person.id}`}>Removal reason</FieldLabel>
        <Textarea
          id={`reason-${person.id}`}
          required
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor={`name-${person.id}`}>
          Type <strong>{person.name}</strong> to confirm
        </FieldLabel>
        <Input
          id={`name-${person.id}`}
          value={confirmation}
          onChange={(event) => setConfirmation(event.target.value)}
        />
      </Field>
      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          variant="destructive"
          type="button"
          disabled={
            pending ||
            reason.trim().length < 5 ||
            confirmation !== person.name ||
            (!demoMode && (!seasonId || !person.enrollmentId))
          }
          onClick={() => void remove()}
        >
          {pending ? 'Removing…' : 'Remove member'}
        </Button>
      </div>
    </FormDialog>
  );
}
