import {
  IconCalendarEvent,
  IconChecklist,
  IconClock,
  IconShieldLock,
  IconUsers,
  IconUsersGroup,
  IconEdit,
  IconPlus,
  IconUserMinus,
} from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useQueryClient } from '@tanstack/react-query';
import { useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';

import { ApiProblem, apiRequest } from '@/api/client';
import type {
  AccountStateMutation,
  Enrollment,
  EnrollmentCandidate,
  EnrollmentCandidatePage,
  EnrollmentCreate,
  EnrollmentPage,
  EnrollmentUpdate,
  GlobalRole,
  GlobalRoleAssignment,
  GlobalRoleAssignmentList,
  GlobalRoleGrant,
  Mentorship,
  MentorshipCreate,
  MentorshipPage,
  MentorshipUpdate,
  PageInfo,
  UserPrivate,
  Week,
  WeekMutation,
  WeekPage,
  WeekUpdate,
} from '@/api/generated/models';
import {
  demoMode,
  enrollmentCandidatesQueryKey,
  seasonEnrollmentsQueryKey,
  seasonMentorshipsQueryKey,
  seasonWeeksQueryKey,
  useAdminUsers,
  useCurrentUser,
  useEnrollmentCandidates,
  usePeople,
  useSeasonEnrollments,
  useSeasonMentorships,
  useSeasonWeeks,
  useSeasons,
} from '@/api/queries';
import { MetricCard, PageHeader, usePageTitle } from '@/components/Common';
import { AppDatePicker } from '@/components/AppDatePicker';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import {
  SeasonEditorDialog,
  SeasonLifecycleDialog,
} from '@/components/SeasonManagement';
import { InlineNotice } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import type { Page as ViewPage, Person, SeasonRole } from '@/types';
import { formatDate, formatDateTimeInput, zonedDateTimeToUtc } from '@/utils';

const adminResources = [
  {
    slug: 'seasons',
    title: 'Seasons',
    description: 'Definitions, dates and lifecycle',
    icon: IconCalendarEvent,
  },
  {
    slug: 'weeks',
    title: 'Weeks',
    description: 'Schedule and resource links',
    icon: IconClock,
  },
  {
    slug: 'users',
    title: 'Users',
    description: 'Profiles and account states',
    icon: IconUsers,
  },
  {
    slug: 'enrollments',
    title: 'Enrollments',
    description: 'Season roles and membership',
    icon: IconChecklist,
  },
  {
    slug: 'mentorships',
    title: 'Mentorships',
    description: 'Mentor–student assignments',
    icon: IconUsersGroup,
  },
] as const;

export function AdminPage() {
  usePageTitle('Administration');
  const seasons = useSeasons();
  const people = usePeople();
  const currentUser = useCurrentUser();
  const [syncStatus, setSyncStatus] = useState('');
  const [syncPending, setSyncPending] = useState(false);
  const sync = async () => {
    setSyncPending(true);
    setSyncStatus('');
    try {
      if (!demoMode)
        await apiRequest<{ status: string }>('/admin/leetcode/sync', {
          method: 'POST',
        });
      setSyncStatus('LeetCode sync queued.');
    } catch (error) {
      setSyncStatus(
        error instanceof Error ? error.message : 'Unable to queue the sync.',
      );
    } finally {
      setSyncPending(false);
    }
  };
  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Audited operations"
        title="Administration"
        description="Manage programme definitions and relationships. Privileged changes require recent MFA and leave immutable audit events."
      />
      <InlineNotice>
        <IconShieldLock size={19} aria-hidden="true" /> Your privileged session
        has recent MFA. Identity and lifecycle operations are always audited.
      </InlineNotice>
      <div className={styles.metricGrid}>
        <MetricCard
          label="Seasons"
          value={seasons.data?.totalCount ?? '—'}
          detail={`${seasons.data?.items.filter((season) => season.status === 'open').length ?? '—'} open`}
        />
        <MetricCard
          label="Visible users"
          value={people.data?.totalCount ?? '—'}
          detail="Active members and alumni"
        />
        <MetricCard
          label="Pending operations"
          value="—"
          detail="No aggregate endpoint is available"
        />
      </div>
      <div className={styles.cardGrid}>
        {adminResources
          .filter(
            (resource) =>
              resource.slug !== 'users' ||
              currentUser.data?.globalRoles.includes('system_admin'),
          )
          .map((resource) => {
            const Icon = resource.icon;
            return (
              <article className={styles.card} key={resource.slug}>
                <span className={styles.avatar}>
                  <Icon size={20} aria-hidden="true" />
                </span>
                <h2 className={`${styles.cardTitle} ${styles.cardTitleSpaced}`}>
                  {resource.title}
                </h2>
                <p className={styles.muted}>{resource.description}</p>
                <Link
                  className={styles.buttonSecondary}
                  to={`/admin/${resource.slug}`}
                >
                  Manage {resource.title.toLowerCase()}
                </Link>
              </article>
            );
          })}
      </div>
      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>Technical operations</h2>
        </div>
        <div className={styles.panel}>
          {syncStatus ? <p role="status">{syncStatus}</p> : null}
          <div className={styles.listRow}>
            <span>
              <strong>LeetCode catalogue sync</strong>
              <span className={`${styles.helper} ${styles.helperBlock}`}>
                Queues an audited worker sync; the regular schedule remains
                Sunday 03:00 UTC.
              </span>
            </span>
            <button
              className={styles.buttonSecondary}
              type="button"
              disabled={syncPending}
              onClick={() => void sync()}
            >
              {syncPending ? 'Queueing…' : 'Run audited sync'}
            </button>
          </div>
          <div className={styles.listRow}>
            <span>
              <strong>Audit export</strong>
              <span className={`${styles.helper} ${styles.helperBlock}`}>
                The API does not yet expose an audit-query endpoint.
              </span>
            </span>
            <span className={styles.badgeNeutral}>Unavailable</span>
          </div>
        </div>
      </section>
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
  actions?: ReactNode;
}

const columns: ColumnDef<AdminRow, any>[] = [
  {
    accessorKey: 'primary',
    header: 'Name',
    cell: ({ row }) => (
      <div>
        <strong>{row.original.primary}</strong>
        <div className={styles.helper}>{row.original.secondary}</div>
      </div>
    ),
  },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ getValue }) => (
      <span
        className={
          getValue() === 'active' || getValue() === 'open'
            ? styles.badgeSuccess
            : getValue() === 'pending' || getValue() === 'unassigned'
              ? styles.badgeWarning
              : styles.badgeNeutral
        }
      >
        {getValue()}
      </span>
    ),
  },
  { accessorKey: 'detail', header: 'Details' },
  {
    accessorKey: 'revision',
    header: 'Revision',
    cell: ({ getValue }) => `r${getValue()}`,
  },
  {
    id: 'actions',
    header: 'Actions',
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => row.original.actions ?? '—',
  },
];

