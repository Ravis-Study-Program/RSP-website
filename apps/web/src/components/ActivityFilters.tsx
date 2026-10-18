import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { IconChevronDown } from '@tabler/icons-react';
import { useState } from 'react';
import { useLeetcodeCategories, useSeasonWeeks } from '@/api/queries';
import { emptyActivityFilters, type ActivityFilters } from '@/activityFilters';
import type { Person } from '@/types';

const clearFilterClass =
  'justify-start px-0 max-[56rem]:basis-full max-[56rem]:self-start';

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
    <Popover>
      <PopoverTrigger
        render={
          <Button
            variant="outline"
            className="h-9 min-w-40 justify-between font-normal"
          />
        }
      >
        {label}
        {selected.length ? ` (${selected.length})` : ': All'}
        <IconChevronDown
          size={16}
          aria-hidden="true"
          className="text-muted-foreground"
        />
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-max min-w-40 max-w-[min(20rem,calc(100vw-2rem))] p-2"
      >
        {options.length > 8 ? (
          <Input
            aria-label={`Search ${label.toLowerCase()}`}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        ) : null}
        <fieldset className="grid max-h-60 gap-1 overflow-auto">
          <legend className="sr-only">{label}</legend>
          {options
            .filter((o) => o.label.toLowerCase().includes(search.toLowerCase()))
            .map((option) => (
              <Label
                className="min-h-9 rounded-md px-2 font-normal hover:bg-muted"
                key={option.value}
              >
                <Checkbox
                  checked={selected.includes(option.value)}
                  onCheckedChange={(checked) =>
                    onChange(
                      checked
                        ? [...selected, option.value]
                        : selected.filter((v) => v !== option.value),
                    )
                  }
                />
                {option.label}
              </Label>
            ))}
          {!options.length ? (
            <p className="px-2 py-1.5 text-sm text-muted-foreground">
              No options in this view.
            </p>
          ) : null}
        </fieldset>
      </PopoverContent>
    </Popover>
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
  const categoriesQuery = useLeetcodeCategories();
  const categories = categoriesQuery.data ?? [];
  return (
    <div
      className="my-3 flex flex-wrap items-end gap-3"
      aria-label="Practice filters"
    >
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
        size="sm"
        type="button"
        className={clearFilterClass}
        onClick={() =>
          onChange({ difficulties: [], categories: [], weeks: [] })
        }
      >
        Clear practice filters
      </Button>
      {categoriesQuery.isError ? (
        <p role="alert">Some filter choices could not be loaded.</p>
      ) : null}
    </div>
  );
}

const resultItems = [
  { label: 'All results', value: '' },
  { label: 'Pass', value: 'true' },
  { label: 'Needs work', value: 'false' },
];

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
    <div
      className="my-3 flex flex-wrap items-end gap-3"
      aria-label="Mock filters"
    >
      <label className="grid gap-2 text-sm font-medium">
        Result
        <Select
          items={resultItems}
          value={filters.passed}
          onValueChange={(value) => onChange({ passed: value ?? '' })}
        >
          <SelectTrigger aria-label="Result" className="min-w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {resultItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
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
        size="sm"
        type="button"
        className={clearFilterClass}
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
