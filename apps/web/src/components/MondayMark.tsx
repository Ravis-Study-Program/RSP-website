import type { MondayUpdateMark } from '@/api/generated/models';
import { Badge } from '@/components/ui/badge';
import { markLabel, markTone, type LatestMark } from '@/mondayMarks';
import { formatDate } from '@/utils';

export function MarkBadge({ mark }: { mark: MondayUpdateMark }) {
  return <Badge className={markTone[mark].badge}>{markLabel(mark)}</Badge>;
}

/**
 * A student's latest Monday mark: the badge (or "Not marked") with the
 * update's date range, or `noUpdateText` when there is no update at all.
 */
export function LatestMarkCell({
  latest,
  noUpdateText,
  showMarker = false,
}: {
  latest: LatestMark | undefined;
  noUpdateText: string;
  showMarker?: boolean;
}) {
  const muted = 'text-sm text-muted-foreground';
  if (!latest) return <span className={muted}>{noUpdateText}</span>;
  return (
    <div className="grid justify-items-start gap-1">
      {latest.mark ? (
        <MarkBadge mark={latest.mark.mark} />
      ) : (
        <span className="text-sm">Not marked</span>
      )}
      <span className={muted}>
        {formatDate(latest.update.startDate)} –{' '}
        {formatDate(latest.update.endDate)}
        {showMarker && latest.mark ? ` · by ${latest.mark.mentor.name}` : null}
      </span>
    </div>
  );
}
