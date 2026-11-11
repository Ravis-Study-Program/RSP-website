import { useEffect, useState } from 'react';
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

import { Card, CardContent } from '@/components/ui/card';
import { legendLabel } from '@/lib/chartLegend';

type ActivityPoint = { week: string; attempts: number; interviews: number };

type ChartPoint = ActivityPoint & {
  expectedAttempts?: number;
  expectedInterviews?: number;
};

const axisTick = { fill: 'var(--text-muted)', fontSize: 12 };

function useCompactAxis() {
  const [compact, setCompact] = useState(false);
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return;
    const query = window.matchMedia('(max-width: 56rem)');
    const apply = () => setCompact(query.matches);
    apply();
    query.addEventListener('change', apply);
    return () => query.removeEventListener('change', apply);
  }, []);
  return compact;
}

export function ActivityChart({
  data,
  expected,
}: {
  data: ActivityPoint[];
  expected?: { attempts: number; interviews: number };
}) {
  const points: ChartPoint[] = data.map((point) =>
    expected
      ? {
          ...point,
          expectedAttempts: expected.attempts,
          expectedInterviews: expected.interviews,
        }
      : point,
  );
  return (
    // Each chart is its own panel, side by side on wide screens.
    <div className="grid grid-cols-1 gap-6 min-[56rem]:grid-cols-2">
      <WeekChart
        title="LeetCode"
        points={points}
        actualKey="attempts"
        expectedKey={expected ? 'expectedAttempts' : undefined}
        color="var(--chart-leetcode)"
        unit="LeetCode"
      />
      <WeekChart
        title="Mock Interviews"
        points={points}
        actualKey="interviews"
        expectedKey={expected ? 'expectedInterviews' : undefined}
        color="var(--chart-mocks)"
        unit="mock interviews"
      />
    </div>
  );
}

function WeekChart({
  title,
  points,
  actualKey,
  expectedKey,
  color,
  unit,
}: {
  title: string;
  points: ChartPoint[];
  actualKey: 'attempts' | 'interviews';
  expectedKey?: 'expectedAttempts' | 'expectedInterviews';
  color: string;
  unit: string;
}) {
  const compact = useCompactAxis();
  const edgeWeeks = [points[0]?.week, points.at(-1)?.week].filter(
    (week): week is string => Boolean(week),
  );
  return (
    <Card>
      <CardContent>
        <section aria-label={`${title} per week`}>
          <h3 className="text-base font-semibold">{title}</h3>
          <p className="sr-only">
            {points
              .map((point) => {
                const actual = `${point[actualKey]} actual ${unit}`;
                const expected = expectedKey
                  ? `, ${point[expectedKey]} expected`
                  : '';
                return `${point.week}: ${actual}${expected}`;
              })
              .join('. ')}
          </p>
          <div
            className="mt-3 h-56 w-full min-w-0 max-[56rem]:h-52"
            aria-hidden="true"
          >
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                accessibilityLayer={false}
                data={points}
                margin={{
                  top: 10,
                  right: compact ? 40 : 12,
                  left: compact ? 12 : 0,
                  bottom: 0,
                }}
              >
                <CartesianGrid
                  stroke="var(--border)"
                  strokeDasharray="3 3"
                  vertical={false}
                />
                <XAxis
                  dataKey="week"
                  stroke="var(--text-muted)"
                  tick={axisTick}
                  tickLine={false}
                  axisLine={false}
                  ticks={compact ? edgeWeeks : undefined}
                  interval={compact ? 0 : 'preserveStartEnd'}
                  minTickGap={compact ? 0 : 28}
                  padding={{ left: 4, right: 4 }}
                />
                {/* A period with nothing recorded still draws its line, flat at 0. */}
                <YAxis
                  allowDecimals={false}
                  domain={[0, (dataMax: number) => Math.max(dataMax, 1)]}
                  stroke="var(--text-muted)"
                  tick={axisTick}
                  tickLine={false}
                  axisLine={false}
                  width={32}
                />
                <Tooltip
                  contentStyle={{
                    background: 'var(--surface-raised)',
                    border: '1px solid var(--border)',
                    borderRadius: 'var(--radius-md)',
                    color: 'var(--text)',
                  }}
                />
                {expectedKey ? <Legend formatter={legendLabel} /> : null}
                {expectedKey ? (
                  <Line
                    isAnimationActive={false}
                    type="monotone"
                    dataKey={expectedKey}
                    name="Expected"
                    stroke="var(--chart-secondary)"
                    strokeWidth={2}
                    strokeDasharray="6 4"
                    dot={false}
                  />
                ) : null}
                <Line
                  isAnimationActive={false}
                  type="monotone"
                  dataKey={actualKey}
                  name="Actual"
                  stroke={color}
                  strokeWidth={2.5}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </section>
      </CardContent>
    </Card>
  );
}
