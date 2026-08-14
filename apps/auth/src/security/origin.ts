const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

export type OriginDecision =
  | { readonly allowed: true; readonly origin?: string }
  | {
      readonly allowed: false;
      readonly reason: 'missing_origin' | 'invalid_origin' | 'untrusted_origin';
    };

export function isStateChangingMethod(method: string): boolean {
  return !SAFE_METHODS.has(method.toUpperCase());
}

export function evaluateBrowserOrigin(
  method: string,
  originHeader: string | null,
  trustedOrigins: readonly string[],
): OriginDecision {
  if (!isStateChangingMethod(method)) return { allowed: true };
  if (!originHeader) return { allowed: false, reason: 'missing_origin' };

  let origin: string;
  try {
    const url = new URL(originHeader);
    if (url.origin === 'null' || url.username || url.password) {
      return { allowed: false, reason: 'invalid_origin' };
    }
    origin = url.origin;
  } catch {
    return { allowed: false, reason: 'invalid_origin' };
  }

  return trustedOrigins.includes(origin)
    ? { allowed: true, origin }
    : { allowed: false, reason: 'untrusted_origin' };
}
