import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';

import { getAuthTables } from 'better-auth/db';
import { afterAll, describe, expect, it } from 'vitest';

import { auth, pool } from '../src/auth.js';

afterAll(async () => {
  await pool.end();
});

describe('pinned Better Auth migration', () => {
  it('contains every table and mapped column exposed by Better Auth 1.6.27', async () => {
    const migration = await readFile(
      new URL('../migrations/better-auth.sql', import.meta.url),
      'utf8',
    );
    const tables = getAuthTables(auth.options);
    for (const table of Object.values(tables)) {
      expect(migration, `missing table ${table.modelName}`).toMatch(
        new RegExp(`CREATE TABLE ${table.modelName}\\s*\\(`),
      );
      expect(migration, `missing id for ${table.modelName}`).toMatch(
        /\bid\s+text\s+PRIMARY KEY/,
      );
      for (const [logicalName, field] of Object.entries(table.fields)) {
        const column = field.fieldName ?? logicalName;
        expect(migration, `missing ${table.modelName}.${column}`).toMatch(
          new RegExp(`\\b${column}\\b`),
        );
      }
    }
  });

  it('pins every versioned migration checksum in the repeatable runner', async () => {
    const baseline = await readFile(
      new URL('../migrations/better-auth.sql', import.meta.url),
      'utf8',
    );
    const companion = await readFile(
      new URL('../migrations/0002-lifecycle-outbox.sql', import.meta.url),
      'utf8',
    );
    const runner = await readFile(
      new URL('../../../deploy/postgres/migrate-auth.sql', import.meta.url),
      'utf8',
    );
    const digest = (value: string) =>
      createHash('sha256').update(value).digest('hex');
    expect(digest(baseline)).toBe(
      '17f1d2e1a897e44eca5a01a7204f658e5bb3f54e92d10f6a03e3c30a374662d5',
    );
    expect(digest(companion)).toBe(
      '44a4e5a1b012d49ca6399b078a76787f6a3e10117bdf3f4dce5a234d9fed900a',
    );
    expect(runner).toContain(
      `'0001-better-auth-1.6.27', '${digest(baseline)}'`,
    );
    expect(runner).toContain(`'0002-lifecycle-outbox', '${digest(companion)}'`);
    expect(companion).toContain(
      'CREATE TABLE IF NOT EXISTS account_deletion_recovery_tokens',
    );
    expect(companion).toContain('CREATE TABLE IF NOT EXISTS lifecycle_outbox');
  });
});
