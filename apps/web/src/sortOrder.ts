import type { Row } from '@tanstack/react-table';

/**
 * The order roles and statuses sort and display in, in every list. Anything
 * not named sorts after the named values, alphabetically.
 */
const roleOrder = [
  'director',
  'system_admin',
  'coordinator',
  'mentor',
  'student',
  'graduate',
  'former_member',
];
const statusOrder = ['active', 'completed', 'kicked', 'withdrawn'];

function rank(order: string[], value: string | null | undefined) {
  const index = order.indexOf(value ?? '');
  return index === -1 ? order.length : index;
}

function compareBy(
  order: string[],
  a: string | null | undefined,
  b: string | null | undefined,
) {
  return rank(order, a) - rank(order, b) || (a ?? '').localeCompare(b ?? '');
}

/** Roles from most to least senior: Coordinator, Mentor, Student. */
export function sortRoles<T extends string>(roles: readonly T[]): T[] {
  return [...roles].sort((a, b) => compareBy(roleOrder, a, b));
}

/** A table sort for a role column, or a roles column of arrays. */
export function roleSortingFn<TData>(
  a: Row<TData>,
  b: Row<TData>,
  columnId: string,
) {
  const first = (row: Row<TData>) => {
    // A roles column may hold an array or a space-separated list.
    const value = row.getValue<string | string[] | null>(columnId);
    const roles = Array.isArray(value) ? value : (value ?? '').split(/\s+/);
    return sortRoles(roles)[0];
  };
  return compareBy(roleOrder, first(a), first(b));
}

/** A table sort for a status column: Active, Completed, Kicked. */
export function statusSortingFn<TData>(
  a: Row<TData>,
  b: Row<TData>,
  columnId: string,
) {
  return compareBy(
    statusOrder,
    a.getValue<string | null>(columnId),
    b.getValue<string | null>(columnId),
  );
}
