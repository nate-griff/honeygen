export interface FileBrowserSelection {
  worldModelID: string;
  generationJobID: string;
  assetID: string;
}

interface DeleteSuccessActionInput {
  currentSelection: FileBrowserSelection;
  deletingSelection: FileBrowserSelection;
}

export function getDeleteSuccessAction({
  currentSelection,
  deletingSelection,
}: DeleteSuccessActionInput): "clear-selection" | "revalidate" {
  if (
    currentSelection.worldModelID === deletingSelection.worldModelID &&
    currentSelection.generationJobID === deletingSelection.generationJobID &&
    currentSelection.assetID === deletingSelection.assetID
  ) {
    return "clear-selection";
  }

  return "revalidate";
}
