import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import type { Season } from '@/types';
import styles from '@/styles/App.module.css';

export function ActivityPeriodFilter({
  section,
  season,
  seasons,
}: {
  section: 'practice' | 'mock-interviews';
  season?: Season;
  seasons: Season[];
}) {
  const { slug } = useParams();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  return (
    <label className={styles.field}>
      View activity
      <select
        className={styles.select}
        value={season?.id ?? ''}
        onChange={(event) => {
          const selected = seasons.find(
            (item) => item.id === event.target.value,
          );
          const next = new URLSearchParams(params);
          next.delete('year');
          next.delete('weekId');
          next.delete('seasonId');
          // Personal history stays on its own route, including for former members.
          if (selected && !slug) next.set('seasonId', selected.id);
          navigate({
            pathname:
              selected && slug
                ? `/seasons/${selected.slug}/${section}`
                : `/${section}`,
            search: next.toString(),
          });
        }}
      >
        <option value="">All time</option>
        {seasons.map((item) => (
          <option key={item.id} value={item.id}>
            {item.name}
          </option>
        ))}
      </select>
    </label>
  );
}
