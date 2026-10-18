import { Button } from '@/components/ui/button';
import { useState } from 'react';
import { ApiError, apiRequest } from '@/api/client';
import { demoMode, suggestDemoProblem } from '@/api/queries';
import type { LeetcodeProblem } from '@/api/generated/models';
import {
  activityFilterQuery,
  emptyActivityFilters,
  type ActivityFilters,
} from '@/activityFilters';

export function ProblemSuggestion({ filters }: { filters: ActivityFilters }) {
  const [suggestion, setSuggestion] = useState<LeetcodeProblem | null>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  async function suggest() {
    setPending(true);
    setError('');
    setSuggestion(undefined);
    try {
      if (demoMode) {
        setSuggestion(suggestDemoProblem(filters));
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
    <div className="mb-4 flex flex-wrap items-center gap-3">
      <Button
        variant="outline"
        type="button"
        disabled={pending}
        onClick={() => void suggest()}
      >
        {pending ? 'Finding a question…' : 'Suggest a question'}
      </Button>
      <span className="min-w-0 text-sm text-muted-foreground">
        Uses the selected difficulty and topics and excludes your previous
        attempts from every season.
      </span>
      {error ? (
        <p role="alert" className="basis-full text-sm text-destructive">
          {error}
        </p>
      ) : null}
      <div aria-live="polite" className="basis-full">
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
