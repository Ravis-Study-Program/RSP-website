/** Recharts legend formatter: series colour stays on the swatch, label text keeps body contrast. */
export function legendLabel(value: string) {
  return <span style={{ color: 'var(--text)' }}>{value}</span>;
}
