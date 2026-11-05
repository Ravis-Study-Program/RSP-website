import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { IconChevronDown, IconChevronUp } from '@tabler/icons-react';
import type { ColumnDef, Row } from '@tanstack/react-table';
import type { ReactNode } from 'react';
import {
  DataTable,
  HighlightText,
  type ExtraFilters,
} from '@/components/DataTable';
import { ExternalLink } from '@/components/ExternalLink';
import { RichTextContent } from '@/components/RichTextContent';
import { Spoiler } from '@/components/Spoiler';
import { cn } from '@/lib/utils';
import { recentDateFilterOptions, withinRecentDays } from '@/tableFilters';
import type { Attempt, Difficulty } from '@/types';
import { difficultyGoal, formatDate, outcomeLabel } from '@/utils';

const badgeClass = 'rounded-sm font-semibold';
const outcomeTone = {
  success: 'border-transparent bg-success-soft text-success',
  warning: 'border-transparent bg-warning-soft text-warning',
  danger: 'border-transparent bg-destructive-soft text-destructive',
};
const difficultyTone: Record<Difficulty, string> = {
  Easy: outcomeTone.success,
  Medium: outcomeTone.warning,
  Hard: outcomeTone.danger,
};

/** Minutes taken, highlighted when within the difficulty's practice goal. */
function AttemptTime({ attempt }: { attempt: Attempt }) {
  return (
    <span
      className={
        attempt.difficulty &&
        attempt.minutes !== null &&
        attempt.minutes <= difficultyGoal(attempt.difficulty)
          ? 'font-semibold text-success'
          : undefined
      }
    >
      {attempt.minutes} min
    </span>
  );
}

const difficultyFilterOptions = (['Easy', 'Medium', 'Hard'] as const).map(
  (value) => ({ label: value, value }),
);
const outcomeFilterOptions = (
  [
    'independently_solved',
    'solved_with_hints',
    'not_solved',
    'unknown',
  ] as const
).map((value) => ({ label: outcomeLabel(value), value }));

const baseColumns: ColumnDef<Attempt, any>[] = [
  {
    accessorKey: 'problem',
    header: 'Problem',
    cell: ({ row }) => (
      <div>
        <strong>
          {row.original.problemUrl ? (
            <ExternalLink href={row.original.problemUrl}>
              <HighlightText text={row.original.problem} />
            </ExternalLink>
          ) : (
            <HighlightText text={row.original.problem} />
          )}
        </strong>
        <div className="text-sm text-muted-foreground">
          {row.original.category ? (
            <Spoiler>
              <HighlightText text={row.original.category} />
            </Spoiler>
          ) : (
            'Category unavailable'
          )}
        </div>
      </div>
    ),
  },
  {
    accessorKey: 'difficulty',
    header: 'Difficulty',
    meta: { width: 6.5, filterOptions: difficultyFilterOptions },
    filterFn: 'equalsString',
    cell: ({ getValue }) => (
      <Badge
        variant="outline"
        className={cn(badgeClass, difficultyTone[getValue() as Difficulty])}
      >
        {getValue()}
      </Badge>
    ),
  },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    meta: { width: 10.5, filterOptions: outcomeFilterOptions },
    filterFn: 'equalsString',
    cell: ({ getValue }) => (
      <Badge
        variant="outline"
        className={cn(
          badgeClass,
          getValue() === 'independently_solved'
            ? outcomeTone.success
            : getValue() === 'not_solved'
              ? outcomeTone.danger
              : outcomeTone.warning,
        )}
      >
        {outcomeLabel(getValue())}
      </Badge>
    ),
  },
  {
    accessorKey: 'confidence',
    header: 'Confidence',
    meta: { width: 6.5 },
    cell: ({ getValue }) => (getValue() ? `${getValue()}/5` : '—'),
  },
  {
    accessorKey: 'minutes',
    header: 'Time',
    meta: { width: 5.5 },
    cell: ({ row }) =>
      row.original.minutes ? <AttemptTime attempt={row.original} /> : '—',
  },
  {
    accessorKey: 'attemptedAt',
    header: 'Date Attempted',
    meta: { width: 8.5, filterOptions: recentDateFilterOptions },
    filterFn: withinRecentDays,
    cell: ({ getValue }) => formatDate(getValue()),
  },
];

