import { readFile } from "node:fs/promises";

import { getAuthTables } from "better-auth/db";
import { afterAll, describe, expect, it } from "vitest";

import { auth, pool } from "../src/auth.js";

afterAll(async () => {
  await pool.end();
});

describe("pinned Better Auth migration", () => {
  it("contains every table and mapped column exposed by Better Auth 1.6.27", async () => {
    const migration = await readFile(new URL("../migrations/better-auth.sql", import.meta.url), "utf8");
    const tables = getAuthTables(auth.options);
    for (const table of Object.values(tables)) {
      expect(migration, `missing table ${table.modelName}`).toMatch(
        new RegExp(`CREATE TABLE ${table.modelName}\\s*\\(`),
      );
      expect(migration, `missing id for ${table.modelName}`).toMatch(/\bid\s+text\s+PRIMARY KEY/);
      for (const [logicalName, field] of Object.entries(table.fields)) {
        const column = field.fieldName ?? logicalName;
        expect(migration, `missing ${table.modelName}.${column}`).toMatch(
          new RegExp(`\\b${column}\\b`),
        );
      }
    }
  });
});
