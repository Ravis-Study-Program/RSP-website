import { Badge } from '@/components/ui/badge';
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
import { programmeTimezone } from '@/activityDates';
import { ActivityPeriodFilter } from '@/components/ActivityPeriodFilter';
import { useActivitySeason } from '@/activityPeriod';
import { zodResolver } from '@hookform/resolvers/zod';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import {
  IconArrowDown,
  IconArrowUp,
  IconChevronDown,
  IconChevronUp,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import type { ColumnDef, Row } from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';
import { useMemo, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useBeforeUnload } from 'react-router-dom';
import { z } from 'zod';

import { adaptCurrentPerson, adaptMockInterview } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  MockIdentityCorrection,
  MockInterviewCreate,
  MockInterviewMutation,
  MockInterviewResult,
  RoundReview,
} from '@/api/generated/models';
import {
  demoMode,
  demoActivityWeekId,
  useCurrentUser,
  useMockInterviewPages,
  type MockInterviewMode,
  useMockParticipants,
} from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ProblemSelect } from '@/components/ProblemSelect';
import { ErrorState } from '@/components/StatusViews';
import { RichTextEditor } from '@/components/RichTextEditor';
import { RichTextContent } from '@/components/RichTextContent';
import { cn } from '@/lib/utils';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import type { MockInterview, MockRound, Person, Season } from '@/types';
import {
  calendarDateKey,
  formatDateTime,
  formatDateTimeInput,
  interviewPassed,
  zonedDateTimeToUtc,
} from '@/utils';

const badgeClass = 'rounded-sm font-semibold';
const outcomeTone = {
  success: 'border-transparent bg-success-soft text-success',
  warning: 'border-transparent bg-warning-soft text-warning',
  danger: 'border-transparent bg-destructive-soft text-destructive',
};

const mockSchema = z.object({
  intervieweeId: z.string().min(1, 'Choose an interviewee.'),
  occurredAt: z.string().min(1, 'Choose a time.'),
  durationMinutes: z.number().int().min(15).max(240),
});
type MockFormValues = z.infer<typeof mockSchema>;

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

function scoreLabel(value: string) {
  return value
    .replaceAll(/([A-Z])/g, ' $1')
    .replace(/^./, (letter) => letter.toUpperCase());
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

const roundTypeItems: { label: string; value: MockRound['apiType'] }[] = [
  { label: 'LeetCode', value: 'leetcode' },
  { label: 'Custom', value: 'custom' },
  { label: 'Behavioural', value: 'behavioural' },
];

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
      score: 5,
      scores: scoresForType(apiType),
    });
  return (
    <article className="py-4 first:pt-0 [&+&]:border-t [&+&]:border-border">
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
            <Button variant="ghost" type="button" onClick={onRemove}>
              <IconTrash size={16} aria-hidden="true" /> Remove
            </Button>
          ) : null}
        </div>
      </div>
      <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
        <Field>
          <FieldLabel htmlFor={`round-type-${round.id}`}>Round type</FieldLabel>
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
              LeetCode problem
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
            <FieldLabel htmlFor={`round-link-${round.id}`}>
              Reference link (optional)
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
              {scoreLabel(criterion)} score (0–10)
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
        label="Round notes"
        value={round.notes}
        onChange={(notes) => onChange({ ...round, notes })}
      />
    </article>
  );
}

