import { useState } from "react";
import { formatDateTime, formatFileSize } from "../../app/format";
import type { EventRecord } from "../../types/events";
import { EmptyState } from "../layout/EmptyState";

interface EventDetailProps {
  event: EventRecord | null;
}

export function EventDetail({ event }: EventDetailProps) {
  const [tab, setTab] = useState<"details" | "json">("details");

  if (!event) {
    return <EmptyState title="No event selected" message="Pick an event from the list to inspect its full details." />;
  }

  return (
    <div className="stack stack--compact">
      <div className="button-row">
        <button
          className={`button ${tab === "details" ? "button--primary" : "button--ghost"}`}
          type="button"
          onClick={() => setTab("details")}
        >
          Details
        </button>
        <button
          className={`button ${tab === "json" ? "button--primary" : "button--ghost"}`}
          type="button"
          onClick={() => setTab("json")}
        >
          Raw JSON
        </button>
      </div>

      {tab === "json" ? (
        <pre className="preview-text" style={{ userSelect: "all" }}>
          {JSON.stringify(event, null, 2)}
        </pre>
      ) : (
        <>
          <dl className="detail-grid">
            <div>
              <dt>Event ID</dt>
              <dd>{event.id}</dd>
            </div>
            <div>
              <dt>World model</dt>
              <dd>{event.world_model_id || "—"}</dd>
            </div>
            <div>
              <dt>Asset ID</dt>
              <dd>{event.asset_id || "—"}</dd>
            </div>
            <div>
              <dt>Timestamp</dt>
              <dd>{formatDateTime(event.timestamp)}</dd>
            </div>
            <div>
              <dt>Method</dt>
              <dd>{event.method || "—"}</dd>
            </div>
            <div>
              <dt>Status</dt>
              <dd>{event.status_code || "—"}</dd>
            </div>
            <div className="detail-grid__full">
              <dt>Path</dt>
              <dd className="code-inline">{event.path}</dd>
            </div>
            <div className="detail-grid__full">
              <dt>Query</dt>
              <dd className="code-inline">{event.query || "—"}</dd>
            </div>
            <div>
              <dt>Source IP</dt>
              <dd>{event.source_ip || "—"}</dd>
            </div>
            <div>
              <dt>Bytes sent</dt>
              <dd>{formatFileSize(event.bytes_sent)}</dd>
            </div>
            <div className="detail-grid__full">
              <dt>User agent</dt>
              <dd>{event.user_agent || "—"}</dd>
            </div>
            <div className="detail-grid__full">
              <dt>Referer</dt>
              <dd className="code-inline">{event.referer || "—"}</dd>
            </div>
          </dl>
          <div>
            <h4>Metadata</h4>
            <pre className="preview-text">{JSON.stringify(event.metadata ?? {}, null, 2)}</pre>
          </div>
        </>
      )}
    </div>
  );
}
