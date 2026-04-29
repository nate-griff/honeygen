import test from "node:test";
import assert from "node:assert/strict";

import { getIPIntelligence } from "./events.ts";

test("getIPIntelligence returns null when coordinates are not numbers", () => {
  const intel = getIPIntelligence({
    ip_intelligence: {
      source: "cache",
      geo: {
        country: "US",
        latitude: "40.7128",
        longitude: "-74.0060",
      },
    },
  });

  assert.equal(intel, null);
});

test("getIPIntelligence preserves valid numeric coordinates", () => {
  const intel = getIPIntelligence({
    ip_intelligence: {
      source: "cache",
      geo: {
        country: "US",
        latitude: 40.7128,
        longitude: -74.006,
      },
    },
  });

  assert.deepEqual(intel, {
    source: "cache",
    geo: {
      country: "US",
      latitude: 40.7128,
      longitude: -74.006,
    },
  });
});

test("getIPIntelligence normalizes the flat backend ip_intelligence payload", () => {
  const intel = getIPIntelligence({
    ip_intelligence: {
      source: "geoip+whois",
      organization: "Example ISP",
      network: "203.0.113.0/24",
      country: "United States",
      region: "New York",
      city: "New York",
      timezone: "America/New_York",
      latitude: 40.7128,
      longitude: -74.006,
    },
  });

  assert.deepEqual(intel, {
    source: "geoip+whois",
    whois: {
      organization: "Example ISP",
      network: "203.0.113.0/24",
    },
    geo: {
      country: "United States",
      region: "New York",
      city: "New York",
      timezone: "America/New_York",
      latitude: 40.7128,
      longitude: -74.006,
    },
  });
});
