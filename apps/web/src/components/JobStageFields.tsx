import { useId, useState } from 'react';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Combobox,
  ComboboxContent,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox';
import { jobStageKey, recommendedJobStages } from '@/jobStages';

export function JobStageNameField({
  value,
  onChange,
  reported,
}: {
  value: string;
  onChange: (value: string) => void;
  reported: string[];
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const query = jobStageKey(value);
  const names = new Map(
    [...reported, ...recommendedJobStages].map((name) => [
      jobStageKey(name),
      name,
    ]),
  );
  const exact = names.get(query);
  const options = Array.from(names)
    .filter(([key]) => exact || key.includes(query))
    .map(([, name]) => name);

  return (
    <Field>
      <FieldLabel htmlFor={id}>Stage Name</FieldLabel>
      <Combobox
        items={options}
        filter={null}
        open={open && options.length > 0}
        onOpenChange={setOpen}
        value={exact ?? null}
        inputValue={value}
        onInputValueChange={(next, details) => {
          if (details.reason === 'input-change') onChange(next);
        }}
        onValueChange={(next) => {
          if (next) onChange(next);
        }}
      >
        <ComboboxInput
          id={id}
          aria-label="Stage Name"
          required
          maxLength={120}
          className="w-full"
          placeholder="Choose or enter a stage…"
        />
        <ComboboxContent>
          <ComboboxList aria-label="Stage suggestions">
            {(option: string) => (
              <ComboboxItem key={option} value={option}>
                <span className="min-w-0 break-words">{option}</span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </Field>
  );
}
