import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { IconDotsVertical, IconEdit, IconUserMinus } from '@tabler/icons-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useState, type ReactNode } from 'react';

import { statusTone } from '@/badgeTones';
import {
  PersonIdentity,
  RoleBadge,
  SeasonBadge,
  StudentLevelBadge,
} from '@/components/Common';
import { type ExtraFilters, DataTable } from '@/components/DataTable';
import { LatestMarkBadge } from '@/components/MondayMark';
import { StatusBadge } from '@/components/StatusBadge';
import { StudentDetailsDialog } from '@/components/StudentDetailsDialog';
import { latestMarkRank, type LatestMark } from '@/mondayMarks';
import { updateStatusTones, type StudentUpdateStatus } from '@/mondayStatus';
import { roleSortingFn, sortRoles, statusSortingFn } from '@/sortOrder';
import { studentLevelLabel } from '@/studentLevels';
import type { Person, Season } from '@/types';
import { formatDateTime } from '@/utils';

/** A row's latest Monday update: a status, or the student's latest update. */
export type PersonLatestMark =
  | 'not-mentee'
  | 'loading'
  | 'error'
  | {
      latest: LatestMark | undefined;
      /** This week's update status; absent once the season is over. */
      thisWeek?: StudentUpdateStatus;
    };

const latestMarkText = {
  'not-mentee': '—',
  loading: 'Loading…',
  error: 'Unavailable',
} as const;

function statusLabel(status: string | null | undefined) {
  const text = (status ?? '').replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
}

function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

/**
 * The people and student lists: one table, one line per cell. The People
 * directory shows roles and seasons and takes no actions; the student lists
 * (All Students, My Students) add the latest Monday update and a row menu.
 */
