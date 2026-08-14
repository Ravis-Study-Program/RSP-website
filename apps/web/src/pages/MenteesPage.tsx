import {
  IconArrowUp,
  IconTargetArrow,
  IconUserMinus,
} from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import type {
  Enrollment,
  Promotion,
  ReasonedRevision,
} from '@/api/generated/models';
import {
  enablePracticeGoals,
  fetchUserPracticeSettings,
} from '@/api/practiceSettingsClient';
import {
  demoMode,
  useCurrentUser,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import styles from '@/styles/App.module.css';
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

export function MenteesPage() {
  usePageTitle('Mentees');
  const { slug } = useParams();
  const seasons = useSeasons();
  const currentUser = useCurrentUser();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  const [removed, setRemoved] = useState<string[]>([]);
  const canManageEveryStudent = Boolean(
    currentUser.data?.globalRoles.length ||
    currentUser.data?.seasonRoles.some(
      (membership) =>
        membership.seasonId === season?.id && membership.role === 'coordinator',
    ),
  );
  const canGrantCoordinator = Boolean(
    currentUser.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const mentees = useMemo(
    () =>
      (query.data?.items ?? []).filter((person) => {
        const activeStudent =
          person.roles.includes('student') &&
          (person.enrollmentState
            ? person.enrollmentState === 'active'
            : person.status === 'active');
        const inScope =
          canManageEveryStudent ||
          person.mentorshipMentorId === currentUser.data?.id;
        return activeStudent && inScope && !removed.includes(person.id);
      }),
    [canManageEveryStudent, currentUser.data?.id, query.data, removed],
  );
  const columns = useMemo<ColumnDef<Person, any>[]>(
    () => [
      {
        accessorKey: 'name',
        header: 'Mentee',
        cell: ({ row }) => <PersonIdentity person={row.original} privateView />,
      },
      { accessorKey: 'attempts', header: 'Attempts' },
      { accessorKey: 'interviews', header: 'Mock interviews' },
      {
        accessorKey: 'lastActiveAt',
        header: 'Last active',
        cell: ({ getValue }) => (getValue() ? formatDateTime(getValue()) : '—'),
      },
      {
        id: 'actions',
        header: 'Actions',
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <div className={styles.inline}>
            <EnableGoalsButton seasonId={season?.id} person={row.original} />
            <PromotionDialog
              seasonId={season?.id}
              person={row.original}
              canGrantCoordinator={canGrantCoordinator}
              onChanged={() => void query.refetch()}
            />
            <RemovalDialog
              seasonId={season?.id}
              person={row.original}
              onRemoved={() => setRemoved((ids) => [...ids, row.original.id])}
            />
          </div>
        ),
      },
    ],
    [canGrantCoordinator, query, season?.id],
  );

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Mentoring"
        title="My mentees"
        description="Review raw activity and take season-scoped actions for active students assigned to you."
      />
      <DataTable
        ariaLabel="Assigned mentees"
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
        emptyTitle="No assigned mentees"
        emptyMessage="A Coordinator can assign students to your mentor team."
        getRowId={(person) => person.id}
        renderCard={(row) => (
          <div>
            <PersonIdentity person={row.original} privateView />
            <p>
              {row.original.attempts ?? '—'} attempts ·{' '}
              {row.original.interviews ?? '—'} interviews
            </p>
            <div className={styles.buttonRow}>
              <EnableGoalsButton seasonId={season?.id} person={row.original} />
              <PromotionDialog
                seasonId={season?.id}
                person={row.original}
                canGrantCoordinator={canGrantCoordinator}
                onChanged={() => void query.refetch()}
              />
              <RemovalDialog
                seasonId={season?.id}
                person={row.original}
                onRemoved={() => setRemoved((ids) => [...ids, row.original.id])}
              />
            </div>
          </div>
        )}
      />
    </div>
  );
}

function EnableGoalsButton({
  seasonId,
  person,
}: {
  seasonId?: string;
  person: Person;
}) {
  const [pending, setPending] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [error, setError] = useState('');
  const enable = async () => {
    if (!seasonId && !demoMode) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode && seasonId) {
        const settings = await fetchUserPracticeSettings(person.id);
        if (!settings.goalsEnabled)
          await enablePracticeGoals(person.id, {
            seasonId,
            revision: settings.revision,
          });
      }
      setEnabled(true);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to enable personal goals.',
      );
    } finally {
      setPending(false);
    }
  };
  if (enabled)
    return <span className={styles.badgeSuccess}>Goals enabled</span>;
  return (
    <span>
      <button
        className={styles.buttonSecondary}
        type="button"
        disabled={pending || (!demoMode && !seasonId)}
        aria-describedby={error ? `goals-enable-error-${person.id}` : undefined}
        onClick={() => void enable()}
      >
        <IconTargetArrow size={17} aria-hidden="true" />{' '}
        {pending ? 'Enabling…' : 'Enable goals'}
      </button>
      {error ? (
        <span
          id={`goals-enable-error-${person.id}`}
          className={styles.fieldError}
          role="alert"
        >
          {error}
        </span>
      ) : null}
    </span>
  );
}

function PromotionDialog({
  seasonId,
  person,
  canGrantCoordinator,
  onChanged,
}: {
  seasonId?: string;
  person: Person;
  canGrantCoordinator: boolean;
  onChanged: () => void;
}) {
  const [role, setRole] = useState<'mentor' | 'coordinator'>('mentor');
  const [reason, setReason] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const promote = async () => {
    if (demoMode) {
      onChanged();
      return;
    }
    if (!seasonId || !person.enrollmentId || !person.enrollmentRevision) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: Promotion = {
          role,
          reason: reason || undefined,
          revision: person.enrollmentRevision,
        };
        await apiRequest<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(person.enrollmentId)}/promote`,
          { method: 'POST', body: JSON.stringify(body) },
        );
      }
      onChanged();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to promote this member.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`Promote ${person.name}?`}
      description="Change this active student's season role. The action is audited."
      trigger={
        <>
          <IconArrowUp size={17} aria-hidden="true" /> Promote
        </>
      }
    >
      {error ? (
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      ) : null}
      <div className={styles.field}>
        <label htmlFor={`promotion-role-${person.id}`}>New role</label>
        <select
          id={`promotion-role-${person.id}`}
          className={styles.select}
          value={canGrantCoordinator ? role : 'mentor'}
          onChange={(event) =>
            setRole(event.target.value as 'mentor' | 'coordinator')
          }
        >
          <option value="mentor">Mentor</option>
          {canGrantCoordinator ? (
            <option value="coordinator">Coordinator</option>
          ) : null}
        </select>
      </div>
      <div className={styles.field}>
        <label htmlFor={`promotion-reason-${person.id}`}>
          Reason (optional)
        </label>
        <textarea
          id={`promotion-reason-${person.id}`}
          className={styles.textarea}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
      </div>
      <div className={styles.dialogActions}>
        <button
          className={styles.button}
          type="button"
          disabled={
            pending || (!demoMode && (!seasonId || !person.enrollmentId))
          }
          onClick={() => void promote()}
        >
          {pending ? 'Promoting…' : 'Promote student'}
        </button>
      </div>
    </FormDialog>
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
    if (!seasonId || !person.enrollmentId || !person.enrollmentRevision) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: ReasonedRevision = {
          reason: reason.trim(),
          revision: person.enrollmentRevision,
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
