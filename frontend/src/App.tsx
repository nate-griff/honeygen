const apiBaseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

export default function App() {
  return (
    <main style={{ fontFamily: "system-ui, sans-serif", margin: "0 auto", maxWidth: "48rem", padding: "3rem 1.5rem" }}>
      <span style={{ color: "#4b5563", fontSize: "0.875rem", fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase" }}>
        Honeygen
      </span>
      <h1 style={{ fontSize: "2.5rem", marginBottom: "0.5rem" }}>Admin web skeleton</h1>
      <p style={{ color: "#374151", lineHeight: 1.6 }}>
        This Vite-powered placeholder confirms the monorepo runtime wiring for the Decoy Research Platform.
      </p>
      <dl style={{ background: "#f3f4f6", borderRadius: "0.75rem", marginTop: "2rem", padding: "1.25rem" }}>
        <dt style={{ fontWeight: 700 }}>Configured API base URL</dt>
        <dd style={{ margin: "0.5rem 0 0" }}><code>{apiBaseUrl}</code></dd>
      </dl>
    </main>
  );
}
