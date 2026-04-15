import { useEffect, useState } from "react";
import { useLoaderData } from "react-router-dom";
import { listDeployments, createDeployment, deleteDeployment, startDeployment, stopDeployment } from "../api/deployments";
import { listGenerationJobs } from "../api/generation";
import { listWorldModels } from "../api/worldModels";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { ErrorAlert } from "../components/layout/ErrorAlert";
import type { Deployment } from "../types/deployments";
import type { GenerationJob } from "../types/generation";
import type { WorldModelSummary } from "../types/worldModels";
import { formatDateTime } from "../app/format";

interface DeploymentsLoaderData {
  deployments: Deployment[];
  jobs: GenerationJob[];
  models: WorldModelSummary[];
}

export async function deploymentsLoader(): Promise<DeploymentsLoaderData> {
  const [deployments, jobs, models] = await Promise.all([
    listDeployments(),
    listGenerationJobs({ limit: 50 }),
    listWorldModels(),
  ]);
  return { deployments, jobs, models };
}

export default function DeploymentsPage() {
  const loaderData = useLoaderData() as DeploymentsLoaderData;
  const [deployments, setDeployments] = useState(loaderData.deployments);
  const [error, setError] = useState<string>();
  const [formError, setFormError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Create form state
  const [selectedJobID, setSelectedJobID] = useState("");
  const [protocol, setProtocol] = useState("http");
  const [port, setPort] = useState("9000");
  const [rootPath, setRootPath] = useState("/");

  useEffect(() => {
    setDeployments(loaderData.deployments);
  }, [loaderData.deployments]);

  // Poll for status changes when any deployment is running
  useEffect(() => {
    const hasRunning = deployments.some((d) => d.status === "running");
    if (!hasRunning) return undefined;

    const interval = window.setInterval(async () => {
      try {
        const updated = await listDeployments();
        setDeployments(updated);
      } catch {
        // silent poll failure
      }
    }, 10000);
    return () => window.clearInterval(interval);
  }, [deployments]);

  function jobLabel(jobID: string): string {
    const job = loaderData.jobs.find((j) => j.id === jobID);
    if (!job) return jobID.slice(0, 8);
    const model = loaderData.models.find((m) => m.id === job.world_model_id);
    return `${model?.name ?? job.world_model_id} — ${job.id.slice(0, 8)}`;
  }

  function connectionTarget(deployment: Deployment): string {
    switch (deployment.protocol) {
      case "http":
        return `http://localhost:${deployment.port}/`;
      case "ftp":
        return `ftp://localhost:${deployment.port}/`;
      case "nfs":
        return `127.0.0.1:${deployment.mount_path ?? "/mount"}`;
      case "smb":
        return `//127.0.0.1/${deployment.share_name ?? "honeygen"}`;
      default:
        return "";
    }
  }

  function connectionHint(deployment: Deployment): string {
    switch (deployment.protocol) {
      case "ftp":
        return "Anonymous read-only. Passive-mode Windows clients work; Docker/Windows active-mode clients such as ftp.exe do not work reliably through Docker NAT.";
      case "nfs":
        return "WSL/Linux mount target. Use nfsvers=3,noacl,tcp,port=<port>,mountport=<port>,nolock,noresvport.";
      case "smb":
        return `Read-only guest share on port ${deployment.port}. Windows Explorer cannot use custom-port SMB on the same host; use WSL/Linux or a host that can expose port 445.`;
      default:
        return "";
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedJobID) return;

    const job = loaderData.jobs.find((j) => j.id === selectedJobID);
    if (!job) return;

    setIsSubmitting(true);
    setFormError(undefined);
    try {
      const created = await createDeployment({
        generation_job_id: selectedJobID,
        world_model_id: job.world_model_id,
        protocol,
        port: parseInt(port, 10),
        root_path: rootPath,
      });
      setDeployments((prev) => [created, ...prev]);
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to create deployment");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleStart(id: string) {
    setError(undefined);
    try {
      const updated = await startDeployment(id);
      setDeployments((prev) => prev.map((d) => (d.id === id ? updated : d)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start deployment");
    }
  }

  async function handleStop(id: string) {
    setError(undefined);
    try {
      const updated = await stopDeployment(id);
      setDeployments((prev) => prev.map((d) => (d.id === id ? updated : d)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to stop deployment");
    }
  }

  async function handleDelete(id: string) {
    setError(undefined);
    try {
      await deleteDeployment(id);
      setDeployments((prev) => prev.filter((d) => d.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete deployment");
    }
  }

  const completedJobs = loaderData.jobs.filter((j) => j.status === "completed");

  return (
    <div className="stack">
      <PageHeader
        title="Deployments"
        description="Deploy generated file trees as standalone servers over HTTP, FTP, NFS, or SMB. Each deployment serves one generation job's output on a dedicated port. The same files can be deployed across multiple protocols."
      />
      {error ? <ErrorAlert message={error} /> : null}
      <div className="two-column">
        <Panel title="Create deployment" subtitle="Serve a completed generation job on a dedicated port via HTTP, FTP, NFS, or SMB">
          <form className="stack stack--compact" onSubmit={handleCreate}>
            {formError ? <ErrorAlert message={formError} /> : null}
            <label>
              Generation job
              <select value={selectedJobID} onChange={(e) => setSelectedJobID(e.target.value)}>
                <option value="">Select a completed job…</option>
                {completedJobs.map((job) => (
                  <option key={job.id} value={job.id}>
                    {jobLabel(job.id)} ({job.status})
                  </option>
                ))}
              </select>
            </label>
            <label>
              Protocol
              <select value={protocol} onChange={(e) => setProtocol(e.target.value)}>
                <option value="http">HTTP</option>
                <option value="ftp">FTP</option>
                <option value="nfs">NFS</option>
                <option value="smb">SMB</option>
              </select>
              <span className="field-hint">
                {protocol === "http" && "Serve files over HTTP — ideal for web content."}
                {protocol === "ftp" && "Serve files over FTP — anonymous read-only access with passive-mode clients and passive data ports reserved inside the deployment port range."}
                {protocol === "nfs" && "Serve files over NFSv3 — mount 127.0.0.1:/mount from WSL or Linux using the selected port for both port and mountport."}
                {protocol === "smb" && "Serve files over SMB using the read-only guest share named honeygen on the selected port. Native Windows SMB clients require port 445, so localhost testing is best done from WSL/Linux clients."}
              </span>
            </label>
            <label>
              Port
              <input
                type="number"
                min="1024"
                max="65535"
                value={port}
                onChange={(e) => setPort(e.target.value)}
              />
              <span className="field-hint">TCP port for the {protocol.toUpperCase()} server (1024–65535).</span>
            </label>
            <label>
              Root path
              <input
                type="text"
                placeholder="/"
                value={rootPath}
                onChange={(e) => setRootPath(e.target.value)}
              />
              <span className="field-hint">Subtree only, e.g. "/" for all files or "/public" for public only. Do not include "/generated/&lt;world&gt;/&lt;job&gt;".</span>
            </label>
            <div className="button-row">
              <button className="button button--primary" type="submit" disabled={isSubmitting || !selectedJobID}>
                {isSubmitting ? "Creating…" : "Create deployment"}
              </button>
            </div>
          </form>
        </Panel>
        <Panel title="Active deployments" subtitle="Manage running and stopped deployments">
          {deployments.length === 0 ? (
            <p className="muted">No deployments yet. Create one to get started.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Protocol</th>
                  <th>Port</th>
                  <th>Job</th>
                  <th>Status</th>
                  <th>Access</th>
                  <th>Root</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {deployments.map((d) => (
                  <tr key={d.id}>
                    <td>
                      <span className="protocol-badge">{d.protocol.toUpperCase()}</span>
                    </td>
                    <td>
                      <strong>{d.port}</strong>
                    </td>
                    <td className="truncate-cell">
                      <span className="truncate">{jobLabel(d.generation_job_id)}</span>
                    </td>
                    <td>
                      <span className={`status-badge status-badge--${d.status}`}>{d.status}</span>
                    </td>
                    <td className="truncate-cell">
                      <div className="stack stack--compact">
                        <code className="truncate">{connectionTarget(d)}</code>
                        {connectionHint(d) ? <span className="field-hint">{connectionHint(d)}</span> : null}
                      </div>
                    </td>
                    <td><code>{d.root_path}</code></td>
                    <td>{formatDateTime(d.created_at)}</td>
                    <td>
                      <div className="button-row button-row--compact">
                        {d.status === "stopped" || d.status === "error" ? (
                          <button className="button button--small button--primary" onClick={() => handleStart(d.id)}>
                            Start
                          </button>
                        ) : null}
                        {d.status === "running" ? (
                          <button className="button button--small button--ghost" onClick={() => handleStop(d.id)}>
                            Stop
                          </button>
                        ) : null}
                        {d.status !== "running" ? (
                          <button className="button button--small button--danger" onClick={() => handleDelete(d.id)}>
                            Delete
                          </button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Panel>
      </div>
    </div>
  );
}
