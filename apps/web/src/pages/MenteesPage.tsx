import { IconUserMinus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import type { Enrollment } from '@/api/generated/models';
import {
  demoMode,
  useCurrentUser,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { StudentLevelDialog } from '@/components/StudentLevelDialog';
import { canSetStudentLevel, studentLevelLabel } from '@/studentLevels';
import { FormDialog } from '@/components/Dialogs';
import styles from '@/styles/App.module.css';
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

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
      {
        id: 'actions',
        header: 'Actions',
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <div className={styles.inline}>
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
      query,
      season,
      setRemoved,
    ],
  );

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Mentoring"
        title={canManageEveryStudent ? 'All students' : 'My students'}
        description={
          canManageEveryStudent
            ? 'Review students in this season.'
            : 'Review students assigned to you. Open People to browse every member in this season.'
        }
      />
      <label className={styles.inline}>
        <input
          type="checkbox"
          checked={showPrevious}
          disabled={season?.status === 'closed'}
          onChange={(event) => setIncludePrevious(event.target.checked)}
        />{' '}
        Include previous students
      </label>
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
            <div className={styles.buttonRow}>
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
      title={`Remove ${person.name}?`}
      description="Removal immediately revokes season resources and directory access. The reason and actor are stored in an immutable audit event."
      trigger={
        <>
          <IconUserMinus size={17} aria-hidden="true" /> Remove
        </>
      }
    >
      <div className={styles.field}>
        <label htmlFor={`reason-${person.id}`}>Removal reason</label>
        <textarea
          id={`reason-${person.id}`}
          className={styles.textarea}
          required
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
      </div>
      <div className={styles.field}>
        <label htmlFor={`name-${person.id}`}>
          Type <strong>{person.name}</strong> to confirm
        </label>
        <input
          id={`name-${person.id}`}
          className={styles.input}
          value={confirmation}
          onChange={(event) => setConfirmation(event.target.value)}
        />
      </div>
      {error ? (
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      ) : null}
      <div className={styles.dialogActions}>
        <button
          className={styles.buttonDanger}
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
        </button>
      </div>
    </FormDialog>
  );
}
