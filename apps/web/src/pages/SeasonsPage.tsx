import { IconArrowRight, IconCalendarEvent } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { Link } from 'react-router-dom';

import { useCurrentUser, useSeasons } from '@/api/queries';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { SeasonEditorDialog } from '@/components/SeasonManagement';
import styles from '@/styles/App.module.css';
import type { Season } from '@/types';
import { formatDate } from '@/utils';

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
          <div className={styles.helper}>{row.original.summary}</div>
        </div>
      ),
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ getValue }) => (
        <span
          className={
            getValue() === 'open' ? styles.badgeSuccess : styles.badgeNeutral
          }
        >
          {getValue() === 'open' ? 'Open' : 'Closed'}
        </span>
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
        <div className={styles.inline}>
          {canCreate ? <SeasonEditorDialog season={row.original} /> : null}
          <Link
            className={styles.buttonQuiet}
            to={`/seasons/${row.original.slug}`}
          >
            Open <IconArrowRight size={17} aria-hidden="true" />
          </Link>
        </div>
      ),
    },
  ];
  return (
    <div className={styles.page}>
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
            <div className={styles.inline}>
              <IconCalendarEvent size={18} aria-hidden="true" />
              <span
                className={
                  row.original.status === 'open'
                    ? styles.badgeSuccess
                    : styles.badgeNeutral
                }
              >
                {row.original.status}
              </span>
            </div>
            <h2 className={`${styles.cardTitle} ${styles.cardTitleSpaced}`}>
              <HighlightText text={row.original.name} />
            </h2>
            <p className={styles.helper}>
              {formatDate(row.original.startsAt)} –{' '}
              {formatDate(row.original.endsAt)}
            </p>
            <p>{row.original.summary}</p>
            <Link
              className={styles.buttonSecondary}
              to={`/seasons/${row.original.slug}`}
            >
              Open season
            </Link>
          </div>
        )}
      />
    </div>
  );
}
