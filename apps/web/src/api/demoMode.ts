export function demoModeForEnvironment(
  value: string | undefined,
  mode: string,
) {
  return value === 'true' && ['development', 'test', 'demo'].includes(mode);
}
