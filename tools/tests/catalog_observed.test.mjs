import assert from "node:assert/strict";
import test from "node:test";

import { mergeCatalogObservation, mergeObservedMetadata } from "../catalog_observed.mjs";

test("authenticated sweep preserves source evidence and merges key names", () => {
  const got = mergeObservedMetadata(
    {
      query: ["guid", "existing"],
      body: ["before"],
      source: "bundle-literal: audited call site",
    },
    {
      method: "POST",
      host: "wts-cert-api",
      query: ["guid", "new"],
      body: ["after", "before"],
    },
    "2026-09-03",
  );

  assert.deepEqual(got, {
    method: "POST",
    host: "wts-cert-api",
    query: ["existing", "guid", "new"],
    body: ["after", "before"],
    at: "2026-09-03",
    source: "bundle-literal: audited call site",
  });
});

test("source stays absent when no audited evidence exists", () => {
  const got = mergeObservedMetadata(
    {},
    { method: "GET", host: "wts-api", query: [], body: [] },
    "2026-09-03",
  );
  assert.equal(Object.hasOwn(got, "source"), false);
});

test("capture sweep wiring merges a live request into the longest catalog prefix", () => {
  const catalog = {
    endpoints: {
      "/api/v2/stock-infos": {
        observed: {
          query: ["existing"],
          body: ["before"],
          source: "bundle-literal: audited call site",
        },
      },
    },
  };

  const key = mergeCatalogObservation(catalog, {
    method: "POST",
    url: "https://wts-info-api.tossinvest.com/api/v2/stock-infos/A005930?guid=private&existing=private",
    postData: JSON.stringify({ after: "private", before: "private" }),
  }, "2026-09-03");

  assert.equal(key, "/api/v2/stock-infos");
  assert.deepEqual(catalog.endpoints[key].observed, {
    method: "POST",
    host: "wts-info-api",
    query: ["existing", "guid"],
    body: ["after", "before"],
    at: "2026-09-03",
    source: "bundle-literal: audited call site",
  });
});
