import { Link, useLoaderData } from "react-router-dom";
import { listEvents } from "../api/events";
import { getStatus } from "../api/status";
import { formatCount, formatDateTime } from "../app/format";
import { EmptyState } from "../components/layout/EmptyState";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { StatusBadge } from "../components/layout/StatusBadge";
import type { EventRecord } from "../types/events";
import type { StatusResponse } from "../types/status";

interface DashboardLoaderData {
  status: StatusResponse;
  recentEvents: EventRecord[];
}

export async function dashboardLoader(): Promise<DashboardLoaderData> {
  const [status, recentEvents] = await Promise.all([getStatus(), listEvents({ limit: 5 })]);
  return { status, recentEvents };
}

export default function DashboardPage() {
  const { status, recentEvents } = useLoaderData() as DashboardLoaderData;

  return (
    <div className="stack">
      <PageHeader
        title="Dashboard"
        description="Monitor system readiness, generated content volume, latest jobs, and recent live events."
      />
      <section className="stats-grid">
        <Panel>
          <div className="stat-card">
            <span>Database</span>
            <StatusBadge value={status.database.ready} />
            <strong>{status.database.path}</strong>
          </div>
        </Panel>
        <Panel>
          <div className="stat-card">
            <span>Provider</span>
            <StatusBadge value={status.provider.ready} />
            <strong>{status.provider.mode}</strong>
          </div>
        </Panel>
        <Panel>
          <div className="stat-card">
            <span>Assets</span>
            <strong>{formatCount(status.counts.assets)}</strong>
            <small>Tracked files</small>
          </div>
        </Panel>
        <Panel>
          <div className="stat-card">
            <span>Recent events</span>
            <strong>{formatCount(status.counts.recent_events)}</strong>
            <small>Past 24 hours</small>
          </div>
        </Panel>
      </section>

      <div className="two-column">
        <Panel title="Latest generation job" subtitle="Most recent job from /api/status">
          {status.latest_job ? (
            <div className="stack stack--compact">
              <div className="list-card__title-row">
                <strong>{status.latest_job.id}</strong>
                <StatusBadge value={status.latest_job.status} />
              </div>
              <dl className="detail-grid detail-grid--compact">
                <div>
                  <dt>World model</dt>
                  <dd>{status.latest_job.world_model_id}</dd>
                </div>
                <div>
                  <dt>Assets</dt>
                  <dd>{formatCount(status.latest_job.asset_count)}</dd>
                </div>
                <div>
                  <dt>Completed</dt>
                  <dd>{formatDateTime(status.latest_job.completed_at)}</dd>
                </div>
              </dl>
              <div className="button-row">
                <Link className="button button--ghost" to={`/generation?world_model_id=${status.latest_job.world_model_id}`}>
                  Open generation
                </Link>
                <Link
                  className="button button--ghost"
                  to={`/files?world_model_id=${status.latest_job.world_model_id}&generation_job_id=${status.latest_job.id}`}
                >
                  Browse assets
                </Link>
              </div>
            </div>
          ) : (
            <EmptyState title="No jobs yet" message="Run the first generation job from the Generation page." />
          )}
        </Panel>

        <Panel title="Recent events" subtitle="Fetched from /api/events for operator review">
          {recentEvents.length === 0 ? (
            <EmptyState title="No recent events" message="Event traffic will appear here once decoy assets are accessed." />
          ) : (
            <ul className="activity-list">
              {recentEvents.map((event) => (
                <li key={event.id}>
                  <Link to={`/events?event_id=${event.id}`}>{event.event_type}</Link> · <span className="code-inline">{event.path}</span>
                  <div className="muted">
                    {formatDateTime(event.timestamp)} · {event.source_ip || "unknown source"}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Panel>
      </div>
    </div>
  );
}
