import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { IconExternalLink } from '@tabler/icons-react';
import { useState } from 'react';
import { ApiError, apiRequest } from '@/api/client';
import {
  demoMode,
  suggestDemoProblem,
  useLeetcodeCategories,
} from '@/api/queries';
import type { LeetcodeProblem } from '@/api/generated/models';
import { activityFilterQuery, emptyActivityFilters } from '@/activityFilters';
import { Choices } from '@/components/ActivityFilters';
import { problemName } from '@/utils';

const difficultyOptions = ['Easy', 'Medium', 'Hard'].map((label) => ({
  label,
  value: label.toLowerCase(),
}));

/** Suggests an unattempted question. Its filters only narrow the suggestion. */
export function ProblemSuggestion() {
  const categoriesQuery = useLeetcodeCategories();
  const [difficulties, setDifficulties] = useState<string[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const filters = { difficulties, categories };
  const [suggestion, setSuggestion] = useState<LeetcodeProblem | null>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const targeted = difficulties.length + categories.length > 0;
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
          difficulties,
          categories,
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
    <section
      aria-labelledby="question-suggestion-title"
      className="flex flex-col gap-2"
    >
      {/* Sized like a field label, matching "View activity" below. */}
      <h2
        id="question-suggestion-title"
        className="text-sm leading-snug font-medium"
      >
        Question suggestion
      </h2>
      <Card className="gap-0 p-4">
        <div className="flex flex-wrap items-center gap-3">
          <Choices
            label="Difficulty"
            options={difficultyOptions}
            selected={difficulties}
            onChange={setDifficulties}
          />
          <Choices
            label="Topics"
            options={(categoriesQuery.data ?? []).map((label) => ({
              label,
              value: label,
            }))}
            selected={categories}
            onChange={setCategories}
          />
          <Button
            variant="outline"
            type="button"
            disabled={pending}
            onClick={() => void suggest()}
          >
            {pending ? 'Finding a question…' : 'Suggest a question'}
          </Button>
          <span className="min-w-0 text-sm text-muted-foreground">
            Picks a question you have not attempted in any season, using the
            selected difficulty and topics.
          </span>
        </div>
        {targeted ? (
          <p className="mt-3 text-sm text-muted-foreground">
            Targeting a difficulty or topic is generally discouraged: interviews
            will not tell you either in advance, so practise without knowing
            them too.
          </p>
        ) : null}
        {error ? (
          <p role="alert" className="mt-3 text-sm text-destructive">
            {error}
          </p>
        ) : null}
        <div aria-live="polite">
          {suggestion === null ? (
            <p className="mt-4 text-base">
              No unattempted questions match. Try another difficulty or topic.
            </p>
          ) : suggestion ? (
            <p className="mt-4 text-base">
              <a
                className="inline-flex items-center gap-1.5 font-medium underline decoration-[0.08em] underline-offset-[0.2em] hover:decoration-[0.14em]"
                href={suggestion.link}
                target="_blank"
                rel="noreferrer"
              >
                {problemName(suggestion)}
                <IconExternalLink size={16} aria-hidden="true" />
              </a>
              {suggestion.premium ? ' · LeetCode Premium' : ''}
            </p>
          ) : null}
        </div>
      </Card>
    </section>
  );
}
