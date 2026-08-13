type LogLevel = "debug" | "info" | "warn" | "error";

function serializeError(error: unknown): unknown {
  if (!(error instanceof Error)) return error;
  return { name: error.name, message: error.message, stack: error.stack };
}

export function log(level: LogLevel, message: string, attributes: Record<string, unknown> = {}): void {
  const serialized = Object.fromEntries(
    Object.entries(attributes).map(([key, value]) => [key, serializeError(value)]),
  );
  process.stdout.write(
    `${JSON.stringify({ time: new Date().toISOString(), level, message, ...serialized })}\n`,
  );
}
