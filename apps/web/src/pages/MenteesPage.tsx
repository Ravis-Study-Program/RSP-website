import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldLabel } from '@/components/ui/field';
import { useCallback, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';

import { programTimezone } from '@/activityDates';
import { apiRequest } from '@/api/client';
import type { Enrollment, MondayUpdate } from '@/api/generated/models';
import {
  demoMode,
  useCurrentUser,
  useMondayUpdates,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import {
  PeopleTable,
  StudentRowActions,
  type PersonLatestMark,
} from '@/components/PeopleTable';
import { pickLatestMark } from '@/mondayMarks';
import { canSetStudentLevel } from '@/studentLevels';
import { FormDialog } from '@/components/Dialogs';
import type { Person } from '@/types';
import { calendarDateKey } from '@/utils';

export function MenteesPage() {
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
  usePageTitle(canManageEveryStudent ? 'All Students' : 'My Students');
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
  const latestMarkState = useCallback(
    (person: Person): PersonLatestMark => {
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
                onRemoved={() => setRemoved((ids) => [...ids, person.id])}
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

/** Removes a student from the season, opened from their row menu. */
function RemovalDialog({
  seasonId,
  person,
  onRemoved,
  open,
  onOpenChange,
}: {
  seasonId?: string;
  person: Person;
  onRemoved: () => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const remove = async () => {
    if (demoMode) {
      onRemoved();
      onOpenChange(false);
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
      onOpenChange(false);
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
      title={`Remove ${person.name}?`}
      description="This student immediately loses access to season resources and the directory. Your name and the removal reason are recorded."
      open={open}
      onOpenChange={onOpenChange}
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
