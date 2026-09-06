import styles from '@/styles/App.module.css';
import { adelaideYear } from '@/activityDates';

export function ActivityYearFilter({
  year,
  onChange,
}: {
  year?: number;
  onChange: (year?: number) => void;
}) {
  const current = adelaideYear(new Date());
  return (
    <label className={styles.field}>
      Year (Adelaide time)
      <select
        className={styles.select}
        value={year ?? ''}
        onChange={(event) =>
          onChange(event.target.value ? Number(event.target.value) : undefined)
        }
      >
        <option value="">All years</option>
        {Array.from(
          { length: Math.max(1, current - 2010 + 1) },
          (_, index) => current - index,
        ).map((value) => (
          <option key={value} value={value}>
            {value}
          </option>
        ))}
      </select>
    </label>
  );
}
