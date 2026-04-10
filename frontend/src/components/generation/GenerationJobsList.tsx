import { Link } from "react-router-dom";
import { formatDateTime, formatCount } from "../../app/format";
import { EmptyState } from "../layout/EmptyState";
import { StatusBadge } from "../layout/StatusBadge";
import type { GenerationJob } from "../../types/generation";

interface GenerationJobsListProps {
  jobs: GenerationJob[];
}

export function GenerationJobsList({ jobs }: GenerationJobsListProps) {
  if (jobs.length === 0) {
    return <EmptyState title="No generation jobs" message="Trigger a run to populate live job history." />;
  }

  return (
    <div className="list">
      {jobs.map((job) => (
        <article className="list-card" key={job.id}>
          <div className="list-card__title-row">
            <strong>{job.id}</strong>
            <StatusBadge value={job.status} />
          </div>
          <dl className="detail-grid detail-grid--compact">
            <div>
              <dt>World model</dt>
              <dd>{job.world_model_id}</dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd>{formatDateTime(job.created_at)}</dd>
            </div>
            <div>
              <dt>Completed</dt>
              <dd>{formatDateTime(job.completed_at)}</dd>
            </div>
            <div>
              <dt>Assets</dt>
              <dd>{formatCount(job.summary.asset_count)}</dd>
            </div>
            <div>
              <dt>Manifests</dt>
              <dd>{formatCount(job.summary.manifest_count)}</dd>
            </div>
          </dl>
          {job.error_message ? <p className="error-text">{job.error_message}</p> : null}
          {job.summary.logs?.length ? (
            <details className="details-block">
              <summary>Execution log</summary>
              <ul className="activity-list">
                {job.summary.logs.map((entry, index) => (
                  <li key={`${job.id}-${index}`}>
                    <strong>{entry.level}</strong> · {entry.message}
                    {entry.path ? <span className="muted"> · {entry.path}</span> : null}
                  </li>
                ))}
              </ul>
            </details>
          ) : null}
          <div className="button-row">
            <Link className="button button--ghost" to={`/files?world_model_id=${job.world_model_id}&generation_job_id=${job.id}`}>
              Browse assets
            </Link>
          </div>
        </article>
      ))}
    </div>
  );
}
