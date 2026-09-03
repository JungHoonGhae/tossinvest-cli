// Merge authenticated sweep metadata without retaining response values. The
// hand-audited source field is intentionally sticky across later live sweeps.
export function mergeObservedMetadata(previous = {}, current, at) {
  const mergeKeys = (left = [], right = []) =>
    [...new Set([...left, ...right])].sort();
  return {
    method: current.method,
    host: current.host,
    query: mergeKeys(previous.query, current.query),
    body: mergeKeys(previous.body, current.body),
    at,
    ...(previous.source && { source: previous.source }),
  };
}
