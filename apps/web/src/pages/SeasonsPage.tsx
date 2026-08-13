import { IconArrowRight, IconCalendarEvent, IconPlus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { Link } from 'react-router-dom';

import { useSeasons } from '@/api/queries';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, HighlightText } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import { InlineNotice } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import type { Season } from '@/types';
import { formatDate } from '@/utils';

const columns: ColumnDef<Season, any>[] = [
  {
    accessorKey: 'name',
    header: 'Season',
    cell: ({ row }) => <div><Link to={`/seasons/${row.original.slug}`}><strong><HighlightText text={row.original.name} /></strong></Link><div className={styles.helper}>{row.original.summary}</div></div>,
  },
  { accessorKey: 'status', header: 'Status', cell: ({ getValue }) => <span className={getValue() === 'open' ? styles.badgeSuccess : styles.badgeNeutral}>{getValue() === 'open' ? 'Open' : 'Closed'}</span> },
  { accessorKey: 'startsAt', header: 'Dates', cell: ({ row }) => `${formatDate(row.original.startsAt)} – ${formatDate(row.original.endsAt)}` },
  { accessorKey: 'memberCount', header: 'Members' },
  { accessorKey: 'weekCount', header: 'Weeks' },
  { id: 'actions', header: 'Actions', enableSorting: false, enableHiding: false, cell: ({ row }) => <Link className={styles.buttonQuiet} to={`/seasons/${row.original.slug}`}>Open <IconArrowRight size={17} aria-hidden="true" /></Link> },
];

export function SeasonsPage() {
  usePageTitle('Seasons');
  const query = useSeasons();
  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Programme"
        title="Seasons"
        description="Move between current work and preserved programme history. Closed seasons stay readable and their records stay locked."
        actions={
          <FormDialog title="Create season" description="Season definitions are available to Directors. The API records every change." trigger={<><IconPlus size={18} aria-hidden="true" /> Create season</>}>
            <InlineNotice>Season creation is connected to the generated v2 contract. Complete the definition in Administration.</InlineNotice>
            <div className={styles.dialogActions}><Link className={styles.button} to="/admin/seasons">Open administration</Link></div>
          </FormDialog>
        }
      />
      <DataTable
        ariaLabel="Seasons"
        data={query.data?.items ?? []}
        columns={columns}
        loading={query.isLoading}
        error={query.isError}
        onRetry={() => void query.refetch()}
        emptyTitle="No seasons yet"
        emptyMessage="Create the first programme season to begin."
        getRowId={(season) => season.id}
        renderCard={(row) => (
          <div>
            <div className={styles.inline}><IconCalendarEvent size={18} aria-hidden="true" /><span className={row.original.status === 'open' ? styles.badgeSuccess : styles.badgeNeutral}>{row.original.status}</span></div>
            <h2 className={styles.cardTitle} style={{ marginTop: '0.65rem' }}><HighlightText text={row.original.name} /></h2>
            <p className={styles.helper}>{formatDate(row.original.startsAt)} – {formatDate(row.original.endsAt)}</p>
            <p>{row.original.summary}</p>
            <Link className={styles.buttonSecondary} to={`/seasons/${row.original.slug}`}>Open season</Link>
          </div>
        )}
      />
    </div>
  );
}
