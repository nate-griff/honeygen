import { isRouteErrorResponse, useRouteError } from "react-router-dom";

export function RouteErrorBoundary() {
  const error = useRouteError();
  const message =
    isRouteErrorResponse(error)
      ? error.statusText || "Request failed"
      : error instanceof Error
        ? error.message
        : "Unexpected application error";

  return (
    <div className="route-error">
      <h2>Unable to load the admin UI</h2>
      <p>{message}</p>
    </div>
  );
}
