import { PageSection } from '@/components/Common';
import { cn } from '@/lib/utils';
import { levelOrder, type ProgramLevel } from '@/levelProgress';
import { studentLevels } from '@/studentLevels';

const progressColor: Record<ProgramLevel, { dot: string; line: string; ring: string }> = {
  beginner: {
    dot: 'bg-[#6b7280]',
    line: 'bg-[#6b7280]',
    ring: 'ring-[#6b7280]',
  },
  novice: {
    dot: 'bg-[#14944a]',
    line: 'bg-[#14944a]',
    ring: 'ring-[#14944a]',
  },
  intermediate: {
    dot: 'bg-[#1a6fd4]',
    line: 'bg-[#1a6fd4]',
    ring: 'ring-[#1a6fd4]',
  },
  advanced: {
    dot: 'bg-[#6d28d9]',
    line: 'bg-[#6d28d9]',
    ring: 'ring-[#6d28d9]',
  },
};

export function LevelProgressSection({ level }: { level: ProgramLevel }) {
  return (
    <PageSection titleId="progress-title" title="Your Progress">
      <LevelTrack level={level} />
    </PageSection>
  );
}

function LevelTrack({ level }: { level: ProgramLevel }) {
  const currentIndex = levelOrder.indexOf(level);
  const color = progressColor[level];

  return (
      <ol className="grid grid-cols-4">
        {levelOrder.map((item, index) => {
          const reached = index <= currentIndex;
          const current = item === level;
          return (
            <li
              key={item}
              className="flex flex-col items-center gap-2 text-center"
              aria-current={current ? 'step' : undefined}
            >
              <span
                className={cn(
                  'text-xs sm:text-sm',
                  current
                    ? 'font-semibold text-foreground'
                    : reached
                      ? 'text-foreground'
                      : 'text-muted-foreground',
                )}
              >
                {studentLevels[item]}
                {current ? <span className="sr-only">, current level</span> : null}
              </span>
              <span className="relative flex h-3 w-full items-center justify-center">
                {index > 0 ? (
                  <span
                    aria-hidden="true"
                    className={cn(
                      'absolute top-1/2 right-1/2 left-0 h-0.5 -translate-y-1/2',
                      index <= currentIndex ? color.line : 'bg-border',
                    )}
                  />
                ) : null}
                {index < levelOrder.length - 1 ? (
                  <span
                    aria-hidden="true"
                    className={cn(
                      'absolute top-1/2 right-0 left-1/2 h-0.5 -translate-y-1/2',
                      index < currentIndex ? color.line : 'bg-border',
                    )}
                  />
                ) : null}
                <span
                  aria-hidden="true"
                  className={cn(
                    'relative rounded-full',
                    reached
                      ? cn('size-2.5', color.dot)
                      : 'size-2.5 border-2 border-muted-foreground/50 bg-background',
                    current && cn('size-3 ring-2 ring-offset-2 ring-offset-background', color.ring),
                  )}
                />
              </span>
            </li>
          );
        })}
      </ol>
  );
}
