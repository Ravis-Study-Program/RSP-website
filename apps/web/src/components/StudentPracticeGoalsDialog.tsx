import { useQuery, useQueryClient } from '@tanstack/react-query';

import type { PracticeSettings } from '@/api/generated/models';
import { setStudentPracticeGoals } from '@/api/practiceSettingsClient';
import {
  demoMode,
  programmePracticeGoalsOptions,
  userPracticeSettingsOptions,
} from '@/api/queries';
import { PracticeGoalsDialog } from '@/components/PracticeGoalsDialog';
import { goalMinutesFrom, practiceGoalsFrom } from '@/practiceGoals';
import type { Person } from '@/types';

/** Lets a mentor or season administrator set one student's time goals. */
export function StudentPracticeGoalsDialog({
  seasonId,
  person,
}: {
  seasonId: string;
  person: Person;
}) {
  const queryClient = useQueryClient();
  const options = userPracticeSettingsOptions(person.id);
  const settings = useQuery(options);
  const programme = useQuery(programmePracticeGoalsOptions);
  const custom = settings.data?.source === 'student';
  const save = async (next: PracticeSettings | null) => {
    if (!person.enrollmentId) return;
    let saved: PracticeSettings;
    if (demoMode) {
      if (!programme.data) return;
      saved = next ?? {
        goalsEnabled: true,
        ...programme.data,
        source: 'programme',
      };
    } else {
      saved = await setStudentPracticeGoals(
        seasonId,
        person.enrollmentId,
        next ? practiceGoalsFrom(goalMinutesFrom(next)) : null,
      );
    }
    queryClient.setQueryData(options.queryKey, saved);
  };
  return (
    <PracticeGoalsDialog
      title={`Set ${person.name}'s time goals`}
      description={
        custom
          ? 'These goals replace the programme goals for this student.'
          : 'This student uses the programme goals. Saving sets their own.'
      }
      triggerLabel={custom ? 'Custom goals' : 'Set goals'}
      goals={settings.data ? goalMinutesFrom(settings.data) : undefined}
      saveLabel="Save goals"
      onSave={(goals) =>
        save({
          goalsEnabled: true,
          ...practiceGoalsFrom(goals),
          source: 'student',
        })
      }
      reset={
        custom
          ? { label: 'Use programme goals', onReset: () => save(null) }
          : undefined
      }
    />
  );
}
