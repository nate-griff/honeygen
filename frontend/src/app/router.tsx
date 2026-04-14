import { createBrowserRouter } from "react-router-dom";
import { AppShell } from "../components/layout/AppShell";
import { RouteErrorBoundary } from "../components/layout/RouteErrorBoundary";
import DashboardPage, { dashboardLoader } from "../pages/DashboardPage";
import DeploymentsPage, { deploymentsLoader } from "../pages/DeploymentsPage";
import EventLogPage, { eventLogLoader } from "../pages/EventLogPage";
import FileBrowserPage, { fileBrowserLoader } from "../pages/FileBrowserPage";
import GenerationPage, { generationLoader } from "../pages/GenerationPage";
import WorldModelsPage, { worldModelsLoader } from "../pages/WorldModelsPage";
import SettingsPage, { settingsLoader } from "../pages/SettingsPage";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <AppShell />,
    errorElement: <RouteErrorBoundary />,
    children: [
      {
        index: true,
        loader: dashboardLoader,
        element: <DashboardPage />,
      },
      {
        path: "world-models",
        loader: worldModelsLoader,
        element: <WorldModelsPage />,
      },
      {
        path: "generation",
        loader: generationLoader,
        element: <GenerationPage />,
      },
      {
        path: "files",
        loader: fileBrowserLoader,
        element: <FileBrowserPage />,
      },
      {
        path: "events",
        loader: eventLogLoader,
        element: <EventLogPage />,
      },
      {
        path: "deployments",
        loader: deploymentsLoader,
        element: <DeploymentsPage />,
      },
      {
        path: "settings",
        loader: settingsLoader,
        element: <SettingsPage />,
      },
    ],
  },
]);
