import { Fragment, type ReactNode } from 'react';
import type { ColumnDef, Row } from '@tanstack/react-table';
import { IconChevronDown, IconChevronUp } from '@tabler/icons-react';
import type { MockInterviewMode } from '@/api/queries';
import { DataTable, type DataTableProps } from '@/components/DataTable';
import { PersonIdentity } from '@/components/Common';
import {
  MockInterviewDetails,
  MockReviewProgress,
} from '@/components/MockInterviewDetails';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { passingScore } from '@/mockInterviewScores';
import { recentDateFilterOptions, withinRecentDays } from '@/tableFilters';
import type { MockInterview } from '@/types';
import { formatDate, interviewPassed } from '@/utils';

const badgeClass = 'rounded-sm font-semibold';
const outcomeTone = {
  success: 'border-transparent bg-success-soft text-success',
  warning: 'border-transparent bg-warning-soft text-warning',
  danger: 'border-transparent bg-destructive-soft text-destructive',
};

// Received and Given badges, styled like the Pass and Fail ones.
const directionTone = {
  received: 'border-transparent bg-info-soft text-info',
  given: 'border-transparent bg-warning-soft text-warning',
};

const columnsForSeason = (
  seasonId: string | undefined,
  subjectId: string | undefined,
  cells: {
    review: (mock: MockInterview) => ReactNode;
    actions?: (row: Row<MockInterview>) => ReactNode;
  },
): ColumnDef<MockInterview, any>[] => [
  // Most important first: who, the result and scores, then the details.
  {
    id: 'expand',
    header: 'Details',
    meta: { width: 4.5 },
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => (
      <Button
        variant="ghost"
        size="icon"
        type="button"
        aria-label={`${row.getIsExpanded() ? 'Hide' : 'Show'} details for ${row.original.interviewee.name}`}
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
  },
  {
    accessorKey: 'interviewee.name',
    id: 'interviewee',
    header: 'Interviewee',
    enableColumnFilter: false,
    cell: ({ row }) => (
      <PersonIdentity person={row.original.interviewee} seasonId={seasonId} />
    ),
  },
  {
    accessorKey: 'interviewer.name',
    id: 'interviewer',
    header: 'Interviewer',
    // The Interviewers dropdown already filters by interviewer.
    enableColumnFilter: false,
    cell: ({ row }) => (
      <PersonIdentity person={row.original.interviewer} seasonId={seasonId} />
    ),
  },
  {
    // In the All view: whoever you interviewed with.
    id: 'with',
    header: 'With',
    enableColumnFilter: false,
    accessorFn: (row) =>
      (row.interviewer.id === subjectId ? row.interviewee : row.interviewer)
        .name,
    cell: ({ row }) => (
      <PersonIdentity
        person={
          row.original.interviewer.id === subjectId
            ? row.original.interviewee
            : row.original.interviewer
        }
        seasonId={seasonId}
      />
    ),
  },
  {
    id: 'type',
    header: 'Type',
    meta: { width: 6.5 },
    enableColumnFilter: false,
    accessorFn: (row) =>
      row.interviewer.id === subjectId ? 'Given' : 'Received',
    cell: ({ row }) => {
      const given = row.original.interviewer.id === subjectId;
      return (
        <Badge
          variant="outline"
          className={cn(
            badgeClass,
            given ? directionTone.given : directionTone.received,
          )}
        >
          {given ? 'Given' : 'Received'}
        </Badge>
      );
    },
  },
  {
    id: 'result',
    header: 'Result',
    meta: { width: 5.5 },
    accessorFn: interviewPassed,
    cell: ({ row }) => (
      <Badge
        variant="outline"
        className={cn(
          badgeClass,
          interviewPassed(row.original)
            ? outcomeTone.success
            : outcomeTone.danger,
        )}
      >
        {interviewPassed(row.original) ? 'Pass' : 'Fail'}
      </Badge>
    ),
  },
  {
    id: 'score',
    header: 'Scores',
    // Wide enough for three rounds, the most an interview has.
    meta: { width: 10.5 },
    accessorFn: (row) =>
      row.rounds
        .map((round) => round.score)
        .filter((score): score is number => score !== null)
        .reduce((sum, score) => sum + score, 0),
    // Each round's score, passing ones in green as on the LeetCode table.
    cell: ({ row }) => (
      <span className="font-semibold tabular-nums">
        {row.original.rounds.map((round, index) => (
          <Fragment key={round.id}>
            {index > 0 ? ' · ' : null}
            <span
              className={
                round.score !== null && round.score >= passingScore
                  ? 'text-success'
                  : undefined
              }
            >
              {round.score === null ? '—' : `${round.score}/10`}
            </span>
          </Fragment>
        ))}
      </span>
    ),
  },
  {
    accessorKey: 'durationMinutes',
    header: 'Duration',
    meta: { width: 6 },
    cell: ({ getValue }) => `${getValue()} min`,
  },
  {
    accessorKey: 'occurredAt',
    header: 'Date',
    meta: { width: 7.5, filterOptions: recentDateFilterOptions },
    filterFn: withinRecentDays,
    cell: ({ getValue }) => formatDate(getValue()),
  },
  {
    id: 'review',
    header: 'Reviewed',
    meta: { width: 5.5, wrap: true },
    accessorFn: (row) => row.rounds.filter((round) => round.reviewed).length,
    cell: ({ row }) => cells.review(row.original),
  },
  {
    id: 'actions',
    header: () => <span className="sr-only">Actions</span>,
    meta: { width: 4, align: 'end' },
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => cells.actions?.(row),
  },
];

type MockInterviewHistoryProps = Omit<
  DataTableProps<MockInterview>,
  'columns' | 'getRowId' | 'renderCard' | 'renderExpanded' | 'defaultExpanded'
> & {
  seasonId?: string;
  subjectId?: string;
  viewerId?: string;
  mode: MockInterviewMode;
  onChanged?: (changed?: MockInterview) => void;
  renderActions?: (row: Row<MockInterview>) => ReactNode;
};

/** Subject controls Given/Received; the viewer controls note visibility. */
export function MockInterviewHistory({
  seasonId,
  subjectId,
  viewerId,
  mode,
  onChanged,
  renderActions,
  ...props
}: MockInterviewHistoryProps) {
  const reviewCell = (mock: MockInterview) => (
    <MockReviewProgress
      mock={mock}
      canReview={Boolean(onChanged) && mock.interviewee.id === viewerId}
      onChanged={onChanged}
    />
  );
  // Each view names the other person its own way; only All needs a Type.
  const hiddenColumns: Record<MockInterviewMode, string[]> = {
    all: ['interviewer', 'interviewee'],
    received: ['interviewee', 'with', 'type'],
    given: ['interviewer', 'with', 'type'],
  };
  const columns = columnsForSeason(seasonId, subjectId, {
    review: reviewCell,
    actions: renderActions,
  }).filter(
    (column) =>
      !hiddenColumns[mode].includes(column.id ?? '') &&
      (column.id !== 'actions' || Boolean(renderActions)),
  );
  return (
    <DataTable
      {...props}
      columns={columns}
      getRowId={(mock) => mock.id}
      renderExpanded={(row) => (
        <MockInterviewDetails
          mock={row.original}
          viewerId={viewerId}
          onChanged={onChanged}
        />
      )}
      renderCard={(row) => (
        <div>
          <div className="flex flex-wrap items-center gap-2.5">
            <Badge
              variant="outline"
              className={cn(
                badgeClass,
                interviewPassed(row.original)
                  ? outcomeTone.success
                  : outcomeTone.danger,
              )}
            >
              {interviewPassed(row.original) ? 'Pass' : 'Fail'}
            </Badge>
            {mode === 'all' ? (
              <Badge
                variant="outline"
                className={cn(
                  badgeClass,
                  row.original.interviewer.id === subjectId
                    ? directionTone.given
                    : directionTone.received,
                )}
              >
                {row.original.interviewer.id === subjectId
                  ? 'Given'
                  : 'Received'}
              </Badge>
            ) : null}
            {reviewCell(row.original)}
            {renderActions ? (
              <div className="ml-auto">{renderActions(row)}</div>
            ) : null}
          </div>
          <h2 className="mt-3 text-base font-semibold tracking-tight">
            <PersonIdentity
              person={
                row.original.interviewer.id === subjectId
                  ? row.original.interviewee
                  : row.original.interviewer
              }
              seasonId={seasonId}
            />
          </h2>
          <p className="text-sm text-muted-foreground">
            {formatDate(row.original.occurredAt)}
          </p>
          <p>
            {row.original.durationMinutes} minutes, {row.original.rounds.length}{' '}
            {row.original.rounds.length === 1 ? 'round' : 'rounds'}
          </p>
          <Button
            variant="outline"
            type="button"
            aria-expanded={row.getIsExpanded()}
            onClick={row.getToggleExpandedHandler()}
          >
            View details
          </Button>
          {row.getIsExpanded() ? (
            <div className="mt-3">
              <MockInterviewDetails
                mock={row.original}
                viewerId={viewerId}
                onChanged={onChanged}
              />
            </div>
          ) : null}
        </div>
      )}
    />
  );
}
