/** Program order. Beginner is where a student starts. */
export const levelOrder = [
  'beginner',
  'novice',
  'intermediate',
  'advanced',
] as const;

export type ProgramLevel = (typeof levelOrder)[number];

export function isProgramLevel(level: string | undefined): level is ProgramLevel {
  return levelOrder.some((item) => item === level);
}
