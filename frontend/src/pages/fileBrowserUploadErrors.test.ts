import test from "node:test";
import assert from "node:assert/strict";
import { getUploadErrorMessage } from "./fileBrowserUploadErrors.ts";

test("getUploadErrorMessage maps path_conflict to the no-overwrite message", () => {
  const message = getUploadErrorMessage({ code: "path_conflict", message: "conflict" });

  assert.equal(message, "A file already exists at that path. Uploads cannot overwrite existing files.");
});

test("getUploadErrorMessage falls back to API messages for other API errors", () => {
  const message = getUploadErrorMessage({ code: "upload_too_large", message: "Too big" });

  assert.equal(message, "Too big");
});

test("getUploadErrorMessage falls back to the generic message for non-API errors", () => {
  const message = getUploadErrorMessage(new Error("boom"));

  assert.equal(message, "Upload failed. Please try again.");
});
