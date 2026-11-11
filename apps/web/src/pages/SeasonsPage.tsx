import { statusTone } from '@/badgeTones';
import { SpreadsheetImport } from '@/components/SpreadsheetImport';
import { StatusBadge } from '@/components/StatusBadge';
import type { ColumnDef } from '@tanstack/react-table';
import { Link } from 'react-router-dom';

import { useCurrentUser, useSeasons } from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import {
  SeasonEditorDialog,
  SeasonRowMenu,
} from '@/components/SeasonManagement';
import type { Season } from '@/types';
import { formatDate } from '@/utils';

const inlineClass = 'flex flex-wrap items-center gap-2.5';
const helperClass = 'text-sm text-muted-foreground';

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
  // Directors and System Admins edit, close, reopen and delete; a season's
  // coordinators can close it, as on the season page.
  const seasonActions = (season: Season) => {
    const coordinates = Boolean(
      user.data?.seasonRoles.some(
        (membership) =>
          membership.seasonId === season.id &&
          membership.role === 'coordinator' &&
          membership.state === 'active',
      ),
    );
    return {
      canEdit: canCreate,
      canClose: season.status === 'open' && (canCreate || coordinates),
      canReopen: season.status === 'closed' && canCreate,
      canDelete: canCreate,
    };
  };
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
        </div>
      ),
    },
    {
      accessorKey: 'status',
      header: 'Status',
      meta: { width: 7.5 },
      cell: ({ getValue }) => (
        <StatusBadge tone={statusTone(getValue())}>
          {getValue() === 'open' ? 'Open' : 'Closed'}
        </StatusBadge>
      ),
    },
    {
      accessorKey: 'startsAt',
      header: 'Dates',
      meta: { width: 13 },
      cell: ({ row }) =>
        `${formatDate(row.original.startsAt)} – ${formatDate(row.original.endsAt)}`,
    },
    {
      accessorKey: 'memberCount',
      header: 'Members',
      meta: { width: 8 },
      cell: ({ getValue }) => getValue() ?? '—',
    },
    {
      accessorKey: 'weekCount',
      header: 'Weeks',
      meta: { width: 7 },
      cell: ({ getValue }) => getValue() ?? '—',
    },
    {
      id: 'actions',
      header: 'Actions',
      meta: { width: 6, align: 'end' },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => (
        <SeasonRowMenu season={row.original} {...seasonActions(row.original)} />
      ),
    },
  ];
  return (
    <Page>
      <PageHeader
        title="Seasons"
        actions={
          canCreate ? (
            <div className="button-group !ms-0">
              <SpreadsheetImport
                kinds={['seasons']}
                title="Import Seasons"
                trigger="Import Seasons"
                seasons={query.data?.items ?? []}
              />
              <SeasonEditorDialog />
            </div>
          ) : undefined
        }
      />
      <DataTable
        ariaLabel="Seasons"
        data={visibleSeasons}
        columns={columns}
        loading={query.isLoading || user.isLoading}
        error={query.isError || user.isError}
        onRetry={() => void Promise.all([query.refetch(), user.refetch()])}
        emptyTitle="No Seasons Yet"
        emptyMessage="Create the first program season to begin."
        getRowId={(season) => season.id}
        rowHref={(season) => `/seasons/${season.slug}`}
        renderCard={(row) => (
          <div>
            <div className={inlineClass}>
              <StatusBadge tone={statusTone(row.original.status)}>
                {row.original.status.charAt(0).toUpperCase() +
                  row.original.status.slice(1)}
              </StatusBadge>
            </div>
            <h2 className="mt-2.5 text-base font-semibold">
              <HighlightText text={row.original.name} />
            </h2>
            <p className={helperClass}>
              {formatDate(row.original.startsAt)} –{' '}
              {formatDate(row.original.endsAt)}
            </p>
            <div className="flex justify-end">
              <SeasonRowMenu
                season={row.original}
                {...seasonActions(row.original)}
              />
            </div>
          </div>
        )}
      />
    </Page>
  );
}
