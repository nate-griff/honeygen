import { formatDateTime, formatFileSize } from "../../app/format";
import type { EventRecord } from "../../types/events";
import { EmptyState } from "../layout/EmptyState";

interface EventDetailProps {
  event: EventRecord | null;
}

export function EventDetail({ event }: EventDetailProps) {
  if (!event) {
    return <EmptyState title="No event selected" message="Pick an event from the list to inspect its full details." />;
  }

  return (
    <div className="stack stack--compact">
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
          <dt>User agent</dt>
          <dd>{event.user_agent || "—"}</dd>
        </div>
        <div>
          <dt>Referer</dt>
          <dd>{event.referer || "—"}</dd>
        </div>
        <div>
          <dt>Bytes sent</dt>
          <dd>{formatFileSize(event.bytes_sent)}</dd>
        </div>
      </dl>
      <div>
        <h4>Metadata</h4>
        <pre className="preview-text">{JSON.stringify(event.metadata ?? {}, null, 2)}</pre>
      </div>
    </div>
  );
}
