import { useActivityFilters, filterMocks } from '@/activityFilters';
import { MockFilters } from '@/components/ActivityFilters';
import { programmeTimezone, adelaideYear } from '@/activityDates';
import { ActivityYearFilter } from '@/components/ActivityYearFilter';
import { zodResolver } from '@hookform/resolvers/zod';
import { Tabs } from '@base-ui/react/tabs';
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
import { useForm } from 'react-hook-form';
import { useBeforeUnload, useParams } from 'react-router-dom';
import { z } from 'zod';

import { adaptCurrentPerson, adaptMockInterview } from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import type {
  LeetcodeProblem,
  MockIdentityCorrection,
  MockInterviewCreate,
  MockInterviewMutation,
  MockInterviewResult,
  RoundReview,
} from '@/api/generated/models';
import {
  demoMode,
  useCurrentUser,
  useLeetcodeProblems,
  useMockInterviews,
  useMockParticipants,
  useSeasons,
} from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ProblemSelect } from '@/components/ProblemSelect';
import { RichTextEditor } from '@/components/RichTextEditor';
import { RichTextContent } from '@/components/RichTextContent';
import styles from '@/styles/App.module.css';
import type { MockInterview, MockRound, Person, Season } from '@/types';
import {
  calendarDateKey,
  formatDateTime,
  formatDateTimeInput,
  interviewPassed,
  zonedDateTimeToUtc,
} from '@/utils';

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
  problems: LeetcodeProblem[],
) {
  return {
    users: new Map(
      [...people, mock.interviewer, mock.interviewee].map((person) => [
        person.id,
        person,
      ]),
    ),
    seasons: new Map(seasons.map((season) => [season.id, season])),
    problems: new Map(problems.map((problem) => [problem.id, problem])),
  };
}

