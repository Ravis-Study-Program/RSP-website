import { IconMail } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo } from 'react';
import { useLocation, useParams } from 'react-router-dom';

import {
  useEnrollmentCandidates,
  useCurrentUser,
  usePeople,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import {
  PageHeader,
  PersonIdentity,
  RoleBadge,
  usePageTitle,
} from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { StudentLevelDialog } from '@/components/StudentLevelDialog';
import { canSetStudentLevel, studentLevelLabel } from '@/studentLevels';
import { EnrollmentDialog } from '@/pages/AdminPage';
import styles from '@/styles/App.module.css';
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

export function PeoplePage() {
  const location = useLocation();
  const { slug } = useParams();
  const graduatesOnly = location.pathname === '/graduates';
  usePageTitle(graduatesOnly ? 'Graduates' : 'People');
  const publicQuery = usePeople();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const currentUser = useCurrentUser();
  const canChangeLevel = canSetStudentLevel(currentUser.data, season);
  const seasonQuery = useSeasonPeople(season?.id, season?.name);
  const query = slug ? seasonQuery : publicQuery;
  const { activeWorkspace } = useWorkspace();
  const privateView = ['coordinator', 'director', 'system_admin'].includes(
    activeWorkspace?.role ?? '',
  );
  const enrollmentCandidates = useEnrollmentCandidates(
    season?.id,
    privateView && Boolean(season),
  );
  const canGrantCoordinator =
    activeWorkspace?.role === 'director' ||
    activeWorkspace?.role === 'system_admin';
  const data = useMemo(
    () =>
      (query.data?.items ?? []).filter(
        (person) => !graduatesOnly || person.roles.includes('graduate'),
      ),
    [graduatesOnly, query.data],
  );

  const columns = useMemo<ColumnDef<Person, any>[]>(
    () => [
      {
        accessorKey: 'name',
        header: 'Person',
        cell: ({ row }) => (
          <PersonIdentity person={row.original} privateView={privateView} />
        ),
      },
      {
        id: 'roles',
        header: 'Roles',
        accessorFn: (person) => person.roles.join(' '),
        cell: ({ row }) => (
          <div className={styles.inline}>
            {row.original.roles.map((role) => (
              <RoleBadge key={role} role={role} />
            ))}
          </div>
        ),
      },
      { accessorKey: 'season', header: 'Season' },
      ...(season
        ? [
            {
              id: 'studentLevel',
              header: 'Level',
              accessorFn: (person: Person) =>
                studentLevelLabel(person.studentLevel),
              cell: ({ row }: { row: { original: Person } }) => (
                <div className={styles.inline}>
                  <span>{studentLevelLabel(row.original.studentLevel)}</span>
                  {canChangeLevel &&
                  row.original.enrollmentState === 'active' &&
                  row.original.seasonRole === 'student' ? (
                    <StudentLevelDialog
                      seasonId={season.id}
                      person={row.original}
                      onChanged={() => void query.refetch()}
                    />
                  ) : null}
                </div>
              ),
            },
          ]
        : []),
      {
        accessorKey: 'status',
        header: 'Status',
        cell: ({ getValue }) => (
          <span
            className={
              getValue() === 'active'
                ? styles.badgeSuccess
                : getValue() === 'unassigned'
                  ? styles.badgeWarning
                  : styles.badgeNeutral
            }
          >
            {getValue()}
          </span>
        ),
      },
      { accessorKey: 'attempts', header: 'Attempts' },
      { accessorKey: 'interviews', header: 'Interviews' },
      {
        accessorKey: 'lastActiveAt',
        header: 'Last recorded activity',
        cell: ({ getValue }) => (getValue() ? formatDateTime(getValue()) : '—'),
      },
    ],
    [privateView, season, canChangeLevel, query],
  );

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow={graduatesOnly ? 'Alumni directory' : 'Season community'}
        title={graduatesOnly ? 'Graduates' : 'People'}
        description={
          graduatesOnly
            ? 'Find programme graduates and view their public practice and interview participation.'
            : 'Approved active members and alumni can find one another here. Private fields stay limited to authorised relationships.'
        }
        actions={
          privateView && season ? (
            <EnrollmentDialog
              seasonId={season.id}
              accounts={enrollmentCandidates.data?.items ?? []}
              accountsLoading={enrollmentCandidates.isLoading}
              accountsError={enrollmentCandidates.isError}
              onRetryAccounts={() => void enrollmentCandidates.refetch()}
              allowCoordinator={canGrantCoordinator}
            />
          ) : undefined
        }
      />
      <DataTable
        ariaLabel={graduatesOnly ? 'Graduate directory' : 'People directory'}
        data={data}
        columns={columns}
        loading={query.isLoading || (Boolean(slug) && seasons.isLoading)}
        error={query.isError || (Boolean(slug) && seasons.isError)}
        onRetry={() => void query.refetch()}
        emptyTitle={graduatesOnly ? 'No graduates yet' : 'No people available'}
        emptyMessage={
          graduatesOnly
            ? 'A completed student enrolment grants graduate access.'
            : 'Approved members will appear when enrolments are active.'
        }
        getRowId={(person) => person.id}
        renderCard={(row) => (
          <div>
            <PersonIdentity person={row.original} privateView={privateView} />
            <div className={`${styles.inline} ${styles.inlineSpaced}`}>
              {row.original.roles.map((role) => (
                <RoleBadge key={role} role={role} />
              ))}
              <span
                className={
                  row.original.status === 'active'
                    ? styles.badgeSuccess
                    : row.original.status === 'unassigned'
                      ? styles.badgeWarning
                      : styles.badgeNeutral
                }
              >
                {row.original.status}
              </span>
            </div>
            <p>{row.original.season}</p>
            {season && row.original.seasonRole === 'student' ? (
              <p>Level: {studentLevelLabel(row.original.studentLevel)}</p>
            ) : null}
            {canChangeLevel &&
            season &&
            row.original.enrollmentState === 'active' &&
            row.original.seasonRole === 'student' ? (
              <StudentLevelDialog
                seasonId={season.id}
                person={row.original}
                onChanged={() => void query.refetch()}
              />
            ) : null}
            <p className={styles.helper}>
              {row.original.attempts} attempts · {row.original.interviews}{' '}
              interviews
            </p>
            {privateView && row.original.email ? (
              <a
                className={styles.buttonSecondary}
                href={`mailto:${row.original.email}`}
              >
                <IconMail size={17} aria-hidden="true" /> Email
              </a>
            ) : null}
          </div>
        )}
      />
    </div>
  );
}
