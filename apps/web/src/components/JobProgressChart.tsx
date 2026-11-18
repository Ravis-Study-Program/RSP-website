import { useId } from 'react';
import type {
  JobPositionProgress,
  JobProgressNode,
} from '@/api/generated/models';
import { useJobPositionProgress } from '@/api/jobs';
import { demoMode } from '@/api/queries';
import { Button } from '@/components/ui/button';

const columnWidth = 174;
const labelHeight = 48;
const nodeWidth = 7;
const padding = 12;

function sankeyLayout(progress: JobPositionProgress) {
  const columns = new Map<number, JobProgressNode[]>();
  for (const stage of progress.stages) {
    const column = columns.get(stage.round) ?? [];
    column.push(stage);
    columns.set(stage.round, column);
  }
  const unit = 144 / progress.total;
  const columnHeight = (stages: JobProgressNode[]) =>
    stages.reduce(
      (height, stage) => height + labelHeight + stage.reached * unit + padding,
      0,
    );
  const bodyHeight = Math.max(...Array.from(columns.values(), columnHeight));
  const nodes = new Map<number, JobProgressNode & { x: number; y: number }>();
  for (const [round, stages] of columns) {
    let y = 34 + (bodyHeight - columnHeight(stages)) / 2;
    const x = padding + round * columnWidth;
    stages.sort(
      (a, b) => b.reached - a.reached || a.title.localeCompare(b.title),
    );
    for (const stage of stages) {
      y += labelHeight;
      nodes.set(stage.id, { ...stage, x, y });
      y += stage.reached * unit + padding;
    }
  }
  const outgoing = new Map<number, number>();
  const incoming = new Map<number, number>();
  const links = [...progress.transitions]
    .sort(
      (a, b) =>
        nodes.get(a.source)!.y - nodes.get(b.source)!.y ||
        nodes.get(a.target)!.y - nodes.get(b.target)!.y,
    )
    .map((link) => {
      const source = nodes.get(link.source)!;
      const target = nodes.get(link.target)!;
      const width = link.count * unit;
      const sourceY = (outgoing.get(source.id) ?? source.y) + width / 2;
      const targetY = (incoming.get(target.id) ?? target.y) + width / 2;
      outgoing.set(source.id, sourceY + width / 2);
      incoming.set(target.id, targetY + width / 2);
      const sourceX = source.x + nodeWidth;
      const mid = (sourceX + target.x) / 2;
      return {
        ...link,
        source,
        target,
        width,
        d: `M${sourceX},${sourceY} C${mid},${sourceY} ${mid},${targetY} ${target.x},${targetY}`,
      };
    });
  return {
    nodes: [...nodes.values()],
    links,
    unit,
    columns: columns.size,
    height: bodyHeight + 34,
    width: columns.size * columnWidth + padding,
  };
}

function stageColor(title: string) {
  if (title === 'Offer') return 'var(--success)';
  if (title === 'Rejected') return 'var(--danger)';
  return 'var(--info)';
}

