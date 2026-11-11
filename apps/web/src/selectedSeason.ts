import { useSyncExternalStore } from 'react';

/**
 * The season a Director, System Admin or coordinator is working in, shared by
 * every season table and remembered between pages and visits.
 */
const key = 'rsp-selected-season';
const listeners = new Set<() => void>();

export function getSelectedSeasonSlug(): string {
  try {
    return window.localStorage.getItem(key) ?? '';
  } catch {
    return '';
  }
}

export function selectSeason(slug: string) {
  if (!slug || slug === getSelectedSeasonSlug()) return;
  try {
    window.localStorage.setItem(key, slug);
  } catch {
    // Remembering is a convenience; private windows may refuse it.
  }
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  window.addEventListener('storage', listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener('storage', listener);
  };
}

/** The selected season's slug, or '' when none has been chosen yet. */
export function useSelectedSeasonSlug(): string {
  return useSyncExternalStore(subscribe, getSelectedSeasonSlug, () => '');
}
