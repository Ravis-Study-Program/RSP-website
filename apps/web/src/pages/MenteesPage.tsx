import { IconArrowUp, IconUserMinus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useMemo, useState } from 'react';

import { usePeople } from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import styles from '@/styles/App.module.css';
import type { Person } from '@/types';
import { formatDateTime } from '@/utils';

export function MenteesPage() {
  usePageTitle('Mentees');
  const query = usePeople();
  const [removed, setRemoved] = useState<string[]>([]);
  const mentees = useMemo(() => (query.data?.items ?? []).filter((person) => person.roles.includes('student') && !removed.includes(person.id)), [query.data, removed]);
  const columns = useMemo<ColumnDef<Person, any>[]>(() => [
    { accessorKey: 'name', header: 'Mentee', cell: ({ row }) => <PersonIdentity person={row.original} privateView /> },
    { accessorKey: 'attempts', header: 'Attempts' },
    { accessorKey: 'interviews', header: 'Mock interviews' },
    { accessorKey: 'lastActiveAt', header: 'Last active', cell: ({ getValue }) => formatDateTime(getValue()) },
    { id: 'actions', header: 'Actions', enableSorting: false, enableHiding: false, cell: ({ row }) => <div className={styles.inline}><PromotionDialog person={row.original} /><RemovalDialog person={row.original} onRemoved={() => setRemoved((ids) => [...ids, row.original.id])} /></div> },
  ], []);

  return (
    <div className={styles.page}>
      <PageHeader eyebrow="Mentoring" title="My mentees" description="Review raw activity and take season-scoped actions for active students assigned to you." />
      <DataTable
        ariaLabel="Assigned mentees"
        data={mentees}
        columns={columns}
        loading={query.isLoading}
        error={query.isError}
        onRetry={() => void query.refetch()}
        emptyTitle="No assigned mentees"
        emptyMessage="A Coordinator can assign students to your mentor team."
        getRowId={(person) => person.id}
        renderCard={(row) => <div><PersonIdentity person={row.original} privateView /><p>{row.original.attempts} attempts · {row.original.interviews} interviews</p><div className={styles.buttonRow}><PromotionDialog person={row.original} /><RemovalDialog person={row.original} onRemoved={() => setRemoved((ids) => [...ids, row.original.id])} /></div></div>}
      />
    </div>
  );
}

function PromotionDialog({ person }: { person: Person }) {
  return (
    <FormDialog title={`Promote ${person.name}?`} description="Promotion completes the student enrolment and grants alumni access. The action is audited." trigger={<><IconArrowUp size={17} aria-hidden="true" /> Promote</>}>
      <p>Confirm that {person.name} has completed the programme requirements for this season.</p>
      <div className={styles.dialogActions}><button className={styles.button} type="button">Promote student</button></div>
    </FormDialog>
  );
}

function RemovalDialog({ person, onRemoved }: { person: Person; onRemoved: () => void }) {
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  return (
    <FormDialog title={`Remove ${person.name}?`} description="Removal immediately revokes season resources and directory access. The reason and actor are stored in an immutable audit event." trigger={<><IconUserMinus size={17} aria-hidden="true" /> Remove</>}>
      <div className={styles.field}><label htmlFor={`reason-${person.id}`}>Removal reason</label><textarea id={`reason-${person.id}`} className={styles.textarea} required value={reason} onChange={(event) => setReason(event.target.value)} /></div>
      <div className={styles.field}><label htmlFor={`name-${person.id}`}>Type <strong>{person.name}</strong> to confirm</label><input id={`name-${person.id}`} className={styles.input} value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></div>
      <div className={styles.dialogActions}><button className={styles.buttonDanger} type="button" disabled={reason.trim().length < 5 || confirmation !== person.name} onClick={onRemoved}>Remove member</button></div>
    </FormDialog>
  );
}
