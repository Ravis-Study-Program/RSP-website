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
import { useId, useState } from 'react';
import { useSeasonWeeks } from '@/api/queries';
import { type ActivityFilters } from '@/activityFilters';
import type { Person } from '@/types';
import { Field, FieldLabel } from '@/components/ui/field';

export function Choices({
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

/** Season weeks as a multi-select dropdown, for use inside another popover. */
export function SeasonWeekChoices({
  seasonId,
  selected,
  onChange,
}: {
  seasonId: string;
  selected: string[];
  onChange: (weeks: string[]) => void;
}) {
  const weeks = useSeasonWeeks(seasonId);
  const selectId = useId();
  // The URL updates in a transition, so quick successive picks would build on
  // a stale `selected`. Track picks locally and resync when the URL changes.
  const [value, setValue] = useState(selected);
  const [synced, setSynced] = useState(selected);
  if (synced !== selected) {
    setSynced(selected);
    setValue(selected);
  }
  const items = (weeks.data?.items ?? []).map((week) => ({
    label: `Week ${week.number}`,
    value: week.id,
  }));
  return (
    <Field>
      <FieldLabel htmlFor={selectId}>Season weeks</FieldLabel>
      <Select
        multiple
        items={items}
        value={value}
        onValueChange={(next) => {
          setValue(next);
          onChange(next);
        }}
        disabled={!items.length}
      >
        <SelectTrigger id={selectId} className="w-full">
          <SelectValue>
            {(value: string[]) =>
              !items.length
                ? 'No weeks in this season'
                : value.length
                  ? items
                      .filter((item) => value.includes(item.value))
                      .map((item) => item.label)
                      .join(', ')
                  : 'All weeks'
            }
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      {weeks.isError ? (
        <p role="alert">Season weeks could not be loaded.</p>
      ) : null}
    </Field>
  );
}

const resultItems = [
  { label: 'All results', value: '' },
  { label: 'Pass', value: 'true' },
  { label: 'Fail', value: 'false' },
];

export function MockFilters({
  seasonId,
  people,
  filters,
  onChange,
  participant = 'interviewer',
}: {
  seasonId?: string;
  /** The people offered for the participant filter. */
  people: Pick<Person, 'id' | 'name'>[];
  filters: ActivityFilters;
  onChange: (change: Partial<ActivityFilters>) => void;
  /** Which side of the interviews the people filter applies to. */
  participant?: 'interviewer' | 'interviewee';
}) {
  const participantKey =
    participant === 'interviewer' ? 'interviewers' : 'interviewees';
  const participantLabel =
    participant === 'interviewer' ? 'Interviewers' : 'Interviewees';
  return (
    <div className="grid gap-3" aria-label="Mock filters">
      <label className="grid gap-2 text-sm font-medium">
        Result
        <Select
          items={resultItems}
          value={filters.passed}
          onValueChange={(value) => onChange({ passed: value ?? '' })}
        >
          <SelectTrigger aria-label="Result" className="w-full">
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
      {people.length ? (
        <label className="grid gap-2 text-sm font-medium">
          {participantLabel}
          <Select
            multiple
            items={people.map((person) => ({
              label: person.name,
              value: person.id,
            }))}
            value={filters[participantKey]}
            onValueChange={(selected) =>
              onChange({ [participantKey]: selected })
            }
          >
            <SelectTrigger aria-label={participantLabel} className="w-full">
              <SelectValue
                placeholder={`All ${participantLabel.toLowerCase()}`}
              />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {people.map((person) => (
                  <SelectItem key={person.id} value={person.id}>
                    {person.name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </label>
      ) : null}
      {seasonId && (
        <SeasonWeekChoices
          seasonId={seasonId}
          selected={filters.weeks}
          onChange={(weeks) => onChange({ weeks })}
        />
      )}
    </div>
  );
}