function RoundEditor({
  round,
  index,
  total,
  problems,
  onChange,
  onRemove,
  onMove,
}: {
  round: MockRound;
  index: number;
  total: number;
  problems: LeetcodeProblem[];
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
      link: undefined,
      score: 5,
      scores: scoresForType(apiType),
    });
  return (
    <article className={styles.round}>
      <div className={styles.roundHeader}>
        <strong>Round {index + 1}</strong>
        <div className={styles.inline}>
          <button
            className={styles.iconButton}
            type="button"
            aria-label={`Move round ${index + 1} up`}
            disabled={index === 0}
            onClick={() => onMove(-1)}
          >
            <IconArrowUp size={16} aria-hidden="true" />
          </button>
          <button
            className={styles.iconButton}
            type="button"
            aria-label={`Move round ${index + 1} down`}
            disabled={index === total - 1}
            onClick={() => onMove(1)}
          >
            <IconArrowDown size={16} aria-hidden="true" />
          </button>
          {total > 1 ? (
            <button
              className={styles.buttonQuiet}
              type="button"
              onClick={onRemove}
            >
              <IconTrash size={16} aria-hidden="true" /> Remove
            </button>
          ) : null}
        </div>
      </div>
      <div className={styles.fieldGrid}>
        <div className={styles.field}>
          <label htmlFor={`round-type-${round.id}`}>Round type</label>
          <select
            id={`round-type-${round.id}`}
            className={styles.select}
            value={round.apiType}
            onChange={(event) =>
              changeType(event.target.value as MockRound['apiType'])
            }
          >
            <option value="leetcode">LeetCode</option>
            <option value="custom">Custom</option>
            <option value="behavioural">Behavioural</option>
          </select>
        </div>
        {round.apiType === 'leetcode' ? (
          <div className={styles.field}>
            <label htmlFor={`round-problem-${round.id}`}>
              LeetCode problem
            </label>
            <ProblemSelect
              id={`round-problem-${round.id}`}
              problems={problems}
              value={round.problemId ?? ''}
              onChange={(problemId) => {
                const problem = problems.find((p) => p.id === problemId);
                onChange({
                  ...round,
                  problemId: problemId || undefined,
                  title: problem?.title ?? 'Choose a LeetCode problem',
                  link: problem?.link,
                });
              }}
            />
          </div>
        ) : null}
        {round.apiType === 'custom' ? (
          <div className={styles.field}>
            <label htmlFor={`round-link-${round.id}`}>
              Reference link (optional)
            </label>
            <input
              id={`round-link-${round.id}`}
              className={styles.input}
              type="url"
              value={round.link ?? ''}
              onChange={(event) =>
                onChange({ ...round, link: event.target.value || undefined })
              }
            />
          </div>
        ) : null}
        {Object.entries(round.scores).map(([criterion, score]) => (
          <div className={styles.field} key={criterion}>
            <label htmlFor={`round-score-${round.id}-${criterion}`}>
              {scoreLabel(criterion)} score (0–10)
            </label>
            <input
              id={`round-score-${round.id}-${criterion}`}
              className={styles.input}
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
          </div>
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
      <button
        className={styles.iconButton}
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
      </button>
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
      <span className={styles.score}>
        {row.original.rounds.map((round) => round.score ?? '—').join(' · ')}/10
      </span>
    ),
  },
  {
    id: 'result',
    header: 'Result',
    accessorFn: interviewPassed,
    cell: ({ row }) => (
      <span
        className={
          interviewPassed(row.original)
            ? styles.badgeSuccess
            : styles.badgeDanger
        }
      >
        {interviewPassed(row.original) ? 'Pass' : 'Needs work'}
      </span>
    ),
  },
  {
    accessorKey: 'reviewStatus',
    header: 'Review',
    cell: ({ getValue }) => (
      <span
        className={
          getValue() === 'reviewed' ? styles.badgeSuccess : styles.badgeWarning
        }
      >
        {getValue() === 'reviewed' ? 'Reviewed' : 'Pending'}
      </span>
    ),
  },
];

type ViewMode = 'received' | 'given' | 'all';

export function MockInterviewsPage() {
  usePageTitle('Mock interviews');
  const { slug } = useParams();
  const [year, setYear] = useState<number>();
  const user = useCurrentUser();
  const peopleQuery = useMockParticipants();
  const seasonsQuery = useSeasons();
  const season = seasonsQuery.data?.items.find((item) => item.slug === slug);
  const columns = useMemo(() => columnsForSeason(season?.id), [season?.id]);
  const [filters, setFilters] = useActivityFilters();
  const query = useMockInterviews(
    !slug || Boolean(season),
    season?.id,
    year,
    undefined,
    filters,
  );
  const [mode, setMode] = useState<ViewMode>('received');
  const [created, setCreated] = useState<MockInterview[]>([]);
  const [updated, setUpdated] = useState<Record<string, MockInterview>>({});
  const [removedIds, setRemovedIds] = useState<string[]>([]);
  const all = useMemo(
    () =>
      [
        ...new Map(
          [
            ...(query.data?.items ?? []),
            ...created,
            ...Object.values(updated),
          ].map((mock) => [mock.id, mock]),
        ).values(),
      ]
        .sort(
          (left, right) =>
            new Date(right.occurredAt).getTime() -
            new Date(left.occurredAt).getTime(),
        )
        .filter((mock) => !removedIds.includes(mock.id))
        .map((mock) => updated[mock.id] ?? mock)
        .filter(
          (mock) =>
            (!year || adelaideYear(mock.occurredAt) === year) &&
            (!season ||
              (demoMode
                ? mock.season === season.name
                : mock.seasonId === season.id)),
        ),
    [created, query.data, removedIds, updated, year, season],
  );
  const filtered = useMemo(() => {
    const matches = filterMocks(all, filters);
    if (mode === 'received')
      return matches.filter((mock) => mock.interviewee.id === user.data?.id);
    if (mode === 'given')
      return matches.filter((mock) => mock.interviewer.id === user.data?.id);
    return matches;
  }, [all, filters, mode, user.data?.id]);
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

  return (
    <div className={styles.page}>
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
      <ActivityYearFilter year={year} onChange={setYear} />
      <MockFilters
        people={availablePeople}
        filters={filters}
        onChange={setFilters}
      />
      <Tabs.Root
        value={mode}
        onValueChange={(value) => setMode(value as ViewMode)}
      >
        <Tabs.List className={styles.tabsList} aria-label="Mock interview view">
          <Tabs.Tab className={styles.tab} value="received">
            Received
          </Tabs.Tab>
          <Tabs.Tab className={styles.tab} value="given">
            Given
          </Tabs.Tab>
          <Tabs.Tab className={styles.tab} value="all">
            All available
          </Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value={mode}>
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
                <div className={styles.inline}>
                  <span
                    className={
                      interviewPassed(row.original)
                        ? styles.badgeSuccess
                        : styles.badgeDanger
                    }
                  >
                    {interviewPassed(row.original) ? 'Pass' : 'Needs work'}
                  </span>
                  <span
                    className={
                      row.original.reviewStatus === 'reviewed'
                        ? styles.badgeSuccess
                        : styles.badgeWarning
                    }
                  >
                    {row.original.reviewStatus}
                  </span>
                </div>
                <h3 className={`${styles.cardTitle} ${styles.cardTitleSpaced}`}>
                  <PersonIdentity
                    person={row.original.interviewee}
                    seasonId={season?.id}
                  />
                </h3>
                <p className={styles.helper}>
                  with {row.original.interviewer.name} ·{' '}
                  {formatDateTime(row.original.occurredAt)}
                </p>
                <p>
                  {row.original.durationMinutes} minutes ·{' '}
                  {row.original.rounds.length}{' '}
                  {row.original.rounds.length === 1 ? 'round' : 'rounds'}
                </p>
                <button
                  className={styles.buttonSecondary}
                  type="button"
                  aria-expanded={row.getIsExpanded()}
                  onClick={row.getToggleExpandedHandler()}
                >
                  View details
                </button>
                {row.getIsExpanded() ? (
                  <div className={styles.expandedDetails}>
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
        </Tabs.Panel>
      </Tabs.Root>
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
  return (
    <div className={styles.grid2}>
      <div>
        <h3 className={styles.cardTitle}>Rounds and scores</h3>
        <RichTextContent
          html={mock.notes}
          empty="No overall interview notes."
        />
        <div className={`${styles.rounds} ${styles.roundsSpaced}`}>
          {mock.rounds.map((round) => (
            <article className={styles.round} key={round.id}>
              <div className={styles.roundHeader}>
                <strong>{round.title}</strong>
                <span
                  className={`${styles.score} ${round.score === null ? '' : round.score >= 5 ? styles.pass : styles.fail}`}
                >
                  {round.score === null ? 'Not scored' : `${round.score}/10`}
                </span>
              </div>
              <span className={styles.badgeNeutral}>{round.type}</span>
              <RichTextContent html={round.notes} />
            </article>
          ))}
        </div>
      </div>
      <div>
        <h3 className={styles.cardTitle}>Interviewee review</h3>
        <RichTextContent
          html={mock.reviewComments}
          empty="A review has not been submitted."
        />
        <div className={styles.buttonRow}>
          {canReview ? (
            <RoundReviewDialog mock={mock} onChanged={onChanged} />
          ) : null}
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
        </div>
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
  const problems = useLeetcodeProblems();
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
          interviewContext(mock, people, seasons, problems.data?.items ?? []),
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
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.fieldGrid}>
          <p>Recorded: {formatDateTime(mock.occurredAt)}</p>
          <div className={styles.field}>
            <label htmlFor={`edit-duration-${mock.id}`}>
              Duration (minutes)
            </label>
            <input
              id={`edit-duration-${mock.id}`}
              className={styles.input}
              type="number"
              min="1"
              value={durationMinutes}
              onChange={(event) =>
                setDurationMinutes(event.target.valueAsNumber)
              }
            />
          </div>
        </div>
        <RichTextEditor
          id={`edit-notes-${mock.id}`}
          label="Interview notes"
          value={notes}
          onChange={setNotes}
        />
        <fieldset className={`${styles.form} ${styles.unstyledFieldset}`}>
          <legend className={styles.legend}>Rounds and scores</legend>
          {rounds.map((round, roundIndex) => (
            <RoundEditor
              key={round.id}
              round={round}
              index={roundIndex}
              total={rounds.length}
              problems={problems.data?.items ?? []}
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
          <div className={styles.buttonRow}>
            {(['leetcode', 'custom', 'behavioural'] as const).map((type) => (
              <button
                className={styles.buttonSecondary}
                type="button"
                key={type}
                onClick={() => setRounds((items) => [...items, newRound(type)])}
              >
                <IconPlus size={17} aria-hidden="true" /> Add {type} round
              </button>
            ))}
          </div>
        </fieldset>
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
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
          </button>
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
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor={`review-round-${mock.id}`}>Round</label>
          <select
            id={`review-round-${mock.id}`}
            className={styles.select}
            value={roundId}
            onChange={(event) => chooseRound(event.target.value)}
          >
            {mock.rounds.map((round) => (
              <option key={round.id} value={round.id}>
                {round.title}
              </option>
            ))}
          </select>
        </div>
        <RichTextEditor
          id={`review-comment-${mock.id}`}
          label="Comments"
          value={comment}
          onChange={setComment}
        />
        <label className={styles.checkLabel}>
          <input
            type="checkbox"
            checked={reviewed}
            onChange={(event) => setReviewed(event.target.checked)}
          />{' '}
          Mark this round reviewed
        </label>
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
            type="button"
            disabled={pending || !roundId || !dirty}
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save review'}
          </button>
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
  const problems = useLeetcodeProblems();
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
            interviewContext(
              mock,
              candidates,
              seasons,
              problems.data?.items ?? [],
            ),
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
      <div className={styles.form}>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.fieldGrid}>
          <div className={styles.field}>
            <label htmlFor={`correct-interviewer-${mock.id}`}>
              Interviewer
            </label>
            <select
              id={`correct-interviewer-${mock.id}`}
              className={styles.select}
              value={interviewerId}
              onChange={(event) => setInterviewerId(event.target.value)}
            >
              {candidates.map((person) => (
                <option value={person.id} key={person.id}>
                  {person.name}
                </option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label htmlFor={`correct-interviewee-${mock.id}`}>
              Interviewee
            </label>
            <select
              id={`correct-interviewee-${mock.id}`}
              className={styles.select}
              value={intervieweeId}
              onChange={(event) => setIntervieweeId(event.target.value)}
            >
              {candidates.map((person) => (
                <option value={person.id} key={person.id}>
                  {person.name}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className={styles.field}>
          <label htmlFor={`correction-reason-${mock.id}`}>Audited reason</label>
          <textarea
            id={`correction-reason-${mock.id}`}
            className={styles.textarea}
            maxLength={2000}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
            type="button"
            disabled={pending || !reason.trim()}
            onClick={() => void save()}
          >
            {pending ? 'Correcting…' : 'Save audited correction'}
          </button>
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
  const problems = useLeetcodeProblems();
  const [open, setOpen] = useState(false);
  const [recordingDay, setRecordingDay] = useState(() =>
    calendarDateKey(new Date(), programmeTimezone),
  );
  const [rounds, setRounds] = useState<MockRound[]>(() => [newRound('custom')]);
  const [roundError, setRoundError] = useState('');
  const [roundsDirty, setRoundsDirty] = useState(false);
  const {
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
            problems: new Map(
              (problems.data?.items ?? []).map((problem) => [
                problem.id,
                problem,
              ]),
            ),
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
      <form className={styles.form} noValidate onSubmit={onSubmit}>
        <div className={styles.field}>
          <label htmlFor="mock-interviewee">Interviewee</label>
          <select
            id="mock-interviewee"
            className={styles.select}
            {...register('intervieweeId')}
          >
            <option value="">Choose an eligible participant</option>
            {people
              .filter((person) => person.id !== interviewer?.id)
              .map((person) => (
                <option key={person.id} value={person.id}>
                  {person.name}
                  {person.roles.length ? ` · ${person.roles.join(', ')}` : ''}
                </option>
              ))}
          </select>
          {errors.intervieweeId ? (
            <p className={styles.fieldError}>{errors.intervieweeId.message}</p>
          ) : null}
        </div>
        <div className={styles.fieldGrid}>
          <div className={styles.field}>
            <label htmlFor="mock-recording-time">Time (Adelaide)</label>
            <input
              id="mock-recording-time"
              className={styles.input}
              type="time"
              {...register('occurredAt')}
            />
            <p className={styles.helper}>
              Today in Adelaide: {recordingDay}. Record mocks on the day they
              happen.
            </p>
            {errors.occurredAt ? (
              <p className={styles.fieldError}>{errors.occurredAt.message}</p>
            ) : null}
          </div>
          <div className={styles.field}>
            <label htmlFor="mock-duration">Duration (minutes)</label>
            <input
              id="mock-duration"
              className={styles.input}
              type="number"
              min="15"
              max="240"
              {...register('durationMinutes', { valueAsNumber: true })}
            />
          </div>
        </div>
        <fieldset className={`${styles.form} ${styles.unstyledFieldset}`}>
          <legend className={styles.legend}>Interview rounds</legend>
          <p className={styles.helper}>
            Add LeetCode, custom or behavioural rounds. LeetCode rounds capture
            all five scoring criteria.
          </p>
          {roundError ? (
            <p className={styles.fieldError} role="alert">
              {roundError}
            </p>
          ) : null}
          {rounds.map((round, roundIndex) => (
            <RoundEditor
              key={round.id}
              round={round}
              index={roundIndex}
              total={rounds.length}
              problems={problems.data?.items ?? []}
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
          <div className={styles.buttonRow}>
            {(['leetcode', 'custom', 'behavioural'] as const).map((type) => (
              <button
                className={styles.buttonSecondary}
                type="button"
                key={type}
                onClick={() => {
                  setRoundsDirty(true);
                  setRounds((items) => [...items, newRound(type)]);
                }}
              >
                <IconPlus size={17} aria-hidden="true" /> Add {type} round
              </button>
            ))}
          </div>
        </fieldset>
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={() => requestOpen(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting ? 'Saving…' : 'Save interview'}
          </button>
        </div>
      </form>
    </FormDialog>
  );
}
