import type { BadgeTone } from '@/badgeTones';
import { StatusBadge } from '@/components/StatusBadge';
import { Checkbox } from '@/components/ui/checkbox';
import { adaptAttempt } from '@/api/adapters';
import { apiRequest } from '@/api/client';
import type {
  Attempt as ApiAttempt,
  AttemptReview,
} from '@/api/generated/models';
import { demoMode, usePracticeGoals } from '@/api/queries';
import type { ColumnDef, Row } from '@tanstack/react-table';
import { useState, type ReactNode } from 'react';
import {
  DataTable,
  HighlightText,
  type ExtraFilters,
  ExpandRowButton,
} from '@/components/DataTable';
import { ExternalLink } from '@/components/ExternalLink';
import { RichTextContent } from '@/components/RichTextContent';
import { Spoiler } from '@/components/Spoiler';
import { recentDateFilterOptions, withinRecentDays } from '@/tableFilters';
import type { Attempt, Difficulty } from '@/types';
import { outcomeLabel, hasRichText, formatDateTime } from '@/utils';

const difficultyTone: Record<Difficulty, BadgeTone> = {
  Easy: 'success',
  Medium: 'warning',
  Hard: 'danger',
};

/** Minutes taken, highlighted when within the difficulty's practice goal. */
function AttemptTime({ attempt }: { attempt: Attempt }) {
  const goals = usePracticeGoals();
  return (
    <span
      className={
        attempt.difficulty &&
        attempt.minutes !== null &&
        attempt.minutes <= goals[attempt.difficulty]
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
const outcomeFilterOptions = (['solved', 'unsolved'] as const).map((value) => ({
  label: outcomeLabel(value),
  value,
}));

/** Solved reads green and Unsolved red. */
function outcomeTone(outcome: Attempt['outcome']): BadgeTone {
  return outcome === 'solved' ? 'success' : 'danger';
}

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
      <StatusBadge tone={difficultyTone[getValue() as Difficulty]}>
        {getValue()}
      </StatusBadge>
    ),
  },
  {
    accessorKey: 'outcome',
    header: 'Outcome',
    meta: { width: 6.5, filterOptions: outcomeFilterOptions },
    filterFn: 'equalsString',
    cell: ({ getValue }) => (
      <StatusBadge tone={outcomeTone(getValue())}>
        {outcomeLabel(getValue())}
      </StatusBadge>
    ),
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
    header: 'Date and Time',
    meta: { width: 10, filterOptions: recentDateFilterOptions },
    filterFn: withinRecentDays,
    cell: ({ getValue }) => formatDateTime(getValue()),
  },
];

/**
 * Whether the attempt's owner has reviewed it, as a checkbox like a mock
 * interview's. Only the owner can tick it; everyone else sees its state.
 */
function AttemptReviewed({
  attempt,
  canReview,
  onReviewed,
}: {
  attempt: Attempt;
  canReview: boolean;
  onReviewed?: (attempt: Attempt) => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const setReviewed = async (reviewed: boolean) => {
    setPending(true);
    setError('');
    try {
      const body: AttemptReview = { reviewed };
      const saved = demoMode
        ? { ...attempt, reviewed }
        : adaptAttempt(
            await apiRequest<ApiAttempt>(
              `/problem-attempts/${encodeURIComponent(attempt.id)}/review`,
              { method: 'PATCH', body: JSON.stringify(body) },
            ),
          );
      onReviewed?.(saved);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to update the review.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <div className="grid gap-1">
      <Checkbox
        aria-label={`${attempt.problem} reviewed`}
        checked={attempt.reviewed}
        disabled={!canReview || pending}
        onCheckedChange={(checked) => void setReviewed(checked)}
      />
      {error ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/**
 * LeetCode attempts as the LeetCode page lists them: a chevron opens an
 * attempt's notes, and the page supplies any row actions. Profiles show the
 * same table without actions, and without notes unless they are your own.
 */
export function AttemptTable({
  renderActions,
  renderCardActions,
  showNotes = true,
  canReview = false,
  onReviewed,
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
  /** Whether the viewer owns these attempts and can mark them reviewed. */
  canReview?: boolean;
  /** Called with an attempt once its reviewed state is saved. */
  onReviewed?: (attempt: Attempt) => void;
}) {
  const reviewCell = (attempt: Attempt) => (
    <AttemptReviewed
      attempt={attempt}
      canReview={canReview}
      onReviewed={onReviewed}
    />
  );
  const expandColumn: ColumnDef<Attempt, any> = {
    id: 'expand',
    header: 'Details',
    meta: { width: 4.5 },
    enableSorting: false,
    enableHiding: false,
    // Only an attempt with notes has anything to expand.
    cell: ({ row }) =>
      hasRichText(row.original.notes) ? (
        <ExpandRowButton
          row={row}
          subject={`notes for ${row.original.problem}`}
        />
      ) : null,
  };
  const columns: ColumnDef<Attempt, any>[] = [
    ...(showNotes ? [expandColumn] : []),
    ...baseColumns,
    {
      id: 'reviewed',
      header: 'Reviewed',
      meta: { width: 5.5, wrap: true },
      accessorFn: (attempt) => attempt.reviewed,
      enableColumnFilter: false,
      cell: ({ row }) => reviewCell(row.original),
    },
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
            {row.original.difficulty ? (
              <StatusBadge tone={difficultyTone[row.original.difficulty]}>
                {row.original.difficulty}
              </StatusBadge>
            ) : null}
            <StatusBadge tone={outcomeTone(row.original.outcome)}>
              {outcomeLabel(row.original.outcome)}
            </StatusBadge>
            <div className="ml-auto">{reviewCell(row.original)}</div>
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
            {formatDateTime(row.original.attemptedAt)}
          </p>
          <p>
            {row.original.minutes ? (
              <AttemptTime attempt={row.original} />
            ) : (
              'No time recorded'
            )}
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
