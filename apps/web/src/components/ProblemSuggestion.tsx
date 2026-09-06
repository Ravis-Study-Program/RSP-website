import { useState } from 'react';
import { ApiError, apiRequest } from '@/api/client';
import { demoMode, useAttempts, useLeetcodeProblems } from '@/api/queries';
import type { LeetcodeProblem } from '@/api/generated/models';
import {
  activityFilterQuery,
  emptyActivityFilters,
  type ActivityFilters,
} from '@/activityFilters';
import styles from '@/styles/App.module.css';

export function ProblemSuggestion({ filters }: { filters: ActivityFilters }) {
  const catalog = useLeetcodeProblems();
  const history = useAttempts(demoMode);
  const [suggestion, setSuggestion] = useState<LeetcodeProblem | null>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  async function suggest() {
    setPending(true);
    setError('');
    setSuggestion(undefined);
    try {
      if (demoMode) {
        const attempted = new Set(history.data?.items.map((a) => a.problemId));
        const candidates = (catalog.data?.items ?? []).filter(
          (p) =>
            !attempted.has(p.id) &&
            (!filters.difficulties.length ||
              filters.difficulties.includes(p.difficulty)) &&
            (!filters.categories.length ||
              p.categories.some((c) => filters.categories.includes(c))),
        );
        setSuggestion(
          candidates[Math.floor(Math.random() * candidates.length)] ?? null,
        );
      } else {
        const query = activityFilterQuery({
          ...emptyActivityFilters,
          difficulties: filters.difficulties,
          categories: filters.categories,
        });
        const result = await apiRequest<{ problem: LeetcodeProblem | null }>(
          `/me/problem-suggestion?${query.slice(1)}`,
        );
        setSuggestion(result.problem);
      }
    } catch (e) {
      setError(
        e instanceof ApiError
          ? e.message
          : 'Could not suggest a question. Please try again.',
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <div className={styles.panel}>
      <div className={styles.inline}>
        <button
          className={styles.buttonSecondary}
          type="button"
          disabled={
            pending || (demoMode && (catalog.isLoading || history.isLoading))
          }
          onClick={() => void suggest()}
        >
          {pending ? 'Finding a question…' : 'Suggest a question'}
        </button>
        <span className={styles.helper}>
          Uses the selected difficulty and topics and excludes your previous
          attempts from every season.
        </span>
      </div>
      {error ? <p role="alert">{error}</p> : null}
      <div aria-live="polite">
        {suggestion === null ? (
          <p>
            No unattempted questions match. Try another difficulty or topic.
          </p>
        ) : suggestion ? (
          <p>
            <a href={suggestion.link} target="_blank" rel="noreferrer">
              {suggestion.number}. {suggestion.title}
            </a>{' '}
            · {suggestion.difficulty} · {suggestion.categories.join(', ')}
            {suggestion.premium ? ' · LeetCode Premium' : ''}
          </p>
        ) : null}
      </div>
    </div>
  );
}
