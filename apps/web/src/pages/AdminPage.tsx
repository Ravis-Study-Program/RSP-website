import { statusTone } from '@/badgeTones';
import { StatusBadge } from '@/components/StatusBadge';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Input } from '@/components/ui/input';
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import {
  IconArrowRight,
  IconCalendarEvent,
  IconChecklist,
  IconClock,
  IconShieldLock,
  IconUsers,
  IconDotsVertical,
  IconEdit,
  IconNotes,
  IconPlus,
  IconSchool,
  IconTrash,
  IconUserMinus,
} from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';

import { ApiError, apiRequest } from '@/api/client';
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
  useCurrentUser,
  useEnrollmentCandidates,
  usePeople,
  useSeasonEnrollments,
  useSeasonMentorships,
  useSeasonWeeks,
  useSeasons,
} from '@/api/queries';
import {
  Avatar,
  MetricCard,
  PageHeader,
  PageSection,
  usePageTitle,
} from '@/components/Common';
import { AppDatePicker } from '@/components/AppDatePicker';
import { ManagePersonDialog } from '@/components/ManagePersonDialog';
import { AdminProfileEditor } from '@/components/AdminProfileEditor';
import { Invitations, type InvitationParts } from '@/components/Invitations';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import type { Page as ViewPage, Person, Season, SeasonRole } from '@/types';
import {
  formatDate,
  formatDateTimeInput,
  initials,
  zonedDateTimeToUtc,
} from '@/utils';
import { defaultSeason } from '@/navigation';
import { statusSortingFn } from '@/sortOrder';

function statusLabel(status: string) {
  const text = status.replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
}

