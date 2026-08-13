import {
  IconCalendarEvent,
  IconChecklist,
  IconClock,
  IconPlus,
  IconShieldLock,
  IconUsers,
  IconUsersGroup,
} from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { Link } from 'react-router-dom';

import { usePeople, useSeasons } from '@/api/queries';
import { MetricCard, PageHeader, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { InlineNotice } from '@/components/StatusViews';
import { seasonWeeks } from '@/data/demo';
import styles from '@/styles/App.module.css';
import { formatDate } from '@/utils';

const adminResources = [
  { slug: 'seasons', title: 'Seasons', description: 'Definitions, dates and lifecycle', icon: IconCalendarEvent },
  { slug: 'weeks', title: 'Weeks', description: 'Schedule and resource links', icon: IconClock },
  { slug: 'users', title: 'Users', description: 'Profiles and account states', icon: IconUsers },
  { slug: 'enrollments', title: 'Enrollments', description: 'Season roles and membership', icon: IconChecklist },
  { slug: 'mentorships', title: 'Mentorships', description: 'Mentor–student assignments', icon: IconUsersGroup },
] as const;

export function AdminPage() {
  usePageTitle('Administration');
  const seasons = useSeasons();
  const people = usePeople();
  return (
    <div className={styles.page}>
      <PageHeader eyebrow="Audited operations" title="Administration" description="Manage programme definitions and relationships. Privileged changes require recent MFA and leave immutable audit events." />
      <InlineNotice><IconShieldLock size={19} aria-hidden="true" /> Your privileged session has recent MFA. Identity and lifecycle operations are always audited.</InlineNotice>
      <div className={styles.metricGrid}>
        <MetricCard label="Seasons" value={seasons.data?.totalCount ?? '—'} detail="One currently open" />
        <MetricCard label="Visible users" value={people.data?.totalCount ?? '—'} detail="Active members and alumni" />
        <MetricCard label="Pending operations" value="3" detail="Assignments and enrolment reviews" />
      </div>
      <div className={styles.cardGrid}>
        {adminResources.map((resource) => {
          const Icon = resource.icon;
          return <article className={styles.card} key={resource.slug}><span className={styles.avatar}><Icon size={20} aria-hidden="true" /></span><h2 className={styles.cardTitle} style={{ marginTop: '0.8rem' }}>{resource.title}</h2><p className={styles.muted}>{resource.description}</p><Link className={styles.buttonSecondary} to={`/admin/${resource.slug}`}>Manage {resource.title.toLowerCase()}</Link></article>;
        })}
      </div>
      <section className={styles.section}><div className={styles.sectionHeader}><h2 className={styles.sectionTitle}>Technical operations</h2></div><div className={styles.panel}><div className={styles.listRow}><span><strong>LeetCode catalogue sync</strong><span className={styles.helper} style={{ display: 'block' }}>Last successful worker run: Sunday 03:02 UTC</span></span><button className={styles.buttonSecondary} type="button">Run audited sync</button></div><div className={styles.listRow}><span><strong>Audit export</strong><span className={styles.helper} style={{ display: 'block' }}>Filtered redacted events for incident review</span></span><button className={styles.buttonSecondary} type="button">Open audit log</button></div></div></section>
    </div>
  );
}

interface AdminRow {
  id: string;
  primary: string;
  secondary: string;
  status: string;
  detail: string;
  revision: number;
}

const columns: ColumnDef<AdminRow, any>[] = [
  { accessorKey: 'primary', header: 'Name', cell: ({ row }) => <div><strong>{row.original.primary}</strong><div className={styles.helper}>{row.original.secondary}</div></div> },
  { accessorKey: 'status', header: 'Status', cell: ({ getValue }) => <span className={getValue() === 'active' || getValue() === 'open' ? styles.badgeSuccess : getValue() === 'pending' || getValue() === 'unassigned' ? styles.badgeWarning : styles.badgeNeutral}>{getValue()}</span> },
  { accessorKey: 'detail', header: 'Details' },
  { accessorKey: 'revision', header: 'Revision', cell: ({ getValue }) => `r${getValue()}` },
  { id: 'actions', header: 'Actions', enableSorting: false, enableHiding: false, cell: ({ row }) => <div className={styles.inline}><button className={styles.buttonSecondary} type="button">Edit</button><NamedConfirmation name={row.original.primary} actionLabel="Remove" description="This action validates the latest revision and creates an audit event." onConfirm={() => undefined} /></div> },
];

export function AdminResourcePage({ resource }: { resource: (typeof adminResources)[number]['slug'] }) {
  const seasons = useSeasons();
  const people = usePeople();
  const config = adminResources.find((item) => item.slug === resource)!;
  usePageTitle(`Admin ${config.title}`);
  const rows: AdminRow[] = resource === 'seasons'
    ? (seasons.data?.items ?? []).map((season) => ({ id: season.id, primary: season.name, secondary: season.slug, status: season.status, detail: `${formatDate(season.startsAt)} – ${formatDate(season.endsAt)}`, revision: season.revision }))
    : resource === 'weeks'
      ? seasonWeeks.map((week) => ({ id: `week_${week.week}`, primary: `Week ${week.week}: ${week.topic}`, secondary: week.resource, status: week.week <= 3 ? 'active' : 'scheduled', detail: formatDate(week.startsAt), revision: 1 }))
      : resource === 'users'
        ? (people.data?.items ?? []).map((person) => ({ id: person.id, primary: person.name, secondary: person.email ?? person.slug, status: person.status, detail: person.roles.join(', '), revision: 1 }))
        : resource === 'enrollments'
          ? (people.data?.items ?? []).map((person) => ({ id: `enrol_${person.id}`, primary: person.name, secondary: person.season, status: person.status, detail: person.roles.filter((role) => ['student', 'mentor', 'coordinator'].includes(role)).join(', ') || 'alumni', revision: 2 }))
          : (people.data?.items ?? []).filter((person) => person.roles.includes('student')).map((person, index) => ({ id: `mentor_${person.id}`, primary: person.name, secondary: 'Student', status: person.status === 'unassigned' ? 'unassigned' : 'active', detail: person.status === 'unassigned' ? 'No mentor assigned' : `Mentor team ${index + 1}`, revision: 1 }));
  const loading = resource === 'seasons' ? seasons.isLoading : resource === 'weeks' ? false : people.isLoading;
  const error = resource === 'seasons' ? seasons.isError : resource === 'weeks' ? false : people.isError;

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Administration"
        title={config.title}
        description={config.description}
        actions={<FormDialog title={`Create ${config.title.slice(0, -1).toLowerCase()}`} description="Required relationships and ranges are validated both here and in PostgreSQL." trigger={<><IconPlus size={18} aria-hidden="true" /> Create</>}><InlineNotice>Creation uses the v2 contract and includes an initial revision of zero.</InlineNotice><div className={styles.dialogActions}><button className={styles.button} type="button">Continue</button></div></FormDialog>}
      />
      <DataTable
        ariaLabel={`Admin ${config.title}`}
        data={rows}
        columns={columns}
        loading={loading}
        error={error}
        onRetry={() => void (resource === 'seasons' ? seasons.refetch() : people.refetch())}
        emptyTitle={`No ${config.title.toLowerCase()}`}
        emptyMessage="Create the first record when the programme is ready."
        getRowId={(row) => row.id}
        renderCard={(row) => <div><div className={styles.inline}><span className={row.original.status === 'active' || row.original.status === 'open' ? styles.badgeSuccess : styles.badgeNeutral}>{row.original.status}</span><span className={styles.badgeNeutral}>r{row.original.revision}</span></div><h2 className={styles.cardTitle} style={{ marginTop: '0.65rem' }}>{row.original.primary}</h2><p className={styles.helper}>{row.original.secondary}</p><p>{row.original.detail}</p><div className={styles.buttonRow}><button className={styles.buttonSecondary} type="button">Edit</button></div></div>}
      />
    </div>
  );
}

export const AdminSeasonsPage = () => <AdminResourcePage resource="seasons" />;
export const AdminWeeksPage = () => <AdminResourcePage resource="weeks" />;
export const AdminUsersPage = () => <AdminResourcePage resource="users" />;
export const AdminEnrollmentsPage = () => <AdminResourcePage resource="enrollments" />;
export const AdminMentorshipsPage = () => <AdminResourcePage resource="mentorships" />;
