import type { ActivityFilters } from '@/activityFilters';
import type { MockInterviewMode } from '@/api/queries';
import { MockFilters } from '@/components/ActivityFilters';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Field, FieldLabel } from '@/components/ui/field';
import type { Person } from '@/types';

const mockTypeItems: { label: string; value: MockInterviewMode }[] = [
  { label: 'All', value: 'all' },
  { label: 'Received', value: 'received' },
  { label: 'Given', value: 'given' },
];
interface ViewProps {
  mode: MockInterviewMode;
  onModeChange: (mode: MockInterviewMode) => void;
}

export function MockInterviewViewTabs({ mode, onModeChange }: ViewProps) {
  return (
    <Tabs
      value={mode}
      onValueChange={(value) => onModeChange(value as MockInterviewMode)}
    >
      <TabsList
        aria-label="Mock interview view"
        className="group-data-[orientation=horizontal]/tabs:h-9"
      >
        {mockTypeItems.map((item) => (
          <TabsTrigger key={item.value} value={item.value}>
            {item.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}

export function MockInterviewFilterControls({
  mode,
  onModeChange,
  seasonId,
  people,
  filters,
  onChange,
}: ViewProps & {
  seasonId?: string;
  people: Pick<Person, 'id' | 'name'>[];
  filters: ActivityFilters;
  onChange: (change: Partial<ActivityFilters>) => void;
}) {
  return (
    <>
      {/* The same choice as the toolbar tabs, one at a time. */}
      <Field>
        <FieldLabel htmlFor="mock-type-filter">Type</FieldLabel>
        <Select
          items={mockTypeItems}
          value={mode}
          onValueChange={(value) => {
            if (value) onModeChange(value as MockInterviewMode);
          }}
        >
          <SelectTrigger id="mock-type-filter" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {mockTypeItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <MockFilters
        embedded
        participant={mode === 'received' ? 'interviewer' : 'interviewee'}
        seasonId={seasonId}
        people={people}
        filters={filters}
        onChange={onChange}
      />
    </>
  );
}
