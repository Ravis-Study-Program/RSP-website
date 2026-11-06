import { Card } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { useState } from 'react';
import {
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ReferenceLine,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  difficultyBreakdown,
  difficultySuccessRates,
} from '@/activityAnalytics';
import { legendLabel } from '@/lib/chartLegend';
import type { Attempt, Difficulty } from '@/types';
import { usePracticeGoals } from '@/api/queries';
import { formatDate, formatDateTime } from '@/utils';

const breakdownColor = (name: string) =>
  difficultyColor[name as Difficulty] ?? 'var(--chart-secondary)';

const difficultyOrder: Difficulty[] = ['Easy', 'Medium', 'Hard'];
const difficultyColor: Record<Difficulty, string> = {
  Easy: 'var(--success)',
  Medium: 'var(--warning)',
  Hard: 'var(--danger)',
};

type TimedPoint = {
  name: string;
  difficulty: Difficulty;
  minutes: number;
  at: number;
  /** Position in date order, so same-day attempts sit side by side. */
  index: number;
};

function TimedTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: { payload: TimedPoint }[];
}) {
  const point = active ? payload?.[0]?.payload : undefined;
  if (!point) return null;
  return (
    <Card className="gap-1 p-3 text-sm">
      <span className="flex items-center gap-2 font-semibold">
        <span
          aria-hidden="true"
          className="size-2 rounded-full"
          style={{ background: difficultyColor[point.difficulty] }}
        />
        {point.name}
      </span>
      <span className="text-muted-foreground">
        {point.difficulty} · {point.minutes} min · {formatDateTime(point.at)}
      </span>
    </Card>
  );
}

