import { IconChevronRight, IconCircleCheckFilled } from '@tabler/icons-react';
import { useLayoutEffect, useRef, useState, type ReactNode } from 'react';

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
  const next = nextLevel(level);
  const [open, setOpen] = useState<ProgramLevel | null>(next ?? level);
  const currentIndex = levelOrder.indexOf(level);

  return (
    <PageSection titleId="progress-title" title="Your Progress">
      <div className="flex flex-wrap items-center gap-2">
        {levelOrder.map((item) => {
          const expanded = open === item;
          const awarded = levelOrder.indexOf(item) <= currentIndex;
          return (
            <Card
              key={item}
              size="sm"
              className={cn(
                'w-fit min-h-14 gap-0 py-0 motion-safe:transition-[background-color,box-shadow] motion-safe:duration-300 motion-safe:ease-out',
                expanded ? 'min-w-0' : 'shrink-0',
                awarded || expanded ? wash[item] : 'bg-card',
                awarded && 'ring-foreground/40',
              )}
            >
              <div className="flex h-full flex-1 items-center">
                <button
                  type="button"
                  className="flex items-center gap-2 py-2.5 pr-1 pl-3.5 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                  aria-expanded={expanded}
                  onClick={() => setOpen(expanded ? null : item)}
                >
                  <span
                    aria-hidden="true"
                    className={cn('size-2 shrink-0 rounded-full', fill[item])}
                  />
                  <span className="text-sm font-medium whitespace-nowrap">
                    {studentLevels[item]}
                    {item === level ? (
                      <span className="sr-only">, current level</span>
                    ) : null}
                    {awarded ? <span className="sr-only">, completed</span> : null}
                  </span>
                  {awarded ? (
                    <IconCircleCheckFilled
                      className="shrink-0"
                      size={16}
                      aria-hidden="true"
                    />
                  ) : null}
                </button>
                <ExpandPanel open={expanded}>
                  <div className="py-2.5 pr-1 pl-2">
                    <LevelBody item={item} awarded={awarded} counts={counts} />
                  </div>
                </ExpandPanel>
                <button
                  type="button"
                  className="px-2.5 py-2.5 text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                  aria-label={
                    expanded
                      ? `Close ${studentLevels[item]}`
                      : `Open ${studentLevels[item]}`
                  }
                  onClick={() => setOpen(expanded ? null : item)}
                >
                  <IconChevronRight
                    className={cn(
                      'motion-safe:transition-transform motion-safe:duration-300 motion-safe:ease-out',
                      expanded && 'rotate-180',
                    )}
                    size={16}
                    aria-hidden="true"
                  />
                </button>
              </div>
            </Card>
          );
        })}
      </div>
    </PageSection>
  );
}

function ExpandPanel({ open, children }: { open: boolean; children: ReactNode }) {
  const innerRef = useRef<HTMLDivElement>(null);
  const first = useRef(true);
  const [width, setWidth] = useState<number | 'auto'>(open ? 'auto' : 0);

  useLayoutEffect(() => {
    const inner = innerRef.current;
    if (!inner) return;
    if (first.current) {
      first.current = false;
      setWidth(open ? 'auto' : 0);
      return;
    }
    const full = inner.scrollWidth;
    // Paint the start width first. A same-frame change never eases.
    setWidth(open ? 0 : full);
    let second = 0;
    const firstFrame = requestAnimationFrame(() => {
      second = requestAnimationFrame(() => setWidth(open ? full : 0));
    });
    return () => {
      cancelAnimationFrame(firstFrame);
      cancelAnimationFrame(second);
    };
  }, [open]);

  return (
    <div
      aria-hidden={open ? undefined : true}
      inert={open ? undefined : true}
      className="overflow-hidden motion-safe:transition-[width,opacity] motion-safe:duration-300 motion-safe:ease-out"
      style={{ width, opacity: open ? 1 : 0 }}
      onTransitionEnd={(event) => {
        if (event.target !== event.currentTarget || event.propertyName !== 'width') return;
        if (open) setWidth('auto');
      }}
    >
      <div ref={innerRef} className="w-max">
        {children}
      </div>
    </div>
  );
}

function LevelBody({
  item,
  awarded,
  counts,
}: {
  item: ProgramLevel;
  awarded: boolean;
  counts: LevelCounts | null;
}) {
  const bars = counts ? barsForLevel(item, counts) : null;
  if (awarded) {
    return (
      <p className="flex items-center gap-1.5 text-sm whitespace-nowrap">
        <IconCircleCheckFilled size={16} aria-hidden="true" />
        Completed
      </p>
    );
  }
  if (!counts) {
    return (
      <p className="text-sm text-muted-foreground">Counting recorded problems…</p>
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
    <div className="grid grid-cols-2 gap-4">
      <ProgressMeter label="LeetCode" bar={bars.leetcode} fill={fill[item]} />
      <ProgressMeter label="Mock interviews" bar={bars.mocks} fill={fill[item]} />
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
  const width = bar.target === 0 ? 0 : Math.min(100, (bar.done / bar.target) * 100);
  return (
    <div className="min-w-36">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span>{label}</span>
        <span className="tabular-nums text-muted-foreground">
          {bar.done} of {bar.target}
        </span>
      </div>
      <div
        className="mt-1 h-1 overflow-hidden rounded-full bg-background/70"
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
      <p className="mt-1 text-xs text-muted-foreground">
        {bar.parts
          .map((part) => `${part.label} ${part.done}/${part.target}`)
          .join(' · ')}
      </p>
    </div>
  );
}
