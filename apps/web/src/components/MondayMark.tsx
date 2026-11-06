import type { MondayUpdateMark } from '@/api/generated/models';
import { StatusBadge } from '@/components/StatusBadge';
import { markLabel, markTone, type LatestMark } from '@/mondayMarks';
import { formatDate } from '@/utils';

export function MarkBadge({ mark }: { mark: MondayUpdateMark }) {
  return (
    <StatusBadge tone={markTone[mark].badge}>{markLabel(mark)}</StatusBadge>
  );
}

/**
 * The date range of a student's latest Monday update, or `noUpdateText`
 * when there is none. One line: the update's mark has its own column.
 */
export function LatestUpdateRange({
  latest,
  noUpdateText,
}: {
  latest: LatestMark | undefined;
  noUpdateText: string;
}) {
  if (!latest)
    return (
      <span className="text-sm text-muted-foreground">{noUpdateText}</span>
    );
  return (
    <span className="whitespace-nowrap">
      {formatDate(latest.update.startDate)} –{' '}
      {formatDate(latest.update.endDate)}
    </span>
  );
}

/**
 * The mark on a student's latest Monday update: its badge, a grey "Not
 * marked" badge, or a dash when there is no update. `showMarker` names the
 * mentor who marked it on hover.
 */
export function LatestMarkBadge({
  latest,
  showMarker = false,
}: {
  latest: LatestMark | undefined;
  showMarker?: boolean;
}) {
  if (!latest) return <span className="text-muted-foreground">—</span>;
  if (!latest.mark) return <StatusBadge tone="muted">Not marked</StatusBadge>;
  const marker = showMarker
    ? `Marked by ${latest.mark.mentor.name}`
    : undefined;
  return (
    <StatusBadge
      tone={markTone[latest.mark.mark].badge}
      title={marker}
      aria-label={
        marker
          ? `${markLabel(latest.mark.mark)}, ${marker.toLowerCase()}`
          : undefined
      }
    >
      {markLabel(latest.mark.mark)}
    </StatusBadge>
  );
}
