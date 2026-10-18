import { useMemo, type FocusEventHandler } from 'react';

import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox';
import { formatTimezoneOption } from '@/utils';

export function TimezoneSelect({
  id,
  name,
  value,
  options,
  getOptionLabel,
  onChange,
  onBlur,
  inputRef,
  invalid = false,
  describedBy,
}: {
  id: string;
  name?: string;
  value: string;
  options: string[];
  getOptionLabel?: (timezone: string) => string;
  onChange: (timezone: string) => void;
  onBlur?: FocusEventHandler<HTMLInputElement>;
  inputRef?: (element: HTMLInputElement | null) => void;
  invalid?: boolean;
  describedBy?: string;
}) {
  const optionLabels = useMemo(
    () =>
      new Map(
        options.map((timezone) => [
          timezone,
          formatTimezoneOption(
            timezone,
            getOptionLabel?.(timezone) ?? timezone,
          ),
        ]),
      ),
    [getOptionLabel, options],
  );

  return (
    <Combobox
      items={options}
      itemToStringLabel={(timezone: string) =>
        optionLabels.get(timezone) ?? timezone
      }
      name={name}
      value={value || null}
      onValueChange={(timezone) => {
        if (timezone) onChange(timezone);
      }}
    >
      <ComboboxInput
        ref={inputRef}
        id={id}
        className="w-full"
        placeholder="Type to filter timezones…"
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        onBlur={onBlur}
      />
      <ComboboxContent>
        <ComboboxEmpty>No matching timezones.</ComboboxEmpty>
        <ComboboxList aria-label="Timezone options">
          {(timezone: string) => (
            <ComboboxItem key={timezone} value={timezone}>
              {optionLabels.get(timezone) ?? timezone}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}
