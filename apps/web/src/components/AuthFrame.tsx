import type { ReactNode } from 'react';

import { Card, CardContent, CardHeader } from '@/components/ui/card';

/**
 * The signed-out pages' frame: the RSP logo and name above one narrow,
 * centred card, as on Sign In.
 */
export function AuthFrame({
  title,
  description,
  notice,
  children,
}: {
  title: string;
  description?: ReactNode;
  /** Shown across the top of the page, above the card. */
  notice?: ReactNode;
  children: ReactNode;
}) {
  return (
    <main className="flex min-h-svh flex-col bg-background">
      {notice ? <div className="px-4 pt-4 sm:px-6">{notice}</div> : null}
      {/* The card is centred on its own; the logo hangs above it, outside
          the flow, with room kept for it by the vertical padding. */}
      <div className="flex flex-1 items-center justify-center px-4 py-24">
        <div className="relative w-full max-w-sm">
          <div className="absolute inset-x-0 bottom-full mb-6 flex items-center justify-center gap-3">
            <img src="/assets/rsp-logo.png" alt="" className="size-9" />
            <span className="text-base font-semibold">
              Ravi’s Study Program
            </span>
          </div>
          <Card className="gap-6 py-6">
            <CardHeader className="gap-2 px-6 text-center">
              <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
              {description ? (
                <p className="text-sm text-muted-foreground">{description}</p>
              ) : null}
            </CardHeader>
            <CardContent className="px-6">{children}</CardContent>
          </Card>
        </div>
      </div>
    </main>
  );
}
