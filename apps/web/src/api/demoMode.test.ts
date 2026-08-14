import { demoModeForEnvironment } from '@/api/demoMode';

describe('demo data release guard', () => {
  it('requires the explicit flag and refuses it in a production build', () => {
    expect(demoModeForEnvironment(undefined, 'development')).toBe(false);
    expect(demoModeForEnvironment('false', 'demo')).toBe(false);
    expect(demoModeForEnvironment('true', 'demo')).toBe(true);
    expect(demoModeForEnvironment('true', 'production')).toBe(false);
  });
});
