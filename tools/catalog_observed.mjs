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

function bodyKeys(raw) {
  if (!raw) return [];
  try {
    const value = JSON.parse(raw);
    return value && typeof value === "object" && !Array.isArray(value)
      ? Object.keys(value)
      : [];
  } catch {
    return [...new URLSearchParams(raw).keys()];
  }
}

function normalizePath(pathname) {
  return pathname
    .replace(/\/[0-9]{3,}(?=\/|$)/g, "/{id}")
    .replace(/[/.]+$/, "");
}

// Applies one authenticated request to the nearest catalog path. Bundle
// literals often stop at a path prefix and append the final identifier at
// runtime, so matching walks from the full path toward /api/vN.
export function mergeCatalogObservation(catalog, request, at) {
  const url = new URL(request.url);
  let key = normalizePath(url.pathname);
  while (key.includes("/")) {
    const entry = catalog?.endpoints?.[key];
    if (entry) {
      entry.observed = mergeObservedMetadata(entry.observed ?? {}, {
        method: request.method,
        host: url.hostname.split(".")[0],
        query: [...url.searchParams.keys()],
        body: bodyKeys(request.postData),
      }, at);
      return key;
    }
    key = key.slice(0, key.lastIndexOf("/"));
    if (key.split("/").length <= 3) break;
  }
  return null;
}
