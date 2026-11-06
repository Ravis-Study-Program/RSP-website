import { buttonVariants } from '@/components/ui/button';
import { statusTone } from '@/badgeTones';
import { StatusBadge } from '@/components/StatusBadge';
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
  SeasonBadge,
  StudentLevelBadge,
} from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { StudentLevelDialog } from '@/components/StudentLevelDialog';
import { canSetStudentLevel, studentLevelLabel } from '@/studentLevels';
import { EnrollmentDialog } from '@/pages/AdminPage';
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const inlineClass = 'flex flex-wrap items-center gap-2.5';
const helperClass = 'text-sm text-muted-foreground';
const personCellClass =
  'min-w-48 whitespace-normal [&_a]:whitespace-normal [&_span]:whitespace-normal';

function statusLabel(status: string | null | undefined) {
  const text = (status ?? '').replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
}

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
          <div className={personCellClass}>
            <PersonIdentity
              seasonId={season?.id}
              person={row.original}
              privateView={privateView}
              showRoles={false}
            />
          </div>
        ),
      },
      {
        id: 'roles',
        header: 'Roles',
        meta: { width: 11 },
        accessorFn: (person) => person.roles.join(' '),
        cell: ({ row }) => (
          <div className={inlineClass}>
            {row.original.roles.map((role) => (
              <RoleBadge key={role} role={role} />
            ))}
          </div>
        ),
      },
      {
        accessorKey: 'season',
        header: 'Season',
        meta: { width: 6.5 },
        cell: ({ row }: { row: { original: Person } }) => {
          // A person's season is its name or slug.
          const match = seasons.data?.items.find(
            (item) =>
              item.name === row.original.season ||
              item.slug === row.original.season,
          );
          return match ? (
            <SeasonBadge name={match.name} startsAt={match.startsAt} />
          ) : (
            (row.original.season ?? '—')
          );
        },
      },
      ...(season
        ? [
            {
              id: 'studentLevel',
              header: 'Level',
              meta: { width: 8 },
              accessorFn: (person: Person) =>
                studentLevelLabel(person.studentLevel),
              cell: ({ row }: { row: { original: Person } }) =>
                row.original.studentLevel &&
                row.original.studentLevel !== 'not_applicable' ? (
                  <StudentLevelBadge level={row.original.studentLevel} />
                ) : (
                  '—'
                ),
            },
            ...(canChangeLevel &&
            data.some(
              (person) =>
                person.enrollmentState === 'active' &&
                person.seasonRole === 'student',
            )
              ? [
                  {
                    id: 'levelActions',
                    header: 'Actions',
                    meta: { width: 10, align: 'end' as const },
                    enableSorting: false,
                    cell: ({ row }: { row: { original: Person } }) =>
                      season &&
                      row.original.enrollmentState === 'active' &&
                      row.original.seasonRole === 'student' ? (
                        <StudentLevelDialog
                          seasonId={season.id}
                          person={row.original}
                          onChanged={() => void query.refetch()}
                        />
                      ) : null,
                  },
                ]
              : []),
          ]
        : []),
      {
        accessorKey: 'status',
        header: 'Status',
        meta: { width: 8 },
        cell: ({ getValue }) => (
          <StatusBadge tone={statusTone(getValue())}>
            {statusLabel(getValue())}
          </StatusBadge>
        ),
      },
      { accessorKey: 'attempts', header: 'Attempts', meta: { width: 7 } },
      {
        accessorKey: 'interviews',
        header: 'Interviews',
        meta: { width: 7.5 },
      },
      {
        accessorKey: 'lastActiveAt',
        header: 'Last Recorded Activity',
        meta: { width: 11.5 },
        cell: ({ getValue }) => (getValue() ? formatDateTime(getValue()) : '—'),
      },
    ],
    [privateView, season, seasons.data?.items, canChangeLevel, data, query],
  );

  return (
    <div className={pageClass}>
      <PageHeader
        title={graduatesOnly ? 'Graduates' : 'People'}
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
        emptyTitle={
          graduatesOnly
            ? 'No Graduated Students Yet'
            : 'No Current Students Yet'
        }
        getRowId={(person) => person.id}
        renderCard={(row) => (
          <div>
            <PersonIdentity
              seasonId={season?.id}
              person={row.original}
              privateView={privateView}
              showRoles={false}
            />
            <div className={`${inlineClass} mt-2.5`}>
              {row.original.roles.map((role) => (
                <RoleBadge key={role} role={role} />
              ))}
              <StatusBadge tone={statusTone(row.original.status)}>
                {statusLabel(row.original.status)}
              </StatusBadge>
            </div>
            <p>{row.original.season}</p>
            {season && row.original.seasonRole === 'student' ? (
              <p>Level: {studentLevelLabel(row.original.studentLevel)}</p>
            ) : null}
            <p className={helperClass}>
              {row.original.attempts} attempts · {row.original.interviews}{' '}
              interviews
            </p>
            <div className={`${inlineClass} mt-2.5 justify-end`}>
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
              {privateView && row.original.email ? (
                <a
                  href={`mailto:${row.original.email}`}
                  className={buttonVariants({ variant: 'outline', size: 'sm' })}
                >
                  <IconMail size={17} aria-hidden="true" /> Email
                </a>
              ) : null}
            </div>
          </div>
        )}
      />
    </div>
  );
}
