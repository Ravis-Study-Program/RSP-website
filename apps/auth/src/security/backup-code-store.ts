import { AsyncLocalStorage } from 'node:async_hooks';
import { randomBytes, scryptSync, timingSafeEqual } from 'node:crypto';

const VERSION = 's1';
const KEY_LENGTH = 32;
const requestCandidate = new AsyncLocalStorage<string | undefined>();

function hashBackupCode(code: string, salt = randomBytes(16)): string {
  const hash = scryptSync(code, salt, KEY_LENGTH);
  return `${VERSION}$${salt.toString('base64url')}$${hash.toString('base64url')}`;
}

function isStoredHash(value: string): boolean {
  return value.startsWith(`${VERSION}$`);
}

function matchesBackupCode(code: string, encoded: string): boolean {
  const [version, encodedSalt, encodedHash] = encoded.split('$');
  if (version !== VERSION || !encodedSalt || !encodedHash) return false;
  try {
    const salt = Buffer.from(encodedSalt, 'base64url');
    const expected = Buffer.from(encodedHash, 'base64url');
    const actual = scryptSync(code, salt, expected.length);
    return (
      actual.length === expected.length && timingSafeEqual(actual, expected)
    );
  } catch {
    return false;
  }
}

function parseArray(value: string): string[] {
  const parsed: unknown = JSON.parse(value);
  if (
    !Array.isArray(parsed) ||
    parsed.some((entry) => typeof entry !== 'string')
  ) {
    throw new Error('invalid backup-code storage payload');
  }
  return parsed;
}

/**
 * Better Auth's custom backup-code codec receives the entire JSON array. Its
 * verifier expects decrypt() to return strings it can compare and then passes
 * the remaining values back through encrypt(). We preserve already-hashed
 * entries and reveal only the request's matching candidate inside that
 * request-local context. The plugin therefore removes one match and writes the
 * remaining hashes without ever storing recoverable backup codes.
 */
export const hashedBackupCodeCodec = {
  async encrypt(serializedCodes: string): Promise<string> {
    return JSON.stringify(
      parseArray(serializedCodes).map((code) =>
        isStoredHash(code) ? code : hashBackupCode(code),
      ),
    );
  },

  async decrypt(serializedHashes: string): Promise<string> {
    const candidate = requestCandidate.getStore();
    const values = parseArray(serializedHashes).map((stored) =>
      candidate && matchesBackupCode(candidate, stored) ? candidate : stored,
    );
    return JSON.stringify(values);
  },
};

export function withBackupCodeCandidate<T>(
  candidate: string | undefined,
  operation: () => T,
): T {
  return requestCandidate.run(candidate, operation);
}

export function isHashedBackupCodePayload(serialized: string): boolean {
  try {
    const values = parseArray(serialized);
    return values.length > 0 && values.every(isStoredHash);
  } catch {
    return false;
  }
}
