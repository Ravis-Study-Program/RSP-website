import { personOptions } from '@/components/personOptions';
import { StatusBadge } from '@/components/StatusBadge';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useActivityFilters, filterMocks } from '@/activityFilters';
import { MockFilters } from '@/components/ActivityFilters';
import { programTimezone } from '@/activityDates';
import { ActivityPeriodFilter } from '@/components/ActivityPeriodFilter';
import { useManagedSeasons } from '@/components/useSeasonSwitcher';
import { useActivitySeason, useActivitySeasons } from '@/activityPeriod';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  IconArrowDown,
  IconArrowUp,
  IconDotsVertical,
  IconEdit,
  IconListDetails,
  IconPlus,
  IconTrash,
  IconUsers,
} from '@tabler/icons-react';
import {
  type CellContext,
  type ColumnDef,
  type Row,
} from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';

import { InfoTip } from '@/components/InfoTip';
import { Fragment, useMemo, useState, type ReactNode } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useBeforeUnload, useNavigate } from 'react-router-dom';
import { z } from 'zod';

import {
  adaptCurrentPerson,
  adaptMockInterview,
  withRoundReview,
} from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  MockIdentityCorrection,
  MockInterviewCreate,
  MockInterviewMutation,
  MockInterviewResult,
  MockIntervieweeNotes,
  RoundReview,
} from '@/api/generated/models';
import {
  demoMode,
  demoActivityWeekId,
  useCurrentUser,
  useMockInterviewPages,
  useMockInterviewSummary,
  useMockParticipants,
} from '@/api/queries';
import {
  Page,
  PageHeader,
  PersonIdentity,
  usePageTitle,
  LabelHint,
} from '@/components/Common';
import { duplicateNames } from '@/duplicateNames';
import { AppDatePicker } from '@/components/AppDatePicker';
import { ExternalLink } from '@/components/ExternalLink';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ProblemSelect } from '@/components/ProblemSelect';
import { ErrorState } from '@/components/StatusViews';
import { RichTextEditor } from '@/components/RichTextEditor';
import { RichTextContent } from '@/components/RichTextContent';
import { renderCell } from '@/lib/renderCell';
import { cn } from '@/lib/utils';
import { recentDateFilterOptions, withinRecentDays } from '@/tableFilters';
import { Checkbox } from '@/components/ui/checkbox';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  DataTable,
  type ExtraFilters,
  type ServerPagination,
  ExpandRowButton,
} from '@/components/DataTable';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Field,
  FieldError,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import type { MockInterview, MockRound, Person, Season } from '@/types';
import {
  calendarDateKey,
  formatDate,
  interviewPassed,
  problemName,
  optionalTimePattern,
  recordedDateTime,
  formatDateTime,
} from '@/utils';

const outcomeTone = { success: 'success', danger: 'danger' } as const;

const mockSchema = z.object({
  intervieweeId: z.string().min(1, 'Choose an interviewee.'),
  occurredAt: z.string().min(1, 'Choose a date.'),
  occurredTime: z
    .string()
    .regex(optionalTimePattern, 'Enter a time such as 14:30.'),
  durationMinutes: z.number().int().min(15).max(240),
});
type MockFormValues = z.infer<typeof mockSchema>;

/**
 * When a mock happened: its date and optional time in Adelaide. Without a
 * time, today's interview keeps the current time and earlier days use noon.
 */
function recordedAt(date: string, time = '') {
  return recordedDateTime(date, time, programTimezone);
}

/** A round passes at this score; an interview passes when every round does. */
const passingScore = 5;

const leetcodeScoreKeys = [
  'confirmQuestions',
  'algorithmDesign',
  'complexityAnalysis',
  'coding',
  'testing',
] as const;

function demoMockSeason(person: Person, occurredAt: string, seasons: Season[]) {
  if (!(person.seasonRole === 'student' || person.roles.includes('student')))
    return null;
  return (
    seasons.find(
      (season) =>
        season.name === person.season &&
        new Date(occurredAt) >= new Date(season.startsAt) &&
        new Date(occurredAt) <= new Date(season.endsAt),
    )?.name ?? null
  );
}

// The criteria's names as interviewers know them.
const scoreLabels: Record<string, string> = {
  confirmQuestions: 'Confirming the Question',
  algorithmDesign: 'Algorithm Design',
  complexityAnalysis: 'Complexity Analysis',
  coding: 'Coding',
  testing: 'Testing',
  behavioural: 'Behavioural',
  custom: 'Custom',
};

/** One-line reminders of what each score is for, shown beside it. */
const scoreHelp: Record<string, string> = {
  confirmQuestions:
    'Clarifying the problem, its inputs, constraints and edge cases before solving it.',
  algorithmDesign:
    'Finding a working approach and explaining it clearly before coding.',
  complexityAnalysis:
    "Working out and justifying the approach's time and space complexity.",
  coding: 'Turning the approach into correct, clean and readable code.',
  testing: 'Walking through examples and edge cases to find and fix bugs.',
  behavioural:
    'Answering with specific, well-structured examples from past experience.',
  custom: 'How well the interviewee did on what this round covered.',
};

function scoreLabel(value: string) {
  return (
    scoreLabels[value] ??
    value
      .replaceAll(/([A-Z])/g, ' $1')
      .replace(/^./, (letter) => letter.toUpperCase())
  );
}

/**
 * A colour on a scale from danger (0) through warning (5) to success (10), so
 * scores read as a range rather than just pass or fail. The end colours are
 * the theme's status colours, which meet contrast in both themes.
 */
function scoreColor(score: number) {
  return score <= 5
    ? `color-mix(in oklch, var(--destructive), var(--warning) ${score * 20}%)`
    : `color-mix(in oklch, var(--warning), var(--success) ${(score - 5) * 20}%)`;
}

/** A round's scores, LeetCode parts first in their interview order. */
function orderedScores(round: MockRound) {
  const order: string[] = [...leetcodeScoreKeys];
  return Object.entries(round.scores).sort(
    ([left], [right]) =>
      (order.indexOf(left) + 1 || order.length + 1) -
      (order.indexOf(right) + 1 || order.length + 1),
  );
}

