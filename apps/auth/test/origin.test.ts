import { describe, expect, it } from 'vitest';

import { evaluateBrowserOrigin } from '../src/security/origin.js';

const trusted = ['https://rsp.example.org', 'http://localhost:5173'];

describe('origin policy', () => {
  it('permits safe reads without an Origin header', () => {
    expect(evaluateBrowserOrigin('GET', null, trusted)).toEqual({
      allowed: true,
    });
  });

  it('permits trusted development and production origins', () => {
    expect(
      evaluateBrowserOrigin('POST', 'https://rsp.example.org', trusted).allowed,
    ).toBe(true);
    expect(
      evaluateBrowserOrigin('POST', 'http://localhost:5173', trusted).allowed,
    ).toBe(true);
  });

  it('rejects missing, malformed, opaque, and untrusted origins on writes', () => {
    expect(evaluateBrowserOrigin('POST', null, trusted)).toMatchObject({
      reason: 'missing_origin',
    });
    expect(evaluateBrowserOrigin('POST', 'not-a-url', trusted)).toMatchObject({
      reason: 'invalid_origin',
    });
    expect(evaluateBrowserOrigin('POST', 'null', trusted)).toMatchObject({
      reason: 'invalid_origin',
    });
    expect(
      evaluateBrowserOrigin('DELETE', 'https://evil.example', trusted),
    ).toMatchObject({
      reason: 'untrusted_origin',
    });
  });
});
