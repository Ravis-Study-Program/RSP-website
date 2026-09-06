import { useState } from 'react';
import { useLeetcodeProblems, useSeasonWeeks } from '@/api/queries';
import { emptyActivityFilters, type ActivityFilters } from '@/activityFilters';
import type { Person } from '@/types';
import styles from '@/styles/App.module.css';

function Choices({
  label,
  options,
  selected,
  onChange,
}: {
  label: string;
  options: { value: string; label: string }[];
  selected: string[];
  onChange: (values: string[]) => void;
}) {
  const [search, setSearch] = useState('');
  return (
    <details className={styles.filterChoices}>
      <summary>
        {label}
        {selected.length ? ` (${selected.length})` : ': All'}
      </summary>
      {options.length > 8 ? (
        <input
          className={styles.input}
          aria-label={`Search ${label.toLowerCase()}`}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      ) : null}
      <fieldset className={styles.filterOptions}>
        <legend className={styles.visuallyHidden}>{label}</legend>
        {options
          .filter((o) => o.label.toLowerCase().includes(search.toLowerCase()))
          .map((option) => (
            <label className={styles.inline} key={option.value}>
              <input
                type="checkbox"
                checked={selected.includes(option.value)}
                onChange={(e) =>
                  onChange(
                    e.target.checked
                      ? [...selected, option.value]
                      : selected.filter((v) => v !== option.value),
                  )
                }
              />
              {option.label}
            </label>
          ))}
        {!options.length ? <span>No options in this view.</span> : null}
      </fieldset>
    </details>
  );
}

export function PracticeFilters({
  seasonId,
  filters,
  onChange,
}: {
  seasonId?: string;
  filters: ActivityFilters;
  onChange: (change: Partial<ActivityFilters>) => void;
}) {
  const problems = useLeetcodeProblems();
  const weeks = useSeasonWeeks(seasonId);
  const categories = [
    ...new Set((problems.data?.items ?? []).flatMap((p) => p.categories)),
  ].sort();
  return (
    <div className={styles.activityFilters} aria-label="Practice filters">
      <Choices
        label="Difficulty"
        options={['Easy', 'Medium', 'Hard'].map((label) => ({
          label,
          value: label.toLowerCase(),
        }))}
        selected={filters.difficulties}
        onChange={(difficulties) => onChange({ difficulties })}
      />
      <Choices
        label="Topics"
        options={categories.map((label) => ({ label, value: label }))}
        selected={filters.categories}
        onChange={(categories) => onChange({ categories })}
      />
      {seasonId ? (
        <Choices
          label="Season weeks"
          options={(weeks.data?.items ?? []).map((w) => ({
            label: `Week ${w.number}`,
            value: w.id,
          }))}
          selected={filters.weeks}
          onChange={(weeks) => onChange({ weeks })}
        />
      ) : null}
      <button
        className={styles.buttonQuiet}
        type="button"
        onClick={() =>
          onChange({ difficulties: [], categories: [], weeks: [] })
        }
      >
        Clear practice filters
      </button>
      {problems.isError || weeks.isError ? (
        <p role="alert">Some filter choices could not be loaded.</p>
      ) : null}
    </div>
  );
}

export function MockFilters({
  people,
  filters,
  onChange,
}: {
  people: Person[];
  filters: ActivityFilters;
  onChange: (change: Partial<ActivityFilters>) => void;
}) {
  return (
    <div className={styles.activityFilters} aria-label="Mock filters">
      <label>
        Result{' '}
        <select
          className={styles.select}
          value={filters.passed}
          onChange={(e) => onChange({ passed: e.target.value })}
        >
          <option value="">All results</option>
          <option value="true">Pass</option>
          <option value="false">Needs work</option>
        </select>
      </label>
      <Choices
        label="Interviewers"
        options={people.map((p) => ({ label: p.name, value: p.id }))}
        selected={filters.interviewers}
        onChange={(interviewers) => onChange({ interviewers })}
      />
      <button
        className={styles.buttonQuiet}
        type="button"
        onClick={() =>
          onChange({ passed: emptyActivityFilters.passed, interviewers: [] })
        }
      >
        Clear mock filters
      </button>
    </div>
  );
}
