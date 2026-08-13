import { z } from "zod";

const LOCAL_SECRET = "rsp-local-auth-secret-change-before-production";
const LOCAL_IDENTITY_TOKEN = "rsp-local-identity-service-token-change-me";

const booleanFromEnvironment = z.preprocess((value) => {
  if (typeof value !== "string") return value;
  const normalized = value.trim().toLowerCase();
  if (normalized === "true" || normalized === "1") return true;
  if (normalized === "false" || normalized === "0") return false;
  return value;
}, z.boolean());

const optionalNonEmptyString = z.preprocess(
  (value) => (typeof value === "string" && value.trim() === "" ? undefined : value),
  z.string().trim().min(1).optional(),
);

const environmentSchema = z
  .object({
    NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
    AUTH_HOST: z.string().trim().min(1).default("0.0.0.0"),
    AUTH_PORT: z.coerce.number().int().min(1).max(65_535).default(3001),
    BETTER_AUTH_URL: z.url().default("http://localhost:8080"),
    BETTER_AUTH_SECRET: z.string().min(32).default(LOCAL_SECRET),
    DATABASE_URL: z
      .string()
      .url()
      .default("postgresql://rsp:rsp@localhost:5432/rsp?options=-c%20search_path%3Dauth"),
    AUTH_TRUSTED_ORIGINS: z
      .string()
      .default("http://localhost:8080,http://localhost:5173"),
    AUTH_COOKIE_SECURE: booleanFromEnvironment.default(false),
    AUTH_TRUST_PROXY_HEADERS: booleanFromEnvironment.default(false),
    AUTH_IP_ADDRESS_HEADER: z.string().trim().min(1).default("x-real-ip"),
    AUTH_MFA_MAX_AGE_SECONDS: z.coerce.number().int().min(60).max(3_600).default(300),
    RSP_API_AUDIENCE: z.string().trim().min(1).default("rsp-api"),
    GOOGLE_CLIENT_ID: optionalNonEmptyString,
    GOOGLE_CLIENT_SECRET: optionalNonEmptyString,
    EMAIL_PROVIDER: z.enum(["smtp", "ses"]).default("smtp"),
    EMAIL_FROM: z.string().trim().min(3).default("RSP <no-reply@rsp.local>"),
    SMTP_HOST: z.string().trim().min(1).default("localhost"),
    SMTP_PORT: z.coerce.number().int().min(1).max(65_535).default(1025),
    SMTP_SECURE: booleanFromEnvironment.default(false),
    AWS_REGION: z.string().trim().min(1).default("ap-southeast-2"),
    IDENTITY_SERVICE_URL: z.url().default("http://localhost:8081"),
    IDENTITY_SERVICE_TOKEN: z.string().min(32).default(LOCAL_IDENTITY_TOKEN),
    IDENTITY_CALLBACK_TIMEOUT_MS: z.coerce.number().int().min(100).max(30_000).default(3_000),
  })
  .superRefine((environment, context) => {
    const googleConfigured = Boolean(environment.GOOGLE_CLIENT_ID);
    if (googleConfigured !== Boolean(environment.GOOGLE_CLIENT_SECRET)) {
      context.addIssue({
        code: "custom",
        message: "GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set together",
        path: [googleConfigured ? "GOOGLE_CLIENT_SECRET" : "GOOGLE_CLIENT_ID"],
      });
    }

    const baseUrl = new URL(environment.BETTER_AUTH_URL);
    if (baseUrl.pathname !== "/" || baseUrl.search || baseUrl.hash) {
      context.addIssue({
        code: "custom",
        message: "BETTER_AUTH_URL must be an origin without a path, query, or fragment",
        path: ["BETTER_AUTH_URL"],
      });
    }

    if (environment.NODE_ENV === "production") {
      if (environment.BETTER_AUTH_SECRET === LOCAL_SECRET) {
        context.addIssue({
          code: "custom",
          message: "BETTER_AUTH_SECRET must be replaced in production",
          path: ["BETTER_AUTH_SECRET"],
        });
      }
      if (environment.IDENTITY_SERVICE_TOKEN === LOCAL_IDENTITY_TOKEN) {
        context.addIssue({
          code: "custom",
          message: "IDENTITY_SERVICE_TOKEN must be replaced in production",
          path: ["IDENTITY_SERVICE_TOKEN"],
        });
      }
      if (!environment.AUTH_COOKIE_SECURE) {
        context.addIssue({
          code: "custom",
          message: "AUTH_COOKIE_SECURE must be true in production",
          path: ["AUTH_COOKIE_SECURE"],
        });
      }
      if (!environment.GOOGLE_CLIENT_ID || !environment.GOOGLE_CLIENT_SECRET) {
        context.addIssue({
          code: "custom",
          message: "Google credentials are required in production",
          path: ["GOOGLE_CLIENT_ID"],
        });
      }
      if (environment.EMAIL_PROVIDER !== "ses") {
        context.addIssue({
          code: "custom",
          message: "EMAIL_PROVIDER must be ses in production",
          path: ["EMAIL_PROVIDER"],
        });
      }
    }
  });

