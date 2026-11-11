import type { ReactNode } from 'react';

/**
 * The columns an import's CSV takes: the header line to copy, then what goes
 * in each column.
 */
export function CsvColumns({
  columns,
  note,
}: {
  columns: Array<{ name: string; description: ReactNode }>;
  /** Shown under the list, such as what happens to existing records. */
  note?: ReactNode;
}) {
  return (
    <div className="grid gap-2 text-sm">
      <p>
        <span className="text-muted-foreground">Columns: </span>
        <code className="rounded-sm bg-muted px-1.5 py-0.5 font-mono text-[0.8125rem]">
          {columns.map((column) => column.name).join(',')}
        </code>
      </p>
      <dl className="grid grid-cols-[max-content_1fr] gap-x-3 gap-y-1">
        {columns.map((column) => (
          <div key={column.name} className="contents">
            <dt className="font-mono text-[0.8125rem]">{column.name}</dt>
            <dd className="text-muted-foreground">{column.description}</dd>
          </div>
        ))}
      </dl>
      {note ? <p className="text-muted-foreground">{note}</p> : null}
    </div>
  );
}
