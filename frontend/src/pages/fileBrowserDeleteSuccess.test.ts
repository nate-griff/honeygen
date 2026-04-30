import test from "node:test";
import assert from "node:assert/strict";

import { getDeleteSuccessAction, type FileBrowserSelection } from "./fileBrowserDeleteSuccess.ts";

function selection(overrides: Partial<FileBrowserSelection> = {}): FileBrowserSelection {
  return {
    worldModelID: "northbridge-financial",
    generationJobID: "job-123",
    assetID: "asset-123",
    ...overrides,
  };
}

test("getDeleteSuccessAction clears the asset query when the deleted asset is still selected", () => {
  assert.equal(
    getDeleteSuccessAction({
      currentSelection: selection(),
      deletingSelection: selection(),
    }),
    "clear-selection",
  );
});

test("getDeleteSuccessAction revalidates when the user picks a different asset during delete", () => {
  assert.equal(
    getDeleteSuccessAction({
      currentSelection: selection({ assetID: "asset-456" }),
      deletingSelection: selection(),
    }),
    "revalidate",
  );
});

test("getDeleteSuccessAction revalidates when the user changes filters during delete", () => {
  assert.equal(
    getDeleteSuccessAction({
      currentSelection: selection({ worldModelID: "other-world", generationJobID: "job-999", assetID: "" }),
      deletingSelection: selection(),
    }),
    "revalidate",
  );
});
