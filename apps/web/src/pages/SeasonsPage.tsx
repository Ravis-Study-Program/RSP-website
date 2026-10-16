import { Badge } from '@/components/ui/badge';
import { buttonVariants } from '@/components/ui/button';
import { IconArrowRight } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { Link } from 'react-router-dom';

import { useCurrentUser, useSeasons } from '@/api/queries';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { SeasonEditorDialog } from '@/components/SeasonManagement';
import type { Season } from '@/types';
import { formatDate } from '@/utils';

const pageClass = 'mx-auto w-full max-w-[92rem]';
const inlineClass = 'flex flex-wrap items-center gap-2.5';
const helperClass = 'text-sm text-muted-foreground';
const statusBadgeClass = 'h-5 rounded-4xl px-2 text-xs font-medium';
const badgeSuccessClass = 'bg-success-soft text-success';
const badgeNeutralClass = 'bg-muted text-muted-foreground';

export function SeasonsPage() {
  usePageTitle('Seasons');
  const query = useSeasons();
  const user = useCurrentUser();
  const canCreate = Boolean(
    user.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const canBrowseAll = Boolean(
    user.data &&
    (user.data.globalRoles.length > 0 ||
      user.data.alumni ||
      user.data.seasonRoles.some((m) => m.state === 'active')),
  );
  const visibleSeasonIds = new Set(
    user.data?.seasonRoles
      .filter(
        (membership) =>
          membership.state === 'active' || membership.state === 'completed',
      )
      .map((membership) => membership.seasonId) ?? [],
  );
  const visibleSeasons = (query.data?.items ?? []).filter(
    (season) => canBrowseAll || visibleSeasonIds.has(season.id),
  );
  const columns: ColumnDef<Season, any>[] = [
    {
      accessorKey: 'name',
      header: 'Season',
      cell: ({ row }) => (
        <div>
          <Link to={`/seasons/${row.original.slug}`}>
            <strong>
              <HighlightText text={row.original.name} />
            </strong>
          </Link>
          <div className={helperClass}>{row.original.summary}</div>
        </div>
      ),
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ getValue }) => (
        <Badge
          variant="secondary"
          className={`${statusBadgeClass} ${
            getValue() === 'open' ? badgeSuccessClass : badgeNeutralClass
          }`}
        >
          {getValue() === 'open' ? 'Open' : 'Closed'}
        </Badge>
      ),
    },
    {
      accessorKey: 'startsAt',
      header: 'Dates',
      cell: ({ row }) =>
        `${formatDate(row.original.startsAt)} – ${formatDate(row.original.endsAt)}`,
    },
    {
      accessorKey: 'memberCount',
      header: 'Members',
      cell: ({ getValue }) => getValue() ?? '—',
    },
    {
      accessorKey: 'weekCount',
      header: 'Weeks',
      cell: ({ getValue }) => getValue() ?? '—',
    },
    {
      id: 'actions',
      header: 'Actions',
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => (
        <div className={inlineClass}>
          {canCreate ? <SeasonEditorDialog season={row.original} /> : null}
          <Link
            to={`/seasons/${row.original.slug}`}
            className={buttonVariants({ variant: 'outline', size: 'sm' })}
          >
            Open <IconArrowRight size={17} aria-hidden="true" />
          </Link>
        </div>
      ),
    },
  ];
  return (
    <div className={pageClass}>
      <PageHeader
        eyebrow="Programme"
        title="Seasons"
        description="Move between current work and preserved programme history. Closed seasons stay readable and their records stay locked."
        actions={canCreate ? <SeasonEditorDialog /> : undefined}
      />
      <DataTable
        ariaLabel="Seasons"
        data={visibleSeasons}
        columns={columns}
        loading={query.isLoading || user.isLoading}
        error={query.isError || user.isError}
        onRetry={() => void Promise.all([query.refetch(), user.refetch()])}
        emptyTitle="No seasons yet"
        emptyMessage="Create the first programme season to begin."
        getRowId={(season) => season.id}
        renderCard={(row) => (
          <div>
            <div className={inlineClass}>
              <Badge
                variant="secondary"
                className={`${statusBadgeClass} ${
                  row.original.status === 'open'
                    ? badgeSuccessClass
                    : badgeNeutralClass
                }`}
              >
                {row.original.status.charAt(0).toUpperCase() +
                  row.original.status.slice(1)}
              </Badge>
            </div>
            <h2 className="mt-2.5 text-base font-semibold">
              <HighlightText text={row.original.name} />
            </h2>
            <p className={helperClass}>
              {formatDate(row.original.startsAt)} –{' '}
              {formatDate(row.original.endsAt)}
            </p>
            <p>{row.original.summary}</p>
            <Link
              to={`/seasons/${row.original.slug}`}
              className={buttonVariants({ variant: 'outline', size: 'sm' })}
            >
              Open <IconArrowRight size={17} aria-hidden="true" />
            </Link>
          </div>
        )}
      />
    </div>
  );
}
