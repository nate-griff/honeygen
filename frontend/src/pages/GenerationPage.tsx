import { useEffect, useMemo, useState } from "react";
import { useLoaderData, useNavigate } from "react-router-dom";
import { cancelGenerationJob, deleteGenerationJob, listGenerationJobs, runGeneration } from "../api/generation";
import { getStatus } from "../api/status";
import { listWorldModels } from "../api/worldModels";
import { ErrorAlert } from "../components/layout/ErrorAlert";
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
  const [cancelError, setCancelError] = useState<string>();
  const [deleteError, setDeleteError] = useState<string>();
  const [pollError, setPollError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [cancelingJobIDs, setCancelingJobIDs] = useState<string[]>([]);
  const [deletingJobIDs, setDeletingJobIDs] = useState<string[]>([]);

  useEffect(() => {
    setJobs(loaderData.jobs);
    setSelectedWorldModelID(loaderData.selectedWorldModelID);
    setCancelError(undefined);
    setCancelingJobIDs([]);
    setDeleteError(undefined);
    setDeletingJobIDs([]);
    setPollError(undefined);
  }, [loaderData.jobs, loaderData.selectedWorldModelID]);

  const shouldPoll = useMemo(
    () => jobs.some((job) => job.status === "pending" || job.status === "running"),
    [jobs],
  );

  useEffect(() => {
    if (!shouldPoll || !selectedWorldModelID) {
      return undefined;
    }

    let isActive = true;
    const pollJobs = async () => {
      try {
        const nextJobs = await listGenerationJobs({ world_model_id: selectedWorldModelID, limit: 10 });
        if (!isActive) {
          return;
        }

        setJobs(nextJobs);
        setPollError(undefined);
      } catch (error) {
        if (!isActive) {
          return;
        }

        setPollError(error instanceof Error ? error.message : "Unable to refresh generation jobs");
      }
    };

    void pollJobs();
    const interval = window.setInterval(async () => {
      await pollJobs();
    }, 5000);

    return () => {
      isActive = false;
      window.clearInterval(interval);
    };
  }, [selectedWorldModelID, shouldPoll]);

  async function handleRun() {
    if (!selectedWorldModelID) {
      return;
    }

    setIsSubmitting(true);
    setRunError(undefined);
    setCancelError(undefined);
    setPollError(undefined);
    try {
      const createdJob = await runGeneration(selectedWorldModelID);
      setJobs((currentJobs) => [createdJob, ...currentJobs.filter((job) => job.id !== createdJob.id)]);
      setPollError(undefined);
    } catch (error) {
      setRunError(error instanceof Error ? error.message : "Unable to run generation");
    } finally {
      setIsSubmitting(false);
    }
  }

  function handleWorldModelChange(id: string) {
    setSelectedWorldModelID(id);
    setCancelError(undefined);
    setPollError(undefined);
    navigate(`/generation?world_model_id=${id}`);
  }

  async function handleCancel(jobID: string) {
    setCancelError(undefined);
    setPollError(undefined);
    setCancelingJobIDs((current) => [...current, jobID]);
    try {
      const canceledJob = await cancelGenerationJob(jobID);
      setJobs((currentJobs) => currentJobs.map((job) => (job.id === canceledJob.id ? canceledJob : job)));
    } catch (error) {
      setCancelError(error instanceof Error ? error.message : "Unable to cancel generation job");
    } finally {
      setCancelingJobIDs((current) => current.filter((id) => id !== jobID));
    }
  }

  async function handleDelete(jobID: string) {
    setDeleteError(undefined);
    setPollError(undefined);
    setDeletingJobIDs((current) => [...current, jobID]);
    try {
      await deleteGenerationJob(jobID);
      setJobs((currentJobs) => currentJobs.filter((job) => job.id !== jobID));
    } catch (error) {
      setDeleteError(error instanceof Error ? error.message : "Unable to delete generation job");
    } finally {
      setDeletingJobIDs((current) => current.filter((id) => id !== jobID));
    }
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
              <dt>Provider mode</dt>
              <dd>{loaderData.status.provider.mode}</dd>
            </div>
          </dl>
        </Panel>
      </div>
      <Panel title="Recent jobs" subtitle="Latest live jobs for the selected world model">
        {cancelError ? <ErrorAlert message={cancelError} /> : null}
        {deleteError ? <ErrorAlert message={deleteError} /> : null}
        {pollError ? <ErrorAlert message={pollError} /> : null}
        <GenerationJobsList cancelingJobIDs={cancelingJobIDs} deletingJobIDs={deletingJobIDs} jobs={jobs} onCancel={handleCancel} onDelete={handleDelete} />
      </Panel>
    </div>
  );
}
