import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
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
        <Input
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
      <SeasonWeekFilter
        seasonId={seasonId}
        filters={filters}
        onChange={onChange}
      />
      <Button
        variant="ghost"
        type="button"
        onClick={() =>
          onChange({ difficulties: [], categories: [], weeks: [] })
        }
      >
        Clear practice filters
      </Button>
      {problems.isError ? (
        <p role="alert">Some filter choices could not be loaded.</p>
      ) : null}
    </div>
  );
}

export function MockFilters({
  seasonId,
  people,
  filters,
  onChange,
}: {
  seasonId?: string;
  people: Person[];
  filters: ActivityFilters;
  onChange: (change: Partial<ActivityFilters>) => void;
}) {
  return (
    <div className={styles.activityFilters} aria-label="Mock filters">
      <label>
        Result{' '}
        <NativeSelect
          className="w-full"
          value={filters.passed}
          onChange={(e) => onChange({ passed: e.target.value })}
        >
          <NativeSelectOption value="">All results</NativeSelectOption>
          <NativeSelectOption value="true">Pass</NativeSelectOption>
          <NativeSelectOption value="false">Needs work</NativeSelectOption>
        </NativeSelect>
      </label>
      <Choices
        label="Interviewers"
        options={people.map((p) => ({ label: p.name, value: p.id }))}
        selected={filters.interviewers}
        onChange={(interviewers) => onChange({ interviewers })}
      />
      <SeasonWeekFilter
        seasonId={seasonId}
        filters={filters}
        onChange={onChange}
      />
      <Button
        variant="ghost"
        type="button"
        onClick={() =>
          onChange({
            passed: emptyActivityFilters.passed,
            interviewers: [],
            weeks: [],
          })
        }
      >
        Clear mock filters
      </Button>
    </div>
  );
}

function SeasonWeekFilter({
  seasonId,
  filters,
  onChange,
}: {
  seasonId?: string;
  filters: ActivityFilters;
  onChange: (change: Partial<ActivityFilters>) => void;
}) {
  const weeks = useSeasonWeeks(seasonId);
  if (!seasonId) return null;
  return (
    <>
      <Choices
        label="Season weeks"
        options={(weeks.data?.items ?? []).map((week) => ({
          label: `Week ${week.number}`,
          value: week.id,
        }))}
        selected={filters.weeks}
        onChange={(weeks) => onChange({ weeks })}
      />
      {weeks.isError ? (
        <p role="alert">Season weeks could not be loaded.</p>
      ) : null}
    </>
  );
}