/** Each part's score for rounds scored in parts, such as LeetCode rounds. */
function RoundScoreParts({ round }: { round: MockRound }) {
  const parts = orderedScores(round);
  if (parts.length < 2) return null;
  return (
    <dl
      className="mt-1.5 grid gap-1.5 text-xs"
      aria-label={`${round.title} score breakdown`}
    >
      {parts.map(([criterion, score]) => (
        <div
          key={criterion}
          className="grid grid-cols-[minmax(0,9rem)_1fr] items-center gap-x-3"
        >
          <dt className="flex min-w-0 items-center gap-1 text-muted-foreground">
            <span className="truncate">{scoreLabel(criterion)}</span>
            <ScoreHelp help={scoreHelp[criterion]} size={13} />
          </dt>
          <dd className="flex items-center gap-3">
            <span
              className="h-1 flex-1 overflow-hidden rounded-full bg-muted"
              aria-hidden="true"
            >
              <span
                className="block h-full rounded-full"
                style={{
                  width: `${score * 10}%`,
                  background: scoreColor(score),
                }}
              />
            </span>
            <span
              className="font-semibold tabular-nums"
              style={{ color: scoreColor(score) }}
            >
              {score}/10
            </span>
          </dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * A round's title. LeetCode rounds name their question with its number and
 * link to it, as on the LeetCode table.
 */
function RoundTitle({ round }: { round: MockRound }) {
  const problem = round.catalogProblem;
  if (round.apiType !== 'leetcode' || !problem) return round.title;
  return (
    <ExternalLink href={problem.link}>{problemName(problem)}</ExternalLink>
  );
}

/** A custom round's reference link, laid out like the part scores. */
function RoundReference({ round }: { round: MockRound }) {
  if (round.apiType !== 'custom' || !round.link) return null;
  return (
    <dl className="mt-1.5 grid grid-cols-[minmax(0,9rem)_1fr] items-center gap-x-3 text-xs">
      <dt className="truncate text-muted-foreground">Reference Link</dt>
      <dd className="min-w-0 truncate">
        <a
          href={round.link}
          target="_blank"
          rel="noreferrer"
          className="font-semibold underline decoration-[0.08em] underline-offset-[0.2em] hover:decoration-[0.14em]"
        >
          {round.link.replace(/^https?:\/\//, '')}
        </a>
      </dd>
    </dl>
  );
}

function scoresForType(type: MockRound['apiType']) {
  if (type === 'leetcode')
    return Object.fromEntries(leetcodeScoreKeys.map((key) => [key, 5]));
  if (type === 'behavioural') return { behavioural: 5 };
  return { custom: 5 };
}

function newRound(type: MockRound['apiType'] = 'custom'): MockRound {
  const scores = scoresForType(type);
  return {
    id: crypto.randomUUID(),
    type: type === 'behavioural' ? 'behavioural' : 'technical',
    apiType: type,
    title:
      type === 'behavioural'
        ? 'Behavioural round'
        : type === 'leetcode'
          ? 'Choose a LeetCode problem'
          : 'Custom round',
    score: 5,
    scores,
    notes: '',
    reviewed: false,
    intervieweeComment: '',
  };
}

/** A new interview starts with the usual layout: behavioural, then two LeetCode rounds. */
function defaultRounds(): MockRound[] {
  return [newRound('behavioural'), newRound('leetcode'), newRound('leetcode')];
}

function interviewContext(
  mock: MockInterview,
  people: Person[],
  seasons: Season[],
) {
  return {
    users: new Map(
      [...people, mock.interviewer, mock.interviewee].map((person) => [
        person.id,
        person,
      ]),
    ),
    seasons: new Map(seasons.map((season) => [season.id, season])),
  };
}

/** Which of your interviews the list shows. */
export type MockView = 'all' | 'received' | 'given';
const mockTypeItems: { label: string; value: MockView }[] = [
  { label: 'All', value: 'all' },
  { label: 'Received', value: 'received' },
  { label: 'Given', value: 'given' },
];
// Received and Given badges, styled like the Pass and Fail ones.
const directionTone = { received: 'info', given: 'warning' } as const;

// Round types in interview order: behavioural, then LeetCode, then custom.
const roundTypeItems: { label: string; value: MockRound['apiType'] }[] = [
  { label: 'Behavioural', value: 'behavioural' },
  { label: 'LeetCode', value: 'leetcode' },
  { label: 'Custom', value: 'custom' },
];

/** One equal-width button per round type, filling the row. */
function AddRoundButtons({
  onAdd,
}: {
  onAdd: (type: MockRound['apiType']) => void;
}) {
  return (
    <div className="grid grid-cols-1 gap-2.5 border-t border-border pt-4 min-[30rem]:grid-cols-3">
      {roundTypeItems.map((item) => (
        <Button
          variant="outline"
          type="button"
          key={item.value}
          className="w-full"
          onClick={() => onAdd(item.value)}
        >
          <IconPlus size={17} aria-hidden="true" /> Add{' '}
          {item.value === 'leetcode' ? item.label : item.label.toLowerCase()}
        </Button>
      ))}
    </div>
  );
}

function RoundEditor({
  round,
  index,
  total,
  onChange,
  onRemove,
  onMove,
}: {
  round: MockRound;
  index: number;
  total: number;
  onChange: (round: MockRound) => void;
  onRemove: () => void;
  onMove: (direction: -1 | 1) => void;
}) {
  const changeType = (apiType: MockRound['apiType']) =>
    onChange({
      ...round,
      apiType,
      type: apiType === 'behavioural' ? 'behavioural' : 'technical',
      title:
        apiType === 'behavioural'
          ? 'Behavioural round'
          : apiType === 'leetcode'
            ? 'Choose a LeetCode problem'
            : 'Custom round',
      problemId: undefined,
      catalogProblem: undefined,
      link: undefined,
      description: undefined,
      score: 5,
      scores: scoresForType(apiType),
    });
  return (
    <article className="grid gap-3.5 py-4 border-t border-border">
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <strong>Round {index + 1}</strong>
        <div className="flex flex-wrap items-center gap-2.5">
          <Button
            variant="ghost"
            size="icon"
            type="button"
            aria-label={`Move round ${index + 1} up`}
            disabled={index === 0}
            onClick={() => onMove(-1)}
          >
            <IconArrowUp size={16} aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            type="button"
            aria-label={`Move round ${index + 1} down`}
            disabled={index === total - 1}
            onClick={() => onMove(1)}
          >
            <IconArrowDown size={16} aria-hidden="true" />
          </Button>
          {total > 1 ? (
            <Button
              variant="ghost"
              size="icon"
              type="button"
              aria-label={`Remove round ${index + 1}`}
              onClick={onRemove}
            >
              <IconTrash size={16} aria-hidden="true" />
            </Button>
          ) : null}
        </div>
      </div>
      <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
        <Field>
          <FieldLabel htmlFor={`round-type-${round.id}`}>Type</FieldLabel>
          <Select
            items={roundTypeItems}
            value={round.apiType}
            onValueChange={(value) => {
              if (value) changeType(value as MockRound['apiType']);
            }}
          >
            <SelectTrigger id={`round-type-${round.id}`} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {roundTypeItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        {round.apiType === 'leetcode' ? (
          <Field>
            <FieldLabel htmlFor={`round-problem-${round.id}`}>
              LeetCode Problem
            </FieldLabel>
            <ProblemSelect
              id={`round-problem-${round.id}`}
              value={round.catalogProblem ?? null}
              onChange={(problem) => {
                onChange({
                  ...round,
                  problemId: problem?.id,
                  catalogProblem: problem ?? undefined,
                  title: problem?.title ?? 'Choose a LeetCode problem',
                  link: problem?.link,
                });
              }}
            />
          </Field>
        ) : null}
        {round.apiType === 'custom' ? (
          <Field>
            <FieldLabel htmlFor={`round-description-${round.id}`}>
              Description
            </FieldLabel>
            <Input
              id={`round-description-${round.id}`}
              maxLength={200}
              placeholder="What the round covered"
              value={round.description ?? ''}
              onChange={(event) =>
                onChange({
                  ...round,
                  description: event.target.value || undefined,
                  title: event.target.value.trim() || 'Custom round',
                })
              }
            />
          </Field>
        ) : null}
        {round.apiType === 'custom' ? (
          <Field>
            <FieldLabel htmlFor={`round-link-${round.id}`}>
              Reference Link <LabelHint>optional</LabelHint>
            </FieldLabel>
            <Input
              id={`round-link-${round.id}`}
              type="url"
              value={round.link ?? ''}
              onChange={(event) =>
                onChange({ ...round, link: event.target.value || undefined })
              }
            />
          </Field>
        ) : null}
        {Object.entries(round.scores).map(([criterion, score]) => (
          <Field key={criterion}>
            <FieldLabel htmlFor={`round-score-${round.id}-${criterion}`}>
              {scoreLabel(criterion)} Score <LabelHint>0–10</LabelHint>
            </FieldLabel>
            <Input
              id={`round-score-${round.id}-${criterion}`}
              type="number"
              min="0"
              max="10"
              value={score}
              onChange={(event) => {
                const value = event.target.valueAsNumber;
                const scores = { ...round.scores, [criterion]: value };
                onChange({
                  ...round,
                  score: Math.min(
                    ...Object.values(scores).filter(
                      (item): item is number => typeof item === 'number',
                    ),
                  ),
                  scores,
                });
              }}
            />
          </Field>
        ))}
      </div>
      <RichTextEditor
        id={`round-content-${round.id}`}
        label="Notes"
        labelHint={<LabelHint>optional</LabelHint>}
        value={round.notes}
        onChange={(notes) => onChange({ ...round, notes })}
      />
    </article>
  );
}

const columnsForSeason = (
  seasonId: string | undefined,
  viewerId: string | undefined,
  cells: {
    review: (mock: MockInterview) => ReactNode;
    actions: (row: Row<MockInterview>) => ReactNode;
    duplicates: ReadonlySet<string>;
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
      <ExpandRowButton
        row={row}
        subject={`details for ${row.original.interviewee.name}`}
      />
    ),
  },
  {
    accessorKey: 'interviewee.name',
    id: 'interviewee',
    header: 'Interviewee',
    enableColumnFilter: false,
    cell: ({ row }) => (
      <PersonIdentity
        person={row.original.interviewee}
        seasonId={seasonId}
        duplicates={cells.duplicates}
      />
    ),
  },
  {
    accessorKey: 'interviewer.name',
    id: 'interviewer',
    header: 'Interviewer',
    // The Interviewers dropdown already filters by interviewer.
    enableColumnFilter: false,
    cell: ({ row }) => (
      <PersonIdentity
        person={row.original.interviewer}
        seasonId={seasonId}
        duplicates={cells.duplicates}
      />
    ),
  },
  {
    // In the All view: whoever you interviewed with.
    id: 'with',
    header: 'With',
    enableColumnFilter: false,
    accessorFn: (row) =>
      (row.interviewer.id === viewerId ? row.interviewee : row.interviewer)
        .name,
    cell: ({ row }) => (
      <PersonIdentity
        person={
          row.original.interviewer.id === viewerId
            ? row.original.interviewee
            : row.original.interviewer
        }
        seasonId={seasonId}
        duplicates={cells.duplicates}
      />
    ),
  },
  {
    id: 'type',
    header: 'Type',
    meta: { width: 6.5 },
    enableColumnFilter: false,
    accessorFn: (row) =>
      row.interviewer.id === viewerId ? 'Given' : 'Received',
    cell: ({ row }) => {
      const given = row.original.interviewer.id === viewerId;
      return (
        <StatusBadge
          tone={given ? directionTone.given : directionTone.received}
        >
          {given ? 'Given' : 'Received'}
        </StatusBadge>
      );
    },
  },
  {
    id: 'result',
    header: 'Result',
    meta: { width: 5.5 },
    accessorFn: interviewPassed,
    cell: ({ row }) => (
      <StatusBadge
        tone={
          interviewPassed(row.original)
            ? outcomeTone.success
            : outcomeTone.danger
        }
      >
        {interviewPassed(row.original) ? 'Pass' : 'Fail'}
      </StatusBadge>
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
    header: 'Date and Time',
    meta: { width: 10, filterOptions: recentDateFilterOptions },
    filterFn: withinRecentDays,
    cell: ({ getValue }) => formatDateTime(getValue()),
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
    header: 'Actions',
    meta: { width: 6, align: 'end' },
    enableSorting: false,
    enableHiding: false,
    cell: ({ row }) => cells.actions(row),
  },
];

export function MockInterviewsPage() {
  usePageTitle('Mock Interviews');
  const {
    seasons: seasonsQuery,
    season,
    hasSelection,
  } = useActivitySeason('mock-interviews');
  const user = useCurrentUser();
  const peopleQuery = useMockParticipants();
  const [urlFilters, setFilters] = useActivityFilters(season?.id);
  const [mode, setMode] = useState<MockView>('all');
  // Received interviews are filtered by interviewer and given ones by
  // interviewee; a filter carried over from another view must not apply, and
  // All has no people filter.
  const filters = useMemo(
    () => ({
      ...urlFilters,
      ...(mode !== 'received' ? { interviewers: [] } : {}),
      ...(mode !== 'given' ? { interviewees: [] } : {}),
    }),
    [mode, urlFilters],
  );
  // Only people you have had interviews with, across all time, are offered.
  const participants = useMockInterviewSummary({ mode });
  const participantOptions =
    (mode === 'received'
      ? participants.data?.interviewers
      : mode === 'given'
        ? participants.data?.interviewees
        : undefined) ?? [];
  // Received mocks belong to seasons where you were a student; ones you gave
  // belong to the interviewee's season, which can be any season you took part in.
  // Directors, System Admins and coordinators pick the season at the top left.
  const managesSeasons = useManagedSeasons().length > 0;
  const periodSeasons = useActivitySeasons(
    user.data?.id,
    seasonsQuery.data?.items ?? [],
    { anyRole: mode !== 'received', selectedId: season?.id },
  );
  const navigate = useNavigate();
  const canCorrectIdentities = Boolean(
    user.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  // Keyed by mode: only the visible tab's pages are requested, and each
  // further page is requested only when the reader asks for it.
  const query = useMockInterviewPages(
    { mode, seasonId: season?.id, filters },
    !hasSelection || Boolean(season),
  );
  const [created, setCreated] = useState<MockInterview[]>([]);
  const [updated, setUpdated] = useState<Record<string, MockInterview>>({});
  const [removedIds, setRemovedIds] = useState<string[]>([]);
  const filtered = useMemo(() => {
    // The server filters loaded pages; only interviews recorded on this page
    // since loading need the scope applied locally.
    const createdInScope = filterMocks(
      created
        .map((mock) =>
          demoMode
            ? {
                ...mock,
                weekId: demoActivityWeekId(season?.id, mock.occurredAt),
              }
            : mock,
        )
        .filter(
          (mock) =>
            (!season ||
              (demoMode
                ? mock.occurredAt >= season.startsAt &&
                  mock.occurredAt <= season.endsAt
                : mock.seasonId === season.id)) &&
            (mode !== 'received' || mock.interviewee.id === user.data?.id) &&
            (mode !== 'given' || mock.interviewer.id === user.data?.id),
        ),
      filters,
    );
    return [
      ...new Map(
        [
          ...createdInScope,
          ...(query.data?.pages.flatMap((page) => page.items) ?? []),
        ].map((mock) => [mock.id, mock]),
      ).values(),
    ]
      .filter((mock) => !removedIds.includes(mock.id))
      .map((mock) => updated[mock.id] ?? mock)
      .sort(
        (left, right) =>
          new Date(right.occurredAt).getTime() -
          new Date(left.occurredAt).getTime(),
      );
  }, [
    created,
    filters,
    mode,
    query.data,
    removedIds,
    season,
    updated,
    user.data?.id,
  ]);
  const serverTotal = query.data?.pages[0]?.totalCount ?? 0;
  const localDelta =
    filtered.length -
    (query.data?.pages.reduce((sum, page) => sum + page.items.length, 0) ?? 0);
  const availablePeople = peopleQuery.data?.items ?? [];
  const interviewer = user.data
    ? (availablePeople.find((person) => person.id === user.data.id) ??
      adaptCurrentPerson(user.data, seasonsQuery.data?.items))
    : undefined;
  const removeInterview = async (mock: MockInterview) => {
    if (!demoMode)
      await apiRequest<void>(
        `/mock-interviews/${encodeURIComponent(mock.id)}`,
        { method: 'DELETE' },
      );
    setRemovedIds((ids) => [...ids, mock.id]);
  };
  const interviewChanged = (changed?: MockInterview) => {
    if (changed) setUpdated((items) => ({ ...items, [changed.id]: changed }));
    else void query.refetch();
  };
  // Mentors and coordinators active in a mock's season, and Directors and
  // System Admins, may edit or delete any mock there (the API agrees).
  const canManageMock = (seasonId?: string | null) =>
    Boolean(user.data?.globalRoles.length) ||
    Boolean(
      seasonId &&
      user.data?.seasonRoles.some(
        (membership) =>
          membership.seasonId === seasonId &&
          membership.state === 'active' &&
          (membership.role === 'mentor' || membership.role === 'coordinator'),
      ),
    );
  const actionsCell = (row: Row<MockInterview>) => (
    <MockRowActions
      row={row}
      canReview={row.original.interviewee.id === user.data?.id}
      canEdit={
        row.original.interviewer.id === user.data?.id ||
        canManageMock(row.original.seasonId)
      }
      canCorrectIdentities={canCorrectIdentities}
      people={availablePeople}
      seasons={seasonsQuery.data?.items ?? []}
      onDelete={() => removeInterview(row.original)}
      onChanged={interviewChanged}
    />
  );

  if (
    seasonsQuery.isError ||
    (hasSelection && !seasonsQuery.isLoading && !season)
  )
    return (
      <Page>
        <ErrorState
          title="Season Unavailable"
          message="The selected season could not be loaded."
          onRetry={() => void seasonsQuery.refetch()}
        />
      </Page>
    );

  return (
    <Page>
      <PageHeader
        title="Mock Interviews"
        actions={
          <NewMockDialog
            people={availablePeople}
            interviewer={interviewer}
            seasons={seasonsQuery.data?.items ?? []}
            onCreated={(mock) => {
              setCreated((items) => [mock, ...items]);
              // You are the interviewer, so the new interview is one you gave.
              if (mode === 'received') setMode('given');
            }}
          />
        }
      />
      <div className="flex flex-col gap-6">
        <MockInterviewTable
          view={mode}
          onViewChange={setMode}
          mocks={filtered}
          subjectId={user.data?.id}
          viewerId={user.data?.id}
          seasonId={season?.id}
          onChanged={interviewChanged}
          renderActions={actionsCell}
          ariaLabel={`${mode} mock interviews`}
          toolbarLead={
            managesSeasons ? (
              <ActivityPeriodFilter
                bare
                section="mock-interviews"
                season={season}
                seasons={periodSeasons}
              />
            ) : undefined
          }
          extraFilters={{
            content: (
              <>
                <ActivityPeriodFilter
                  label="Season"
                  section="mock-interviews"
                  season={season}
                  seasons={periodSeasons}
                />
                <MockTypeField view={mode} onViewChange={setMode} />
                <MockFilters
                  participant={
                    mode === 'received' ? 'interviewer' : 'interviewee'
                  }
                  seasonId={season?.id}
                  people={participantOptions}
                  filters={filters}
                  onChange={setFilters}
                />
              </>
            ),
            activeCount:
              Number(Boolean(season)) +
              Number(Boolean(filters.passed)) +
              filters.interviewers.length +
              filters.interviewees.length +
              filters.weeks.length,
            onClear: () => navigate('/mock-interviews'),
          }}
          loading={
            query.isLoading ||
            user.isLoading ||
            peopleQuery.isLoading ||
            seasonsQuery.isLoading
          }
          error={
            query.isError ||
            user.isError ||
            peopleQuery.isError ||
            seasonsQuery.isError
          }
          serverPagination={{
            totalCount: serverTotal + localDelta,
            hasMore: Boolean(query.hasNextPage),
            loadingMore: query.isFetchingNextPage,
            onLoadMore: () => void query.fetchNextPage(),
          }}
          onRetry={() =>
            void Promise.all([
              query.refetch(),
              user.refetch(),
              peopleQuery.refetch(),
              seasonsQuery.refetch(),
            ])
          }
          emptyTitle={
            mode === 'all' ? 'No Mock Interviews' : `No ${mode} Mock Interviews`
          }
          emptyMessage={
            mode === 'received'
              ? 'Interviews where you are the interviewee will appear here.'
              : mode === 'given'
                ? 'Interviews you conduct will appear here.'
                : 'Interviews you give or receive will appear here.'
          }
        />
      </div>
    </Page>
  );
}

/** The Type filter: the same choice as the table's All, Received and Given tabs. */
export function MockTypeField({
  view,
  onViewChange,
}: {
  view: MockView;
  onViewChange: (view: MockView) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor="mock-type-filter">Type</FieldLabel>
      <Select
        items={mockTypeItems}
        value={view}
        onValueChange={(value) => {
          if (value) onViewChange(value as MockView);
        }}
      >
        <SelectTrigger id="mock-type-filter" className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {mockTypeItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </Field>
  );
}

/**
 * Mock interviews as the Mock Interviews page lists them, for the page and
 * for profiles. Received and Given are relative to `subjectId`, the person
 * whose interviews these are; review checkboxes and private notes follow the
 * viewer. Without `renderActions` the table is read-only, and
 * `canViewDetails` limits whose details can be opened.
 */
export function MockInterviewTable({
  view,
  onViewChange,
  mocks,
  subjectId,
  viewerId,
  seasonId,
  onChanged,
  renderActions,
  canViewDetails = () => true,
  ...props
}: {
  view: MockView;
  onViewChange: (view: MockView) => void;
  mocks: MockInterview[];
  subjectId?: string;
  viewerId?: string;
  seasonId?: string;
  onChanged: (changed?: MockInterview) => void;
  renderActions?: (row: Row<MockInterview>) => ReactNode;
  /** Whether the viewer may open an interview's details and notes. */
  canViewDetails?: (mock: MockInterview) => boolean;
  ariaLabel: string;
  loading?: boolean;
  error?: boolean;
  onRetry?: () => void;
  emptyTitle: string;
  emptyMessage?: string;
  extraFilters?: ExtraFilters;
  serverPagination?: ServerPagination;
  renderSummary?: (rows: MockInterview[]) => ReactNode;
  toolbarLead?: ReactNode;
}) {
  const reviewCell = (mock: MockInterview) => (
    <ReviewProgress
      mock={mock}
      canReview={mock.interviewee.id === viewerId}
      onChanged={onChanged}
    />
  );
  // Each view names the other person its own way; only All needs a Type.
  const hiddenColumns: Record<MockView, string[]> = {
    all: ['interviewer', 'interviewee'],
    received: ['interviewee', 'with', 'type'],
    given: ['interviewer', 'with', 'type'],
  };
  const duplicates = duplicateNames(
    mocks.flatMap((mock) => [mock.interviewee, mock.interviewer]),
  );
  const columns = columnsForSeason(seasonId, subjectId, {
    duplicates,
    review: reviewCell,
    actions: (row) => renderActions?.(row),
  })
    .filter(
      (column) =>
        !hiddenColumns[view].includes(column.id ?? '') &&
        (renderActions || column.id !== 'actions') &&
        // No Details column when none of the details can be opened.
        (column.id !== 'expand' || mocks.some(canViewDetails)),
    )
    .map((column) =>
      column.id === 'expand'
        ? {
            ...column,
            cell: (context: CellContext<MockInterview, unknown>) =>
              canViewDetails(context.row.original)
                ? renderCell(column.cell, context)
                : null,
          }
        : column,
    );
  return (
    <DataTable
      {...props}
      toolbarStart={
        // Received or given, beside the search, at the search's height.
        <Tabs
          value={view}
          onValueChange={(value) => onViewChange(value as MockView)}
        >
          <TabsList
            aria-label="Mock interview view"
            className="group-data-[orientation=horizontal]/tabs:h-9"
          >
            {mockTypeItems.map((item) => (
              <TabsTrigger key={item.value} value={item.value}>
                {item.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      }
      data={mocks}
      columns={columns}
      getRowId={(mock) => mock.id}
      rowCanExpand={canViewDetails}
      renderExpanded={(row) =>
        canViewDetails(row.original) ? (
          <MockDetails
            row={row}
            currentUserId={viewerId}
            onChanged={onChanged}
          />
        ) : null
      }
      renderCard={(row) => (
        <div>
          <div className="flex flex-wrap items-center gap-2.5">
            <StatusBadge
              tone={
                interviewPassed(row.original)
                  ? outcomeTone.success
                  : outcomeTone.danger
              }
            >
              {interviewPassed(row.original) ? 'Pass' : 'Fail'}
            </StatusBadge>
            {view === 'all' ? (
              <StatusBadge
                tone={
                  row.original.interviewer.id === subjectId
                    ? directionTone.given
                    : directionTone.received
                }
              >
                {row.original.interviewer.id === subjectId
                  ? 'Given'
                  : 'Received'}
              </StatusBadge>
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
              duplicates={duplicates}
            />
          </h2>
          <p className="text-sm text-muted-foreground">
            {formatDateTime(row.original.occurredAt)}
          </p>
          <p>
            {row.original.durationMinutes} minutes, {row.original.rounds.length}{' '}
            {row.original.rounds.length === 1 ? 'round' : 'rounds'}
          </p>
          {canViewDetails(row.original) ? (
            <Button
              variant="outline"
              type="button"
              aria-expanded={row.getIsExpanded()}
              onClick={row.getToggleExpandedHandler()}
            >
              View Details
            </Button>
          ) : null}
          {row.getIsExpanded() && canViewDetails(row.original) ? (
            <div className="mt-3">
              <MockDetails
                row={row}
                currentUserId={viewerId}
                onChanged={onChanged}
              />
            </div>
          ) : null}
        </div>
      )}
    />
  );
}

/**
 * Saves the reviewed state of some rounds, keeping each round's comment, and
 * reports the interview after each saved round so progress shows at once.
 */
function useRoundReviewToggle(
  mock: MockInterview,
  onChanged: (changed?: MockInterview) => void,
) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const setReviewed = async (roundIds: string[], reviewed: boolean) => {
    setPending(true);
    setError('');
    let next = mock;
    try {
      for (const roundId of roundIds) {
        const comment =
          next.rounds.find((round) => round.id === roundId)
            ?.intervieweeComment ?? '';
        if (!demoMode) {
          const body: RoundReview = { reviewed, comment };
          await apiRequest<MockInterviewResult>(
            `/mock-interviews/${encodeURIComponent(mock.id)}/rounds/${encodeURIComponent(roundId)}/review`,
            { method: 'PATCH', body: JSON.stringify(body) },
          );
        }
        next = withRoundReview(next, roundId, { reviewed, comment });
      }
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        onChanged();
        setError(
          'This interview changed elsewhere. Its latest version is loading.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to update the review.',
        );
    } finally {
      // Rounds saved before any failure still count.
      if (next !== mock) onChanged(next);
      setPending(false);
    }
  };
  return { setReviewed, pending, error };
}

/**
 * Whether the interviewee has reviewed every round: ticked when all are,
 * dashed when some are. The interviewee can tick it to mark every round
 * reviewed (or unreviewed) at once; for anyone else it only shows the state.
 */
function ReviewProgress({
  mock,
  canReview,
  onChanged,
}: {
  mock: MockInterview;
  canReview: boolean;
  onChanged: (changed?: MockInterview) => void;
}) {
  const { setReviewed, pending, error } = useRoundReviewToggle(mock, onChanged);
  const total = mock.rounds.length;
  const reviewed = mock.rounds.filter((round) => round.reviewed).length;
  const complete = reviewed === total;
  const progress = `${reviewed} of ${total} rounds reviewed`;
  return (
    <div className="grid gap-1">
      <Checkbox
        aria-label={
          canReview
            ? `${complete ? 'Mark every round unreviewed' : 'Mark every round reviewed'} (${progress})`
            : progress
        }
        title={progress}
        checked={complete}
        indeterminate={!complete && reviewed > 0}
        disabled={!canReview || pending}
        onCheckedChange={() =>
          void setReviewed(
            mock.rounds
              .filter((round) => round.reviewed === complete)
              .map((round) => round.id),
            !complete,
          )
        }
      />
      {error ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/** A row's actions behind a kebab menu, as on the LeetCode table. */
function MockRowActions({
  row,
  canReview,
  canEdit,
  canCorrectIdentities,
  people,
  seasons,
  onDelete,
  onChanged,
}: {
  row: Row<MockInterview>;
  canReview: boolean;
  canEdit: boolean;
  canCorrectIdentities: boolean;
  people: Person[];
  seasons: Season[];
  onDelete: () => Promise<void>;
  onChanged: (changed?: MockInterview) => void;
}) {
  const mock = row.original;
  const [open, setOpen] = useState<
    'review' | 'edit' | 'identities' | 'delete' | null
  >(null);
  const close = (next: boolean) => {
    if (!next) setOpen(null);
  };
  // Name the other participant: on interviews you received, that is the interviewer.
  const otherPerson = canReview ? mock.interviewer : mock.interviewee;
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for the interview with ${otherPerson.name}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={row.getToggleExpandedHandler()}>
            <IconListDetails size={17} aria-hidden="true" />
            {row.getIsExpanded() ? 'Hide Details' : 'Show Details'}
          </DropdownMenuItem>
          {canReview ? (
            <DropdownMenuItem onClick={() => setOpen('review')}>
              <IconEdit size={17} aria-hidden="true" /> Edit Notes
            </DropdownMenuItem>
          ) : null}
          {canEdit ? (
            <DropdownMenuItem onClick={() => setOpen('edit')}>
              <IconEdit size={17} aria-hidden="true" /> Edit Notes and Scores
            </DropdownMenuItem>
          ) : null}
          {canCorrectIdentities ? (
            <DropdownMenuItem onClick={() => setOpen('identities')}>
              <IconUsers size={17} aria-hidden="true" /> Correct Identities
            </DropdownMenuItem>
          ) : null}
          {canEdit ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('delete')}
            >
              <IconTrash size={17} aria-hidden="true" /> Delete Interview
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {/* Mounted on open so each opening starts from the current interview. */}
      {open === 'review' ? (
        <RoundReviewDialog
          mock={mock}
          onChanged={onChanged}
          open
          onOpenChange={close}
        />
      ) : null}
      {open === 'edit' ? (
        <EditMockDialog
          mock={mock}
          people={people}
          seasons={seasons}
          onChanged={onChanged}
          open
          onOpenChange={close}
        />
      ) : null}
      {open === 'identities' ? (
        <IdentityCorrectionDialog
          mock={mock}
          people={people}
          seasons={seasons}
          onChanged={onChanged}
          open
          onOpenChange={close}
        />
      ) : null}
      {canEdit ? (
        <NamedConfirmation
          name={mock.interviewee.name}
          actionLabel="Delete Interview"
          description="This removes the interview from your history. Earlier versions and the record of changes are retained."
          open={open === 'delete'}
          onOpenChange={close}
          onConfirm={onDelete}
        />
      ) : null}
    </>
  );
}

// The heading row and each round row are separate grids, so every column
// needs a fixed or fractional width; an auto column would size per row and
// pull the headings out of line with their cells. The interviewer does not see
// the interviewee's notes, so their rows have one column fewer.
function roundRowClass(showIntervieweeNotes: boolean) {
  return cn(
    'grid gap-3 p-4 min-[56rem]:gap-x-4',
    showIntervieweeNotes
      ? 'min-[56rem]:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1fr)_5rem]'
      : 'min-[56rem]:grid-cols-[minmax(0,1.2fr)_minmax(0,2fr)_5rem]',
  );
}
const roundCellClass =
  'grid content-start gap-1 min-[56rem]:border-l min-[56rem]:pl-4';

/** An info icon that says what a score is for, on hover or click. */
function ScoreHelp({ help, size = 15 }: { help?: string; size?: number }) {
  if (!help) return null;
  return <InfoTip label={help} size={size} />;
}

/**
 * The first details row: the lowest round score beside both people's notes
 * on the whole interview, and the interview's reviewed checkbox, mirroring
 * the one in the table row.
 */
function OverallRow({
  mock,
  showIntervieweeNotes,
  canReview,
  onChanged,
}: {
  mock: MockInterview;
  showIntervieweeNotes: boolean;
  canReview: boolean;
  onChanged: (changed?: MockInterview) => void;
}) {
  return (
    <li className={roundRowClass(showIntervieweeNotes)}>
      <article aria-label="Overall" className="grid content-start gap-1.5">
        <strong className="inline-flex items-center gap-1.5 font-semibold">
          Overall
        </strong>
      </article>
      <div
        aria-label="Interviewer notes on the interview"
        className={roundCellClass}
      >
        <h3 className="text-xs font-semibold text-muted-foreground min-[56rem]:sr-only">
          Interviewer Notes
        </h3>
        <RichTextContent
          html={mock.notes}
          empty="No notes from the interviewer."
        />
      </div>
      {showIntervieweeNotes ? (
        <div
          aria-label="Interviewee notes on the interview"
          className={roundCellClass}
        >
          <h3 className="text-xs font-semibold text-muted-foreground min-[56rem]:sr-only">
            Interviewee Notes
          </h3>
          <RichTextContent
            html={mock.intervieweeNotes}
            empty="No notes from the interviewee."
          />
        </div>
      ) : null}
      <div className="flex items-center gap-2 self-stretch min-[56rem]:items-start min-[56rem]:border-l min-[56rem]:pt-0.5 min-[56rem]:pl-4">
        <ReviewProgress
          mock={mock}
          canReview={canReview}
          onChanged={onChanged}
        />
        <span className="text-sm font-medium min-[56rem]:sr-only">
          Overall reviewed
        </span>
      </div>
    </li>
  );
}

/**
 * One row per round lining up its scores, the
 * interviewer's notes on it, the interviewee's notes on it, and whether the
 * interviewee has marked it reviewed.
 */
function MockDetails({
  row,
  currentUserId,
  onChanged,
}: {
  row: Row<MockInterview>;
  currentUserId?: string;
  onChanged: (changed?: MockInterview) => void;
}) {
  const mock = row.original;
  const canReview = mock.interviewee.id === currentUserId;
  // The interviewee's notes are private from the interviewer.
  const showIntervieweeNotes = mock.interviewer.id !== currentUserId;
  const rowClass = roundRowClass(showIntervieweeNotes);
  const { setReviewed, pending, error } = useRoundReviewToggle(mock, onChanged);
  // Sub-table headings match the table's own column headings.
  const headingClass = 'text-xs font-semibold text-muted-foreground';
  // Column headings show on wide screens; stacked rows label each part instead.
  const cellHeadingClass =
    'text-xs font-semibold text-muted-foreground min-[56rem]:sr-only';
  return (
    <div className="overflow-hidden rounded-lg border bg-card">
      <section aria-label="Rounds and review" className="min-w-0">
        <div className={cn(rowClass, 'hidden border-b py-3 min-[56rem]:grid')}>
          <h2 className={headingClass}>Rounds and Scores</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Interviewer Notes</h2>
          {showIntervieweeNotes ? (
            <h2 className={cn(headingClass, 'pl-4')}>Interviewee Notes</h2>
          ) : null}
          <h2 className={cn(headingClass, 'pl-4')}>Reviewed</h2>
        </div>
        <ol className="divide-y">
          <OverallRow
            mock={mock}
            showIntervieweeNotes={showIntervieweeNotes}
            canReview={canReview}
            onChanged={onChanged}
          />
          {mock.rounds.map((round) => (
            <li key={round.id} className={rowClass}>
              <article
                aria-label={round.title}
                className="grid content-start gap-1.5"
              >
                <div className="flex items-baseline justify-between gap-3">
                  <strong className="inline-flex items-center gap-1.5 font-semibold">
                    <RoundTitle round={round} />
                    {/* LeetCode rounds explain each part instead. */}
                    {round.apiType === 'leetcode' ? null : (
                      <ScoreHelp help={scoreHelp[round.apiType]} />
                    )}
                  </strong>
                  {/* LeetCode rounds show each part's score instead. */}
                  {round.apiType === 'leetcode' ? null : (
                    <span
                      className="font-semibold tabular-nums"
                      style={
                        round.score === null
                          ? undefined
                          : { color: scoreColor(round.score) }
                      }
                    >
                      {round.score === null
                        ? 'Not scored'
                        : `${round.score}/10`}
                    </span>
                  )}
                </div>
                {round.score === null || round.apiType === 'leetcode' ? null : (
                  <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full rounded-full"
                      style={{
                        width: `${round.score * 10}%`,
                        background: scoreColor(round.score),
                      }}
                    />
                  </div>
                )}
                <RoundScoreParts round={round} />
                <RoundReference round={round} />
              </article>
              <div
                aria-label={`Interviewer notes on ${round.title}`}
                className={roundCellClass}
              >
                <h3 className={cellHeadingClass}>Interviewer Notes</h3>
                <RichTextContent
                  html={round.notes}
                  empty="No notes from the interviewer."
                />
              </div>
              {showIntervieweeNotes ? (
                <div
                  aria-label={`Interviewee notes on ${round.title}`}
                  className={roundCellClass}
                >
                  <h3 className={cellHeadingClass}>Interviewee Notes</h3>
                  <RichTextContent
                    html={round.intervieweeComment}
                    empty="No notes from the interviewee."
                  />
                </div>
              ) : null}
              <Field
                orientation="horizontal"
                className="w-auto items-center self-stretch min-[56rem]:items-start min-[56rem]:border-l min-[56rem]:pt-0.5 min-[56rem]:pl-4"
              >
                <Checkbox
                  id={`round-reviewed-${round.id}`}
                  checked={round.reviewed}
                  disabled={!canReview || pending}
                  onCheckedChange={(checked) =>
                    void setReviewed([round.id], checked)
                  }
                />
                <FieldLabel
                  htmlFor={`round-reviewed-${round.id}`}
                  className="min-[56rem]:sr-only"
                >
                  {round.title} reviewed
                </FieldLabel>
              </Field>
            </li>
          ))}
        </ol>
        {error ? (
          <p
            role="alert"
            className="border-t px-4 py-3 text-sm text-destructive"
          >
            {error}
          </p>
        ) : null}
      </section>
    </div>
  );
}

function EditMockDialog({
  mock,
  people,
  seasons,
  onChanged,
  open,
  onOpenChange: setOpen,
}: {
  mock: MockInterview;
  people: Person[];
  seasons: Season[];
  onChanged: (changed?: MockInterview) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [durationMinutes, setDurationMinutes] = useState(mock.durationMinutes);
  const [notes, setNotes] = useState(mock.notes);
  const [rounds, setRounds] = useState(() =>
    mock.rounds.map((round) => ({ ...round, scores: { ...round.scores } })),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const resetDraft = () => {
    setDurationMinutes(mock.durationMinutes);
    setNotes(mock.notes);
    setRounds(
      mock.rounds.map((round) => ({ ...round, scores: { ...round.scores } })),
    );
    setError('');
  };
  const dirty =
    durationMinutes !== mock.durationMinutes ||
    notes !== mock.notes ||
    JSON.stringify(rounds) !== JSON.stringify(mock.rounds);
  useBeforeUnload((event) => {
    if (dirty) event.preventDefault();
  });
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard your unsaved interview changes?')
    )
      return;
    if (next) resetDraft();
    setOpen(next);
  };
  const save = async () => {
    setPending(true);
    setError('');
    try {
      let changed: MockInterview;
      if (!demoMode) {
        const body: MockInterviewMutation = {
          occurredAt: mock.occurredAt,
          durationMinutes,
          notes,
          rounds: rounds.map((round) => ({
            id: round.id,
            type: round.apiType,
            ...(round.problemId ? { problemId: round.problemId } : {}),
            ...(round.notes ? { content: round.notes } : {}),
            ...(round.link ? { link: round.link } : {}),
            ...(round.description ? { description: round.description } : {}),
            scores: round.scores,
          })),
        };
        const result = await apiRequest<MockInterviewResult>(
          `/mock-interviews/${encodeURIComponent(mock.id)}`,
          { method: 'PATCH', body: JSON.stringify(body) },
        );
        changed = adaptMockInterview(
          result.interview,
          interviewContext(mock, people, seasons),
        );
      } else {
        changed = {
          ...mock,
          occurredAt: mock.occurredAt,
          durationMinutes,
          notes,
          rounds,
        };
      }
      onChanged(changed);
      setOpen(false);
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        onChanged();
        setError(
          'This interview changed elsewhere. The latest version is being loaded; close and reopen before editing.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to update this interview.',
        );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Edit Notes and Scores"
      open={open}
      onOpenChange={requestOpenChange}
    >
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
          <Field>
            <FieldLabel htmlFor={`edit-date-${mock.id}`}>Date</FieldLabel>
            {/* Recorded on the day; the date cannot be changed afterwards. */}
            <Input
              id={`edit-date-${mock.id}`}
              value={formatDate(mock.occurredAt)}
              disabled
              className="text-muted-foreground"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor={`edit-duration-${mock.id}`}>
              Duration <LabelHint>minutes</LabelHint>
            </FieldLabel>
            <Input
              id={`edit-duration-${mock.id}`}
              type="number"
              min="1"
              value={durationMinutes}
              onChange={(event) =>
                setDurationMinutes(event.target.valueAsNumber)
              }
            />
          </Field>
        </div>
        <div className="border-t pt-4">
          <RichTextEditor
            id={`edit-notes-${mock.id}`}
            label="Overall Notes"
            labelHint={<LabelHint>optional</LabelHint>}
            value={notes}
            onChange={setNotes}
          />
        </div>
        <FieldSet>
          <FieldLegend className="sr-only">Rounds and scores</FieldLegend>
          {rounds.map((round, roundIndex) => (
            <RoundEditor
              key={round.id}
              round={round}
              index={roundIndex}
              total={rounds.length}
              onChange={(changed) =>
                setRounds((items) =>
                  items.map((item) =>
                    item.id === changed.id ? changed : item,
                  ),
                )
              }
              onRemove={() =>
                setRounds((items) =>
                  items.filter((item) => item.id !== round.id),
                )
              }
              onMove={(direction) =>
                setRounds((items) => {
                  const next = [...items];
                  const target = roundIndex + direction;
                  [next[roundIndex], next[target]] = [
                    next[target],
                    next[roundIndex],
                  ];
                  return next;
                })
              }
            />
          ))}
          <AddRoundButtons
            onAdd={(type) => setRounds((items) => [...items, newRound(type)])}
          />
        </FieldSet>
        <div className="mt-5 button-group">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={
              pending ||
              !dirty ||
              durationMinutes < 1 ||
              rounds.some(
                (round) => round.apiType === 'leetcode' && !round.problemId,
              )
            }
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save Changes'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

/**
 * The interviewee's notes on every round, edited together and scrolling like
 * the interviewer's notes and scores dialog. Only changed rounds are saved.
 */
function RoundReviewDialog({
  mock,
  onChanged,
  open,
  onOpenChange: setOpen,
}: {
  mock: MockInterview;
  onChanged: (changed?: MockInterview) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const saved = () =>
    Object.fromEntries(
      mock.rounds.map((round) => [
        round.id,
        { comment: round.intervieweeComment, reviewed: round.reviewed },
      ]),
    );
  const [drafts, setDrafts] = useState(saved);
  const [overall, setOverall] = useState(mock.intervieweeNotes);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const changed = mock.rounds.filter(
    (round) =>
      drafts[round.id].comment !== round.intervieweeComment ||
      drafts[round.id].reviewed !== round.reviewed,
  );
  const overallChanged = overall !== mock.intervieweeNotes;
  const dirty = overallChanged || changed.length > 0;
  useBeforeUnload((event) => {
    if (dirty) event.preventDefault();
  });
  const updateDraft = (
    roundId: string,
    change: Partial<{ comment: string; reviewed: boolean }>,
  ) =>
    setDrafts((items) => ({
      ...items,
      [roundId]: { ...items[roundId], ...change },
    }));
  const save = async () => {
    setPending(true);
    setError('');
    let next = mock;
    try {
      if (overallChanged) {
        if (!demoMode) {
          const body: MockIntervieweeNotes = { notes: overall };
          await apiRequest<MockInterviewResult>(
            `/mock-interviews/${encodeURIComponent(mock.id)}/interviewee-notes`,
            { method: 'PATCH', body: JSON.stringify(body) },
          );
        }
        next = { ...next, intervieweeNotes: overall };
      }
      for (const round of changed) {
        const review = drafts[round.id];
        if (!demoMode) {
          const body: RoundReview = review;
          await apiRequest<MockInterviewResult>(
            `/mock-interviews/${encodeURIComponent(mock.id)}/rounds/${encodeURIComponent(round.id)}/review`,
            { method: 'PATCH', body: JSON.stringify(body) },
          );
        }
        next = withRoundReview(next, round.id, review);
      }
      setOpen(false);
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        onChanged();
        setError(
          'This interview changed elsewhere. The latest version is being loaded; reopen your notes before trying again.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to save your notes.',
        );
    } finally {
      // Show what was saved straight away, including rounds saved before a failure.
      if (next !== mock) onChanged(next);
      setPending(false);
    }
  };
  const requestOpenChange = (next: boolean) => {
    if (!next && dirty && !window.confirm('Discard your unsaved notes?'))
      return;
    if (next) {
      setDrafts(saved());
      setOverall(mock.intervieweeNotes);
    }
    setOpen(next);
  };
  return (
    <FormDialog title="Edit Notes" open={open} onOpenChange={requestOpenChange}>
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <RichTextEditor
          id={`interviewee-notes-${mock.id}`}
          label="Overall Notes"
          labelHint={<LabelHint>optional</LabelHint>}
          value={overall}
          onChange={setOverall}
        />
        <FieldSet>
          <FieldLegend className="sr-only">Rounds</FieldLegend>
          {mock.rounds.map((round, index) => (
            <article
              key={round.id}
              className="grid gap-3.5 py-4 border-t border-border"
            >
              <strong>
                Round {index + 1}: <RoundTitle round={round} />
              </strong>
              <RichTextEditor
                id={`review-comment-${round.id}`}
                label="Notes"
                labelHint={<LabelHint>optional</LabelHint>}
                value={drafts[round.id].comment}
                onChange={(comment) => updateDraft(round.id, { comment })}
              />
              <Field orientation="horizontal">
                <Checkbox
                  id={`review-reviewed-${round.id}`}
                  checked={drafts[round.id].reviewed}
                  onCheckedChange={(reviewed) =>
                    updateDraft(round.id, { reviewed })
                  }
                />
                <FieldLabel htmlFor={`review-reviewed-${round.id}`}>
                  Mark this round reviewed
                </FieldLabel>
              </Field>
            </article>
          ))}
        </FieldSet>
        <div className="button-group border-t border-border pt-4">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || !dirty}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save Notes'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function IdentityCorrectionDialog({
  mock,
  people,
  seasons,
  onChanged,
  open,
  onOpenChange: setOpen,
}: {
  mock: MockInterview;
  people: Person[];
  seasons: Season[];
  onChanged: (changed?: MockInterview) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const candidates = [
    ...new Map(
      [...people, mock.interviewer, mock.interviewee].map((person) => [
        person.id,
        person,
      ]),
    ).values(),
  ];
  const [interviewerId, setInterviewerId] = useState(mock.interviewer.id);
  const [intervieweeId, setIntervieweeId] = useState(mock.interviewee.id);
  const [reason, setReason] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    interviewerId !== mock.interviewer.id ||
    intervieweeId !== mock.interviewee.id ||
    reason.length > 0;
  useBeforeUnload((event) => {
    if (dirty) event.preventDefault();
  });
  const resetDraft = () => {
    setInterviewerId(mock.interviewer.id);
    setIntervieweeId(mock.interviewee.id);
    setReason('');
    setError('');
  };
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard this unsaved identity correction?')
    )
      return;
    if (next) resetDraft();
    setOpen(next);
  };
  const save = async () => {
    if (!reason.trim())
      return setError('Explain why the participants need correcting.');
    if (interviewerId === intervieweeId)
      return setError('Interviewer and interviewee must be different people.');
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: MockIdentityCorrection = {
          interviewerId,
          intervieweeId,
          reason: reason.trim(),
        };
        const result = await apiRequest<MockInterviewResult>(
          `/mock-interviews/${encodeURIComponent(mock.id)}/identity-correction`,
          { method: 'POST', body: JSON.stringify(body) },
        );
        onChanged(
          adaptMockInterview(
            result.interview,
            interviewContext(mock, candidates, seasons),
          ),
        );
      } else {
        onChanged({
          ...mock,
          interviewer:
            candidates.find((person) => person.id === interviewerId) ??
            mock.interviewer,
          interviewee:
            candidates.find((person) => person.id === intervieweeId) ??
            mock.interviewee,
          season: demoMockSeason(
            candidates.find((person) => person.id === intervieweeId) ??
              mock.interviewee,
            mock.occurredAt,
            seasons,
          ),
        });
      }
      setOpen(false);
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        onChanged();
        setError(
          'This interview changed elsewhere. The latest version is being loaded.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to correct interview identities.',
        );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Correct Interview Identities"
      open={open}
      onOpenChange={requestOpenChange}
    >
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
          <Field>
            <FieldLabel htmlFor={`correct-interviewer-${mock.id}`}>
              Interviewer
            </FieldLabel>
            <Select
              items={personOptions(candidates)}
              value={interviewerId}
              onValueChange={(value) => setInterviewerId(value ?? '')}
            >
              <SelectTrigger
                id={`correct-interviewer-${mock.id}`}
                className="w-full"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {candidates.map((person) => (
                    <SelectItem key={person.id} value={person.id}>
                      {person.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor={`correct-interviewee-${mock.id}`}>
              Interviewee
            </FieldLabel>
            <Select
              items={personOptions(candidates)}
              value={intervieweeId}
              onValueChange={(value) => setIntervieweeId(value ?? '')}
            >
              <SelectTrigger
                id={`correct-interviewee-${mock.id}`}
                className="w-full"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {candidates.map((person) => (
                    <SelectItem key={person.id} value={person.id}>
                      {person.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
        </div>
        <Field>
          <FieldLabel htmlFor={`correction-reason-${mock.id}`}>
            Audited Reason
          </FieldLabel>
          <Textarea
            id={`correction-reason-${mock.id}`}
            maxLength={2000}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </Field>
        <div className="mt-5 button-group">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || !reason.trim()}
            onClick={() => void save()}
          >
            {pending ? 'Correcting…' : 'Save Audited Correction'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function NewMockDialog({
  people,
  interviewer,
  seasons,
  onCreated,
}: {
  people: Person[];
  interviewer?: Person;
  seasons: Season[];
  onCreated: (mock: MockInterview) => void;
}) {
  const [open, setOpen] = useState(false);
  const [rounds, setRounds] = useState<MockRound[]>(defaultRounds);
  const [roundError, setRoundError] = useState('');
  const [roundsDirty, setRoundsDirty] = useState(false);
  const [notes, setNotes] = useState('');
  const {
    control,
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<MockFormValues>({
    resolver: zodResolver(mockSchema as never) as Resolver<MockFormValues>,
    defaultValues: {
      intervieweeId: '',
      occurredAt: calendarDateKey(new Date(), programTimezone),
      occurredTime: '',
      durationMinutes: 60,
    },
  });
  useBeforeUnload((event) => {
    if (isDirty || roundsDirty || notes) event.preventDefault();
  });
  const onSubmit = handleSubmit(async (values) => {
    const interviewee = people.find(
      (person) => person.id === values.intervieweeId,
    );
    if (!interviewee || !interviewer) return;
    if (
      rounds.some((round) => round.apiType === 'leetcode' && !round.problemId)
    ) {
      setRoundError('Choose a problem for every LeetCode round.');
      return;
    }
    if (
      rounds.some((round) =>
        Object.values(round.scores).some(
          (score) =>
            typeof score !== 'number' ||
            !Number.isInteger(score) ||
            score < 0 ||
            score > 10,
        ),
      )
    ) {
      setRoundError('Every score must be a whole number from 0 to 10.');
      return;
    }
    setRoundError('');
    if (values.occurredAt > calendarDateKey(new Date(), programTimezone)) {
      setRoundError('Choose a date that has already happened.');
      return;
    }
    // A time later today is in the future too.
    if (
      new Date(recordedAt(values.occurredAt, values.occurredTime)) > new Date()
    ) {
      setError('occurredTime', {
        message: 'Interviews cannot be in the future.',
      });
      return;
    }
    const mock: MockInterview = {
      id: `mock_local_${Date.now()}`,
      interviewee,
      interviewer,
      season: null,
      occurredAt: recordedAt(values.occurredAt, values.occurredTime),
      durationMinutes: values.durationMinutes,
      notes,
      intervieweeNotes: '',
      rounds,
      reviewStatus: 'pending',
    };
    try {
      if (!demoMode) {
        const request: MockInterviewCreate = {
          interviewee: { userId: values.intervieweeId },
          occurredAt: mock.occurredAt,
          durationMinutes: values.durationMinutes,
          ...(notes ? { notes } : {}),
          rounds: rounds.map((round) => ({
            id: round.id,
            type: round.apiType,
            ...(round.problemId ? { problemId: round.problemId } : {}),
            ...(round.notes ? { content: round.notes } : {}),
            ...(round.link ? { link: round.link } : {}),
            ...(round.description ? { description: round.description } : {}),
            scores: round.scores,
          })),
        };
        const result = await apiRequest<MockInterviewResult>(
          '/mock-interviews',
          { method: 'POST', body: JSON.stringify(request) },
        );
        const userMap = new Map(people.map((person) => [person.id, person]));
        userMap.set(interviewer.id, interviewer);
        onCreated(
          adaptMockInterview(result.interview, {
            users: userMap,
            seasons: new Map(seasons.map((season) => [season.id, season])),
          }),
        );
      } else {
        mock.season = demoMockSeason(interviewee, mock.occurredAt, seasons);
        onCreated(mock);
      }
    } catch (requestError) {
      setRoundError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to record this interview.',
      );
      return;
    }
    reset();
    setRounds(defaultRounds());
    setRoundsDirty(false);
    setNotes('');
    setOpen(false);
  });
  const requestOpen = (next: boolean) => {
    if (
      !next &&
      (isDirty || roundsDirty || notes) &&
      !window.confirm('Discard your unsaved interview?')
    )
      return;
    {
      reset({
        intervieweeId: '',
        occurredAt: calendarDateKey(new Date(), programTimezone),
        durationMinutes: 60,
      });
      setRounds(defaultRounds());
      setRoundsDirty(false);
      setNotes('');
      setRoundError('');
    }
    setOpen(next);
  };

  const updateRound = (changed: MockRound) => {
    setRoundsDirty(true);
    setRounds((items) =>
      items.map((item) => (item.id === changed.id ? changed : item)),
    );
  };
  const moveRound = (roundIndex: number, direction: -1 | 1) => {
    setRoundsDirty(true);
    setRounds((items) => {
      const next = [...items];
      const target = roundIndex + direction;
      [next[roundIndex], next[target]] = [next[target], next[roundIndex]];
      return next;
    });
  };

  return (
    <FormDialog
      title="New Mock Interview"
      open={open}
      onOpenChange={requestOpen}
      trigger={
        <>
          <IconPlus size={18} aria-hidden="true" /> New Interview
        </>
      }
    >
      <form className="grid gap-4" noValidate onSubmit={onSubmit}>
        <Field>
          <FieldLabel htmlFor="mock-interviewee">Interviewee</FieldLabel>
          <Controller
            name="intervieweeId"
            control={control}
            render={({ field }) => {
              const candidates = people.filter(
                (person) => person.id !== interviewer?.id,
              );
              // People who share a name also show their slug, in grey.
              const nameCounts = new Map<string, number>();
              for (const person of candidates)
                nameCounts.set(
                  person.name.toLowerCase(),
                  (nameCounts.get(person.name.toLowerCase()) ?? 0) + 1,
                );
              const slugSuffix = new Map(
                candidates.map((person) => [
                  person.id,
                  (nameCounts.get(person.name.toLowerCase()) ?? 0) > 1
                    ? ` (${person.slug})`
                    : '',
                ]),
              );
              const labels = new Map(
                candidates.map((person) => [
                  person.id,
                  `${person.name}${slugSuffix.get(person.id)}${person.roles.length ? ` · ${person.roles.join(', ')}` : ''}`,
                ]),
              );
              return (
                <Combobox
                  items={candidates.map((person) => person.id)}
                  itemToStringLabel={(id: string) => labels.get(id) ?? ''}
                  name={field.name}
                  value={field.value || null}
                  onValueChange={(value) => field.onChange(value ?? '')}
                >
                  <ComboboxInput
                    id="mock-interviewee"
                    className="w-full"
                    placeholder="Search eligible participants"
                    aria-invalid={errors.intervieweeId ? true : undefined}
                    onBlur={field.onBlur}
                  />
                  <ComboboxContent>
                    <ComboboxEmpty>No matching participants.</ComboboxEmpty>
                    <ComboboxList>
                      {(id: string) => (
                        <ComboboxItem key={id} value={id}>
                          {slugSuffix.get(id) ? (
                            <>
                              {labels.get(id)?.replace(slugSuffix.get(id)!, '')}
                              <span className="text-muted-foreground">
                                {slugSuffix.get(id)}
                              </span>
                            </>
                          ) : (
                            labels.get(id)
                          )}
                        </ComboboxItem>
                      )}
                    </ComboboxList>
                  </ComboboxContent>
                </Combobox>
              );
            }}
          />
          {errors.intervieweeId ? (
            <FieldError>{errors.intervieweeId.message}</FieldError>
          ) : null}
        </Field>
        <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
          {/* Defaults to today; interviews cannot be recorded in the future. */}
          <Controller
            name="occurredAt"
            control={control}
            render={({ field }) => (
              <AppDatePicker
                label="Date"
                value={field.value}
                onChange={field.onChange}
                error={errors.occurredAt?.message}
              />
            )}
          />
          <Field>
            <FieldLabel htmlFor="mock-time">
              Time <LabelHint>optional</LabelHint>
            </FieldLabel>
            <Input
              id="mock-time"
              type="time"
              aria-invalid={Boolean(errors.occurredTime)}
              {...register('occurredTime')}
            />
            {errors.occurredTime ? (
              <FieldError>{errors.occurredTime.message}</FieldError>
            ) : null}
          </Field>
          <Field>
            <FieldLabel htmlFor="mock-duration">
              Duration <LabelHint>minutes</LabelHint>
            </FieldLabel>
            <Input
              id="mock-duration"
              type="number"
              min="15"
              max="240"
              {...register('durationMinutes', { valueAsNumber: true })}
            />
          </Field>
        </div>
        <div className="border-t pt-4">
          <RichTextEditor
            id="new-mock-notes"
            label="Overall Notes"
            labelHint={<LabelHint>optional</LabelHint>}
            value={notes}
            onChange={setNotes}
          />
        </div>
        <FieldSet>
          <FieldLegend className="sr-only">Interview rounds</FieldLegend>
          {roundError ? <FieldError>{roundError}</FieldError> : null}
          {rounds.map((round, roundIndex) => (
            <RoundEditor
              key={round.id}
              round={round}
              index={roundIndex}
              total={rounds.length}
              onChange={updateRound}
              onRemove={() => {
                setRoundsDirty(true);
                setRounds((items) =>
                  items.filter((item) => item.id !== round.id),
                );
              }}
              onMove={(direction) => moveRound(roundIndex, direction)}
            />
          ))}
          <AddRoundButtons
            onAdd={(type) => {
              setRoundsDirty(true);
              setRounds((items) => [...items, newRound(type)]);
            }}
          />
        </FieldSet>
        <div className="mt-5 button-group">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpen(false)}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Creating…' : 'Create Interview'}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
}
