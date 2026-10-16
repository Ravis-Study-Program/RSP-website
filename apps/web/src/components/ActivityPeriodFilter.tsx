import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { useId } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import type { Season } from '@/types';
import { Field, FieldLabel } from '@/components/ui/field';

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
  const selectId = useId();
  return (
    <Field>
      <FieldLabel htmlFor={selectId}>View activity</FieldLabel>
      <NativeSelect
        id={selectId}
        className="w-full"
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
        <NativeSelectOption value="">All time</NativeSelectOption>
        {seasons.map((item) => (
          <NativeSelectOption key={item.id} value={item.id}>
            {item.name}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </Field>
  );
}
