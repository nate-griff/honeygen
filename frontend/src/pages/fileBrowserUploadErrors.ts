interface UploadAPIErrorShape {
  code: string;
  message: string;
}

function isUploadAPIError(error: unknown): error is UploadAPIErrorShape {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    typeof error.code === "string" &&
    "message" in error &&
    typeof error.message === "string"
  );
}

export function getUploadErrorMessage(error: unknown): string {
  if (isUploadAPIError(error)) {
    if (error.code === "path_conflict") {
      return "A file already exists at that path. Uploads cannot overwrite existing files.";
    }
    if (error.code === "job_not_completed") {
      return "Uploads are only allowed for completed generation jobs.";
    }
    if (error.code === "validation_error") {
      return `Validation error: ${error.message}`;
    }
    return error.message || "Upload failed. Please try again.";
  }

  return "Upload failed. Please try again.";
}
