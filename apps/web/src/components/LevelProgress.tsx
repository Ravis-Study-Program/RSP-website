import { IconCircleCheckFilled } from '@tabler/icons-react';

import { PageSection } from '@/components/Common';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';
import {
  barsForLevel,
  levelOrder,
  nextLevel,
  type LevelCounts,
  type ProgramLevel,
  type ProgressBar,
} from '@/levelProgress';
import { studentLevels } from '@/studentLevels';

const fill: Record<ProgramLevel, string> = {
  beginner: 'bg-[#6b7280]',
  novice: 'bg-[#14944a]',
  intermediate: 'bg-[#1a6fd4]',
  advanced: 'bg-[#6d28d9]',
};

const wash: Record<ProgramLevel, string> = {
  beginner: 'bg-muted',
  novice: 'bg-success-soft',
  intermediate: 'bg-info-soft',
  advanced: 'bg-purple-soft',
};

export function LevelProgressSection({
  level,
  counts,
}: {
  level: ProgramLevel;
  counts: LevelCounts | null;
}) {
  // The level being worked towards shows its progress; the rest stay compact.
  const focus = nextLevel(level);
  const currentIndex = levelOrder.indexOf(level);

  return (
    <PageSection titleId="progress-title" title="Your Progress">
      <div className="flex flex-wrap items-stretch gap-2">
        {levelOrder.map((item) => {
          const focused = focus === item;
          const awarded = levelOrder.indexOf(item) <= currentIndex;
          return (
            <Card
              key={item}
              size="sm"
              className={cn(
                'min-h-14 gap-0 py-0',
                // Compact panels share a minimum width so short names don't look cramped.
                focused ? 'w-auto min-w-0 grow' : 'w-fit min-w-36 shrink-0',
                awarded || focused ? wash[item] : 'bg-card',
              )}
            >
              <div className="flex h-full flex-1 items-center">
                <div className="flex items-center gap-2 py-2.5 pr-3.5 pl-3.5">
                  <span
                    aria-hidden="true"
                    className={cn('size-2 shrink-0 rounded-full', fill[item])}
                  />
                  <span className="text-sm font-medium whitespace-nowrap">
                    {studentLevels[item]}
                    {item === level ? (
                      <span className="sr-only">, current level</span>
                    ) : null}
                    {awarded ? (
                      <span className="sr-only">, completed</span>
                    ) : null}
                  </span>
                  {awarded ? (
                    <IconCircleCheckFilled
                      className="shrink-0"
                      size={16}
                      aria-hidden="true"
                    />
                  ) : null}
                </div>
                {focused ? (
                  <div className="min-w-0 grow py-2 pr-2 pl-1">
                    <LevelBody item={item} counts={counts} />
                  </div>
                ) : null}
              </div>
            </Card>
          );
        })}
      </div>
    </PageSection>
  );
}

function LevelBody({
  item,
  counts,
}: {
  item: ProgramLevel;
  counts: LevelCounts | null;
}) {
  const bars = counts ? barsForLevel(item, counts) : null;
  if (!counts) {
    return (
      <p className="text-sm text-muted-foreground">
        Counting recorded problems…
      </p>
    );
  }
  if (!bars) {
    return (
      <p className="text-sm whitespace-nowrap text-muted-foreground">
        Awarded for the reading and implementation list.
      </p>
    );
  }
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      <div className="rounded-lg bg-background/40 px-3.5 py-2.5">
        <ProgressMeter label="LeetCode" bar={bars.leetcode} fill={fill[item]} />
      </div>
      <div className="rounded-lg bg-background/40 px-3.5 py-2.5">
        <ProgressMeter
          label="Mock Interviews"
          bar={bars.mocks}
          fill={fill[item]}
        />
      </div>
    </div>
  );
}

function ProgressMeter({
  label,
  bar,
  fill: barFill,
}: {
  label: string;
  bar: ProgressBar;
  fill: string;
}) {
  const width =
    bar.target === 0 ? 0 : Math.min(100, (bar.done / bar.target) * 100);
  return (
    <div className="min-w-48">
      <div className="flex items-baseline justify-between gap-2 text-sm">
        <span>{label}</span>
        <span className="tabular-nums text-foreground/75">
          {bar.done} of {bar.target}
        </span>
      </div>
      <div
        className="mt-1.5 h-2 overflow-hidden rounded-full bg-background/70"
        role="meter"
        aria-valuenow={bar.done}
        aria-valuemin={0}
        aria-valuemax={bar.target}
        aria-label={label}
      >
        <div
          className={cn('h-full rounded-full', barFill)}
          style={{ width: `${width}%` }}
        />
      </div>
      <p className="mt-1.5 text-sm text-foreground/75">
        {bar.parts
          .map((part) => `${part.label} ${part.done}/${part.target}`)
          .join(' · ')}
      </p>
    </div>
  );
}