export function AdminResourcePage({
  resource,
}: {
  resource: (typeof adminResources)[number]['slug'];
}) {
  const seasons = useSeasons();
  const people = usePeople(resource !== 'users');
  const currentUser = useCurrentUser();
  const canGrantCoordinator = Boolean(
    currentUser.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const canAdministerUsers = Boolean(
    currentUser.data?.globalRoles.includes('system_admin'),
  );
  const adminUsers = useAdminUsers(resource === 'users' && canAdministerUsers);
  const [selectedSeasonId, setSelectedSeasonId] = useState('');
  const globallyManaged = Boolean(currentUser.data?.globalRoles.length);
  const activeCoordinatorSeasonIds = new Set(
    currentUser.data?.seasonRoles
      .filter(
        (membership) =>
          membership.role === 'coordinator' && membership.state === 'active',
      )
      .map((membership) => membership.seasonId) ?? [],
  );
  const manageableSeasons = (seasons.data?.items ?? []).filter(
    (season) => globallyManaged || activeCoordinatorSeasonIds.has(season.id),
  );
  const selectedSeason =
    manageableSeasons.find((season) => season.id === selectedSeasonId) ??
    manageableSeasons.find((season) => season.status === 'open') ??
    manageableSeasons[0];
  const weeks = useSeasonWeeks(selectedSeason?.id);
  const enrollments = useSeasonEnrollments(selectedSeason?.id);
  const mentorships = useSeasonMentorships(selectedSeason?.id);
  const enrollmentCandidates = useEnrollmentCandidates(
    selectedSeason?.id,
    resource === 'enrollments',
  );
  const config = adminResources.find((item) => item.slug === resource)!;
  usePageTitle(`Admin ${config.title}`);
  const peopleById = useMemo(
    () =>
      new Map((people.data?.items ?? []).map((person) => [person.id, person])),
    [people.data],
  );
  const administeredPeople = useMemo(
    () =>
      (adminUsers.data?.items ?? []).map((account) => ({
        account,
        person: {
          id: account.id,
          name: account.name,
          slug: account.slug,
          initials: account.name
            .split(/\s+/)
            .map((part) => part[0])
            .join('')
            .slice(0, 2)
            .toUpperCase(),
          avatarUrl: account.avatarUrl ?? null,
          roles: [
            ...new Set([
              ...account.globalRoles,
              ...account.seasonRoles.map((membership) => membership.role),
            ]),
          ],
          season:
            account.seasonRoles.find(
              (membership) => membership.state === 'active',
            )?.seasonSlug ?? null,
          status:
            account.accountState === 'active' ? ('active' as const) : null,
          attempts: account.attemptCount,
          interviews: account.mockInterviewCount,
          email: account.email,
          lastActiveAt: null,
          revision: account.revision,
        } satisfies Person,
      })),
    [adminUsers.data],
  );
  const rows: AdminRow[] =
    resource === 'seasons'
      ? (seasons.data?.items ?? []).map((season) => ({
          id: season.id,
          primary: season.name,
          secondary: season.slug,
          status: season.status,
          detail: `${formatDate(season.startsAt)} – ${formatDate(season.endsAt)}`,
          revision: season.revision,
          actions: (
            <div className={styles.inline}>
              <SeasonEditorDialog season={season} />
              {season.status === 'open' ? (
                <SeasonLifecycleDialog season={season} action="close" />
              ) : (
                <SeasonLifecycleDialog season={season} action="reopen" />
              )}
            </div>
          ),
        }))
      : resource === 'weeks'
        ? (weeks.data?.items ?? []).map((week) => ({
            id: week.id,
            primary: `Week ${week.number}`,
            secondary: week.resourceUrl || 'No resource link',
            status:
              new Date(week.endAt).getTime() < Date.now()
                ? 'completed'
                : new Date(week.startAt).getTime() > Date.now()
                  ? 'scheduled'
                  : 'active',
            detail: `${formatDate(week.startAt)} – ${formatDate(week.endAt)}`,
            revision: week.revision,
            actions: selectedSeason ? (
              <div className={styles.inline}>
                <WeekDialog seasonId={selectedSeason.id} week={week} />
                <DeleteWeekButton seasonId={selectedSeason.id} week={week} />
              </div>
            ) : undefined,
          }))
        : resource === 'users'
          ? administeredPeople.map(({ account, person }) => ({
              id: account.id,
              primary: account.name,
              secondary: account.email,
              status: account.accountState,
              detail: person.roles.join(', ') || 'Registered nonmember',
              revision: account.revision,
              actions: (
                <div className={styles.inline}>
                  <Link
                    className={styles.buttonQuiet}
                    to={`/people/${account.slug}`}
                  >
                    Open profile
                  </Link>
                  <UserAdministrationDialog
                    person={person}
                    currentUserId={currentUser.data?.id}
                  />
                </div>
              ),
            }))
          : resource === 'enrollments'
            ? (enrollments.data?.items ?? []).map((enrollment) => ({
                id: enrollment.id,
                primary:
                  peopleById.get(enrollment.userId)?.name ?? enrollment.userId,
                secondary:
                  peopleById.get(enrollment.userId)?.slug ?? enrollment.userId,
                status: enrollment.state,
                detail: `${enrollment.role}${enrollment.role === 'student' ? ` · ${enrollment.studentLevel.replace('_', ' ')}` : ''}${enrollment.assignmentState === 'pending_mfa' ? ' · pending MFA' : ''}`,
                revision: enrollment.revision,
                actions: selectedSeason ? (
                  <div className={styles.inline}>
                    {enrollment.role !== 'coordinator' ? (
                      <EnrollmentDialog
                        seasonId={selectedSeason.id}
                        accounts={people.data?.items ?? []}
                        enrollment={enrollment}
                      />
                    ) : null}
                    <RemoveEnrollmentDialog
                      seasonId={selectedSeason.id}
                      enrollment={enrollment}
                      name={
                        peopleById.get(enrollment.userId)?.name ??
                        enrollment.userId
                      }
                    />
                  </div>
                ) : undefined,
              }))
            : (mentorships.data?.items ?? []).map((mentorship) => ({
                id: mentorship.id,
                primary:
                  peopleById.get(mentorship.studentUserId)?.name ??
                  mentorship.studentUserId,
                secondary: `Student · ${peopleById.get(mentorship.studentUserId)?.slug ?? mentorship.studentUserId}`,
                status: 'active',
                detail: `Mentored by ${peopleById.get(mentorship.mentorUserId)?.name ?? mentorship.mentorUserId}`,
                revision: mentorship.revision,
                actions: selectedSeason ? (
                  <div className={styles.inline}>
                    <MentorshipDialog
                      seasonId={selectedSeason.id}
                      people={people.data?.items ?? []}
                      mentorship={mentorship}
                    />
                    <DeleteMentorshipButton
                      seasonId={selectedSeason.id}
                      mentorship={mentorship}
                    />
                  </div>
                ) : undefined,
              }));
  const selectedQuery =
    resource === 'weeks'
      ? weeks
      : resource === 'enrollments'
        ? enrollments
        : resource === 'mentorships'
          ? mentorships
          : null;
  const loading =
    resource === 'seasons'
      ? seasons.isLoading
      : resource === 'users'
        ? adminUsers.isLoading || currentUser.isLoading
        : seasons.isLoading ||
          people.isLoading ||
          Boolean(selectedQuery?.isLoading);
  const error =
    resource === 'seasons'
      ? seasons.isError
      : resource === 'users'
        ? adminUsers.isError || currentUser.isError
        : seasons.isError || people.isError || Boolean(selectedQuery?.isError);
  const createAction =
    resource === 'seasons' ? (
      <SeasonEditorDialog />
    ) : selectedSeason && resource === 'weeks' ? (
      <WeekDialog seasonId={selectedSeason.id} />
    ) : selectedSeason && resource === 'enrollments' ? (
      <EnrollmentDialog
        seasonId={selectedSeason.id}
        accounts={enrollmentCandidates.data?.items ?? []}
        accountsLoading={enrollmentCandidates.isLoading}
        accountsError={enrollmentCandidates.isError}
        onRetryAccounts={() => void enrollmentCandidates.refetch()}
        allowCoordinator={canGrantCoordinator}
      />
    ) : selectedSeason && resource === 'mentorships' ? (
      <MentorshipDialog
        seasonId={selectedSeason.id}
        people={people.data?.items ?? []}
      />
    ) : undefined;

  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Administration"
        title={config.title}
        description={config.description}
        actions={createAction}
      />
      {resource !== 'seasons' && resource !== 'users' ? (
        <div className={`${styles.panel} ${styles.panelSpaced}`}>
          <div className={styles.field}>
            <label htmlFor={`admin-${resource}-season`}>Season</label>
            <select
              id={`admin-${resource}-season`}
              className={styles.select}
              value={selectedSeason?.slug ?? ''}
              disabled={!manageableSeasons.length}
              onChange={(event) => {
                const season = manageableSeasons.find(
                  (item) => item.slug === event.target.value,
                );
                setSelectedSeasonId(season?.id ?? '');
              }}
            >
              {manageableSeasons.length ? (
                manageableSeasons.map((season) => (
                  <option value={season.slug} key={season.id}>
                    {season.name} · {season.status}
                  </option>
                ))
              ) : (
                <option value="">No manageable season</option>
              )}
            </select>
          </div>
        </div>
      ) : null}
      <DataTable
        ariaLabel={`Admin ${config.title}`}
        data={rows}
        columns={columns}
        loading={loading}
        error={error}
        onRetry={() =>
          void Promise.all([
            seasons.refetch(),
            resource === 'users' ? adminUsers.refetch() : people.refetch(),
            selectedQuery?.refetch(),
            resource === 'enrollments'
              ? enrollmentCandidates.refetch()
              : undefined,
          ])
        }
        emptyTitle={`No ${config.title.toLowerCase()}`}
        emptyMessage="Create the first record when the programme is ready."
        getRowId={(row) => row.id}
        renderCard={(row) => (
          <div>
            <div className={styles.inline}>
              <span
                className={
                  row.original.status === 'active' ||
                  row.original.status === 'open'
                    ? styles.badgeSuccess
                    : styles.badgeNeutral
                }
              >
                {row.original.status}
              </span>
              <span className={styles.badgeNeutral}>
                r{row.original.revision}
              </span>
            </div>
            <h2 className={`${styles.cardTitle} ${styles.cardTitleSpaced}`}>
              {row.original.primary}
            </h2>
            <p className={styles.helper}>{row.original.secondary}</p>
            <p>{row.original.detail}</p>
            {row.original.actions ? (
              <div className={styles.buttonRow}>{row.original.actions}</div>
            ) : null}
          </div>
        )}
      />
    </div>
  );
}

