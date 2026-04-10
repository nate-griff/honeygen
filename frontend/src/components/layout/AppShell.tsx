import { NavLink, Outlet, useNavigation } from "react-router-dom";

const navItems = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/world-models", label: "World Models" },
  { to: "/generation", label: "Generation" },
  { to: "/files", label: "File Browser" },
  { to: "/events", label: "Event Log" },
];

export function AppShell() {
  const navigation = useNavigation();
  const isNavigating = navigation.state !== "idle";

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
            <strong>Live API mode</strong>
            <span className="app-toolbar__subtitle"> Route loaders fetch backend data directly.</span>
          </div>
          {isNavigating ? <span className="loading-chip">Refreshing…</span> : null}
        </header>
        <main className="app-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
