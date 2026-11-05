export function compareFindings(base, head, metadataFields = []) {
  const result = { added: [], changedExposure: [], inherited: [], removed: [] };
  for (const [key, finding] of head) {
    const previous = base.get(key);
    if (!previous) {
      result.added.push(finding);
      continue;
    }
    for (const field of metadataFields) {
      if (finding[field] !== previous[field]) {
        throw new Error(`Advisory data changed between scans: ${finding.id}`);
      }
    }
    const addedExposures = finding.exposures.filter(
      (exposure) => !previous.exposures.includes(exposure),
    );
    if (addedExposures.length) {
      result.changedExposure.push({ ...finding, addedExposures });
    } else {
      result.inherited.push(finding);
    }
  }
  for (const [key, finding] of base) {
    if (!head.has(key)) result.removed.push(finding);
  }
  return result;
}