const columnsForSeason = (
  seasonId?: string,
): ColumnDef<MockInterview, any>[] => [
  {
    id: 'expand',
    header: 'Details',
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
    cell: ({ row }) => (
      <PersonIdentity person={row.original.interviewee} seasonId={seasonId} />
    ),
  },
  {
    accessorKey: 'interviewer.name',
    id: 'interviewer',
    header: 'Interviewer',
    cell: ({ row }) => (
      <PersonIdentity person={row.original.interviewer} seasonId={seasonId} />
    ),
  },
  {
    accessorKey: 'occurredAt',
    header: 'Date',
    cell: ({ getValue }) => formatDateTime(getValue()),
  },
  {
    accessorKey: 'durationMinutes',
    header: 'Duration',
    cell: ({ getValue }) => `${getValue()} min`,
  },
  {
    id: 'score',
    header: 'Scores',
    accessorFn: (row) =>
      row.rounds
        .map((round) => round.score)
        .filter((score): score is number => score !== null)
        .reduce((sum, score) => sum + score, 0),
    cell: ({ row }) => (
      <span className="font-semibold tabular-nums">
        {row.original.rounds.map((round) => round.score ?? '—').join(' · ')}/10
      </span>
    ),
  },
  {
    id: 'result',
    header: 'Result',
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
        {interviewPassed(row.original) ? 'Pass' : 'Needs work'}
      </Badge>
    ),
  },
  {
    accessorKey: 'reviewStatus',
    header: 'Review',
    cell: ({ getValue }) => (
      <Badge
        variant="outline"
        className={cn(
          badgeClass,
          getValue() === 'reviewed' ? outcomeTone.success : outcomeTone.warning,
        )}
      >
        {getValue() === 'reviewed' ? 'Reviewed' : 'Pending'}
      </Badge>
    ),
  },
];

