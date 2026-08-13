import { IconMail, IconUserPlus, IconUsers } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo } from 'react';
import { Link, useLocation } from 'react-router-dom';

import { usePeople } from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { PageHeader, PersonIdentity, RoleBadge, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import { InlineNotice } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

export function PeoplePage() {
  const location = useLocation();
  const graduatesOnly = location.pathname === '/graduates';
  usePageTitle(graduatesOnly ? 'Graduates' : 'People');
  const query = usePeople();
  const { activeWorkspace } = useWorkspace();
  const privateView = ['coordinator', 'director', 'system_admin'].includes(activeWorkspace?.role ?? '');
  const data = useMemo(() => (query.data?.items ?? []).filter((person) => !graduatesOnly || person.roles.includes('graduate')), [graduatesOnly, query.data]);

  const columns = useMemo<ColumnDef<Person, any>[]>(() => [
    { accessorKey: 'name', header: 'Person', cell: ({ row }) => <PersonIdentity person={row.original} privateView={privateView} /> },
    { id: 'roles', header: 'Roles', accessorFn: (person) => person.roles.join(' '), cell: ({ row }) => <div className={styles.inline}>{row.original.roles.map((role) => <RoleBadge key={role} role={role} />)}</div> },
    { accessorKey: 'season', header: 'Season' },
    { accessorKey: 'status', header: 'Status', cell: ({ getValue }) => <span className={getValue() === 'active' ? styles.badgeSuccess : getValue() === 'unassigned' ? styles.badgeWarning : styles.badgeNeutral}>{getValue()}</span> },
    { accessorKey: 'attempts', header: 'Attempts' },
    { accessorKey: 'interviews', header: 'Interviews' },
    { accessorKey: 'lastActiveAt', header: 'Last active', cell: ({ getValue }) => formatDateTime(getValue()) },
  ], [privateView]);

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow={graduatesOnly ? 'Alumni directory' : 'Season community'}
        title={graduatesOnly ? 'Graduates' : 'People'}
        description={graduatesOnly ? 'Find programme graduates and view their public practice and interview participation.' : 'Approved active members and alumni can find one another here. Private fields stay limited to authorised relationships.'}
        actions={privateView ? <FormDialog title="Add member" description="Add an existing verified account to this season. A person can hold exactly one season role." trigger={<><IconUserPlus size={18} aria-hidden="true" /> Add member</>}><InlineNotice>Use the enrolment administration screen to search verified accounts and assign a season role.</InlineNotice><div className={styles.dialogActions}><Link className={styles.button} to="/admin/enrollments">Open enrolments</Link></div></FormDialog> : undefined}
      />
      <DataTable
        ariaLabel={graduatesOnly ? 'Graduate directory' : 'People directory'}
        data={data}
        columns={columns}
        loading={query.isLoading}
        error={query.isError}
        onRetry={() => void query.refetch()}
        emptyTitle={graduatesOnly ? 'No graduates yet' : 'No people available'}
        emptyMessage={graduatesOnly ? 'A completed student enrolment grants graduate access.' : 'Approved members will appear when enrolments are active.'}
        getRowId={(person) => person.id}
        renderCard={(row) => <div><PersonIdentity person={row.original} privateView={privateView} /><div className={styles.inline} style={{ marginTop: '0.7rem' }}>{row.original.roles.map((role) => <RoleBadge key={role} role={role} />)}<span className={row.original.status === 'active' ? styles.badgeSuccess : row.original.status === 'unassigned' ? styles.badgeWarning : styles.badgeNeutral}>{row.original.status}</span></div><p>{row.original.season}</p><p className={styles.helper}>{row.original.attempts} attempts · {row.original.interviews} interviews</p>{privateView && row.original.email ? <a className={styles.buttonSecondary} href={`mailto:${row.original.email}`}><IconMail size={17} aria-hidden="true" /> Email</a> : null}</div>}
      />
    </div>
  );
}
