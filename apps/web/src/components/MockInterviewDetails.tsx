import { useState } from 'react';
import { IconInfoCircle } from '@tabler/icons-react';
import { ApiError, apiRequest } from '@/api/client';
import { withRoundReview } from '@/api/adapters';
import { demoMode } from '@/api/queries';
import type { RoundReview, MockInterviewResult } from '@/api/generated/models';
import { ExternalLink } from '@/components/ExternalLink';
import { RichTextContent } from '@/components/RichTextContent';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldLabel } from '@/components/ui/field';
import { cn } from '@/lib/utils';
import {
  orderedScores,
  scoreColor,
  scoreHelp,
  scoreLabel,
} from '@/mockInterviewScores';
import type { MockInterview, MockRound } from '@/types';
import { problemName } from '@/utils';

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
export function RoundTitle({ round }: { round: MockRound }) {
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
      <dt className="truncate text-muted-foreground">Reference link</dt>
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

/**
 * Saves the reviewed state of some rounds, keeping each round's comment, and
 * reports the interview after each saved round so progress shows at once.
 */
function useRoundReviewToggle(
  mock: MockInterview,
  onChanged?: (changed?: MockInterview) => void,
) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const setReviewed = async (roundIds: string[], reviewed: boolean) => {
    if (!onChanged) return;
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
        onChanged?.();
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
      if (next !== mock) onChanged?.(next);
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
export function MockReviewProgress({
  mock,
  canReview,
  onChanged,
}: {
  mock: MockInterview;
  canReview: boolean;
  onChanged?: (changed?: MockInterview) => void;
}) {
  const { setReviewed, pending, error } = useRoundReviewToggle(mock, onChanged);
  const editable = canReview && Boolean(onChanged);
  const total = mock.rounds.length;
  const reviewed = mock.rounds.filter((round) => round.reviewed).length;
  const complete = reviewed === total;
  const progress = `${reviewed} of ${total} rounds reviewed`;
  return (
    <div className="grid gap-1">
      <Checkbox
        aria-label={
          editable
            ? `${complete ? 'Mark every round unreviewed' : 'Mark every round reviewed'} (${progress})`
            : progress
        }
        title={progress}
        checked={complete}
        indeterminate={!complete && reviewed > 0}
        disabled={!editable || pending}
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

/** An info icon whose hover text says what a score is for. */
function ScoreHelp({ help, size = 15 }: { help?: string; size?: number }) {
  if (!help) return null;
  return (
    <button
      type="button"
      title={help}
      aria-label={help}
      className="inline-flex rounded-sm text-muted-foreground outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <IconInfoCircle size={size} aria-hidden="true" />
    </button>
  );
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
  onChanged?: (changed?: MockInterview) => void;
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
          Interviewer notes
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
            Interviewee notes
          </h3>
          <RichTextContent
            html={mock.intervieweeNotes}
            empty="No notes from the interviewee."
          />
        </div>
      ) : null}
      <div className="flex items-center gap-2 self-stretch min-[56rem]:items-start min-[56rem]:border-l min-[56rem]:pt-0.5 min-[56rem]:pl-4">
        <MockReviewProgress
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
export function MockInterviewDetails({
  mock,
  viewerId,
  onChanged,
}: {
  mock: MockInterview;
  viewerId?: string;
  onChanged?: (changed?: MockInterview) => void;
}) {
  const canReview = Boolean(onChanged) && mock.interviewee.id === viewerId;
  // The interviewee's notes are private from the interviewer.
  const showIntervieweeNotes =
    Boolean(viewerId) && mock.interviewer.id !== viewerId;
  const rowClass = roundRowClass(showIntervieweeNotes);
  const { setReviewed, pending, error } = useRoundReviewToggle(mock, onChanged);
  const headingClass = 'text-sm font-semibold text-muted-foreground';
  // Column headings show on wide screens; stacked rows label each part instead.
  const cellHeadingClass =
    'text-xs font-semibold text-muted-foreground min-[56rem]:sr-only';
  return (
    <div className="overflow-hidden rounded-lg border bg-card">
      <section aria-label="Rounds and review" className="min-w-0">
        <div className={cn(rowClass, 'hidden border-b py-3 min-[56rem]:grid')}>
          <h2 className={headingClass}>Rounds and scores</h2>
          <h2 className={cn(headingClass, 'pl-4')}>Interviewer notes</h2>
          {showIntervieweeNotes ? (
            <h2 className={cn(headingClass, 'pl-4')}>Interviewee notes</h2>
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
                <h3 className={cellHeadingClass}>Interviewer notes</h3>
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
                  <h3 className={cellHeadingClass}>Interviewee notes</h3>
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
