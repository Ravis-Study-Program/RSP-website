import { Combobox } from '@base-ui/react/combobox';
import { useEffect, useState, type Ref } from 'react';
import type { LeetcodeProblem } from '@/api/generated/models';
import { displayDifficulty } from '@/api/adapters';
import { useProblemSearch } from '@/api/queries';

const searchDelayMs = 250;

const label = (problem: LeetcodeProblem) =>
  `${problem.number}. ${problem.title} · ${displayDifficulty(problem.difficulty)}`;

/** Picks a catalogue problem by searching the API as the user types. */
export function ProblemSelect({
  id,
  name,
  value,
  onChange,
  onBlur,
  inputRef,
  disabled,
  invalid,
  describedBy,
}: {
  id: string;
  name?: string;
  value: LeetcodeProblem | null;
  onChange: (problem: LeetcodeProblem | null) => void;
  onBlur?: () => void;
  inputRef?: Ref<HTMLInputElement>;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
}) {
  const [input, setInput] = useState('');
  const [query, setQuery] = useState('');
  useEffect(() => {
    const timer = window.setTimeout(
      () => setQuery(input.trim()),
      searchDelayMs,
    );
    return () => window.clearTimeout(timer);
  }, [input]);
  const search = useProblemSearch(query);
  const results = query ? (search.data ?? []) : [];
  const items =
    value && !results.some((problem) => problem.id === value.id)
      ? [value, ...results]
      : results;
  const pending = input.trim() !== query || search.isFetching;
  const emptyMessage = !input.trim()
    ? 'Type a question number or title.'
    : pending
      ? 'Searching…'
      : search.isError
        ? 'Questions could not be loaded.'
        : 'No matching questions.';

  return (
    <Combobox.Root
      items={items}
      filter={null}
      value={value}
      name={name}
      disabled={disabled}
      itemToStringLabel={label}
      itemToStringValue={(p) => p.id}
      isItemEqualToValue={(a, b) => a.id === b.id}
      autoHighlight
      onInputValueChange={(next, details) => {
        if (details.reason !== 'input-change') return;
        setInput(next);
        if (value) onChange(null);
      }}
      onValueChange={(problem) => onChange(problem ?? null)}
    >
      <Combobox.Input
        id={id}
        ref={inputRef}
        className="h-9 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive disabled:opacity-50"
        placeholder="Search by question number or title…"
        onBlur={onBlur}
        aria-invalid={invalid}
        aria-describedby={describedBy}
      />
      <Combobox.Portal>
        <Combobox.Positioner
          sideOffset={4}
          className="z-100 w-(--anchor-width) max-w-[calc(100vw-2rem)]"
        >
          <Combobox.Popup className="rounded-lg border border-border bg-popover text-popover-foreground shadow-md outline-none">
            <Combobox.Empty className="m-0 px-2.5 py-2 text-sm text-muted-foreground">
              {emptyMessage}
            </Combobox.Empty>
            <Combobox.List className="max-h-[min(18.75rem,var(--available-height))] overflow-auto p-1">
              {(problem: LeetcodeProblem) => (
                <Combobox.Item
                  key={problem.id}
                  value={problem}
                  className="cursor-pointer rounded-md px-3 py-2.5 text-sm [overflow-wrap:anywhere] data-highlighted:bg-muted"
                >
                  {label(problem)}
                </Combobox.Item>
              )}
            </Combobox.List>
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  );
}
