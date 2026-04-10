import { formatDateTime } from "../../app/format";
import { EmptyState } from "../layout/EmptyState";
import { StatusBadge } from "../layout/StatusBadge";
import type { EventRecord } from "../../types/events";

interface EventListProps {
  events: EventRecord[];
  selectedEventID?: string;
  onSelect: (id: string) => void;
}

export function EventList({ events, selectedEventID, onSelect }: EventListProps) {
  if (events.length === 0) {
    return <EmptyState title="No matching events" message="Adjust filters or wait for new live events to arrive." />;
  }

  return (
    <div className="list">
      {events.map((event) => (
        <button
          key={event.id}
          className={`list-card${selectedEventID === event.id ? " is-selected" : ""}`}
          onClick={() => onSelect(event.id)}
          type="button"
        >
          <div className="list-card__title-row">
            <strong>{event.event_type}</strong>
            <StatusBadge value={event.level} />
          </div>
          <p className="code-inline">{event.path}</p>
          <dl className="inline-metrics inline-metrics--events">
            <div>
              <dt>Method</dt>
              <dd>{event.method || "—"}</dd>
            </div>
            <div>
              <dt>Status</dt>
              <dd>{event.status_code || "—"}</dd>
            </div>
            <div>
              <dt>Source</dt>
              <dd>{event.source_ip || "—"}</dd>
            </div>
          </dl>
          <span className="list-card__timestamp">{formatDateTime(event.timestamp)}</span>
        </button>
      ))}
    </div>
  );
}