export function PeopleTable({
  variant,
  ariaLabel,
  data,
  seasons = [],
  seasonId,
  showLevel = true,
  latestMark,
  showThisWeek = false,
  renderActions,
  loading,
  error,
  onRetry,
  emptyTitle,
  emptyMessage,
  toolbarStart,
  extraFilters,
}: {
  variant: 'people' | 'students';
  ariaLabel: string;
  data: Person[];
  /** Every season, to badge each person's season. */
  seasons?: Season[];
  seasonId?: string;
  showLevel?: boolean;
  /** Each student's latest Monday update; omit to hide those columns. */
  latestMark?: (person: Person) => PersonLatestMark;
  /** The Monday Update column (this week's status), while the season runs. */
  showThisWeek?: boolean;
  /** The row menu; omit for a read-only table. */
  renderActions?: (person: Person) => ReactNode;
  loading?: boolean;
  error?: boolean;
  onRetry?: () => void;
  emptyTitle: string;
  emptyMessage?: string;
  toolbarStart?: ReactNode;
  extraFilters?: ExtraFilters;
}) {
  const people = variant === 'people';
  const seasonFor = (person: Person) =>
    seasons.find(
      (item) => item.name === person.season || item.slug === person.season,
    );
  const columns: ColumnDef<Person, any>[] = [
    {
      accessorKey: 'name',
      header: people ? 'Person' : 'Student',
      // Wide enough for a full name beside its avatar.
      meta: { width: 12 },
      cell: ({ row }) => (
        <PersonIdentity
          seasonId={seasonId}
          person={row.original}
          showRoles={false}
        />
      ),
    },
    ...(people
      ? [
          {
            id: 'roles',
            header: 'Roles',
            meta: { width: 9.5 },
            accessorFn: (person: Person) => sortRoles(person.roles).join(' '),
            sortingFn: roleSortingFn,
            cell: ({ row }: { row: { original: Person } }) => (
              <div className="flex items-center gap-2">
                {sortRoles(row.original.roles).map((role) => (
                  <RoleBadge key={role} role={role} />
                ))}
              </div>
            ),
          } satisfies ColumnDef<Person, string>,
          {
            accessorKey: 'season',
            header: 'Season',
            meta: { width: 5.5 },
            cell: ({ row }: { row: { original: Person } }) => {
              const match = seasonFor(row.original);
              return match ? (
                <SeasonBadge name={match.name} startsAt={match.startsAt} />
              ) : (
                <Dash />
              );
            },
          } satisfies ColumnDef<Person, string>,
        ]
      : []),
    ...(showLevel
      ? [
          {
            id: 'studentLevel',
            header: 'Level',
            meta: { width: 7.5 },
            accessorFn: (person: Person) =>
              studentLevelLabel(person.studentLevel),
            cell: ({ row }: { row: { original: Person } }) =>
              row.original.studentLevel &&
              row.original.studentLevel !== 'not_applicable' ? (
                <StudentLevelBadge level={row.original.studentLevel} />
              ) : (
                <Dash />
              ),
          } satisfies ColumnDef<Person, string>,
        ]
      : []),
    {
      accessorKey: 'status',
      header: 'Status',
      meta: { width: 7.5 },
      sortingFn: statusSortingFn,
      cell: ({ getValue }) => (
        <StatusBadge tone={statusTone(getValue())}>
          {statusLabel(getValue())}
        </StatusBadge>
      ),
    },
    { accessorKey: 'attempts', header: 'LeetCodes', meta: { width: 6.5 } },
    {
      accessorKey: 'interviews',
      header: 'Mock Interviews',
      meta: { width: 8.5 },
    },
    ...(latestMark
      ? [
          ...(showThisWeek
            ? [
                {
                  id: 'thisWeek',
                  header: 'Monday Update',
                  meta: { width: 9 },
                  accessorFn: (person: Person) => {
                    const state = latestMark(person);
                    return typeof state === 'string'
                      ? ''
                      : (state.thisWeek ?? '');
                  },
                  cell: ({ row }: { row: { original: Person } }) => {
                    const state = latestMark(row.original);
                    if (state === 'loading' || state === 'error')
                      return (
                        <span className="text-sm text-muted-foreground">
                          {latestMarkText[state]}
                        </span>
                      );
                    return typeof state === 'string' || !state.thisWeek ? (
                      <Dash />
                    ) : (
                      <StatusBadge tone={updateStatusTones[state.thisWeek]}>
                        {state.thisWeek}
                      </StatusBadge>
                    );
                  },
                } satisfies ColumnDef<Person, string>,
              ]
            : []),
          {
            id: 'latestMark',
            header: 'Mark',
            meta: { width: 8 },
            accessorFn: (person: Person) => {
              const state = latestMark(person);
              return typeof state === 'string'
                ? latestMarkRank(null)
                : latestMarkRank(state.latest);
            },
            cell: ({ row }: { row: { original: Person } }) => {
              const state = latestMark(row.original);
              return typeof state === 'string' ? (
                <Dash />
              ) : (
                <LatestMarkBadge latest={state.latest} />
              );
            },
          } satisfies ColumnDef<Person, number>,
        ]
      : []),
    {
      accessorKey: 'lastActiveAt',
      header: 'Last Recorded Activity',
      meta: { width: 10.5 },
      cell: ({ getValue }) =>
        getValue() ? formatDateTime(getValue()) : <Dash />,
    },
    ...(renderActions
      ? [
          {
            id: 'actions',
            header: 'Actions',
            meta: { width: 6, align: 'end' as const },
            enableSorting: false,
            enableHiding: false,
            cell: ({ row }: { row: { original: Person } }) =>
              renderActions(row.original),
          } satisfies ColumnDef<Person, unknown>,
        ]
      : []),
  ];
  return (
    <DataTable
      ariaLabel={ariaLabel}
      data={data}
      columns={columns}
      loading={loading}
      error={error}
      onRetry={onRetry}
      emptyTitle={emptyTitle}
      emptyMessage={emptyMessage}
      toolbarStart={toolbarStart}
      extraFilters={extraFilters}
      getRowId={(person) => person.id}
      rowHref={(person) =>
        `/people/${person.slug}${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`
      }
      renderCard={(row) => {
        const person = row.original;
        const match = people ? seasonFor(person) : undefined;
        const state = latestMark?.(person);
        return (
          <div className="grid gap-2.5">
            <div className="flex items-center justify-between gap-3">
              <PersonIdentity
                seasonId={seasonId}
                person={person}
                showRoles={false}
              />
              {renderActions?.(person)}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {people
                ? sortRoles(person.roles).map((role) => (
                    <RoleBadge key={role} role={role} />
                  ))
                : null}
              {match ? (
                <SeasonBadge name={match.name} startsAt={match.startsAt} />
              ) : null}
              {showLevel ? (
                <StudentLevelBadge level={person.studentLevel} />
              ) : null}
              <StatusBadge tone={statusTone(person.status)}>
                {statusLabel(person.status)}
              </StatusBadge>
              {showThisWeek &&
              state &&
              typeof state !== 'string' &&
              state.thisWeek ? (
                <StatusBadge tone={updateStatusTones[state.thisWeek]}>
                  {state.thisWeek}
                </StatusBadge>
              ) : null}
              {state && typeof state !== 'string' ? (
                <LatestMarkBadge latest={state.latest} />
              ) : null}
            </div>
            <p className="text-sm text-muted-foreground">
              {person.attempts ?? 0} LeetCodes, {person.interviews ?? 0} mock
              interviews
            </p>
          </div>
        );
      }}
    />
  );
}

/**
 * A student row's menu: edit their details (level, goals, weekly hours)
 * and, for their mentor or a
 * coordinator, remove them. Draws nothing when neither is allowed.
 */
export function StudentRowActions({
  person,
  seasonId,
  canSetLevel,
  canRemove,
  onLevelChanged,
  renderRemoveDialog,
}: {
  person: Person;
  seasonId?: string;
  canSetLevel: boolean;
  canRemove: boolean;
  onLevelChanged: () => void;
  /** The removal dialog, opened from the menu. */
  renderRemoveDialog: (props: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
  }) => ReactNode;
}) {
  const [open, setOpen] = useState<'level' | 'remove' | null>(null);
  if (!canSetLevel && !canRemove) return null;
  const close = (next: boolean) => {
    if (!next) setOpen(null);
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${person.name}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {canSetLevel ? (
            <DropdownMenuItem onClick={() => setOpen('level')}>
              <IconEdit size={17} aria-hidden="true" /> Edit Details
            </DropdownMenuItem>
          ) : null}
          {canRemove ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('remove')}
            >
              <IconUserMinus size={17} aria-hidden="true" /> Remove Student
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {canSetLevel && seasonId && open === 'level' ? (
        <StudentDetailsDialog
          seasonId={seasonId}
          person={person}
          onChanged={onLevelChanged}
          open
          onOpenChange={close}
        />
      ) : null}
      {canRemove && open === 'remove'
        ? renderRemoveDialog({ open: true, onOpenChange: close })
        : null}
    </>
  );
}