/* eslint-disable jsx-a11y/no-noninteractive-tabindex -- Keep the overflow region scrollable by keyboard. */
function Sankey({ progress }: { progress: JobPositionProgress }) {
  const { nodes, links, unit, columns, height, width } = sankeyLayout(progress);
  return (
    <div
      className="min-w-0 overflow-x-auto rounded-lg border border-border bg-card focus-visible:outline-2 focus-visible:outline-ring"
      tabIndex={0}
      role="region"
      aria-label="Scrollable application flow chart"
    >
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label="Sankey diagram of stages applicants reached"
        className="block max-w-none"
      >
        {Array.from({ length: columns }, (_, round) => (
          <g key={round}>
            {round > 0 && (
              <line
                x1={round * columnWidth}
                x2={round * columnWidth}
                y1={0}
                y2={height}
                className="stroke-border"
                strokeDasharray="3 5"
              />
            )}
            <text
              x={padding + round * columnWidth}
              y={23}
              className="fill-muted-foreground text-xs font-semibold"
            >
              {round === 0 ? 'Submitted' : `Stage ${round}`}
            </text>
          </g>
        ))}
        {links.map((link) => (
          <path
            key={`${link.source.id}-${link.target.id}`}
            d={link.d}
            fill="none"
            stroke={stageColor(link.target.title)}
            strokeWidth={link.width}
            className="opacity-25 hover:opacity-50 motion-safe:transition-opacity"
          >
            <title>{`${link.source.title} → ${link.target.title}: ${link.count}`}</title>
          </path>
        ))}
        {nodes.map((node) => (
          <g key={node.id}>
            <title>{`${node.title}: ${node.reached} reached, ${node.current} here now`}</title>
            <rect
              x={node.x}
              y={node.y}
              width={nodeWidth}
              height={Math.max(1, node.reached * unit)}
              rx={1}
              fill={stageColor(node.title)}
            />
            <foreignObject
              x={node.x}
              y={node.y - labelHeight}
              width={columnWidth - 2 * padding}
              height={labelHeight}
            >
              <div className="flex h-full flex-col items-start justify-end pb-1">
                <span className="line-clamp-2 rounded-sm bg-card px-1 text-sm leading-4 font-semibold break-words">
                  {node.title}
                </span>
                <span className="rounded-sm bg-card px-1 text-xs leading-4 text-muted-foreground tabular-nums">
                  {node.reached} reached
                  {node.current > 0 && ` · ${node.current} here now`}
                </span>
              </div>
            </foreignObject>
          </g>
        ))}
      </svg>
    </div>
  );
}

/* eslint-enable jsx-a11y/no-noninteractive-tabindex */

export function JobProgressChart({ positionId }: { positionId: string }) {
  const heading = useId();
  const progress = useJobPositionProgress(positionId);
  return (
    <section aria-labelledby={heading} className="grid min-w-0 gap-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h3 id={heading} className="text-sm font-semibold">
          Application Flow
        </h3>
        {!!progress.data?.total && (
          <p className="text-sm text-muted-foreground">
            {progress.data.total}{' '}
            {progress.data.total === 1
              ? 'applicant sharing'
              : 'applicants sharing'}
            {demoMode && ' (demo data)'}
          </p>
        )}
      </div>
      {progress.isPending ? (
        <p role="status" className="text-sm text-muted-foreground">
          Loading shared progress…
        </p>
      ) : progress.isError ? (
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <p role="status">Couldn’t load shared progress.</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => void progress.refetch()}
          >
            Retry
          </Button>
        </div>
      ) : progress.data.total === 0 ? (
        <p className="text-sm text-muted-foreground">
          No applicants have shared their progress yet.
        </p>
      ) : (
        <>
          {progress.data.stages.some((stage) => stage.round > 0) && (
            <p className="text-xs text-muted-foreground sm:hidden">
              Scroll across to see more stages.
            </p>
          )}
          <Sankey progress={progress.data} />
          <details className="text-sm">
            <summary className="w-fit cursor-pointer rounded-sm font-medium focus-visible:outline-2 focus-visible:outline-ring">
              Stage Counts
            </summary>
            <table className="mt-2 w-full max-w-xl text-left">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th scope="col" className="px-2 py-1 font-medium">
                    Stage
                  </th>
                  <th scope="col" className="px-2 py-1 text-right font-medium">
                    Reached
                  </th>
                  <th scope="col" className="px-2 py-1 text-right font-medium">
                    Here Now
                  </th>
                </tr>
              </thead>
              <tbody>
                {Array.from(progress.data.stages)
                  .sort((a, b) => a.round - b.round || b.current - a.current)
                  .map((stage) => (
                    <tr key={stage.id} className="border-b border-border">
                      <th
                        scope="row"
                        className="px-2 py-1 font-normal [overflow-wrap:anywhere]"
                      >
                        {stage.title}
                        {stage.round > 0 && (
                          <span className="ml-2 text-xs text-muted-foreground">
                            Stage {stage.round}
                          </span>
                        )}
                      </th>
                      <td className="px-2 py-1 text-right tabular-nums">
                        {stage.reached}
                      </td>
                      <td className="px-2 py-1 text-right tabular-nums">
                        {stage.current}
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </details>
        </>
      )}
    </section>
  );
}
