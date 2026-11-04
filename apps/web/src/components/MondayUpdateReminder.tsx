import { IconAlertCircle } from '@tabler/icons-react';
import { Link } from 'react-router-dom';

import { mondayReminderWeek, sameUpdateWeek } from '@/activityDates';
import type { Week } from '@/api/generated/models';
import { useMondayUpdates } from '@/api/queries';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';

export function MondayUpdateReminder({
  seasonId,
  seasonSlug,
  studentId,
  weeks,
}: {
  seasonId: string;
  seasonSlug: string;
  studentId: string;
  weeks: Week[];
}) {
  const week = mondayReminderWeek(weeks);
  const updates = useMondayUpdates(Boolean(week), seasonId, {
    startDate: week?.startDate ?? '',
    endDate: week?.endDate ?? '',
    studentIds: [studentId],
  });
  if (
    !week ||
    !updates.data ||
    updates.isError ||
    updates.isLoading ||
    updates.data.items.some(
      (update) =>
        update.student.id === studentId && sameUpdateWeek(update, week),
    )
  )
    return null;

  return (
    <Alert
      variant="destructive"
      className="mb-6 border-destructive/40 bg-destructive/5 p-4"
    >
      <IconAlertCircle aria-hidden="true" />
      <AlertTitle>Your Week {week.number} Monday update is due</AlertTitle>
      <AlertDescription>
        <Link
          to={`/seasons/${seasonSlug}/monday-updates?week=${encodeURIComponent(week.id)}`}
        >
          Write your update
        </Link>
      </AlertDescription>
    </Alert>
  );
}
