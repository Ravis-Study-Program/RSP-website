import { useId } from 'react';
import { useJobCompanies } from '@/api/jobs';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox';

export function JobCompanyField({
  value,
  onChange,
}: {
  value: string;
  onChange: (name: string) => void;
}) {
  const id = useId();
  const companies = useJobCompanies();
  const name = value.trim();
  const query = name.toLowerCase();
  const names = (companies.data ?? []).map((company) => company.name);
  const existing = names.find((option) => option.toLowerCase() === query);
  const options = names.filter((option) =>
    option.toLowerCase().includes(query),
  );
  const newName = name && !existing && companies.isSuccess ? name : undefined;
  if (newName) options.push(newName);

  return (
    <Field>
      <FieldLabel htmlFor={id}>Company</FieldLabel>
      <Combobox
        items={options}
        filter={null}
        value={existing ?? null}
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
          required
          maxLength={120}
          className="w-full"
          placeholder="Search or add a company…"
        />
        <ComboboxContent>
          <ComboboxEmpty>
            {companies.isPending
              ? 'Loading companies…'
              : companies.isError
                ? 'Suggestions unavailable. You can still enter a company.'
                : 'Type a company name to add it.'}
          </ComboboxEmpty>
          <ComboboxList aria-label="Companies">
            {(option: string) => (
              <ComboboxItem key={option} value={option}>
                <span className="min-w-0 break-words">
                  {option === newName ? `Add “${option}”` : option}
                </span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </Field>
  );
}
