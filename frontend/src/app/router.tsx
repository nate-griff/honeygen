import { createBrowserRouter, redirect, type LoaderFunctionArgs } from "react-router-dom";
import { APIClientError } from "../api/client";
import { getAdminSession, requireAdminSession } from "../api/auth";
import { AppShell } from "../components/layout/AppShell";
import { RouteErrorBoundary } from "../components/layout/RouteErrorBoundary";
import DashboardPage, { dashboardLoader } from "../pages/DashboardPage";
import DeploymentsPage, { deploymentsLoader } from "../pages/DeploymentsPage";
import EventLogPage, { eventLogLoader } from "../pages/EventLogPage";
import FileBrowserPage, { fileBrowserLoader } from "../pages/FileBrowserPage";
import GenerationPage, { generationLoader } from "../pages/GenerationPage";
import LoginPage from "../pages/LoginPage";
import SettingsPage, { settingsLoader } from "../pages/SettingsPage";
import WorldModelsPage, { worldModelsLoader } from "../pages/WorldModelsPage";
import { sanitizeNextPath } from "./safeRedirect";

function withAuth<T>(loader: (args: LoaderFunctionArgs) => Promise<T>) {
  return async (args: LoaderFunctionArgs) => {
    const url = new URL(args.request.url);
    await requireAdminSession(`${url.pathname}${url.search}`);
    return loader(args);
  };
}

async function loginLoader({ request }: LoaderFunctionArgs) {
  const url = new URL(request.url);
  try {
    await getAdminSession();
    throw redirect(sanitizeNextPath(url.searchParams.get("next")));
  } catch (error) {
    if (error instanceof APIClientError && error.status === 401) {
      return null;
    }
    throw error;
  }
}

export const router = createBrowserRouter([
  {
    path: "/login",
    loader: loginLoader,
    element: <LoginPage />,
    errorElement: <RouteErrorBoundary />,
  },
  {
    path: "/",
    element: <AppShell />,
    errorElement: <RouteErrorBoundary />,
    children: [
      {
        index: true,
        loader: withAuth(async () => dashboardLoader()),
        element: <DashboardPage />,
      },
      {
        path: "world-models",
        loader: withAuth(async (args) => worldModelsLoader(args)),
        element: <WorldModelsPage />,
      },
      {
        path: "generation",
        loader: withAuth(async (args) => generationLoader(args)),
        element: <GenerationPage />,
      },
      {
        path: "files",
        loader: withAuth(async (args) => fileBrowserLoader(args)),
        element: <FileBrowserPage />,
      },
      {
        path: "events",
        loader: withAuth(async (args) => eventLogLoader(args)),
        element: <EventLogPage />,
      },
      {
        path: "deployments",
        loader: withAuth(async (args) => deploymentsLoader(args)),
        element: <DeploymentsPage />,
      },
      {
        path: "settings",
        loader: withAuth(async () => settingsLoader()),
        element: <SettingsPage />,
      },
    ],
  },
]);
