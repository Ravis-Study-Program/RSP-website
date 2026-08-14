type LogLevel = 'debug' | 'info' | 'warn' | 'error';

const SENSITIVE_KEY =
  /(?:password|token|cookie|authorization|email|secret|credential|idtoken|accesstoken|refreshtoken|details|args)/i;

function sanitizeString(value: string): string {
  return value
    .replace(/https?:\/\/[^\s?#]+[^\s]*[?#][^\s]*/gi, '[redacted-url]')
    .replace(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/gi, '[redacted-email]')
    .replace(
      /\b(password|token|cookie|authorization|secret|credential)\b([\s:=]+)[^\s,;]+/gi,
      '$1$2[redacted]',
    )
    .slice(0, 2_000);
}

export function sanitizeLogValue(
  value: unknown,
  key = '',
  seen = new WeakSet<object>(),
): unknown {
  if (SENSITIVE_KEY.test(key)) return '[redacted]';
  if (typeof value === 'string') return sanitizeString(value);
  if (value === null || typeof value !== 'object') return value;
  if (seen.has(value)) return '[circular]';
  seen.add(value);
  if (value instanceof Error) {
    return { name: value.name, message: sanitizeString(value.message) };
  }
  if (Array.isArray(value))
    return value.slice(0, 100).map((item) => sanitizeLogValue(item, key, seen));
  return Object.fromEntries(
    Object.entries(value)
      .slice(0, 100)
      .map(([childKey, childValue]) => [
        childKey,
        sanitizeLogValue(childValue, childKey, seen),
      ]),
  );
}

export function log(
  level: LogLevel,
  message: string,
  attributes: Record<string, unknown> = {},
): void {
  const serialized = sanitizeLogValue(attributes) as Record<string, unknown>;
  process.stdout.write(
    `${JSON.stringify({ time: new Date().toISOString(), level, message: sanitizeString(message), ...serialized })}\n`,
  );
}
