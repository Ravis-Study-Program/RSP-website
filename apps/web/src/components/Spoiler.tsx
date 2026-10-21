import { useState, type ReactNode } from 'react';

/**
 * Hides content that gives away how to solve a question, such as its topics,
 * until the viewer asks to see it.
 */
export function Spoiler({
  label = 'Show topics',
  children,
}: {
  label?: string;
  children: ReactNode;
}) {
  const [revealed, setRevealed] = useState(false);
  if (revealed) return <>{children}</>;
  return (
    <button
      type="button"
      className="cursor-pointer underline decoration-dotted underline-offset-2 hover:text-foreground"
      onClick={() => setRevealed(true)}
    >
      {label}
    </button>
  );
}
