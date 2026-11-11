import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * One part of a dialog whose edits are held until the dialog's Confirm
 * button commits them all together. `save` throws when it fails, and may
 * return a note to show the person afterwards.
 */
export interface DraftSection {
  dirty: boolean;
  save: () => Promise<string | void>;
}

/** Registers a section's current state with its dialog. */
export function useDraftSection(
  dirty: boolean,
  save: DraftSection['save'],
  register?: (section: DraftSection | null) => void,
) {
  const latest = useRef(save);
  const registerRef = useRef(register);
  useEffect(() => {
    latest.current = save;
    registerRef.current = register;
  });
  useEffect(() => {
    registerRef.current?.({ dirty, save: () => latest.current() });
  }, [dirty]);
  useEffect(() => () => registerRef.current?.(null), []);
}

/** The dialog's side of {@link useDraftSection}. */
export function useDraftSections() {
  const [sections, setSections] = useState<Record<string, DraftSection>>({});
  const register = useCallback(
    (key: string) => (section: DraftSection | null) =>
      setSections((current) => {
        if (!section) {
          if (!(key in current)) return current;
          const rest = { ...current };
          delete rest[key];
          return rest;
        }
        if (current[key]?.dirty === section.dirty && key in current)
          return current;
        return { ...current, [key]: section };
      }),
    [],
  );
  const dirty = Object.values(sections).some((section) => section.dirty);
  /** Saves the changed sections in the order given; returns their notes. */
  const saveAll = async (order: string[]) => {
    const notes: string[] = [];
    for (const key of order) {
      const section = sections[key];
      if (!section?.dirty) continue;
      const note = await section.save();
      if (note) notes.push(note);
    }
    return notes;
  };
  return { register, dirty, saveAll };
}
