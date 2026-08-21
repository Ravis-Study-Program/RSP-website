import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FocusEventHandler,
} from 'react';

import styles from '@/styles/App.module.css';
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
  const rootRef = useRef<HTMLDivElement>(null);
  const inputElement = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return options.filter((timezone) =>
      timezone.toLocaleLowerCase().includes(normalizedQuery),
    );
  }, [options, query]);
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

  useEffect(() => {
    if (!open) return;
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('pointerdown', closeOnOutsidePointer);
    return () =>
      document.removeEventListener('pointerdown', closeOnOutsidePointer);
  }, [open]);

  const setRefs = (element: HTMLInputElement | null) => {
    inputElement.current = element;
    inputRef?.(element);
  };
  const openOptions = () => {
    setOpen(true);
    setQuery('');
    setHighlightedIndex(0);
    window.requestAnimationFrame(() => inputElement.current?.select());
  };
  const choose = (timezone: string) => {
    onChange(timezone);
    setQuery('');
    setOpen(false);
    setHighlightedIndex(0);
  };
  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setOpen(true);
      setHighlightedIndex((current) =>
        Math.min(current + 1, Math.max(filteredOptions.length - 1, 0)),
      );
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setOpen(true);
      setHighlightedIndex((current) => Math.max(current - 1, 0));
    } else if (event.key === 'Enter' && open && filteredOptions.length) {
      event.preventDefault();
      choose(filteredOptions[highlightedIndex] ?? filteredOptions[0]);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      setOpen(false);
      setQuery('');
    }
  };

  return (
    <div ref={rootRef} className={styles.timezoneSelect}>
      <input
        ref={setRefs}
        id={id}
        name={name}
        className={styles.input}
        role="combobox"
        aria-autocomplete="list"
        aria-controls={`${id}-options`}
        aria-expanded={open}
        aria-activedescendant={
          open && filteredOptions.length
            ? `${id}-option-${highlightedIndex}`
            : undefined
        }
        aria-invalid={invalid}
        aria-describedby={describedBy}
        value={open ? query : value}
        placeholder={open ? 'Type to filter timezones…' : undefined}
        onFocus={openOptions}
        onChange={(event) => {
          setQuery(event.target.value);
          setOpen(true);
          setHighlightedIndex(0);
        }}
        onBlur={(event) => {
          setOpen(false);
          setQuery('');
          onBlur?.(event);
        }}
        onKeyDown={handleKeyDown}
        autoComplete="off"
      />
      {open ? (
        <div
          id={`${id}-options`}
          className={styles.timezoneMenu}
          role="listbox"
          aria-label="Timezone options"
        >
          {filteredOptions.length ? (
            filteredOptions.map((timezone, index) => (
              <div
                id={`${id}-option-${index}`}
                className={`${styles.timezoneOption} ${index === highlightedIndex ? styles.timezoneOptionActive : ''}`}
                key={timezone}
                role="option"
                tabIndex={-1}
                aria-selected={timezone === value}
                onMouseDown={(event) => event.preventDefault()}
                onMouseEnter={() => setHighlightedIndex(index)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    choose(timezone);
                  }
                }}
                onClick={() => choose(timezone)}
              >
                {optionLabels.get(timezone) ?? timezone}
              </div>
            ))
          ) : (
            <p className={styles.timezoneEmpty}>No matching timezones.</p>
          )}
        </div>
      ) : null}
    </div>
  );
}
