import { Card } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
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
import { activitySeries, difficultyBreakdown } from '@/activityAnalytics';
import { ActivityChart } from '@/components/ActivityChart';
import { legendLabel } from '@/lib/chartLegend';
import type { Attempt, Season } from '@/types';
import { practiceGoalMinutes } from '@/utils';

const colors = [
  'var(--accent)',
  'var(--text-muted)',
  'var(--chart-secondary)',
  'var(--border)',
];
export function PracticeAnalytics({
  attempts,
  season,
}: {
  attempts: Attempt[];
  season?: Season;
}) {
  const [goals, setGoals] = useState(true);
  const breakdown = difficultyBreakdown(attempts);
  const points = attempts
    .filter((a) => a.minutes !== null && a.difficulty)
    .map((a) => ({
      name: a.problem,
      difficulty: a.difficulty!,
      minutes: a.minutes!,
    }));
  return (
    <section className="mt-6" aria-label="Practice charts">
      <h2 className="text-xl font-semibold tracking-tight">
        Practice in this period
      </h2>
      <p className="mt-1 text-sm text-muted-foreground">
        Charts include every matching attempt, across all table pages.
      </p>
      <div className="grid gap-6">
        <ActivityChart data={activitySeries(attempts, [], { season })} />
        <div className="grid gap-4 grid-cols-[repeat(auto-fit,minmax(min(100%,20rem),1fr))]">
          <Card
            role="region"
            aria-label="Difficulty breakdown"
            className="gap-3 p-5"
          >
            <h3 className="text-base font-semibold">Difficulty breakdown</h3>
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
                      >
                        {breakdown.map((item, index) => (
                          <Cell key={item.name} fill={colors[index]} />
                        ))}
                      </Pie>
                      <Tooltip />
                      <Legend formatter={legendLabel} />
                    </PieChart>
                  </ResponsiveContainer>
                </div>
                <p>
                  {breakdown
                    .map((item) => `${item.name}: ${item.value}`)
                    .join(' · ')}
                </p>
              </>
            ) : (
              <p>No attempts match these filters.</p>
            )}
          </Card>
          <Card
            role="region"
            aria-label="Time by difficulty"
            className="gap-3 p-5"
          >
            <h3 className="text-base font-semibold">Time by difficulty</h3>
            <Label className="font-normal">
              <Checkbox
                checked={goals}
                onCheckedChange={(checked) => setGoals(checked)}
              />
              Show practice goals (20 / 35 / 50 minutes)
            </Label>
            {points.length ? (
              <>
                <div
                  className="mt-3 h-72 w-full min-w-0 max-[56rem]:h-56"
                  aria-hidden="true"
                >
                  <ResponsiveContainer width="100%" height="100%">
                    <ScatterChart
                      accessibilityLayer={false}
                      margin={{ top: 20, right: 30, bottom: 10, left: 0 }}
                    >
                      <CartesianGrid stroke="var(--border)" />
                      <XAxis
                        type="category"
                        dataKey="difficulty"
                        name="Difficulty"
                        allowDuplicatedCategory={false}
                      />
                      <YAxis
                        type="number"
                        dataKey="minutes"
                        name="Time"
                        unit=" min"
                        domain={[0, 'auto']}
                      />
                      <Tooltip
                        cursor={{ strokeDasharray: '3 3' }}
                        content={({ active, payload }) =>
                          active && payload?.[0] ? (
                            <Card className="gap-4 p-5">
                              {payload[0].payload.name}:{' '}
                              {payload[0].payload.minutes} min
                            </Card>
                          ) : null
                        }
                      />
                      <Scatter
                        isAnimationActive={false}
                        data={points}
                        fill="var(--accent)"
                      />
                      {goals
                        ? Object.entries(practiceGoalMinutes).map(
                            ([difficulty, minutes]) => (
                              <ReferenceLine
                                key={difficulty}
                                y={minutes}
                                stroke="var(--text-muted)"
                                strokeDasharray="4 4"
                                label={`${difficulty} ${minutes}`}
                                ifOverflow="extendDomain"
                              />
                            ),
                          )
                        : null}
                    </ScatterChart>
                  </ResponsiveContainer>
                </div>
                <p>
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
