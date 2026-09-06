export const programmeTimezone = 'Australia/Adelaide';
export function adelaideYear(value: string | Date) {
  return Number(
    new Intl.DateTimeFormat('en-AU', {
      timeZone: programmeTimezone,
      year: 'numeric',
    }).format(new Date(value)),
  );
}
