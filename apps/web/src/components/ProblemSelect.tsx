import { Combobox } from '@base-ui/react/combobox';
import type { Ref } from 'react';
import type { LeetcodeProblem } from '@/api/generated/models';
import { displayDifficulty } from '@/api/adapters';
import styles from '@/styles/App.module.css';

const label = (problem: LeetcodeProblem) =>
  `${problem.number}. ${problem.title} · ${displayDifficulty(problem.difficulty)}`;

export function ProblemSelect({
  id,
  name,
  problems,
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
  problems: LeetcodeProblem[];
  value: string;
  onChange: (id: string) => void;
  onBlur?: () => void;
  inputRef?: Ref<HTMLInputElement>;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
}) {
  return (
    <Combobox.Root
      items={problems}
      value={problems.find((p) => p.id === value) ?? null}
      name={name}
      disabled={disabled}
      itemToStringLabel={label}
      itemToStringValue={(p) => p.id}
      isItemEqualToValue={(a, b) => a.id === b.id}
      limit={50}
      autoHighlight
      onInputValueChange={(_, details) => {
        if (details.reason === 'input-change') onChange('');
      }}
      onValueChange={(problem) => onChange(problem?.id ?? '')}
    >
      <Combobox.Input
        id={id}
        ref={inputRef}
        className={styles.input}
        placeholder="Search by question number or title…"
        onBlur={onBlur}
        aria-invalid={invalid}
        aria-describedby={describedBy}
      />
      <Combobox.Portal>
        <Combobox.Positioner
          sideOffset={4}
          className={styles.problemPositioner}
        >
          <Combobox.Popup className={styles.problemPopup}>
            <Combobox.Empty className={styles.timezoneEmpty}>
              No matching questions.
            </Combobox.Empty>
            <Combobox.List className={styles.problemList}>
              {(problem: LeetcodeProblem) => (
                <Combobox.Item
                  key={problem.id}
                  value={problem}
                  className={styles.problemOption}
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
