/** The names that more than one of these people have, lower-cased. */
export function duplicateNames(people: Array<{ id?: string; name: string }>) {
  const counts = new Map<string, number>();
  const seen = new Set<string>();
  for (const person of people) {
    // Someone listed several times (in several interviews) is still one person.
    if (person.id) {
      if (seen.has(person.id)) continue;
      seen.add(person.id);
    }
    const key = person.name.trim().toLowerCase();
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return new Set(
    [...counts].filter(([, count]) => count > 1).map(([name]) => name),
  );
}
