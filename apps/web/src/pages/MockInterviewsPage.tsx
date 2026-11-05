import { MockInterviewHistory } from '@/components/MockInterviewHistory';
import {
  MockInterviewViewTabs,
  MockInterviewFilterControls,
} from '@/components/MockInterviewFilters';
import { RoundTitle } from '@/components/MockInterviewDetails';
import { leetcodeScoreKeys, scoreLabel } from '@/mockInterviewScores';
import type { MockInterviewMode } from '@/api/queries';
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
import { programmeTimezone } from '@/activityDates';
import { ActivityPeriodFilter } from '@/components/ActivityPeriodFilter';
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
import type { Row } from '@tanstack/react-table';
import type { Resolver } from 'react-hook-form';
import { useMemo, useState } from 'react';
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
import { PageHeader, usePageTitle } from '@/components/Common';
import { AppDatePicker } from '@/components/AppDatePicker';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ProblemSelect } from '@/components/ProblemSelect';
import { ErrorState } from '@/components/StatusViews';
import { RichTextEditor } from '@/components/RichTextEditor';
import { Checkbox } from '@/components/ui/checkbox';
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
import { calendarDateKey, formatDate, zonedDateTimeToUtc } from '@/utils';

const mockSchema = z.object({
  intervieweeId: z.string().min(1, 'Choose an interviewee.'),
  occurredAt: z.string().min(1, 'Choose a time.'),
  durationMinutes: z.number().int().min(15).max(240),
});
type MockFormValues = z.infer<typeof mockSchema>;

/**
 * Mocks are dated, not timed. Today's interview keeps the current time so it
 * is never in the future; earlier days use noon in Adelaide, like attempts.
 */
function recordedAt(date: string) {
  return date === calendarDateKey(new Date(), programmeTimezone)
    ? new Date().toISOString()
    : zonedDateTimeToUtc(`${date}T12:00`, programmeTimezone);
}

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
            <Button variant="ghost" type="button" onClick={onRemove}>
              <IconTrash size={16} aria-hidden="true" /> Remove
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
        label="Notes"
        value={round.notes}
        onChange={(notes) => onChange({ ...round, notes })}
      />
    </article>
  );
}

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
  const [mode, setMode] = useState<MockInterviewMode>('all');
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
  const actionsCell = (row: Row<MockInterview>) => (
    <MockRowActions
      row={row}
      canReview={row.original.interviewee.id === user.data?.id}
      canEdit={row.original.interviewer.id === user.data?.id}
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
        <MockInterviewHistory
          toolbarStart={
            <MockInterviewViewTabs mode={mode} onModeChange={setMode} />
          }
          ariaLabel={`${mode} mock interviews`}
          data={filtered}
          extraFilters={{
            content: (
              <>
                <ActivityPeriodFilter
                  label="Season"
                  section="mock-interviews"
                  season={season}
                  seasons={periodSeasons}
                />
                <MockInterviewFilterControls
                  mode={mode}
                  onModeChange={setMode}
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
            mode === 'all' ? 'No mock interviews' : `No ${mode} mock interviews`
          }
          emptyMessage={
            mode === 'received'
              ? 'Interviews where you are the interviewee will appear here.'
              : mode === 'given'
                ? 'Interviews you conduct will appear here.'
                : 'Interviews you give or receive will appear here.'
          }
          seasonId={season?.id}
          subjectId={user.data?.id}
          viewerId={user.data?.id}
          mode={mode}
          onChanged={interviewChanged}
          renderActions={actionsCell}
        />
      </div>
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
            {row.getIsExpanded() ? 'Hide details' : 'Show details'}
          </DropdownMenuItem>
          {canReview ? (
            <DropdownMenuItem onClick={() => setOpen('review')}>
              <IconEdit size={17} aria-hidden="true" /> Edit notes
            </DropdownMenuItem>
          ) : null}
          {canEdit ? (
            <DropdownMenuItem onClick={() => setOpen('edit')}>
              <IconEdit size={17} aria-hidden="true" /> Edit notes and scores
            </DropdownMenuItem>
          ) : null}
          {canCorrectIdentities ? (
            <DropdownMenuItem onClick={() => setOpen('identities')}>
              <IconUsers size={17} aria-hidden="true" /> Correct identities
            </DropdownMenuItem>
          ) : null}
          {canEdit ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('delete')}
            >
              <IconTrash size={17} aria-hidden="true" /> Delete interview
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
          actionLabel="Delete interview"
          description="This removes the interview from your history. Earlier versions and the record of changes are retained."
          open={open === 'delete'}
          onOpenChange={close}
          onConfirm={onDelete}
        />
      ) : null}
    </>
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
      title="Edit notes and scores"
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
              readOnly
              aria-readonly="true"
              className="text-muted-foreground"
            />
          </Field>
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
        <div className="border-t pt-4">
          <RichTextEditor
            id={`edit-notes-${mock.id}`}
            label="Overall notes"
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
            {pending ? 'Saving…' : 'Save changes'}
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
    <FormDialog title="Edit notes" open={open} onOpenChange={requestOpenChange}>
      <div className="grid gap-4">
        {error ? <FieldError>{error}</FieldError> : null}
        <RichTextEditor
          id={`interviewee-notes-${mock.id}`}
          label="Overall notes"
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
        <div className="flex justify-end gap-2.5 border-t border-border pt-4">
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
            {pending ? 'Saving…' : 'Save notes'}
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
      title="Correct interview identities"
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
  const [rounds, setRounds] = useState<MockRound[]>(() => [newRound('custom')]);
  const [roundError, setRoundError] = useState('');
  const [roundsDirty, setRoundsDirty] = useState(false);
  const [notes, setNotes] = useState('');
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
      occurredAt: calendarDateKey(new Date(), programmeTimezone),
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
    const mock: MockInterview = {
      id: `mock_local_${Date.now()}`,
      interviewee,
      interviewer,
      season: null,
      occurredAt: recordedAt(values.occurredAt),
      durationMinutes: values.durationMinutes,
      notes,
      intervieweeNotes: '',
      rounds,
      reviewStatus: 'pending',
    };
    if (values.occurredAt > calendarDateKey(new Date(), programmeTimezone)) {
      setRoundError('Choose a date that has already happened.');
      return;
    }
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
    setRounds([newRound('custom')]);
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
        occurredAt: calendarDateKey(new Date(), programmeTimezone),
        durationMinutes: 60,
      });
      setRounds([newRound('custom')]);
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
      title="New mock interview"
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
        <div className="border-t pt-4">
          <RichTextEditor
            id="new-mock-notes"
            label="Overall notes"
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
        <div className="mt-5 flex justify-end gap-2.5">
          <Button
            variant="outline"
            type="button"
            onClick={() => requestOpen(false)}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Creating…' : 'Create interview'}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
}
