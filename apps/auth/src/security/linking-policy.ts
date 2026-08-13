export interface LinkingDecision {
  readonly allowed: boolean;
  readonly reason?:
    | "not_authenticated"
    | "reauthentication_required"
    | "unsupported_provider"
    | "different_email";
}

export function evaluateFreshSession(options: {
  readonly authenticatedUserId?: string;
  readonly sessionCreatedAt?: Date | string;
  readonly now: Date;
  readonly freshAgeSeconds: number;
}): LinkingDecision {
  if (!options.authenticatedUserId || !options.sessionCreatedAt) {
    return { allowed: false, reason: "not_authenticated" };
  }
  const createdAtMs = new Date(options.sessionCreatedAt).getTime();
  if (
    !Number.isFinite(createdAtMs) ||
    createdAtMs > options.now.getTime() ||
    options.now.getTime() - createdAtMs > options.freshAgeSeconds * 1_000
  ) {
    return { allowed: false, reason: "reauthentication_required" };
  }
  return { allowed: true };
}

export function evaluateExplicitLink(options: {
  readonly authenticatedUserId?: string;
  readonly sessionCreatedAt?: Date | string;
  readonly now: Date;
  readonly freshAgeSeconds: number;
  readonly provider: string;
  readonly currentEmail: string;
  readonly providerEmail?: string;
}): LinkingDecision {
  const freshness = evaluateFreshSession(options);
  if (!freshness.allowed) return freshness;
  if (options.provider !== "google") {
    return { allowed: false, reason: "unsupported_provider" };
  }
  if (
    options.providerEmail &&
    options.providerEmail.trim().toLowerCase() !== options.currentEmail.trim().toLowerCase()
  ) {
    return { allowed: false, reason: "different_email" };
  }
  return { allowed: true };
}
