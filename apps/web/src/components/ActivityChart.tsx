import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

import { legendLabel } from '@/lib/chartLegend';

export function ActivityChart({
  data,
}: {
  data: Array<{ week: string; attempts: number; interviews: number }>;
}) {
  const attempts = data.reduce((total, item) => total + item.attempts, 0);
  const interviews = data.reduce((total, item) => total + item.interviews, 0);
  const busiest = data.reduce<(typeof data)[number] | null>(
    (current, item) =>
      !current ||
      item.attempts + item.interviews > current.attempts + current.interviews
        ? item
        : current,
    null,
  );
  const hasActivity = attempts + interviews > 0;
  const summary = (
    <p className="mt-3 text-sm text-muted-foreground">
      Activity summary: {attempts} practice attempts and {interviews} mock
      interviews across {data.length} periods.
      {busiest && busiest.attempts + busiest.interviews > 0
        ? ` ${busiest.week} had the highest recorded activity.`
        : ' No activity was recorded in this period.'}
    </p>
  );
  if (!hasActivity) return summary;
  return (
    <div className="min-w-0">
      <div
        className="mt-3 h-72 w-full min-w-0 max-[56rem]:h-56"
        aria-hidden="true"
      >
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart
            accessibilityLayer={false}
            data={data}
            margin={{ top: 10, right: 16, left: 0, bottom: 0 }}
          >
            <CartesianGrid
              stroke="var(--border)"
              strokeDasharray="3 3"
              vertical={false}
            />
            <XAxis
              dataKey="week"
              stroke="var(--text-muted)"
              tickLine={false}
              axisLine={false}
              interval="preserveStartEnd"
              padding={{ left: 8, right: 8 }}
            />
            <YAxis
              allowDecimals={false}
              stroke="var(--text-muted)"
              tickLine={false}
              axisLine={false}
            />
            <Tooltip
              contentStyle={{
                background: 'var(--surface-raised)',
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius-md)',
                color: 'var(--text)',
              }}
            />
            <Legend formatter={legendLabel} />
            <Area
              isAnimationActive={false}
              type="monotone"
              dataKey="attempts"
              name="Attempts"
              stroke="var(--accent)"
              strokeWidth={2}
              fill="none"
            />
            <Area
              isAnimationActive={false}
              type="monotone"
              dataKey="interviews"
              name="Mock interviews"
              stroke="var(--chart-secondary)"
              strokeWidth={2}
              fill="none"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
      {summary}
    </div>
  );
}