export function MockInterviewsPage() {
  usePageTitle('Mock interviews');
  const { seasons: seasonsQuery, season, hasSelection } = useActivitySeason();
  const user = useCurrentUser();
  const peopleQuery = useMockParticipants();
  const columns = useMemo(() => columnsForSeason(season?.id), [season?.id]);
  const [filters, setFilters] = useActivityFilters(season?.id);
  const [mode, setMode] = useState<MockInterviewMode>('received');
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

  if (
    seasonsQuery.isError ||
    (hasSelection && !seasonsQuery.isLoading && !season)
  )
    return (
      <div className="mx-auto w-full max-w-[92rem]">
        <ErrorState
          title="Season unavailable"
          message="The selected season could not be loaded."
          onRetry={() => void seasonsQuery.refetch()}
        />
      </div>
    );

  return (
    <div className="mx-auto w-full max-w-[92rem]">
      <PageHeader
        eyebrow="Interview preparation"
        title="Mock interviews"
        description="Keep rounds, scores and feedback together. Interview details are versioned, and only the interviewee controls their review."
        actions={
          <NewMockDialog
            people={availablePeople}
            interviewer={interviewer}
            seasons={seasonsQuery.data?.items ?? []}
            onCreated={(mock) => setCreated((items) => [mock, ...items])}
          />
        }
      />
      <ActivityPeriodFilter
        section="mock-interviews"
        season={season}
        seasons={seasonsQuery.data?.items ?? []}
      />
      <MockFilters
        seasonId={season?.id}
        people={availablePeople}
        filters={filters}
        onChange={setFilters}
      />
      <Tabs
        className="mt-6"
        value={mode}
        onValueChange={(value) => setMode(value as MockInterviewMode)}
      >
        <TabsList aria-label="Mock interview view">
          <TabsTrigger value="received">Received</TabsTrigger>
          <TabsTrigger value="given">Given</TabsTrigger>
          <TabsTrigger value="all">All available</TabsTrigger>
        </TabsList>
        <TabsContent value={mode}>
          <DataTable
            ariaLabel={`${mode} mock interviews`}
            data={filtered}
            columns={columns}
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
            emptyTitle={`No ${mode} mock interviews`}
            emptyMessage={
              mode === 'received'
                ? 'Interviews where you are the interviewee will appear here.'
                : mode === 'given'
                  ? 'Interviews you conduct will appear here.'
                  : 'Create the first eligible mock interview.'
            }
            getRowId={(mock) => mock.id}
            renderExpanded={(row) => (
              <MockDetails
                row={row}
                currentUserId={user.data?.id}
                canCorrectIdentities={Boolean(
                  user.data?.globalRoles.some(
                    (role) => role === 'director' || role === 'system_admin',
                  ),
                )}
                people={availablePeople}
                seasons={seasonsQuery.data?.items ?? []}
                onDelete={() => removeInterview(row.original)}
                onChanged={(changed) => {
                  if (changed)
                    setUpdated((items) => ({
                      ...items,
                      [changed.id]: changed,
                    }));
                  else void query.refetch();
                }}
              />
            )}
            defaultExpanded
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
                    {interviewPassed(row.original) ? 'Pass' : 'Needs work'}
                  </Badge>
                  <Badge
                    variant="outline"
                    className={cn(
                      badgeClass,
                      row.original.reviewStatus === 'reviewed'
                        ? outcomeTone.success
                        : outcomeTone.warning,
                    )}
                  >
                    {row.original.reviewStatus}
                  </Badge>
                </div>
                <h2 className="mt-3 text-base font-semibold tracking-tight">
                  <PersonIdentity
                    person={row.original.interviewee}
                    seasonId={season?.id}
                  />
                </h2>
                <p className="text-sm text-muted-foreground">
                  with {row.original.interviewer.name} ·{' '}
                  {formatDateTime(row.original.occurredAt)}
                </p>
                <p>
                  {row.original.durationMinutes} minutes ·{' '}
                  {row.original.rounds.length}{' '}
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
                    <MockDetails
                      row={row}
                      currentUserId={user.data?.id}
                      canCorrectIdentities={Boolean(
                        user.data?.globalRoles.some(
                          (role) =>
                            role === 'director' || role === 'system_admin',
                        ),
                      )}
                      people={availablePeople}
                      seasons={seasonsQuery.data?.items ?? []}
                      onDelete={() => removeInterview(row.original)}
                      onChanged={(changed) => {
                        if (changed)
                          setUpdated((items) => ({
                            ...items,
                            [changed.id]: changed,
                          }));
                        else void query.refetch();
                      }}
                    />
                  </div>
                ) : null}
              </div>
            )}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function MockDetails({
  row,
  currentUserId,
  canCorrectIdentities,
  people,
  seasons,
  onDelete,
  onChanged,
}: {
  row: Row<MockInterview>;
  currentUserId?: string;
  canCorrectIdentities: boolean;
  people: Person[];
  seasons: Season[];
  onDelete: () => Promise<void>;
  onChanged: (changed?: MockInterview) => void;
}) {
  const mock = row.original;
  const canEditDetails = mock.interviewer.id === currentUserId;
  const canReview = mock.interviewee.id === currentUserId;
  const scoreTone = (score: number | null) =>
    score === null
      ? 'text-muted-foreground'
      : score >= 5
        ? 'text-success'
        : 'text-destructive';
  const hasNotes = (html: string | null | undefined) =>
    Boolean(html && html.replace(/<[^>]*>/g, '').trim());
  const actions = (
    <>
      {canEditDetails ? (
        <EditMockDialog
          mock={mock}
          people={people}
          seasons={seasons}
          onChanged={onChanged}
        />
      ) : null}
      {canCorrectIdentities ? (
        <IdentityCorrectionDialog
          mock={mock}
          people={people}
          seasons={seasons}
          onChanged={onChanged}
        />
      ) : null}
      {canEditDetails ? (
        <NamedConfirmation
          name={mock.interviewee.name}
          actionLabel="Delete interview"
          description="The interview will be soft-deleted. Its immutable version history and audit record will remain."
          onConfirm={onDelete}
        />
      ) : null}
    </>
  );
  return (
    <div className="grid gap-5">
      {canEditDetails || canCorrectIdentities ? (
        <div className="flex flex-wrap items-center justify-end gap-2.5">
          {actions}
        </div>
      ) : null}
      <div className="grid grid-cols-1 gap-px overflow-hidden rounded-lg border bg-border min-[56rem]:grid-cols-3">
        <section className="grid content-start gap-2 bg-card p-4">
          <h2 className="text-sm font-semibold text-muted-foreground">
            Interviewer notes
          </h2>
          <RichTextContent
            html={mock.notes}
            empty="No overall interview notes."
          />
        </section>
        <section className="grid content-start gap-3 bg-card p-4">
          <h2 className="text-sm font-semibold text-muted-foreground">
            Rounds and scores
          </h2>
          {mock.rounds.map((round) => (
            <article key={round.id} className="grid gap-1.5">
              <div className="flex items-baseline justify-between gap-3">
                <strong className="font-semibold">{round.title}</strong>
                <span
                  className={cn(
                    'font-semibold tabular-nums',
                    scoreTone(round.score),
                  )}
                >
                  {round.score === null ? 'Not scored' : `${round.score}/10`}
                </span>
              </div>
              {round.score === null ? null : (
                <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn(
                      'h-full rounded-full',
                      round.score >= 5 ? 'bg-success' : 'bg-destructive',
                    )}
                    style={{ width: `${round.score * 10}%` }}
                  />
                </div>
              )}
              <span className="text-xs text-muted-foreground capitalize">
                {round.type}
              </span>
              {hasNotes(round.notes) ? (
                <RichTextContent html={round.notes} />
              ) : null}
            </article>
          ))}
        </section>
        <section className="grid content-start gap-2 bg-card p-4">
          <h2 className="text-sm font-semibold text-muted-foreground">
            Interviewee review
          </h2>
          <RichTextContent
            html={mock.reviewComments}
            empty="A review has not been submitted."
          />
          {canReview ? (
            <div className="mt-2">
              <RoundReviewDialog mock={mock} onChanged={onChanged} />
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}

function EditMockDialog({
  mock,
  people,
  seasons,
  onChanged,
}: {
  mock: MockInterview;
  people: Person[];
  seasons: Season[];
  onChanged: (changed?: MockInterview) => void;
}) {
  const [open, setOpen] = useState(false);
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
      title="Edit interview details"
      description="Saving creates an immutable interview version."
      open={open}
      onOpenChange={requestOpenChange}
      trigger="Edit details"
    >
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
          <p>Recorded: {formatDateTime(mock.occurredAt)}</p>
          <Field>
            <FieldLabel htmlFor={`edit-duration-${mock.id}`}>
              Duration (minutes)
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
        <RichTextEditor
          id={`edit-notes-${mock.id}`}
          label="Interview notes"
          value={notes}
          onChange={setNotes}
        />
        <FieldSet>
          <FieldLegend variant="label">Rounds and scores</FieldLegend>
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
          <div className="flex flex-wrap items-center gap-2.5">
            {(['leetcode', 'custom', 'behavioural'] as const).map((type) => (
              <Button
                variant="outline"
                type="button"
                key={type}
                onClick={() => setRounds((items) => [...items, newRound(type)])}
              >
                <IconPlus size={17} aria-hidden="true" /> Add {type} round
              </Button>
            ))}
          </div>
        </FieldSet>
        <div className="mt-5 flex justify-end gap-2.5">
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
            {pending ? 'Saving…' : 'Save interview'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}

function RoundReviewDialog({
  mock,
  onChanged,
}: {
  mock: MockInterview;
  onChanged: (changed?: MockInterview) => void;
}) {
  const [open, setOpen] = useState(false);
  const [roundId, setRoundId] = useState(mock.rounds[0]?.id ?? '');
  const selected = mock.rounds.find((round) => round.id === roundId);
  const [comment, setComment] = useState(selected?.intervieweeComment ?? '');
  const [reviewed, setReviewed] = useState(selected?.reviewed ?? true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty =
    comment !== (selected?.intervieweeComment ?? '') ||
    reviewed !== (selected?.reviewed ?? true);
  useBeforeUnload((event) => {
    if (dirty) event.preventDefault();
  });
  const chooseRound = (id: string) => {
    const round = mock.rounds.find((item) => item.id === id);
    setRoundId(id);
    setComment(round?.intervieweeComment ?? '');
    setReviewed(round?.reviewed ?? true);
  };
  const save = async () => {
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: RoundReview = {
          reviewed,
          comment,
        };
        await apiRequest<MockInterviewResult>(
          `/mock-interviews/${encodeURIComponent(mock.id)}/rounds/${encodeURIComponent(roundId)}/review`,
          { method: 'PATCH', body: JSON.stringify(body) },
        );
      }
      onChanged();
      setOpen(false);
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.status === 409) {
        onChanged();
        setError(
          'This interview changed elsewhere. The latest version is being loaded; reopen the review before trying again.',
        );
      } else
        setError(
          requestError instanceof Error
            ? requestError.message
            : 'Unable to save your review.',
        );
    } finally {
      setPending(false);
    }
  };
  const requestOpenChange = (next: boolean) => {
    if (!next && dirty && !window.confirm('Discard your unsaved review?'))
      return;
    if (next) chooseRound(mock.rounds[0]?.id ?? '');
    setOpen(next);
  };
  return (
    <FormDialog
      title="Review interview round"
      description="Only the interviewee can change these review comments."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        mock.reviewStatus === 'reviewed' ? 'Edit my review' : 'Add my review'
      }
    >
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <Field>
          <FieldLabel htmlFor={`review-round-${mock.id}`}>Round</FieldLabel>
          <Select
            items={mock.rounds.map((round) => ({
              label: round.title,
              value: round.id,
            }))}
            value={roundId}
            onValueChange={(value) => {
              if (value) chooseRound(value);
            }}
          >
            <SelectTrigger id={`review-round-${mock.id}`} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {mock.rounds.map((round) => (
                  <SelectItem key={round.id} value={round.id}>
                    {round.title}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <RichTextEditor
          id={`review-comment-${mock.id}`}
          label="Comments"
          value={comment}
          onChange={setComment}
        />
        <Field orientation="horizontal">
          <Checkbox
            id={`review-reviewed-${mock.id}`}
            checked={reviewed}
            onCheckedChange={(checked) => setReviewed(checked)}
          />
          <FieldLabel htmlFor={`review-reviewed-${mock.id}`}>
            Mark this round reviewed
          </FieldLabel>
        </Field>
        <div className="mt-5 flex justify-end gap-2.5">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || !roundId || !dirty}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save review'}
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
}: {
  mock: MockInterview;
  people: Person[];
  seasons: Season[];
  onChanged: (changed?: MockInterview) => void;
}) {
  const candidates = [
    ...new Map(
      [...people, mock.interviewer, mock.interviewee].map((person) => [
        person.id,
        person,
      ]),
    ).values(),
  ];
  const [open, setOpen] = useState(false);
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
      return setError(
        'Explain why these immutable identities need correction.',
      );
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
      title="Correct interview identities"
      description="Director and System Admin corrections are permanently audited. Interviewers cannot use this flow to rewrite ownership."
      open={open}
      onOpenChange={requestOpenChange}
      trigger="Correct identities"
    >
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="grid grid-cols-1 gap-3.5 min-[56rem]:grid-cols-2">
          <Field>
            <FieldLabel htmlFor={`correct-interviewer-${mock.id}`}>
              Interviewer
            </FieldLabel>
            <Select
              items={candidates.map((person) => ({
                label: person.name,
                value: person.id,
              }))}
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
              items={candidates.map((person) => ({
                label: person.name,
                value: person.id,
              }))}
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
            Audited reason
          </FieldLabel>
          <Textarea
            id={`correction-reason-${mock.id}`}
            maxLength={2000}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </Field>
        <div className="mt-5 flex justify-end gap-2.5">
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
            {pending ? 'Correcting…' : 'Save audited correction'}
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
  const [recordingDay, setRecordingDay] = useState(() =>
    calendarDateKey(new Date(), programmeTimezone),
  );
  const [rounds, setRounds] = useState<MockRound[]>(() => [newRound('custom')]);
  const [roundError, setRoundError] = useState('');
  const [roundsDirty, setRoundsDirty] = useState(false);
  const {
    control,
    register,
    handleSubmit,
    reset,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<MockFormValues>({
    resolver: zodResolver(mockSchema as never) as Resolver<MockFormValues>,
    defaultValues: {
      intervieweeId: '',
      occurredAt: formatDateTimeInput(new Date(), programmeTimezone).slice(11),
      durationMinutes: 60,
    },
  });
  useBeforeUnload((event) => {
    if (isDirty || roundsDirty) event.preventDefault();
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
    const mock: MockInterview = {
      id: `mock_local_${Date.now()}`,
      interviewee,
      interviewer,
      season: null,
      occurredAt: zonedDateTimeToUtc(
        `${recordingDay}T${values.occurredAt}`,
        programmeTimezone,
      ),
      durationMinutes: values.durationMinutes,
      notes: '',
      rounds,
      reviewStatus: 'pending',
    };
    if (
      recordingDay !== calendarDateKey(new Date(), programmeTimezone) ||
      new Date(mock.occurredAt).getTime() > Date.now()
    ) {
      setRoundError(
        'Choose a time today in Adelaide. Future times are not allowed. Reopen the form if the day has changed.',
      );
      return;
    }
    try {
      if (!demoMode) {
        const request: MockInterviewCreate = {
          interviewee: { userId: values.intervieweeId },
          occurredAt: mock.occurredAt,
          durationMinutes: values.durationMinutes,
          rounds: rounds.map((round) => ({
            id: round.id,
            type: round.apiType,
            ...(round.problemId ? { problemId: round.problemId } : {}),
            ...(round.notes ? { content: round.notes } : {}),
            ...(round.link ? { link: round.link } : {}),
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
    setRounds([newRound('custom')]);
    setRoundsDirty(false);
    setOpen(false);
  });
  const requestOpen = (next: boolean) => {
    if (
      !next &&
      (isDirty || roundsDirty) &&
      !window.confirm('Discard your unsaved interview?')
    )
      return;
    {
      setRecordingDay(calendarDateKey(new Date(), programmeTimezone));
      reset({
        intervieweeId: '',
        occurredAt: formatDateTimeInput(new Date(), programmeTimezone).slice(
          11,
        ),
        durationMinutes: 60,
      });
      setRounds([newRound('custom')]);
      setRoundsDirty(false);
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
      title="Record mock interview"
      description="You are recorded as the interviewer. Participant identities cannot be changed after creation without an audited correction."
      open={open}
      onOpenChange={requestOpen}
      trigger={
        <>
          <IconPlus size={18} aria-hidden="true" /> New interview
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
              const labels = new Map(
                candidates.map((person) => [
                  person.id,
                  `${person.name}${person.roles.length ? ` · ${person.roles.join(', ')}` : ''}`,
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
                          {labels.get(id)}
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
          <Field>
            <FieldLabel htmlFor="mock-recording-time">
              Time (Adelaide)
            </FieldLabel>
            <Input
              id="mock-recording-time"
              type="time"
              {...register('occurredAt')}
            />
            <FieldDescription>
              Today in Adelaide: {recordingDay}. Record mocks on the day they
              happen.
            </FieldDescription>
            {errors.occurredAt ? (
              <FieldError>{errors.occurredAt.message}</FieldError>
            ) : null}
          </Field>
          <Field>
            <FieldLabel htmlFor="mock-duration">Duration (minutes)</FieldLabel>
            <Input
              id="mock-duration"
              type="number"
              min="15"
              max="240"
              {...register('durationMinutes', { valueAsNumber: true })}
            />
          </Field>
        </div>
        <FieldSet>
          <FieldLegend variant="label">Interview rounds</FieldLegend>
          <FieldDescription>
            Add LeetCode, custom or behavioural rounds. LeetCode rounds capture
            all five scoring criteria.
          </FieldDescription>
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
          <div className="flex flex-wrap items-center gap-2.5">
            {(['leetcode', 'custom', 'behavioural'] as const).map((type) => (
              <Button
                variant="outline"
                type="button"
                key={type}
                onClick={() => {
                  setRoundsDirty(true);
                  setRounds((items) => [...items, newRound(type)]);
                }}
              >
                <IconPlus size={17} aria-hidden="true" /> Add {type} round
              </Button>
            ))}
          </div>
        </FieldSet>
        <div className="mt-5 flex justify-end gap-2.5">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpen(false)}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Saving…' : 'Save interview'}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
}
