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
import type { Attempt, Season } from '@/types';
import { practiceGoalMinutes } from '@/utils';
import styles from '@/styles/App.module.css';

const colors = [
  'var(--accent)',
  'var(--chart-secondary)',
  '#c87838',
  'var(--text-muted)',
];
export function PracticeAnalytics({
  attempts,
  season,
  year,
}: {
  attempts: Attempt[];
  season?: Season;
  year?: number;
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
    <section className={styles.section} aria-label="Practice charts">
      <h2 className={styles.sectionTitle}>Practice in this period</h2>
      <p className={styles.helper}>
        Charts include every matching attempt, across all table pages.
      </p>
      <ActivityChart data={activitySeries(attempts, [], { season, year })} />
      <div className={styles.analyticsGrid}>
        <section className={styles.panel} aria-label="Difficulty breakdown">
          <h3>Difficulty breakdown</h3>
          {breakdown.length ? (
            <>
              <div className={styles.chartWrap} aria-hidden="true">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart accessibilityLayer={false}>
                    <Pie
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
                    <Legend />
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
        </section>
        <section className={styles.panel} aria-label="Time by difficulty">
          <h3>Time by difficulty</h3>
          <label className={styles.inline}>
            <input
              type="checkbox"
              checked={goals}
              onChange={(e) => setGoals(e.target.checked)}
            />
            Show practice goals (20 / 35 / 50 minutes)
          </label>
          {points.length ? (
            <>
              <div className={styles.chartWrap} aria-hidden="true">
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
                          <div className={styles.panel}>
                            {payload[0].payload.name}:{' '}
                            {payload[0].payload.minutes} min
                          </div>
                        ) : null
                      }
                    />
                    <Scatter data={points} fill="var(--accent)" />
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
        </section>
      </div>
    </section>
  );
}
