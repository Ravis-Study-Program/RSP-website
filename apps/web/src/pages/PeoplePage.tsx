import { buttonVariants } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
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
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const inlineClass = 'flex flex-wrap items-center gap-2.5';
const helperClass = 'text-sm text-muted-foreground';
const badgeSuccessClass = 'bg-success-soft text-success';
const badgeWarningClass = 'bg-warning-soft text-warning';
const badgeNeutralClass = 'bg-muted text-muted-foreground';
const statusBadgeClass = 'h-5 rounded-4xl px-2 text-xs font-medium';
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
            />
          </div>
        ),
      },
      {
        id: 'roles',
        header: 'Roles',
        accessorFn: (person) => person.roles.join(' '),
        cell: ({ row }) => (
          <div className={inlineClass}>
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
              cell: ({ row }: { row: { original: Person } }) =>
                studentLevelLabel(row.original.studentLevel),
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
        cell: ({ getValue }) => (
          <Badge
            variant="secondary"
            className={`${statusBadgeClass} ${
              getValue() === 'active'
                ? badgeSuccessClass
                : getValue() === 'unassigned'
                  ? badgeWarningClass
                  : badgeNeutralClass
            }`}
          >
            {statusLabel(getValue())}
          </Badge>
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
    [privateView, season, canChangeLevel, data, query],
  );

  return (
    <div className={pageClass}>
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
            <PersonIdentity
              seasonId={season?.id}
              person={row.original}
              privateView={privateView}
            />
            <div className={`${inlineClass} mt-2.5`}>
              {row.original.roles.map((role) => (
                <RoleBadge key={role} role={role} />
              ))}
              <Badge
                variant="secondary"
                className={`${statusBadgeClass} ${
                  row.original.status === 'active'
                    ? badgeSuccessClass
                    : row.original.status === 'unassigned'
                      ? badgeWarningClass
                      : badgeNeutralClass
                }`}
              >
                {statusLabel(row.original.status)}
              </Badge>
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