function DifficultyKey() {
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
      {difficultyOrder.map((difficulty) => (
        <li key={difficulty} className="flex items-center gap-1.5">
          <span
            aria-hidden="true"
            className="size-2.5 rounded-full"
            style={{ background: difficultyColor[difficulty] }}
          />
          {difficulty}
        </li>
      ))}
    </ul>
  );
}
export function PracticeAnalytics({ attempts }: { attempts: Attempt[] }) {
  const [goals, setGoals] = useState(true);
  const breakdown = difficultyBreakdown(attempts);
  const practiceGoals = usePracticeGoals();
  const successRates = difficultySuccessRates(attempts, practiceGoals);
  const points: TimedPoint[] = attempts
    .filter((a) => a.minutes !== null && a.difficulty)
    .map((a) => ({
      name: a.problem,
      difficulty: a.difficulty!,
      minutes: a.minutes!,
      at: Date.parse(a.attemptedAt),
    }))
    .sort((a, b) => a.at - b.at)
    .map((point, index) => ({ ...point, index }));
  const byDifficulty = difficultyOrder
    .map((difficulty) => ({
      difficulty,
      data: points.filter((p) => p.difficulty === difficulty),
    }))
    .filter((group) => group.data.length);
  // One even step per attempt rather than a time scale: attempts logged on the
  // same day (including bulk-migrated history) would otherwise overlap.
  const attemptAxis = {
    type: 'number' as const,
    dataKey: 'index',
    name: 'Attempt',
    domain: ['dataMin', 'dataMax'] as [string, string],
    allowDecimals: false,
    padding: { left: 16, right: 16 },
    tickFormatter: (value: number) =>
      points[value] ? formatDate(points[value].at) : '',
    tick: { fill: 'var(--text-muted)', fontSize: 12 },
    stroke: 'var(--border-strong)',
  };
  const minutesAxis = {
    type: 'number' as const,
    dataKey: 'minutes',
    name: 'Time',
    unit: ' min',
    domain: [0, 'auto'] as [number, string],
    tick: { fill: 'var(--text-muted)', fontSize: 12 },
    stroke: 'var(--border-strong)',
  };
  return (
    <section aria-label="Practice charts">
      <div className="grid gap-6">
        <div className="grid gap-6 grid-cols-[repeat(auto-fit,minmax(min(100%,20rem),1fr))]">
          <Card
            role="region"
            aria-label="Difficulty breakdown"
            className="gap-3 p-4"
          >
            <h3 className="text-base font-semibold">Difficulty Breakdown</h3>
            {breakdown.length ? (
              <>
                <div
                  className="mt-3 h-72 w-full min-w-0 max-[56rem]:h-56"
                  aria-hidden="true"
                >
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart accessibilityLayer={false}>
                      <Pie
                        isAnimationActive={false}
                        data={breakdown}
                        dataKey="value"
                        nameKey="name"
                        innerRadius="45%"
                        outerRadius="75%"
                        stroke="var(--surface)"
                        strokeWidth={2}
                      >
                        {breakdown.map((item) => (
                          <Cell
                            key={item.name}
                            fill={breakdownColor(item.name)}
                          />
                        ))}
                      </Pie>
                      <Tooltip />
                      <Legend formatter={legendLabel} />
                    </PieChart>
                  </ResponsiveContainer>
                </div>
                <Table className="table-fixed tabular-nums">
                  <caption className="sr-only">
                    Success: solved independently within the practice goal.
                  </caption>
                  <colgroup>
                    <col className="w-28" />
                    {successRates.map(({ difficulty }) => (
                      <col key={difficulty} />
                    ))}
                  </colgroup>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead>
                        <span className="sr-only">Measure</span>
                      </TableHead>
                      {successRates.map(({ difficulty }) => (
                        <TableHead key={difficulty} scope="col">
                          {difficulty}
                        </TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow className="hover:bg-transparent">
                      <TableHead scope="row">Count</TableHead>
                      {successRates.map(({ difficulty, count }) => (
                        <TableCell key={difficulty}>{count}</TableCell>
                      ))}
                    </TableRow>
                    <TableRow className="hover:bg-transparent">
                      <TableHead scope="row">Success rate</TableHead>
                      {successRates.map(({ difficulty, count, successes }) => (
                        <TableCell key={difficulty}>
                          {count
                            ? `${Math.round((successes / count) * 100)}%`
                            : '—'}
                        </TableCell>
                      ))}
                    </TableRow>
                  </TableBody>
                </Table>
              </>
            ) : (
              <p>No attempts match these filters.</p>
            )}
          </Card>
          <Card
            role="region"
            aria-label="Time by difficulty"
            className="gap-3 p-4"
          >
            <h3 className="text-base font-semibold">Time by Difficulty</h3>
            <Label className="font-normal">
              <Checkbox
                checked={goals}
                onCheckedChange={(checked) => setGoals(checked)}
              />
              Show practice goals ({practiceGoals.Easy} / {practiceGoals.Medium}{' '}
              / {practiceGoals.Hard} minutes)
            </Label>
            {points.length ? (
              <>
                <div className="mt-3 grid flex-1 grid-rows-[auto_1fr] gap-2">
                  <DifficultyKey />
                  <div
                    className="h-full min-h-72 w-full min-w-0 max-[56rem]:min-h-56"
                    aria-hidden="true"
                  >
                    <ResponsiveContainer width="100%" height="100%">
                      <ScatterChart
                        accessibilityLayer={false}
                        margin={{ top: 12, right: 24, bottom: 4, left: 0 }}
                      >
                        <CartesianGrid
                          stroke="var(--border)"
                          vertical={false}
                        />
                        <XAxis {...attemptAxis} />
                        <YAxis {...minutesAxis} />
                        <Tooltip
                          cursor={{ strokeDasharray: '3 3' }}
                          content={TimedTooltip}
                        />
                        {goals
                          ? difficultyOrder.map((difficulty) => (
                              <ReferenceLine
                                key={difficulty}
                                y={practiceGoals[difficulty]}
                                stroke={difficultyColor[difficulty]}
                                strokeOpacity={0.55}
                                strokeDasharray="4 4"
                                label={{
                                  value: `${difficulty} goal`,
                                  position: 'insideTopRight',
                                  fill: difficultyColor[difficulty],
                                  fontSize: 12,
                                }}
                                ifOverflow="extendDomain"
                              />
                            ))
                          : null}
                        {byDifficulty.map(({ difficulty, data }) => (
                          <Scatter
                            key={difficulty}
                            name={difficulty}
                            isAnimationActive={false}
                            data={data}
                            fill={difficultyColor[difficulty]}
                          />
                        ))}
                      </ScatterChart>
                    </ResponsiveContainer>
                  </div>
                </div>
                <p className="sr-only">
                  {points.length} attempts with recorded times. Lowest:{' '}
                  {Math.min(...points.map((p) => p.minutes))} min; highest:{' '}
                  {Math.max(...points.map((p) => p.minutes))} min. Individual
                  times appear in the table.
                </p>
              </>
            ) : (
              <p>No timed attempts match these filters.</p>
            )}
          </Card>
        </div>
      </div>
    </section>
  );
}
