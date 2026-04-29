import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate, useNavigation } from "react-router-dom";
import { logoutAdminSession } from "../../api/auth";
import { unauthorizedEventName } from "../../api/client";

const navItems = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/world-models", label: "World Models" },
  { to: "/generation", label: "Generation" },
  { to: "/files", label: "File Browser" },
  { to: "/events", label: "Event Log" },
  { to: "/deployments", label: "Deployments" },
  { to: "/settings", label: "Settings" },
];

export function AppShell() {
  const location = useLocation();
  const navigate = useNavigate();
  const navigation = useNavigation();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const isNavigating = navigation.state !== "idle";

  useEffect(() => {
    const handleUnauthorized = () => {
      const next = `${location.pathname}${location.search}`;
      void navigate(`/login?next=${encodeURIComponent(next)}`, { replace: true });
    };

    window.addEventListener(unauthorizedEventName, handleUnauthorized);
    return () => window.removeEventListener(unauthorizedEventName, handleUnauthorized);
  }, [location.pathname, location.search, navigate]);

  async function handleLogout() {
    setIsLoggingOut(true);
    try {
      await logoutAdminSession();
    } finally {
      setIsLoggingOut(false);
      await navigate("/login", { replace: true });
    }
  }

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <div className="app-brand">
          <span className="app-brand__eyebrow">Honeygen</span>
          <h1>Admin Console</h1>
          <p>Operate live world models, generation jobs, assets, and event review.</p>
        </div>
        <nav className="app-nav" aria-label="Primary">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              className={({ isActive }) => `app-nav__link${isActive ? " is-active" : ""}`}
              end={item.end}
              to={item.to}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="app-main">
        <header className="app-toolbar">
          <div>
            <strong>Authenticated admin session</strong>
            <span className="app-toolbar__subtitle"> Protected API loaders use the server-managed session cookie.</span>
          </div>
          <div className="app-toolbar__actions">
            {isNavigating ? <span className="loading-chip">Refreshing…</span> : null}
            <button className="button button--ghost button--small" disabled={isLoggingOut} onClick={handleLogout}>
              {isLoggingOut ? "Signing out…" : "Sign out"}
            </button>
          </div>
        </header>
        <main className="app-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