function parseOrigins(value: string, baseUrl: string): readonly string[] {
  const origins = value
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean)
    .map((entry) => {
      const url = new URL(entry);
      if (url.origin !== entry.replace(/\/$/, "")) {
        throw new Error(`AUTH_TRUSTED_ORIGINS entry must be an origin: ${entry}`);
      }
      return url.origin;
    });
  return [...new Set([new URL(baseUrl).origin, ...origins])];
}

export interface AuthConfig {
  readonly nodeEnvironment: "development" | "test" | "production";
  readonly host: string;
  readonly port: number;
  readonly baseUrl: string;
  readonly secret: string;
  readonly databaseUrl: string;
  readonly trustedOrigins: readonly string[];
  readonly secureCookies: boolean;
  readonly trustProxyHeaders: boolean;
  readonly ipAddressHeader: string;
  readonly mfaMaxAgeSeconds: number;
  readonly apiAudience: string;
  readonly google?: { readonly clientId: string; readonly clientSecret: string };
  readonly email:
    | {
        readonly provider: "smtp";
        readonly from: string;
        readonly host: string;
        readonly port: number;
        readonly secure: boolean;
      }
    | { readonly provider: "ses"; readonly from: string; readonly region: string };
  readonly identityService: {
    readonly baseUrl: string;
    readonly token: string;
    readonly timeoutMs: number;
  };
}

export function loadConfig(environment: NodeJS.ProcessEnv = process.env): AuthConfig {
  const parsed = environmentSchema.parse(environment);
  const google =
    parsed.GOOGLE_CLIENT_ID && parsed.GOOGLE_CLIENT_SECRET
      ? { clientId: parsed.GOOGLE_CLIENT_ID, clientSecret: parsed.GOOGLE_CLIENT_SECRET }
      : undefined;
  const email =
    parsed.EMAIL_PROVIDER === "ses"
      ? ({ provider: "ses", from: parsed.EMAIL_FROM, region: parsed.AWS_REGION } as const)
      : ({
          provider: "smtp",
          from: parsed.EMAIL_FROM,
          host: parsed.SMTP_HOST,
          port: parsed.SMTP_PORT,
          secure: parsed.SMTP_SECURE,
        } as const);

  return {
    nodeEnvironment: parsed.NODE_ENV,
    host: parsed.AUTH_HOST,
    port: parsed.AUTH_PORT,
    baseUrl: parsed.BETTER_AUTH_URL.replace(/\/$/, ""),
    secret: parsed.BETTER_AUTH_SECRET,
    databaseUrl: parsed.DATABASE_URL,
    trustedOrigins: parseOrigins(parsed.AUTH_TRUSTED_ORIGINS, parsed.BETTER_AUTH_URL),
    secureCookies: parsed.AUTH_COOKIE_SECURE,
    trustProxyHeaders: parsed.AUTH_TRUST_PROXY_HEADERS,
    ipAddressHeader: parsed.AUTH_IP_ADDRESS_HEADER.toLowerCase(),
    mfaMaxAgeSeconds: parsed.AUTH_MFA_MAX_AGE_SECONDS,
    apiAudience: parsed.RSP_API_AUDIENCE,
    google,
    email,
    identityService: {
      baseUrl: parsed.IDENTITY_SERVICE_URL.replace(/\/$/, ""),
      token: parsed.IDENTITY_SERVICE_TOKEN,
      timeoutMs: parsed.IDENTITY_CALLBACK_TIMEOUT_MS,
    },
  };
}

export const config = loadConfig();