/**
 * LeetCode attempts as the LeetCode page lists them: a chevron opens an
 * attempt's notes, and the page supplies any row actions. Profiles show the
 * same table without actions, and without notes unless they are your own.
 */
export function AttemptTable({
  renderActions,
  renderCardActions,
  showNotes = true,
  ...props
}: {
  ariaLabel: string;
  data: Attempt[];
  loading?: boolean;
  error?: boolean;
  onRetry?: () => void;
  emptyTitle: string;
  emptyMessage?: string;
  extraFilters?: ExtraFilters;
  toolbarLead?: ReactNode;
  renderSummary?: (rows: Attempt[]) => ReactNode;
  /** The row's kebab menu, in a right-aligned actions column. */
  renderActions?: (row: Row<Attempt>) => ReactNode;
  /** The same actions as buttons, on narrow-screen cards. */
  renderCardActions?: (attempt: Attempt) => ReactNode;
  /** False hides every attempt's notes, such as on someone else's profile. */
  showNotes?: boolean;
}) {
  const expandColumn: ColumnDef<Attempt, any> = {
    id: 'expand',
    header: 'Notes',
    meta: { width: 4.5 },
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => (
      <Button
        variant="ghost"
        size="icon"
        type="button"
        aria-label={`${row.getIsExpanded() ? 'Hide' : 'Show'} notes for ${row.original.problem}`}
        aria-expanded={row.getIsExpanded()}
        onClick={row.getToggleExpandedHandler()}
      >
        {row.getIsExpanded() ? (
          <IconChevronUp size={18} />
        ) : (
          <IconChevronDown size={18} />
        )}
      </Button>
    ),
  };
  const columns: ColumnDef<Attempt, any>[] = [
    ...(showNotes ? [expandColumn] : []),
    ...baseColumns,
    ...(renderActions
      ? [
          {
            id: 'actions',
            header: () => <span className="sr-only">Actions</span>,
            meta: { width: 4, align: 'end' },
            enableSorting: false,
            enableHiding: false,
            cell: ({ row }) => renderActions(row),
          } satisfies ColumnDef<Attempt, any>,
        ]
      : []),
  ];
  return (
    <DataTable
      {...props}
      columns={columns}
      getRowId={(attempt) => attempt.id}
      renderExpanded={
        showNotes
          ? (row) => (
              <section aria-label={`Notes for ${row.original.problem}`}>
                <RichTextContent html={row.original.notes} />
              </section>
            )
          : undefined
      }
      renderCard={(row) => (
        <div>
          <div className="flex flex-wrap items-center gap-2.5">
            <Badge
              variant="outline"
              className={cn(
                badgeClass,
                row.original.difficulty &&
                  difficultyTone[row.original.difficulty],
              )}
            >
              {row.original.difficulty}
            </Badge>
            <Badge
              variant="outline"
              className={cn(
                badgeClass,
                row.original.outcome === 'independently_solved'
                  ? outcomeTone.success
                  : row.original.outcome === 'not_solved'
                    ? outcomeTone.danger
                    : outcomeTone.warning,
              )}
            >
              {outcomeLabel(row.original.outcome)}
            </Badge>
          </div>
          <h3 className="mt-3 text-base font-semibold tracking-tight">
            {row.original.problemUrl ? (
              <ExternalLink href={row.original.problemUrl}>
                <HighlightText text={row.original.problem} />
              </ExternalLink>
            ) : (
              <HighlightText text={row.original.problem} />
            )}
          </h3>
          <p className="text-sm text-muted-foreground">
            {row.original.category ? (
              <>
                <Spoiler>{row.original.category}</Spoiler> ·{' '}
              </>
            ) : null}
            {formatDate(row.original.attemptedAt)}
          </p>
          <p>
            {row.original.minutes ? (
              <AttemptTime attempt={row.original} />
            ) : (
              'No time recorded'
            )}{' '}
            ·{' '}
            {row.original.confidence
              ? `confidence ${row.original.confidence}/5`
              : 'confidence not recorded'}
          </p>
          {showNotes ? <RichTextContent html={row.original.notes} /> : null}
          {renderCardActions ? (
            <div className="flex flex-wrap items-center gap-2.5">
              {renderCardActions(row.original)}
            </div>
          ) : null}
        </div>
      )}
    />
  );
}
