import { useEffect, useMemo, useState } from "react";
import { useLoaderData, useNavigate } from "react-router-dom";
import { listGenerationJobs, runGeneration } from "../api/generation";
import { getStatus } from "../api/status";
import { listWorldModels } from "../api/worldModels";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { GenerationJobsList } from "../components/generation/GenerationJobsList";
import { GenerationRunPanel } from "../components/generation/GenerationRunPanel";
import type { GenerationJob } from "../types/generation";
import type { StatusResponse } from "../types/status";
import type { WorldModelSummary } from "../types/worldModels";

interface GenerationLoaderData {
  models: WorldModelSummary[];
  jobs: GenerationJob[];
  selectedWorldModelID: string;
  status: StatusResponse;
}

export async function generationLoader({ request }: { request: Request }): Promise<GenerationLoaderData> {
  const url = new URL(request.url);
  const requestedID = url.searchParams.get("world_model_id")?.trim() ?? "";
  const [models, status] = await Promise.all([listWorldModels(), getStatus()]);
  const selectedWorldModelID = requestedID || models[0]?.id || "";
  const jobs = await listGenerationJobs({
    world_model_id: selectedWorldModelID || undefined,
    limit: 10,
  });

  return { models, jobs, selectedWorldModelID, status };
}

export default function GenerationPage() {
  const loaderData = useLoaderData() as GenerationLoaderData;
  const navigate = useNavigate();
  const [jobs, setJobs] = useState(loaderData.jobs);
  const [selectedWorldModelID, setSelectedWorldModelID] = useState(loaderData.selectedWorldModelID);
  const [runError, setRunError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    setJobs(loaderData.jobs);
    setSelectedWorldModelID(loaderData.selectedWorldModelID);
  }, [loaderData.jobs, loaderData.selectedWorldModelID]);

  const shouldPoll = useMemo(
    () => jobs.some((job) => job.status === "pending" || job.status === "running"),
    [jobs],
  );

  useEffect(() => {
    if (!shouldPoll || !selectedWorldModelID) {
      return undefined;
    }

    const interval = window.setInterval(async () => {
      const nextJobs = await listGenerationJobs({ world_model_id: selectedWorldModelID, limit: 10 });
      setJobs(nextJobs);
    }, 5000);

    return () => window.clearInterval(interval);
  }, [selectedWorldModelID, shouldPoll]);

  async function handleRun() {
    if (!selectedWorldModelID) {
      return;
    }

    setIsSubmitting(true);
    setRunError(undefined);
    try {
      const createdJob = await runGeneration(selectedWorldModelID);
      const nextJobs = await listGenerationJobs({ world_model_id: selectedWorldModelID, limit: 10 });
      setJobs(nextJobs.some((job) => job.id === createdJob.id) ? nextJobs : [createdJob, ...nextJobs]);
    } catch (error) {
      setRunError(error instanceof Error ? error.message : "Unable to run generation");
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleWorldModelChange(id: string) {
    setSelectedWorldModelID(id);
    navigate(`/generation?world_model_id=${id}`);
  }

  return (
    <div className="stack">
      <PageHeader
        title="Generation"
        description="Trigger live generation runs, monitor job history, and poll while work is still active."
      />
      <div className="two-column">
        <Panel title="Run generation" subtitle="POST /api/generation/run with the selected world model">
          <GenerationRunPanel
            isRunning={isSubmitting}
            models={loaderData.models}
            onRun={handleRun}
            onSelectWorldModel={handleWorldModelChange}
            providerReady={loaderData.status.provider.ready}
            runError={runError}
            selectedWorldModelID={selectedWorldModelID}
          />
        </Panel>
        <Panel title="System context" subtitle="Current readiness snapshot from /api/status">
          <dl className="detail-grid">
            <div>
              <dt>Service</dt>
              <dd>{loaderData.status.service.name}</dd>
            </div>
            <div>
              <dt>Version</dt>
              <dd>{loaderData.status.service.version}</dd>
            </div>
            <div>
              <dt>Provider model</dt>
              <dd>{loaderData.status.provider.model || "—"}</dd>
            </div>
            <div>
              <dt>Base URL</dt>
              <dd>{loaderData.status.provider.base_url || "—"}</dd>
            </div>
          </dl>
        </Panel>
      </div>
      <Panel title="Recent jobs" subtitle="Latest live jobs for the selected world model">
        <GenerationJobsList jobs={jobs} />
      </Panel>
    </div>
  );
}