const adminResources = [
  {
    slug: 'seasons',
    title: 'Seasons',
    description: 'Definitions, dates and lifecycle',
    icon: IconCalendarEvent,
    // Managed on the Seasons page, which carries the lifecycle actions.
    to: '/seasons',
  },
  {
    slug: 'weeks',
    title: 'Weeks',
    description: 'Schedule and week resources',
    icon: IconClock,
  },
  {
    slug: 'users',
    title: 'People',
    description: 'Profiles and account states',
    icon: IconUsers,
    // Accounts are managed from People's row menu.
    to: '/people',
  },
  {
    slug: 'enrollments',
    title: 'Enrollments',
    description: 'Season roles and membership',
    icon: IconChecklist,
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
  const visibleResources = adminResources;
  // Mentor teams belong to a season, so the link opens the default one.
  const mentorTeamsSeason = defaultSeason(
    currentUser.data,
    seasons.data?.items,
  );
  const pageFor = (slug: string) => {
    const resource = visibleResources.find((item) => item.slug === slug);
    return resource
      ? [
          {
            title: resource.title,
            icon: resource.icon,
            to: 'to' in resource ? resource.to : `/admin/${resource.slug}`,
          },
        ]
      : [];
  };
  // People and their season roles first, then the program's structure.
  const adminPages = [
    ...pageFor('users'),
    ...pageFor('enrollments'),
    ...(mentorTeamsSeason
      ? [
          {
            title: 'Mentor Teams',
            icon: IconSchool,
            to: `/seasons/${mentorTeamsSeason.slug}/mentor-teams`,
          },
        ]
      : []),
    ...pageFor('seasons'),
    ...pageFor('weeks'),
  ];
  return (
    <div>
      <PageHeader title="Administration" />
      <div className="flex flex-col gap-6">
        <div className="grid gap-6 sm:grid-cols-2">
          <MetricCard label="Seasons" value={seasons.data?.totalCount ?? '—'} />
          <MetricCard
            label="Visible users"
            value={people.data?.totalCount ?? '—'}
          />
        </div>
        <PageSection titleId="admin-pages-title" title="Admin Pages">
          <Card>
            <CardContent>
              <ul className="m-0 list-none divide-y p-0">
                {adminPages.map((page) => {
                  const Icon = page.icon;
                  return (
                    <li key={page.to}>
                      <Link className={adminLinkClass} to={page.to}>
                        <span className="flex flex-wrap items-center gap-2.5">
                          <Icon size={18} aria-hidden="true" /> {page.title}
                        </span>
                        <IconArrowRight size={17} aria-hidden="true" />
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </CardContent>
          </Card>
        </PageSection>
        <section className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-lg font-semibold">Technical Operations</h2>
          </div>
          <Card className="gap-4 p-5">
            {syncStatus ? <p role="status">{syncStatus}</p> : null}
            <div className="flex flex-wrap items-center justify-between gap-3">
              <span>
                <strong>LeetCode catalogue sync</strong>
                <span className="block text-sm text-muted-foreground">
                  Scheduled for Sundays at 03:00 UTC.
                </span>
              </span>
              <Button
                variant="outline"
                size="sm"
                type="button"
                disabled={syncPending}
                onClick={() => void sync()}
              >
                {syncPending ? 'Queueing…' : 'Sync now'}
              </Button>
            </div>
          </Card>
        </section>
      </div>
    </div>
  );
}

interface AdminPerson {
  name: string;
  slug: string;
  avatarUrl?: string | null;
}

/** A person as tables show them: picture and name, linking to the profile. */
function PersonCell({ person }: { person: AdminPerson }) {
  return (
    <span className="flex min-w-0 items-center gap-3">
      <Avatar initials={initials(person.name)} avatarUrl={person.avatarUrl} />
      {person.slug ? (
        <Link
          className="truncate font-medium text-foreground"
          to={`/people/${person.slug}`}
        >
          {person.name}
        </Link>
      ) : (
        <span className="truncate font-medium">{person.name}</span>
      )}
    </span>
  );
}

interface AdminRow {
  id: string;
  primary: string;
  secondary: string;
  status: string;
  detail: string;
  /** Weeks only: whether the week's resources page has content. */
  hasResources?: boolean;
  /** Where clicking the row goes. */
  href?: string;
  /** The person a row is about (enrollments, users, mentorship students). */
  member?: AdminPerson;
  /** Mentorships only: the student's mentor. */
  mentor?: AdminPerson;
  /** Enrollments: how the member was invited, when, and to which email. */
  invitationStatus?: 'pending' | 'expired' | 'accepted' | 'cancelled';
  sentAt?: string;
  email?: string;
  actions?: ReactNode;
}

const adminLinkClass =
  'flex min-h-11 items-center justify-between gap-4 py-3 font-medium text-foreground hover:underline';

const columns: ColumnDef<AdminRow, any>[] = [
  {
    accessorKey: 'primary',
    header: 'Name',
    cell: ({ row }) => (
      <div>
        <strong>{row.original.primary}</strong>
        {row.original.secondary ? (
          <div className="text-sm text-muted-foreground">
            {row.original.secondary}
          </div>
        ) : null}
      </div>
    ),
  },
  {
    accessorKey: 'status',
    header: 'Status',
    meta: { width: 9 },
    sortingFn: statusSortingFn,
    cell: ({ getValue }) => (
      <StatusBadge tone={statusTone(getValue())}>
        {statusLabel(getValue())}
      </StatusBadge>
    ),
  },
  { accessorKey: 'detail', header: 'Details' },
  {
    id: 'actions',
    header: 'Actions',
    meta: { width: 6, align: 'end' },
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => row.original.actions ?? '—',
  },
];

/** The enrollments table, laid out like the people tables. */
function invitationLabel(status: NonNullable<AdminRow['invitationStatus']>) {
  return status === 'pending'
    ? 'Invited'
    : status.charAt(0).toUpperCase() + status.slice(1);
}

/**
 * The enrollments table: each member or invitee, the student's mentor, and
 * how they were invited. Roles, levels and states live on People.
 */
const enrollmentColumns: ColumnDef<AdminRow, any>[] = [
  {
    accessorKey: 'primary',
    header: 'Name',
    meta: { width: 12 },
    cell: ({ row }) =>
      row.original.member ? (
        <PersonCell person={row.original.member} />
      ) : (
        row.original.primary
      ),
  },
  {
    id: 'mentor',
    header: 'Mentor',
    meta: { width: 12 },
    accessorFn: (row) => row.mentor?.name ?? '',
    cell: ({ row }) =>
      row.original.mentor ? <PersonCell person={row.original.mentor} /> : '—',
  },
  {
    id: 'invitation',
    header: 'Invitation',
    meta: { width: 8 },
    accessorFn: (row) =>
      row.invitationStatus ? invitationLabel(row.invitationStatus) : '',
    cell: ({ row }) =>
      row.original.invitationStatus ? (
        <StatusBadge
          tone={
            row.original.invitationStatus === 'accepted'
              ? 'success'
              : row.original.invitationStatus === 'pending'
                ? 'info'
                : 'muted'
          }
        >
          {invitationLabel(row.original.invitationStatus)}
        </StatusBadge>
      ) : (
        '—'
      ),
  },
  {
    accessorKey: 'sentAt',
    header: 'Sent',
    meta: { width: 8 },
    cell: ({ row }) =>
      row.original.sentAt ? formatDate(row.original.sentAt) : '—',
  },
  {
    accessorKey: 'email',
    header: 'Email',
    meta: { width: 14 },
    cell: ({ row }) => row.original.email ?? '—',
  },
  columns[columns.length - 1],
];

const nameColumn = columns[0];
const statusColumn = columns[1];
const datesColumn: ColumnDef<AdminRow, any> = {
  accessorKey: 'detail',
  header: 'Dates',
};
const actionsColumn = columns[columns.length - 1];

/** The weeks table: its own Resources column, and actions in a row menu. */
const weekColumns: ColumnDef<AdminRow, any>[] = [
  nameColumn,
  statusColumn,
  {
    id: 'hasResources',
    header: 'Has Resources',
    meta: { width: 11 },
    accessorFn: (row) => (row.hasResources ? 'Yes' : 'No'),
    cell: ({ getValue }) => (
      <StatusBadge tone={getValue() === 'Yes' ? 'success' : 'muted'}>
        {getValue()}
      </StatusBadge>
    ),
  },
  datesColumn,
  actionsColumn,
];

export function AdminResourcePage({
  resource,
}: {
  resource: 'weeks' | 'enrollments';
}) {
  const seasons = useSeasons();
  const people = usePeople();
  const currentUser = useCurrentUser();
  const canGrantCoordinator = Boolean(
    currentUser.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const canAdministerUsers = Boolean(
    currentUser.data?.globalRoles.includes('system_admin'),
  );
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
  const seasonItems = manageableSeasons.length
    ? manageableSeasons.map((season) => ({
        label: `${season.name} · ${statusLabel(season.status)}`,
        value: season.slug,
      }))
    : [{ label: 'No manageable season', value: '' }];
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
  const rows: AdminRow[] =
    resource === 'weeks'
      ? (weeks.data?.items ?? []).map((week) => ({
          id: week.id,
          primary: `Week ${week.number}`,
          secondary: '',
          href: selectedSeason
            ? `/seasons/${selectedSeason.slug}/weeks/${week.number}`
            : undefined,
          hasResources: Boolean(week.resourcesHtml),
          status:
            new Date(week.endAt).getTime() < Date.now()
              ? 'completed'
              : new Date(week.startAt).getTime() > Date.now()
                ? 'scheduled'
                : 'active',
          detail: `${formatDate(week.startAt)} – ${formatDate(week.endAt)}`,
          actions: selectedSeason ? (
            <WeekRowMenu season={selectedSeason} week={week} />
          ) : undefined,
        }))
      : [];
  const selectedQuery = resource === 'weeks' ? weeks : enrollments;
  const loading =
    seasons.isLoading || people.isLoading || Boolean(selectedQuery?.isLoading);
  const error =
    seasons.isError || people.isError || Boolean(selectedQuery?.isError);
  const createAction =
    selectedSeason && resource === 'weeks' ? (
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
    ) : undefined;
  const mentorshipAction =
    selectedSeason && resource === 'enrollments' ? (
      <MentorshipDialog
        seasonId={selectedSeason.id}
        people={people.data?.items ?? []}
      />
    ) : undefined;
  const mentorPeople = (people.data?.items ?? []).filter((person) =>
    (enrollments.data?.items ?? []).some(
      (enrollment) =>
        enrollment.userId === person.id &&
        enrollment.state === 'active' &&
        (enrollment.role === 'mentor' || enrollment.role === 'coordinator'),
    ),
  );
  const personFor = (userId: string) => {
    const person = peopleById.get(userId);
    return person
      ? { name: person.name, slug: person.slug, avatarUrl: person.avatarUrl }
      : undefined;
  };
  // The enrollments table: current members and the invitations not yet
  // accepted, with each student's mentor and how each member was invited.
  const enrollmentRows = (parts?: InvitationParts): AdminRow[] => {
    const invitations = parts?.invitations ?? [];
    const members = (enrollments.data?.items ?? [])
      .filter((enrollment) => enrollment.state === 'active')
      .map((enrollment): AdminRow => {
        const name =
          enrollment.member?.name ??
          peopleById.get(enrollment.userId)?.name ??
          enrollment.userId;
        const invitation = invitations.find(
          (item) => item.acceptedBy === enrollment.userId,
        );
        const mentorship = (mentorships.data?.items ?? []).find(
          (item) => item.studentUserId === enrollment.userId,
        );
        const slug =
          enrollment.member?.slug ?? peopleById.get(enrollment.userId)?.slug;
        return {
          id: enrollment.id,
          primary: name,
          secondary: '',
          detail: '',
          status: enrollment.state,
          href: slug ? `/people/${slug}` : undefined,
          member: {
            name,
            slug: slug ?? '',
            avatarUrl:
              enrollment.member?.avatarUrl ??
              peopleById.get(enrollment.userId)?.avatarUrl,
          },
          mentor: mentorship ? personFor(mentorship.mentorUserId) : undefined,
          invitationStatus: invitation ? 'accepted' : undefined,
          sentAt: invitation?.sentAt ?? undefined,
          email: invitation?.email,
          actions:
            selectedSeason?.status === 'open' ? (
              <EnrollmentRowMenu
                season={selectedSeason}
                seasons={manageableSeasons}
                enrollment={enrollment}
                accounts={people.data?.items ?? []}
                person={peopleById.get(enrollment.userId)}
                currentUserId={currentUser.data?.id}
                canManageAccount={canAdministerUsers}
                mentorship={mentorship}
                name={name}
              />
            ) : undefined,
        };
      });
    const invited = invitations
      .filter((item) => item.status === 'pending' || item.status === 'expired')
      .map((invitation): AdminRow => ({
        id: `invitation:${invitation.id}`,
        primary: invitation.name,
        secondary: '',
        detail: '',
        status: invitation.status,
        member: { name: invitation.name, slug: '' },
        mentor: invitation.mentorUserId
          ? personFor(invitation.mentorUserId)
          : undefined,
        invitationStatus: invitation.status,
        sentAt: invitation.sentAt ?? undefined,
        email: invitation.email,
        actions: parts?.menuFor(invitation),
      }));
    return [...invited, ...members];
  };

  const page = (parts?: InvitationParts) => (
    <div>
      <PageHeader
        title={config.title}
        titleHint="Admin"
        actions={
          createAction || parts ? (
            <div className="flex flex-wrap items-center gap-2.5">
              {createAction}
              {mentorshipAction}
              {parts?.invite}
            </div>
          ) : undefined
        }
      />
      <div className="flex flex-col gap-6">
        {parts?.status ? (
          <p role="status" className="text-sm text-muted-foreground">
            {parts.status}
          </p>
        ) : null}
        <DataTable
          ariaLabel={`Admin ${config.title}`}
          data={resource === 'enrollments' ? enrollmentRows(parts) : rows}
          columns={resource === 'weeks' ? weekColumns : enrollmentColumns}
          rowHref={(row) => row.href}
          // The season sits top-left of the table, as the week does on
          // Monday Updates.
          toolbarLead={
            <Select
              items={seasonItems}
              value={selectedSeason?.slug ?? ''}
              disabled={!manageableSeasons.length}
              onValueChange={(value) => {
                const season = manageableSeasons.find(
                  (item) => item.slug === value,
                );
                setSelectedSeasonId(season?.id ?? '');
              }}
            >
              <SelectTrigger
                id={`admin-${resource}-season`}
                aria-label="Season"
                className="w-64"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {seasonItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          }
          loading={loading || Boolean(parts?.loading)}
          error={error || Boolean(parts?.error)}
          onRetry={() =>
            void Promise.all([
              seasons.refetch(),
              people.refetch(),
              selectedQuery?.refetch(),
              resource === 'enrollments'
                ? enrollmentCandidates.refetch()
                : undefined,
              mentorships.refetch(),
              parts?.refetch(),
            ])
          }
          emptyTitle={`No ${config.title.toLowerCase()}`}
          emptyMessage="Create the first record when the program is ready."
          getRowId={(row) => row.id}
          renderCard={(row) => (
            <div className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge tone={statusTone(row.original.status)}>
                  {statusLabel(row.original.status)}
                </StatusBadge>
              </div>
              <h2 className="text-base font-semibold">
                {row.original.primary}
              </h2>
              {row.original.secondary ? (
                <p className="text-sm text-muted-foreground">
                  {row.original.secondary}
                </p>
              ) : null}
              {row.original.detail ? <p>{row.original.detail}</p> : null}
              {row.original.email ? (
                <p className="text-sm text-muted-foreground">
                  {row.original.email}
                </p>
              ) : null}
              {row.original.mentor ? (
                <p className="text-sm text-muted-foreground">
                  Mentor: {row.original.mentor.name}
                </p>
              ) : null}
              {row.original.invitationStatus ? (
                <p className="text-sm text-muted-foreground">
                  {invitationLabel(row.original.invitationStatus)}
                  {row.original.sentAt
                    ? ` · sent ${formatDate(row.original.sentAt)}`
                    : ''}
                </p>
              ) : null}
              {row.original.hasResources !== undefined ? (
                <p className="text-sm text-muted-foreground">
                  Has resources: {row.original.hasResources ? 'Yes' : 'No'}
                </p>
              ) : null}
              {row.original.actions ? (
                <div className="flex flex-wrap justify-end gap-2">
                  {row.original.actions}
                </div>
              ) : null}
            </div>
          )}
        />
      </div>
    </div>
  );
  // Enrollments carry the season's invitations: the invite button joins the
  // header and the invitations table sits above the members.
  return resource === 'enrollments' && selectedSeason ? (
    <Invitations
      key={selectedSeason.id}
      season={selectedSeason}
      mentors={mentorPeople}
      render={page}
    />
  ) : (
    page()
  );
}

export function UserAdministrationDialog({
  person,
  currentUserId,
  open: controlledOpen,
  onOpenChange: onControlledOpenChange,
  before,
}: {
  person: Person;
  currentUserId?: string;
  /** Shown above the account controls, such as an enrollment correction. */
  before?: ReactNode;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onControlledOpenChange?.(next);
  };
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
          email: person.email ?? `${person.slug}@example.test`,
          accountState: 'active',
        });
        setAssignments(
          globalRoles.map((role) => ({
            id: `role_${person.id}_${role}`,
            userId: person.id,
            role,
            state: 'active',
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
  // A row menu mounts this dialog already open, so load straight away.
  useEffect(() => {
    if (controlledOpen) void load();
    // Only on mount; later opens load through onOpenChange.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
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
        ? { ...user, accountState: state }
        : await apiRequest<UserPrivate>(
            `/admin/users/${encodeURIComponent(person.id)}/account-state`,
            {
              method: 'POST',
              body: JSON.stringify({
                state,
                reason: accountReason.trim(),
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
          `/admin/users/${encodeURIComponent(person.id)}/global-roles/${encodeURIComponent(assignment.role)}?reason=${encodeURIComponent(roleReason.trim())}`,
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
  const grantRoleItems = availableRoles.map((role) => ({
    label: roleLabel(role),
    value: role,
  }));

  return (
    <FormDialog
      title={`Administer ${person.name}`}
      open={open}
      onOpenChange={onOpenChange}
      contentClassName="sm:max-w-3xl"
      triggerVariant="outline"
      triggerSize="sm"
      trigger={
        controlled ? undefined : (
          <>
            <IconShieldLock size={17} aria-hidden="true" /> Manage account
          </>
        )
      }
    >
      <div className="flex flex-col gap-4">
        {before}
        {loading ? <p role="status">Loading private account details…</p> : null}
        {error ? <FieldError>{error}</FieldError> : null}
        {message ? (
          <p role="status" className="text-sm text-muted-foreground">
            {message}
          </p>
        ) : null}
        {!loading && !user && error ? (
          <Button variant="outline" type="button" onClick={() => void load()}>
            Retry
          </Button>
        ) : null}
        {user ? (
          <>
            <AdminProfileEditor
              user={user}
              onSaved={(updated) => setUser({ ...user, ...updated })}
            />
            <section
              aria-labelledby={`account-state-${person.id}`}
              className="flex flex-col gap-4"
            >
              <h3
                id={`account-state-${person.id}`}
                className="text-base font-semibold"
              >
                Account Lifecycle
              </h3>
              <p className="text-sm text-muted-foreground">
                {user.email} ·{' '}
                <StatusBadge tone={statusTone(user.accountState)}>
                  {statusLabel(user.accountState)}
                </StatusBadge>{' '}
              </p>
              <Field>
                <FieldLabel htmlFor={`account-reason-${person.id}`}>
                  Audited reason
                </FieldLabel>
                <Textarea
                  id={`account-reason-${person.id}`}
                  maxLength={500}
                  value={accountReason}
                  onChange={(event) => setAccountReason(event.target.value)}
                />
              </Field>
              {user.accountState !== 'suspended' ? (
                <Field>
                  <FieldLabel htmlFor={`account-confirm-${person.id}`}>
                    Type <strong>{person.name}</strong> to confirm suspension
                  </FieldLabel>
                  <Input
                    id={`account-confirm-${person.id}`}
                    value={accountConfirmation}
                    onChange={(event) =>
                      setAccountConfirmation(event.target.value)
                    }
                  />
                </Field>
              ) : null}
              <div>
                <Button
                  variant={
                    user.accountState === 'suspended'
                      ? 'outline'
                      : 'destructive'
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
                </Button>
              </div>
              {person.id === currentUserId &&
              user.accountState !== 'suspended' ? (
                <p className="text-sm text-muted-foreground">
                  You cannot suspend your own System Admin account.
                </p>
              ) : null}
            </section>
            <section
              aria-labelledby={`global-roles-${person.id}`}
              className="flex flex-col gap-4"
            >
              <h3
                id={`global-roles-${person.id}`}
                className="text-base font-semibold"
              >
                Global Roles
              </h3>
              {assignments.length ? (
                <ul className="m-0 flex list-none flex-col p-0">
                  {assignments.map((assignment) => (
                    <li
                      className="flex flex-wrap items-center justify-between gap-3 border-b border-border py-3 last:border-b-0"
                      key={assignment.id}
                    >
                      <span>
                        <strong>{roleLabel(assignment.role)}</strong>
                        <span className="block text-sm text-muted-foreground">
                          {assignment.state.replace('_', ' ')}
                        </span>
                      </span>
                      <Button
                        variant="destructive"
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
                      </Button>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-sm text-muted-foreground">
                  No global roles assigned.
                </p>
              )}
              {availableRoles.length ? (
                <>
                  <Field>
                    <FieldLabel htmlFor={`grant-reason-${person.id}`}>
                      Reason for role grant
                    </FieldLabel>
                    <Textarea
                      id={`grant-reason-${person.id}`}
                      maxLength={500}
                      value={grantReason}
                      onChange={(event) => setGrantReason(event.target.value)}
                    />
                  </Field>
                  <div className="flex flex-wrap items-end gap-2">
                    <Field className="min-w-40 flex-1">
                      <FieldLabel htmlFor={`grant-role-${person.id}`}>
                        Grant role
                      </FieldLabel>
                      <Select
                        items={grantRoleItems}
                        value={
                          availableRoles.includes(grantRole)
                            ? grantRole
                            : availableRoles[0]
                        }
                        onValueChange={(value) => {
                          if (value) setGrantRole(value as GlobalRole);
                        }}
                      >
                        <SelectTrigger
                          id={`grant-role-${person.id}`}
                          className="w-full"
                        >
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            {grantRoleItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                    <Button
                      type="button"
                      disabled={Boolean(pending) || !grantReason.trim()}
                      onClick={() => void grantGlobalRole()}
                    >
                      {pending.startsWith('grant-')
                        ? 'Granting…'
                        : 'Grant role'}
                    </Button>
                  </div>
                </>
              ) : null}
              {assignments.length ? (
                <>
                  <Field>
                    <FieldLabel htmlFor={`role-reason-${person.id}`}>
                      Reason for role revocation
                    </FieldLabel>
                    <Textarea
                      id={`role-reason-${person.id}`}
                      maxLength={500}
                      value={roleReason}
                      onChange={(event) => setRoleReason(event.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor={`role-confirm-${person.id}`}>
                      Type <strong>{person.name}</strong> to confirm a
                      revocation
                    </FieldLabel>
                    <Input
                      id={`role-confirm-${person.id}`}
                      value={roleConfirmation}
                      onChange={(event) =>
                        setRoleConfirmation(event.target.value)
                      }
                    />
                  </Field>
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

function WeekDialog({
  seasonId,
  week,
  open: controlledOpen,
  onOpenChange,
}: {
  seasonId: string;
  week?: Week;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const initialValues = weekDialogValues(week);
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
  const [number, setNumber] = useState(initialValues.number);
  const [startAt, setStartAt] = useState(initialValues.startAt);
  const [endAt, setEndAt] = useState(initialValues.endAt);
  const [baseline, setBaseline] = useState(initialValues);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    number !== baseline.number ||
    startAt !== baseline.startAt ||
    endAt !== baseline.endAt;
  const requestOpenChange = (next: boolean) => {
    if (!next && dirty && !window.confirm('Discard unsaved week changes?'))
      return;
    if (next) {
      const values = weekDialogValues(week);
      setNumber(values.number);
      setStartAt(values.startAt);
      setEndAt(values.endAt);
      setBaseline(values);
      setError('');
    }
    setOpen(next);
  };
  const save = async () => {
    if (
      !Number.isInteger(number) ||
      number < 1 ||
      new Date(endAt) <= new Date(startAt)
    )
      return setError('Enter a positive week number and valid dates.');
    setPending(true);
    setError('');
    const mutation: WeekMutation = {
      number,
      startAt: zonedDateTimeToUtc(startAt),
      endAt: zonedDateTimeToUtc(endAt),
    };
    try {
      const saved = demoMode
        ? {
            id: week?.id ?? `week_demo_${Date.now()}`,
            seasonId,
            ...mutation,
            resourcesHtml: week?.resourcesHtml ?? '',
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
      if (requestError instanceof ApiError && requestError.status === 409) {
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
      title={week ? `Edit Week ${week.number}` : 'Create Week'}
      open={open}
      onOpenChange={requestOpenChange}
      triggerVariant={week ? 'outline' : undefined}
      triggerSize={week ? 'sm' : undefined}
      trigger={
        controlled ? undefined : week ? (
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
      <div className="flex flex-col gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="grid gap-6 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor={`week-number-${week?.id ?? 'new'}`}>
              Week number
            </FieldLabel>
            <Input
              id={`week-number-${week?.id ?? 'new'}`}
              type="number"
              min="1"
              value={number}
              onChange={(event) => setNumber(event.target.valueAsNumber)}
            />
          </Field>
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
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" disabled={pending} onClick={() => void save()}>
            {pending ? 'Saving…' : week ? 'Save week' : 'Create week'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function DeleteWeekButton({
  seasonId,
  week,
  open,
  onOpenChange,
}: {
  seasonId: string;
  week: Week;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const remove = async () => {
    if (!demoMode)
      await apiRequest<void>(
        `/seasons/${encodeURIComponent(seasonId)}/weeks/${encodeURIComponent(week.id)}`,
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
      open={open}
      onOpenChange={onOpenChange}
      onConfirm={remove}
    />
  );
}

/** Correct, edit and remove for an enrollment, behind a kebab menu. */
function EnrollmentRowMenu({
  season,
  seasons,
  enrollment,
  accounts,
  name,
  person,
  currentUserId,
  canManageAccount,
  mentorship,
}: {
  season: Season;
  seasons: Season[];
  enrollment: Enrollment;
  accounts: Person[];
  name: string;
  person?: Person;
  currentUserId?: string;
  canManageAccount: boolean;
  /** A student's current mentorship, changed or ended from the menu. */
  mentorship?: Mentorship;
}) {
  const [open, setOpen] = useState<
    'manage' | 'edit' | 'remove' | 'mentor' | 'end-mentorship' | null
  >(null);
  const canAssignMentor =
    enrollment.state === 'active' && enrollment.role === 'student';
  const canEdit =
    enrollment.state === 'active' && enrollment.role !== 'coordinator';
  const canRemove = enrollment.state === 'active';
  const close = (next: boolean) => {
    if (!next) setOpen(null);
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${name}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {canEdit ? (
            <DropdownMenuItem onClick={() => setOpen('edit')}>
              <IconEdit size={17} aria-hidden="true" /> Edit
            </DropdownMenuItem>
          ) : null}
          {canAssignMentor ? (
            <DropdownMenuItem onClick={() => setOpen('mentor')}>
              <IconSchool size={17} aria-hidden="true" />{' '}
              {mentorship ? 'Change mentor' : 'Assign mentor'}
            </DropdownMenuItem>
          ) : null}
          {canAssignMentor && mentorship ? (
            <DropdownMenuItem onClick={() => setOpen('end-mentorship')}>
              <IconTrash size={17} aria-hidden="true" /> End mentorship
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem onClick={() => setOpen('manage')}>
            <IconShieldLock size={17} aria-hidden="true" /> Manage account
          </DropdownMenuItem>
          {canRemove ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('remove')}
            >
              <IconUserMinus size={17} aria-hidden="true" /> Remove
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {open === 'manage' ? (
        <ManagePersonDialog
          person={person}
          name={name}
          currentUserId={currentUserId}
          canManageAccount={canManageAccount}
          enrollment={enrollment}
          seasons={seasons}
          onOpenChange={close}
        />
      ) : null}
      {open === 'mentor' ? (
        <MentorshipDialog
          seasonId={season.id}
          people={accounts}
          mentorship={mentorship}
          studentUserId={enrollment.userId}
          open
          onOpenChange={close}
        />
      ) : null}
      {open === 'end-mentorship' && mentorship ? (
        <DeleteMentorshipButton
          seasonId={season.id}
          mentorship={mentorship}
          open
          onOpenChange={close}
        />
      ) : null}
      {open === 'edit' ? (
        <EnrollmentDialog
          seasonId={season.id}
          accounts={accounts}
          enrollment={enrollment}
          open
          onOpenChange={close}
        />
      ) : null}
      {open === 'remove' ? (
        <RemoveEnrollmentDialog
          seasonId={season.id}
          enrollment={enrollment}
          name={name}
          open
          onOpenChange={close}
        />
      ) : null}
    </>
  );
}

/** Edit, resources and delete for a week, behind a kebab menu. */
function WeekRowMenu({ season, week }: { season: Season; week: Week }) {
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for Week ${week.number}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => setEditOpen(true)}>
            <IconEdit size={17} aria-hidden="true" /> Edit
          </DropdownMenuItem>
          <DropdownMenuItem
            render={
              <Link to={`/seasons/${season.slug}/weeks/${week.number}`} />
            }
          >
            <IconNotes size={17} aria-hidden="true" /> Edit Resources
          </DropdownMenuItem>
          <DropdownMenuItem
            variant="destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <IconTrash size={17} aria-hidden="true" /> Delete week
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {editOpen ? (
        <WeekDialog
          seasonId={season.id}
          week={week}
          open
          onOpenChange={setEditOpen}
        />
      ) : null}
      {deleteOpen ? (
        <DeleteWeekButton
          seasonId={season.id}
          week={week}
          open
          onOpenChange={setDeleteOpen}
        />
      ) : null}
    </>
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
  open: controlledOpen,
  onOpenChange,
}: {
  seasonId: string;
  accounts: EnrollmentAccountOption[];
  enrollment?: Enrollment;
  allowCoordinator?: boolean;
  accountsLoading?: boolean;
  accountsError?: boolean;
  onRetryAccounts?: () => void;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
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
  const studentLevelItems = [
    { label: 'Novice', value: 'novice' },
    { label: 'Beginner', value: 'beginner' },
    { label: 'Intermediate', value: 'intermediate' },
    { label: 'Advanced', value: 'advanced' },
    { label: 'Not applicable', value: 'not_applicable' },
  ];
  const accountItems = [
    {
      label: accountsLoading
        ? 'Loading verified accounts…'
        : accounts.length
          ? 'Choose an account'
          : 'No eligible verified accounts',
      value: '',
    },
    ...accounts.map((person) => ({
      label: `${person.name} · @${person.slug}`,
      value: person.id,
    })),
  ];
  const enrollmentRoleItems = [
    ...(!enrollment && allowCoordinator
      ? [{ label: 'Coordinator', value: 'coordinator' }]
      : []),
    { label: 'Mentor', value: 'mentor' },
    { label: 'Student', value: 'student' },
  ];
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
        };
      else if (enrollment) {
        const mutation: EnrollmentUpdate = {
          role: role === 'coordinator' ? 'mentor' : role,
          ...(role === 'student' ? { studentLevel } : {}),
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
      if (requestError instanceof ApiError && requestError.status === 409) {
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
      title={enrollment ? 'Edit Enrollment' : 'Add Season Member'}
      open={open}
      onOpenChange={requestOpenChange}
      triggerVariant="outline"
      triggerSize={enrollment ? 'sm' : undefined}
      trigger={
        controlled ? undefined : enrollment ? (
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
      <div className="flex flex-col gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        {accountsError && !enrollment ? (
          <div className="flex flex-col items-start gap-2">
            <FieldError>
              Verified enrollment candidates could not be loaded.
            </FieldError>
            {onRetryAccounts ? (
              <Button variant="outline" type="button" onClick={onRetryAccounts}>
                Retry accounts
              </Button>
            ) : null}
          </div>
        ) : null}
        <Field>
          <FieldLabel htmlFor={`enrollment-user-${enrollment?.id ?? 'new'}`}>
            Account
          </FieldLabel>
          <Select
            items={accountItems}
            disabled={Boolean(enrollment) || accountsLoading || accountsError}
            value={userId}
            onValueChange={(value) => setUserId(value ?? '')}
          >
            <SelectTrigger
              id={`enrollment-user-${enrollment?.id ?? 'new'}`}
              className="w-full"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {accountItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <Field>
          <FieldLabel htmlFor={`enrollment-role-${enrollment?.id ?? 'new'}`}>
            Season role
          </FieldLabel>
          <Select
            items={enrollmentRoleItems}
            value={role}
            onValueChange={(value) => {
              if (value) setRole(value as SeasonRole);
            }}
          >
            <SelectTrigger
              id={`enrollment-role-${enrollment?.id ?? 'new'}`}
              className="w-full"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {enrollmentRoleItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        {enrollment && role === 'student' ? (
          <Field>
            <FieldLabel htmlFor={`enrollment-level-${enrollment.id}`}>
              Student level
            </FieldLabel>
            <Select
              items={studentLevelItems}
              value={studentLevel}
              onValueChange={(value) => {
                if (value) setStudentLevel(value as typeof studentLevel);
              }}
            >
              <SelectTrigger
                id={`enrollment-level-${enrollment.id}`}
                className="w-full"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {studentLevelItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
        ) : null}
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            disabled={pending || !userId}
            onClick={() => void save()}
          >
            {pending
              ? 'Saving…'
              : enrollment
                ? 'Save enrollment'
                : 'Add member'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function RemoveEnrollmentDialog({
  seasonId,
  enrollment,
  name,
  open: controlledOpen,
  onOpenChange,
}: {
  seasonId: string;
  enrollment: Enrollment;
  name: string;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
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
            state:
              enrollment.role === 'student'
                ? ('kicked' as const)
                : ('withdrawn' as const),
            assignmentState: 'revoked' as const,
            removalReason: reason,
          }
        : await apiRequest<Enrollment>(
            `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(enrollment.id)}/remove`,
            {
              method: 'POST',
              body: JSON.stringify({
                reason: reason.trim(),
              }),
            },
          );
      queryClient.setQueryData<EnrollmentPage>(
        seasonEnrollmentsQueryKey(seasonId),
        (current) => upsertPageItem(current, removed),
      );
      if (!demoMode) await queryClient.invalidateQueries();
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
  if (enrollment.state !== 'active') return null;
  return (
    <FormDialog
      title={`Remove ${name}?`}
      description="This member loses season access immediately. Their history is kept, and administrators can see the removal reason."
      open={open}
      onOpenChange={setOpen}
      triggerVariant="destructive"
      triggerSize="sm"
      trigger={
        controlled ? undefined : (
          <>
            <IconUserMinus size={17} aria-hidden="true" /> Remove
          </>
        )
      }
    >
      <div className="flex flex-col gap-4">
        <Field>
          <FieldLabel htmlFor={`admin-remove-reason-${enrollment.id}`}>
            Reason
          </FieldLabel>
          <Textarea
            id={`admin-remove-reason-${enrollment.id}`}
            maxLength={500}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`admin-remove-confirm-${enrollment.id}`}>
            Type <strong>{name}</strong> to confirm
          </FieldLabel>
          <Input
            id={`admin-remove-confirm-${enrollment.id}`}
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </Field>
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="destructive"
            type="button"
            disabled={
              pending || reason.trim().length < 5 || confirmation !== name
            }
            onClick={() => void remove()}
          >
            {pending ? 'Removing…' : 'Remove member'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function MentorshipDialog({
  seasonId,
  people,
  mentorship,
  studentUserId: presetStudentId,
  open: controlledOpen,
  onOpenChange,
}: {
  seasonId: string;
  people: Person[];
  mentorship?: Mentorship;
  /** Assigning a mentor from a student's row starts with that student. */
  studentUserId?: string;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const mentors = people.filter((person) => person.roles.includes('mentor'));
  const students = people.filter((person) => person.roles.includes('student'));
  const mentorItems = [
    { label: 'Choose a mentor', value: '' },
    ...mentors.map((person) => ({ label: person.name, value: person.id })),
  ];
  const studentItems = [
    { label: 'Choose a student', value: '' },
    ...students.map((person) => ({ label: person.name, value: person.id })),
  ];
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
  const [mentorUserId, setMentorUserId] = useState(
    mentorship?.mentorUserId ?? '',
  );
  const [studentUserId, setStudentUserId] = useState(
    mentorship?.studentUserId ?? presetStudentId ?? '',
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    mentorUserId !== (mentorship?.mentorUserId ?? '') ||
    studentUserId !== (mentorship?.studentUserId ?? presetStudentId ?? '');
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved mentorship changes?')
    )
      return;
    if (next) {
      setMentorUserId(mentorship?.mentorUserId ?? '');
      setStudentUserId(mentorship?.studentUserId ?? presetStudentId ?? '');
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
        (current) =>
          upsertPageItem(
            mentorship && current
              ? {
                  ...current,
                  items: current.items.filter(
                    (item) => item.id !== mentorship.id,
                  ),
                  totalCount: Math.max(0, current.totalCount - 1),
                }
              : current,
            saved,
          ),
      );
      if (!demoMode)
        await queryClient.invalidateQueries({
          queryKey: seasonMentorshipsQueryKey(seasonId),
        });
      setOpen(false);
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
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
      title={mentorship ? 'Change Mentor' : 'Add Mentorship'}
      open={open}
      onOpenChange={requestOpenChange}
      triggerVariant="outline"
      triggerSize={mentorship ? 'sm' : undefined}
      trigger={
        controlled ? undefined : mentorship ? (
          <>
            <IconEdit size={17} aria-hidden="true" /> Edit
          </>
        ) : (
          <>
            <IconPlus size={17} aria-hidden="true" /> Add mentorship
          </>
        )
      }
    >
      <div className="flex flex-col gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor={`admin-student-${mentorship?.id ?? 'new'}`}>
            Student
          </FieldLabel>
          <Select
            items={studentItems}
            value={studentUserId}
            onValueChange={(value) => setStudentUserId(value ?? '')}
          >
            <SelectTrigger
              id={`admin-student-${mentorship?.id ?? 'new'}`}
              className="w-full"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {studentItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <Field>
          <FieldLabel htmlFor={`admin-mentor-${mentorship?.id ?? 'new'}`}>
            Mentor
          </FieldLabel>
          <Select
            items={mentorItems}
            value={mentorUserId}
            onValueChange={(value) => setMentorUserId(value ?? '')}
          >
            <SelectTrigger
              id={`admin-mentor-${mentorship?.id ?? 'new'}`}
              className="w-full"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {mentorItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            disabled={pending || !mentorUserId || !studentUserId}
            onClick={() => void save()}
          >
            {pending
              ? 'Saving…'
              : mentorship
                ? 'Save mentorship'
                : 'Create mentorship'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function DeleteMentorshipButton({
  seasonId,
  mentorship,
  open,
  onOpenChange,
}: {
  seasonId: string;
  mentorship: Mentorship;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const remove = async () => {
    if (!demoMode)
      await apiRequest<void>(
        `/seasons/${encodeURIComponent(seasonId)}/mentorships/${encodeURIComponent(mentorship.id)}`,
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
      description="This ends the mentorship. Past records are kept."
      open={open}
      onOpenChange={onOpenChange}
      onConfirm={remove}
    />
  );
}

export const AdminWeeksPage = () => <AdminResourcePage resource="weeks" />;
export const AdminEnrollmentsPage = () => (
  <AdminResourcePage resource="enrollments" />
);
