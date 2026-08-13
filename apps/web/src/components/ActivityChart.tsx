import { Area, AreaChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import styles from '@/styles/App.module.css';

export function ActivityChart({ data }: { data: Array<{ week: string; attempts: number; interviews: number }> }) {
  const attempts = data.reduce((total, item) => total + item.attempts, 0);
  const interviews = data.reduce((total, item) => total + item.interviews, 0);
  return (
    <div>
      <div className={styles.chartWrap} aria-hidden="true">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart accessibilityLayer={false} data={data} margin={{ top: 10, right: 12, left: -20, bottom: 0 }}>
            <defs>
              <linearGradient id="attempts-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#6848c7" stopOpacity={0.38} /><stop offset="95%" stopColor="#6848c7" stopOpacity={0.02} /></linearGradient>
              <linearGradient id="interviews-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#1fb6ca" stopOpacity={0.35} /><stop offset="95%" stopColor="#1fb6ca" stopOpacity={0.02} /></linearGradient>
            </defs>
            <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
            <XAxis dataKey="week" stroke="var(--text-muted)" tickLine={false} axisLine={false} />
            <YAxis allowDecimals={false} stroke="var(--text-muted)" tickLine={false} axisLine={false} />
            <Tooltip contentStyle={{ background: 'var(--surface-raised)', border: '1px solid var(--border)', borderRadius: '0.5rem', color: 'var(--text)' }} />
            <Legend />
            <Area type="monotone" dataKey="attempts" name="Attempts" stroke="#6848c7" strokeWidth={2} fill="url(#attempts-fill)" />
            <Area type="monotone" dataKey="interviews" name="Mock interviews" stroke="#1fb6ca" strokeWidth={2} fill="url(#interviews-fill)" />
          </AreaChart>
        </ResponsiveContainer>
      </div>
      <p className={styles.chartSummary}>Activity summary: {attempts} practice attempts and {interviews} mock interviews across {data.length} weeks. Week 4 had the highest practice volume.</p>
    </div>
  );
}
