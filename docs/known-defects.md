# Legacy defects and intentional fixes

- Request-body user IDs allowed actor spoofing in several legacy operations.
- Mock-interview ownership checks relied on caller-provided identities.
- Rich text could reach `dangerouslySetInnerHTML` without a server allowlist.
- Some cache keys omitted filters and malformed cursors fell back to page one.
- Graduate state was stored and updated by a job instead of derived from a
  completed student enrollment.

These are security/correctness fixes, not parity requirements.

