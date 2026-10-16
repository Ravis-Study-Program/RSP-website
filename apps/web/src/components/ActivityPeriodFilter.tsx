import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
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
  const seasonItems = [
    { label: 'All time', value: '' },
    ...seasons.map((item) => ({ label: item.name, value: item.id })),
  ];
  return (
    <Field>
      <FieldLabel htmlFor={selectId}>View activity</FieldLabel>
      <Select
        items={seasonItems}
        value={season?.id ?? ''}
        onValueChange={(value) => {
          const selected = seasons.find((item) => item.id === (value ?? ''));
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
        <SelectTrigger id={selectId} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {seasonItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </Field>
  );
}
