import test from "node:test";
import assert from "node:assert/strict";

import { sanitizeNextPath } from "./safeRedirect.ts";

test("sanitizeNextPath preserves safe in-app routes", () => {
  assert.equal(sanitizeNextPath("/events?filter=recent#top"), "/events?filter=recent#top");
});

test("sanitizeNextPath rejects external next targets", () => {
  assert.equal(sanitizeNextPath("https://evil.example/steal"), "/");
  assert.equal(sanitizeNextPath("//evil.example/steal"), "/");
});

test("sanitizeNextPath rejects non-root relative targets", () => {
  assert.equal(sanitizeNextPath("events"), "/");
  assert.equal(sanitizeNextPath("javascript:alert(1)"), "/");
});