function UserAdministrationDialog({
  person,
  currentUserId,
}: {
  person: Person;
  currentUserId?: string;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [user, setUser] = useState<UserPrivate | null>(null);
  const [assignments, setAssignments] = useState<GlobalRoleAssignment[]>([]);
  const [loading, setLoading] = useState(false);
  const [pending, setPending] = useState('');
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [accountReason, setAccountReason] = useState('');
  const [accountConfirmation, setAccountConfirmation] = useState('');
  const [grantRole, setGrantRole] = useState<GlobalRole>('director');
  const [grantReason, setGrantReason] = useState('');
  const [roleReason, setRoleReason] = useState('');
  const [roleConfirmation, setRoleConfirmation] = useState('');

  const load = async () => {
    setLoading(true);
    setUser(null);
    setAssignments([]);
    setError('');
    setMessage('');
    try {
      if (demoMode) {
        const globalRoles = person.roles.filter(
          (role): role is GlobalRole =>
            role === 'director' || role === 'system_admin',
        );
        setUser({
          id: person.id,
          slug: person.slug,
          name: person.name,
          avatarUrl: person.avatarUrl,
          timezone: 'Australia/Adelaide',
          timezoneConfigured: true,
          globalRoles,
          seasonRoles: [],
          attemptCount: person.attempts ?? 0,
          mockInterviewCount: person.interviews ?? 0,
          revision: person.revision ?? 1,
          email: person.email ?? `${person.slug}@example.test`,
          accountState: 'active',
        });
        setAssignments(
          globalRoles.map((role, index) => ({
            id: `role_${person.id}_${role}`,
            userId: person.id,
            role,
            state: 'active',
            revision: index + 1,
          })),
        );
      } else {
        const [privateUser, roleList] = await Promise.all([
          apiRequest<UserPrivate>(`/users/${encodeURIComponent(person.id)}`),
          apiRequest<GlobalRoleAssignmentList>(
            `/admin/users/${encodeURIComponent(person.id)}/global-roles`,
          ),
        ]);
        setUser(privateUser);
        setAssignments(
          roleList.items.filter((assignment) => assignment.state !== 'revoked'),
        );
      }
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to load account administration details.',
      );
    } finally {
      setLoading(false);
    }
  };
  const onOpenChange = (next: boolean) => {
    if (pending) return;
    setOpen(next);
    if (next) void load();
    else {
      setAccountReason('');
      setAccountConfirmation('');
      setGrantReason('');
      setRoleReason('');
      setRoleConfirmation('');
    }
  };
  const changeAccountState = async () => {
    if (!user) return;
    const state: AccountStateMutation['state'] =
      user.accountState === 'suspended' ? 'active' : 'suspended';
    if (
      !accountReason.trim() ||
      (state === 'suspended' && accountConfirmation !== person.name)
    )
      return;
    setPending('account');
    setError('');
    setMessage('');
    try {
      const updated: UserPrivate = demoMode
        ? { ...user, accountState: state, revision: user.revision + 1 }
        : await apiRequest<UserPrivate>(
            `/admin/users/${encodeURIComponent(person.id)}/account-state`,
            {
              method: 'POST',
              body: JSON.stringify({
                state,
                reason: accountReason.trim(),
                revision: user.revision,
              } satisfies AccountStateMutation),
            },
          );
      setUser(updated);
      setAccountReason('');
      setAccountConfirmation('');
      setMessage(
        state === 'suspended'
          ? 'Account suspended and sessions revoked.'
          : 'Account reactivated.',
      );
      queryClient.setQueryData<ViewPage<UserPrivate>>(
        ['admin-users'],
        (current) => upsertPageItem(current, updated),
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-users'] }),
        queryClient.invalidateQueries({ queryKey: ['people'] }),
      ]);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to change the account state.',
      );
      if (!demoMode) void load();
    } finally {
      setPending('');
    }
  };
  const grantGlobalRole = async () => {
    if (!grantReason.trim()) return;
    setPending(`grant-${grantRole}`);
    setError('');
    setMessage('');
    try {
      const saved = demoMode
        ? {
            id: `role_${person.id}_${grantRole}`,
            userId: person.id,
            role: grantRole,
            state: 'pending_mfa' as const,
            revision: 1,
          }
        : await apiRequest<GlobalRoleAssignment>(
            `/admin/users/${encodeURIComponent(person.id)}/global-roles`,
            {
              method: 'POST',
              body: JSON.stringify({
                role: grantRole,
                reason: grantReason.trim(),
              } satisfies GlobalRoleGrant),
            },
          );
      setAssignments((current) => [
        ...current.filter((assignment) => assignment.role !== saved.role),
        saved,
      ]);
      setGrantReason('');
      setMessage(
        saved.state === 'pending_mfa'
          ? `${roleLabel(saved.role)} granted pending MFA configuration.`
          : `${roleLabel(saved.role)} granted.`,
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-users'] }),
        queryClient.invalidateQueries({ queryKey: ['people'] }),
      ]);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to grant the global role.',
      );
    } finally {
      setPending('');
    }
  };
  const revokeGlobalRole = async (assignment: GlobalRoleAssignment) => {
    if (!roleReason.trim() || roleConfirmation !== person.name) return;
    setPending(`revoke-${assignment.role}`);
    setError('');
    setMessage('');
    try {
      if (!demoMode) {
        await apiRequest<GlobalRoleAssignment>(
          `/admin/users/${encodeURIComponent(person.id)}/global-roles/${encodeURIComponent(assignment.role)}?revision=${assignment.revision}&reason=${encodeURIComponent(roleReason.trim())}`,
          { method: 'DELETE' },
        );
      }
      setAssignments((current) =>
        current.filter((item) => item.id !== assignment.id),
      );
      setRoleReason('');
      setRoleConfirmation('');
      setMessage(`${roleLabel(assignment.role)} revoked.`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-users'] }),
        queryClient.invalidateQueries({ queryKey: ['people'] }),
      ]);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to revoke the global role.',
      );
      if (!demoMode) void load();
    } finally {
      setPending('');
    }
  };
  const assignedRoles = new Set(
    assignments.map((assignment) => assignment.role),
  );
  const availableRoles = (['director', 'system_admin'] as const).filter(
    (role) => !assignedRoles.has(role),
  );

  return (
    <FormDialog
      title={`Administer ${person.name}`}
      description="System Admin actions require recent MFA and create immutable audit records."
      open={open}
      onOpenChange={onOpenChange}
      trigger={
        <>
          <IconShieldLock size={17} aria-hidden="true" /> Manage account
        </>
      }
    >
      <div className={styles.form}>
        {loading ? <p role="status">Loading private account details…</p> : null}
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        {message ? (
          <p className={styles.inlineNotice} role="status">
            {message}
          </p>
        ) : null}
        {!loading && !user && error ? (
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => void load()}
          >
            Retry
          </button>
        ) : null}
        {user ? (
          <>
            <section aria-labelledby={`account-state-${person.id}`}>
              <h3
                id={`account-state-${person.id}`}
                className={styles.cardTitle}
              >
                Account lifecycle
              </h3>
              <p className={styles.helper}>
                {user.email} ·{' '}
                <span
                  className={
                    user.accountState === 'active'
                      ? styles.badgeSuccess
                      : styles.badgeWarning
                  }
                >
                  {user.accountState.replace('_', ' ')}
                </span>{' '}
                · revision {user.revision}
              </p>
              <div className={styles.field}>
                <label htmlFor={`account-reason-${person.id}`}>
                  Audited reason
                </label>
                <textarea
                  id={`account-reason-${person.id}`}
                  className={styles.textarea}
                  maxLength={500}
                  value={accountReason}
                  onChange={(event) => setAccountReason(event.target.value)}
                />
              </div>
              {user.accountState !== 'suspended' ? (
                <div className={styles.field}>
                  <label htmlFor={`account-confirm-${person.id}`}>
                    Type <strong>{person.name}</strong> to confirm suspension
                  </label>
                  <input
                    id={`account-confirm-${person.id}`}
                    className={styles.input}
                    value={accountConfirmation}
                    onChange={(event) =>
                      setAccountConfirmation(event.target.value)
                    }
                  />
                </div>
              ) : null}
              <button
                className={
                  user.accountState === 'suspended'
                    ? styles.buttonSecondary
                    : styles.buttonDanger
                }
                type="button"
                disabled={
                  Boolean(pending) ||
                  !accountReason.trim() ||
                  (user.accountState !== 'suspended' &&
                    (accountConfirmation !== person.name ||
                      person.id === currentUserId))
                }
                onClick={() => void changeAccountState()}
              >
                {pending === 'account'
                  ? 'Applying…'
                  : user.accountState === 'suspended'
                    ? 'Reactivate account'
                    : 'Suspend account'}
              </button>
              {person.id === currentUserId &&
              user.accountState !== 'suspended' ? (
                <p className={styles.helper}>
                  You cannot suspend your own System Admin account.
                </p>
              ) : null}
            </section>
            <section aria-labelledby={`global-roles-${person.id}`}>
              <h3 id={`global-roles-${person.id}`} className={styles.cardTitle}>
                Global roles
              </h3>
              {assignments.length ? (
                <ul className={styles.cleanList}>
                  {assignments.map((assignment) => (
                    <li className={styles.listRow} key={assignment.id}>
                      <span>
                        <strong>{roleLabel(assignment.role)}</strong>
                        <span
                          className={`${styles.helper} ${styles.helperBlock}`}
                        >
                          {assignment.state.replace('_', ' ')} · revision{' '}
                          {assignment.revision}
                        </span>
                      </span>
                      <button
                        className={styles.buttonDanger}
                        type="button"
                        disabled={
                          Boolean(pending) ||
                          !roleReason.trim() ||
                          roleConfirmation !== person.name ||
                          (person.id === currentUserId &&
                            assignment.role === 'system_admin')
                        }
                        onClick={() => void revokeGlobalRole(assignment)}
                      >
                        {pending === `revoke-${assignment.role}`
                          ? 'Revoking…'
                          : 'Revoke'}
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className={styles.muted}>No global roles assigned.</p>
              )}
              {availableRoles.length ? (
                <>
                  <div className={styles.field}>
                    <label htmlFor={`grant-reason-${person.id}`}>
                      Reason for role grant
                    </label>
                    <textarea
                      id={`grant-reason-${person.id}`}
                      className={styles.textarea}
                      maxLength={500}
                      value={grantReason}
                      onChange={(event) => setGrantReason(event.target.value)}
                    />
                  </div>
                  <div className={styles.inline}>
                    <label htmlFor={`grant-role-${person.id}`}>
                      Grant role
                    </label>
                    <select
                      id={`grant-role-${person.id}`}
                      className={styles.select}
                      value={
                        availableRoles.includes(grantRole)
                          ? grantRole
                          : availableRoles[0]
                      }
                      onChange={(event) =>
                        setGrantRole(event.target.value as GlobalRole)
                      }
                    >
                      {availableRoles.map((role) => (
                        <option value={role} key={role}>
                          {roleLabel(role)}
                        </option>
                      ))}
                    </select>
                    <button
                      className={styles.button}
                      type="button"
                      disabled={Boolean(pending) || !grantReason.trim()}
                      onClick={() => void grantGlobalRole()}
                    >
                      {pending.startsWith('grant-')
                        ? 'Granting…'
                        : 'Grant role'}
                    </button>
                  </div>
                </>
              ) : null}
              {assignments.length ? (
                <>
                  <div className={styles.field}>
                    <label htmlFor={`role-reason-${person.id}`}>
                      Reason for role revocation
                    </label>
                    <textarea
                      id={`role-reason-${person.id}`}
                      className={styles.textarea}
                      maxLength={500}
                      value={roleReason}
                      onChange={(event) => setRoleReason(event.target.value)}
                    />
                  </div>
                  <div className={styles.field}>
                    <label htmlFor={`role-confirm-${person.id}`}>
                      Type <strong>{person.name}</strong> to confirm a
                      revocation
                    </label>
                    <input
                      id={`role-confirm-${person.id}`}
                      className={styles.input}
                      value={roleConfirmation}
                      onChange={(event) =>
                        setRoleConfirmation(event.target.value)
                      }
                    />
                  </div>
                </>
              ) : null}
            </section>
          </>
        ) : null}
      </div>
    </FormDialog>
  );
}

function roleLabel(role: GlobalRole) {
  return role === 'system_admin' ? 'System Admin' : 'Director';
}

function localDateTime(value: string) {
  const date = new Date(value);
  return formatDateTimeInput(date);
}

function weekDialogValues(week?: Week) {
  const now = new Date();
  return {
    number: week?.number ?? 1,
    startAt: localDateTime(week?.startAt ?? now.toISOString()),
    endAt: localDateTime(
      week?.endAt ?? new Date(now.getTime() + 6 * 86_400_000).toISOString(),
    ),
    resourceUrl: week?.resourceUrl ?? '',
  };
}

function upsertPageItem<T extends { id: string }>(
  page: { items: T[]; totalCount: number; pageInfo: PageInfo } | undefined,
  item: T,
) {
  if (!page)
    return {
      items: [item],
      totalCount: 1,
      pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
    };
  const exists = page.items.some((current) => current.id === item.id);
  return {
    ...page,
    items: exists
      ? page.items.map((current) => (current.id === item.id ? item : current))
      : [item, ...page.items],
    totalCount: exists ? page.totalCount : page.totalCount + 1,
  };
}

function WeekDialog({ seasonId, week }: { seasonId: string; week?: Week }) {
  const queryClient = useQueryClient();
  const initialValues = weekDialogValues(week);
  const [open, setOpen] = useState(false);
  const [number, setNumber] = useState(initialValues.number);
  const [startAt, setStartAt] = useState(initialValues.startAt);
  const [endAt, setEndAt] = useState(initialValues.endAt);
  const [resourceUrl, setResourceUrl] = useState(initialValues.resourceUrl);
  const [baseline, setBaseline] = useState(initialValues);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    number !== baseline.number ||
    startAt !== baseline.startAt ||
    endAt !== baseline.endAt ||
    resourceUrl !== baseline.resourceUrl;
  const requestOpenChange = (next: boolean) => {
    if (!next && dirty && !window.confirm('Discard unsaved week changes?'))
      return;
    if (next) {
      const values = weekDialogValues(week);
      setNumber(values.number);
      setStartAt(values.startAt);
      setEndAt(values.endAt);
      setResourceUrl(values.resourceUrl);
      setBaseline(values);
      setError('');
    }
    setOpen(next);
  };
  const save = async () => {
    if (
      !Number.isInteger(number) ||
      number < 1 ||
      new Date(endAt) <= new Date(startAt) ||
      (resourceUrl && !/^https:\/\//i.test(resourceUrl))
    )
      return setError(
        'Enter a positive week number, valid dates and an HTTPS resource URL.',
      );
    setPending(true);
    setError('');
    const mutation: WeekMutation = {
      number,
      startAt: zonedDateTimeToUtc(startAt),
      endAt: zonedDateTimeToUtc(endAt),
      ...(resourceUrl.trim() ? { resourceUrl: resourceUrl.trim() } : {}),
    };
    try {
      const saved = demoMode
        ? {
            id: week?.id ?? `week_demo_${Date.now()}`,
            seasonId,
            ...mutation,
            resourceUrl: mutation.resourceUrl ?? '',
            revision: (week?.revision ?? 0) + 1,
          }
        : await apiRequest<Week>(
            week
              ? `/seasons/${encodeURIComponent(seasonId)}/weeks/${encodeURIComponent(week.id)}`
              : `/seasons/${encodeURIComponent(seasonId)}/weeks`,
            {
              method: week ? 'PATCH' : 'POST',
              body: JSON.stringify(
                week
                  ? ({
                      ...mutation,
                      revision: week.revision,
                    } satisfies WeekUpdate)
                  : mutation,
              ),
            },
          );
      queryClient.setQueryData<WeekPage>(
        seasonWeeksQueryKey(seasonId),
        (current) => upsertPageItem(current, saved),
      );
      setOpen(false);
    } catch (requestError) {
      if (
        requestError instanceof ApiProblem &&
        requestError.problem.status === 409
      ) {
        await queryClient.invalidateQueries({
          queryKey: seasonWeeksQueryKey(seasonId),
        });
        setError(
          'This week changed elsewhere. The latest schedule has been fetched; reopen the form before trying again.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to save this week.',
        );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={week ? `Edit week ${week.number}` : 'Create week'}
      description="Weeks and resource links stay inside this open season."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        week ? (
          <>
            <IconEdit size={17} aria-hidden="true" /> Edit
          </>
        ) : (
          <>
            <IconPlus size={17} aria-hidden="true" /> Create week
          </>
        )
      }
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.fieldGrid}>
          <div className={styles.field}>
            <label htmlFor={`week-number-${week?.id ?? 'new'}`}>
              Week number
            </label>
            <input
              id={`week-number-${week?.id ?? 'new'}`}
              className={styles.input}
              type="number"
              min="1"
              value={number}
              onChange={(event) => setNumber(event.target.valueAsNumber)}
            />
          </div>
          <AppDatePicker
            label="Starts"
            granularity="minute"
            value={startAt}
            onChange={setStartAt}
          />
          <AppDatePicker
            label="Ends"
            granularity="minute"
            value={endAt}
            onChange={setEndAt}
          />
          <div className={styles.field}>
            <label htmlFor={`week-resource-${week?.id ?? 'new'}`}>
              Resource URL (optional)
            </label>
            <input
              id={`week-resource-${week?.id ?? 'new'}`}
              className={styles.input}
              type="url"
              value={resourceUrl}
              onChange={(event) => setResourceUrl(event.target.value)}
            />
          </div>
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="button"
            disabled={pending}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : week ? 'Save week' : 'Create week'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function DeleteWeekButton({
  seasonId,
  week,
}: {
  seasonId: string;
  week: Week;
}) {
  const queryClient = useQueryClient();
  const remove = async () => {
    if (!demoMode)
      await apiRequest<void>(
        `/seasons/${encodeURIComponent(seasonId)}/weeks/${encodeURIComponent(week.id)}?revision=${week.revision}`,
        { method: 'DELETE' },
      );
    queryClient.setQueryData<WeekPage>(
      seasonWeeksQueryKey(seasonId),
      (current) =>
        current
          ? {
              ...current,
              items: current.items.filter((item) => item.id !== week.id),
              totalCount: Math.max(0, current.totalCount - 1),
            }
          : current,
    );
  };
  return (
    <NamedConfirmation
      name={`Week ${week.number}`}
      actionLabel="Delete week"
      description="The week is permanently removed. Historical records may prevent deletion."
      onConfirm={remove}
    />
  );
}

type EnrollmentAccountOption =
  Pick<Person, 'id' | 'name' | 'slug'> | EnrollmentCandidate;

export function EnrollmentDialog({
  seasonId,
  accounts,
  enrollment,
  allowCoordinator = false,
  accountsLoading = false,
  accountsError = false,
  onRetryAccounts,
}: {
  seasonId: string;
  accounts: EnrollmentAccountOption[];
  enrollment?: Enrollment;
  allowCoordinator?: boolean;
  accountsLoading?: boolean;
  accountsError?: boolean;
  onRetryAccounts?: () => void;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [userId, setUserId] = useState(enrollment?.userId ?? '');
  const [role, setRole] = useState<SeasonRole>(enrollment?.role ?? 'student');
  const [studentLevel, setStudentLevel] = useState<
    NonNullable<EnrollmentUpdate['studentLevel']>
  >(enrollment?.studentLevel ?? 'not_applicable');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    userId !== (enrollment?.userId ?? '') ||
    role !== (enrollment?.role ?? 'student') ||
    studentLevel !== (enrollment?.studentLevel ?? 'not_applicable');
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved enrollment changes?')
    )
      return;
    if (next) {
      setUserId(enrollment?.userId ?? '');
      setRole(enrollment?.role ?? 'student');
      setStudentLevel(enrollment?.studentLevel ?? 'not_applicable');
      setError('');
    }
    setOpen(next);
  };
  const save = async () => {
    if (!userId) return setError('Choose a verified account.');
    setPending(true);
    setError('');
    try {
      let saved: Enrollment;
      if (demoMode)
        saved = {
          id: enrollment?.id ?? `enrol_demo_${Date.now()}`,
          seasonId,
          userId,
          role,
          studentLevel: role === 'student' ? studentLevel : 'not_applicable',
          state: enrollment?.state ?? 'active',
          assignmentState: enrollment?.assignmentState ?? 'active',
          removalReason: null,
          revision: (enrollment?.revision ?? 0) + 1,
        };
      else if (enrollment) {
        const mutation: EnrollmentUpdate = {
          role: role === 'coordinator' ? 'mentor' : role,
          ...(role === 'student' ? { studentLevel } : {}),
          revision: enrollment.revision,
        };
        saved = await apiRequest<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(enrollment.id)}`,
          { method: 'PATCH', body: JSON.stringify(mutation) },
        );
      } else {
        const mutation: EnrollmentCreate = { userId, role };
        saved = await apiRequest<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members`,
          { method: 'POST', body: JSON.stringify(mutation) },
        );
      }
      queryClient.setQueryData<EnrollmentPage>(
        seasonEnrollmentsQueryKey(seasonId),
        (current) => upsertPageItem(current, saved),
      );
      if (!enrollment) {
        const selectedAccount = accounts.find(
          (account) => account.id === saved.userId,
        );
        queryClient.setQueryData<EnrollmentCandidatePage>(
          enrollmentCandidatesQueryKey(seasonId),
          (current) =>
            current
              ? {
                  ...current,
                  items: current.items.filter(
                    (candidate) => candidate.id !== saved.userId,
                  ),
                  totalCount: Math.max(0, current.totalCount - 1),
                }
              : current,
        );
        if (demoMode && selectedAccount) {
          const newMember: Person = {
            id: selectedAccount.id,
            name: selectedAccount.name,
            slug: selectedAccount.slug,
            initials: selectedAccount.name
              .split(/\s+/)
              .map((part) => part[0])
              .join('')
              .slice(0, 2)
              .toUpperCase(),
            avatarUrl:
              'avatarUrl' in selectedAccount
                ? (selectedAccount.avatarUrl ?? null)
                : null,
            roles: [role],
            season: null,
            status: 'active',
            attempts: 0,
            interviews: 0,
            lastActiveAt: null,
            revision:
              'revision' in selectedAccount ? selectedAccount.revision : 1,
          };
          queryClient.setQueryData<ViewPage<Person>>(['people'], (current) =>
            upsertPageItem(current, newMember),
          );
        } else {
          await Promise.all([
            queryClient.invalidateQueries({
              queryKey: enrollmentCandidatesQueryKey(seasonId),
            }),
            queryClient.invalidateQueries({ queryKey: ['people'] }),
          ]);
        }
      }
      setOpen(false);
    } catch (requestError) {
      if (
        requestError instanceof ApiProblem &&
        requestError.problem.status === 409
      ) {
        await queryClient.invalidateQueries({
          queryKey: seasonEnrollmentsQueryKey(seasonId),
        });
        setError(
          'This enrollment changed elsewhere. The latest roster has been fetched; reopen the form before trying again.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to save this enrollment.',
        );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={enrollment ? 'Edit enrollment' : 'Add season member'}
      description="Assign exactly one season role to an existing account."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        enrollment ? (
          <>
            <IconEdit size={17} aria-hidden="true" /> Edit
          </>
        ) : (
          <>
            <IconPlus size={17} aria-hidden="true" /> Add member
          </>
        )
      }
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        {accountsError && !enrollment ? (
          <div>
            <p className={styles.fieldError} role="alert">
              Verified enrollment candidates could not be loaded.
            </p>
            {onRetryAccounts ? (
              <button
                className={styles.buttonSecondary}
                type="button"
                onClick={onRetryAccounts}
              >
                Retry accounts
              </button>
            ) : null}
          </div>
        ) : null}
        <div className={styles.field}>
          <label htmlFor={`enrollment-user-${enrollment?.id ?? 'new'}`}>
            Account
          </label>
          <select
            id={`enrollment-user-${enrollment?.id ?? 'new'}`}
            className={styles.select}
            disabled={Boolean(enrollment) || accountsLoading || accountsError}
            value={userId}
            onChange={(event) => setUserId(event.target.value)}
          >
            <option value="">
              {accountsLoading
                ? 'Loading verified accounts…'
                : accounts.length
                  ? 'Choose an account'
                  : 'No eligible verified accounts'}
            </option>
            {accounts.map((person) => (
              <option value={person.id} key={person.id}>
                {person.name} · @{person.slug}
              </option>
            ))}
          </select>
        </div>
        <div className={styles.field}>
          <label htmlFor={`enrollment-role-${enrollment?.id ?? 'new'}`}>
            Season role
          </label>
          <select
            id={`enrollment-role-${enrollment?.id ?? 'new'}`}
            className={styles.select}
            value={role}
            onChange={(event) => setRole(event.target.value as SeasonRole)}
          >
            {!enrollment && allowCoordinator ? (
              <option value="coordinator">Coordinator</option>
            ) : null}
            <option value="mentor">Mentor</option>
            <option value="student">Student</option>
          </select>
        </div>
        {enrollment && role === 'student' ? (
          <div className={styles.field}>
            <label htmlFor={`enrollment-level-${enrollment.id}`}>
              Student level
            </label>
            <select
              id={`enrollment-level-${enrollment.id}`}
              className={styles.select}
              value={studentLevel}
              onChange={(event) =>
                setStudentLevel(event.target.value as typeof studentLevel)
              }
            >
              <option value="novice">Novice</option>
              <option value="beginner">Beginner</option>
              <option value="intermediate">Intermediate</option>
              <option value="advanced">Advanced</option>
              <option value="not_applicable">Not applicable</option>
            </select>
          </div>
        ) : null}
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="button"
            disabled={pending || !userId}
            onClick={() => void save()}
          >
            {pending
              ? 'Saving…'
              : enrollment
                ? 'Save enrollment'
                : 'Add member'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function RemoveEnrollmentDialog({
  seasonId,
  enrollment,
  name,
}: {
  seasonId: string;
  enrollment: Enrollment;
  name: string;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const remove = async () => {
    setPending(true);
    setError('');
    try {
      const removed = demoMode
        ? {
            ...enrollment,
            state: 'kicked' as const,
            removalReason: reason,
            revision: enrollment.revision + 1,
          }
        : await apiRequest<Enrollment>(
            `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(enrollment.id)}/remove`,
            {
              method: 'POST',
              body: JSON.stringify({
                reason: reason.trim(),
                revision: enrollment.revision,
              }),
            },
          );
      queryClient.setQueryData<EnrollmentPage>(
        seasonEnrollmentsQueryKey(seasonId),
        (current) => upsertPageItem(current, removed),
      );
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to remove this member.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`Remove ${name}?`}
      description="Season access ends immediately and an immutable audit event records the reason."
      open={open}
      onOpenChange={setOpen}
      trigger={
        <>
          <IconUserMinus size={17} aria-hidden="true" /> Remove
        </>
      }
    >
      <div className={styles.form}>
        <div className={styles.field}>
          <label htmlFor={`admin-remove-reason-${enrollment.id}`}>Reason</label>
          <textarea
            id={`admin-remove-reason-${enrollment.id}`}
            className={styles.textarea}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor={`admin-remove-confirm-${enrollment.id}`}>
            Type <strong>{name}</strong> to confirm
          </label>
          <input
            id={`admin-remove-confirm-${enrollment.id}`}
            className={styles.input}
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonDanger}
            type="button"
            disabled={
              pending || reason.trim().length < 5 || confirmation !== name
            }
            onClick={() => void remove()}
          >
            {pending ? 'Removing…' : 'Remove member'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function MentorshipDialog({
  seasonId,
  people,
  mentorship,
}: {
  seasonId: string;
  people: Person[];
  mentorship?: Mentorship;
}) {
  const queryClient = useQueryClient();
  const mentors = people.filter((person) => person.roles.includes('mentor'));
  const students = people.filter((person) => person.roles.includes('student'));
  const [open, setOpen] = useState(false);
  const [mentorUserId, setMentorUserId] = useState(
    mentorship?.mentorUserId ?? '',
  );
  const [studentUserId, setStudentUserId] = useState(
    mentorship?.studentUserId ?? '',
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    mentorUserId !== (mentorship?.mentorUserId ?? '') ||
    studentUserId !== (mentorship?.studentUserId ?? '');
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved mentorship changes?')
    )
      return;
    if (next) {
      setMentorUserId(mentorship?.mentorUserId ?? '');
      setStudentUserId(mentorship?.studentUserId ?? '');
      setError('');
    }
    setOpen(next);
  };
  const save = async () => {
    if (!mentorUserId || !studentUserId)
      return setError('Choose a mentor and student.');
    setPending(true);
    setError('');
    try {
      const saved = demoMode
        ? {
            id: mentorship?.id ?? `mentorship_demo_${Date.now()}`,
            seasonId,
            mentorUserId,
            studentUserId,
            revision: (mentorship?.revision ?? 0) + 1,
          }
        : await apiRequest<Mentorship>(
            mentorship
              ? `/seasons/${encodeURIComponent(seasonId)}/mentorships/${encodeURIComponent(mentorship.id)}`
              : `/seasons/${encodeURIComponent(seasonId)}/mentorships`,
            {
              method: mentorship ? 'PATCH' : 'POST',
              body: JSON.stringify(
                mentorship
                  ? ({
                      mentorUserId,
                      studentUserId,
                      revision: mentorship.revision,
                    } satisfies MentorshipUpdate)
                  : ({
                      mentorUserId,
                      studentUserId,
                    } satisfies MentorshipCreate),
              ),
            },
          );
      queryClient.setQueryData<MentorshipPage>(
        seasonMentorshipsQueryKey(seasonId),
        (current) => upsertPageItem(current, saved),
      );
      setOpen(false);
    } catch (requestError) {
      if (
        requestError instanceof ApiProblem &&
        requestError.problem.status === 409
      ) {
        await queryClient.invalidateQueries({
          queryKey: seasonMentorshipsQueryKey(seasonId),
        });
        setError(
          'This mentorship changed elsewhere. The latest assignments have been fetched; reopen the form before trying again.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to save this mentorship.',
        );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={mentorship ? 'Edit mentorship' : 'Create mentorship'}
      description="Both people must have active roles in the selected season."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        mentorship ? (
          <>
            <IconEdit size={17} aria-hidden="true" /> Edit
          </>
        ) : (
          <>
            <IconPlus size={17} aria-hidden="true" /> Create mentorship
          </>
        )
      }
    >
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor={`admin-mentor-${mentorship?.id ?? 'new'}`}>
            Mentor
          </label>
          <select
            id={`admin-mentor-${mentorship?.id ?? 'new'}`}
            className={styles.select}
            value={mentorUserId}
            onChange={(event) => setMentorUserId(event.target.value)}
          >
            <option value="">Choose a mentor</option>
            {mentors.map((person) => (
              <option value={person.id} key={person.id}>
                {person.name}
              </option>
            ))}
          </select>
        </div>
        <div className={styles.field}>
          <label htmlFor={`admin-student-${mentorship?.id ?? 'new'}`}>
            Student
          </label>
          <select
            id={`admin-student-${mentorship?.id ?? 'new'}`}
            className={styles.select}
            value={studentUserId}
            onChange={(event) => setStudentUserId(event.target.value)}
          >
            <option value="">Choose a student</option>
            {students.map((person) => (
              <option value={person.id} key={person.id}>
                {person.name}
              </option>
            ))}
          </select>
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.button}
            type="button"
            disabled={pending || !mentorUserId || !studentUserId}
            onClick={() => void save()}
          >
            {pending
              ? 'Saving…'
              : mentorship
                ? 'Save mentorship'
                : 'Create mentorship'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}

function DeleteMentorshipButton({
  seasonId,
  mentorship,
}: {
  seasonId: string;
  mentorship: Mentorship;
}) {
  const queryClient = useQueryClient();
  const remove = async () => {
    if (!demoMode)
      await apiRequest<void>(
        `/seasons/${encodeURIComponent(seasonId)}/mentorships/${encodeURIComponent(mentorship.id)}?revision=${mentorship.revision}`,
        { method: 'DELETE' },
      );
    queryClient.setQueryData<MentorshipPage>(
      seasonMentorshipsQueryKey(seasonId),
      (current) =>
        current
          ? {
              ...current,
              items: current.items.filter((item) => item.id !== mentorship.id),
              totalCount: Math.max(0, current.totalCount - 1),
            }
          : current,
    );
  };
  return (
    <NamedConfirmation
      name="END MENTORSHIP"
      actionLabel="End mentorship"
      description="This ends the active mentor relationship. Historical programme records remain."
      onConfirm={remove}
    />
  );
}

export const AdminSeasonsPage = () => <AdminResourcePage resource="seasons" />;
export const AdminWeeksPage = () => <AdminResourcePage resource="weeks" />;
export const AdminUsersPage = () => <AdminResourcePage resource="users" />;
export const AdminEnrollmentsPage = () => (
  <AdminResourcePage resource="enrollments" />
);
export const AdminMentorshipsPage = () => (
  <AdminResourcePage resource="mentorships" />
);
