import type { ReactNode } from 'react';

/**
 * Options for a dropdown of people. Two people can share a name, so any whose
 * name appears more than once also show their slug, in grey and brackets.
 */
export function personOptions<
  T extends { id: string; name: string; slug?: string },
>(people: T[]): Array<{ label: ReactNode; value: string }> {
  const counts = new Map<string, number>();
  for (const person of people) {
    const key = person.name.trim().toLowerCase();
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return people.map((person) => ({
    value: person.id,
    label:
      person.slug && (counts.get(person.name.trim().toLowerCase()) ?? 0) > 1 ? (
        <>
          {person.name}{' '}
          <span className="text-muted-foreground">({person.slug})</span>
        </>
      ) : (
        person.name
      ),
  }));
}
