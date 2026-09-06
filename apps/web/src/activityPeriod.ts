import { useParams, useSearchParams } from 'react-router-dom';
import { useSeasons } from '@/api/queries';

export function useActivitySeason() {
  const { slug } = useParams();
  const [params] = useSearchParams();
  const seasons = useSeasons();
  const selectedId = params.get('seasonId');
  const season = seasons.data?.items.find((item) =>
    slug ? item.slug === slug : item.id === selectedId,
  );
  return { seasons, season, hasSelection: Boolean(slug || selectedId) };
}
