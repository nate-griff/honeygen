import { useLoaderData, useNavigate } from "react-router-dom";
import { APIClientError } from "../api/client";
import { getEvent, listEvents } from "../api/events";
import { listWorldModels } from "../api/worldModels";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { EventDetail } from "../components/events/EventDetail";
import { EventFilters, type EventFilterValues } from "../components/events/EventFilters";
import { EventList } from "../components/events/EventList";
import type { EventRecord } from "../types/events";
import type { WorldModelSummary } from "../types/worldModels";

interface EventLogLoaderData {
  models: WorldModelSummary[];
  filters: EventFilterValues;
  events: EventRecord[];
  selectedEventID: string;
  selectedEvent: EventRecord | null;
}

export async function eventLogLoader({ request }: { request: Request }): Promise<EventLogLoaderData> {
  const url = new URL(request.url);
  const filters: EventFilterValues = {
    world_model_id: url.searchParams.get("world_model_id")?.trim() ?? "",
    path: url.searchParams.get("path")?.trim() ?? "",
    source_ip: url.searchParams.get("source_ip")?.trim() ?? "",
    status_code: url.searchParams.get("status_code")?.trim() ?? "",
  };
  const selectedEventID = url.searchParams.get("event_id")?.trim() ?? "";
  const [models, events] = await Promise.all([
    listWorldModels(),
    listEvents({
      limit: 100,
      world_model_id: filters.world_model_id || undefined,
      path: filters.path || undefined,
      source_ip: filters.source_ip || undefined,
      status_code: filters.status_code ? Number(filters.status_code) : undefined,
    }),
  ]);

  let selectedEvent: EventRecord | null = null;
  if (selectedEventID) {
    try {
      selectedEvent = await getEvent(selectedEventID);
    } catch (error) {
      if (!(error instanceof APIClientError && error.status === 404)) {
        throw error;
      }
    }
  }

  return { models, filters, events, selectedEventID, selectedEvent };
}

export default function EventLogPage() {
  const { models, filters, events, selectedEvent, selectedEventID } = useLoaderData() as EventLogLoaderData;
  const navigate = useNavigate();

  function applyFilters(next: EventFilterValues) {
    const params = new URLSearchParams();
    if (next.world_model_id) {
      params.set("world_model_id", next.world_model_id);
    }
    if (next.path) {
      params.set("path", next.path);
    }
    if (next.source_ip) {
      params.set("source_ip", next.source_ip);
    }
    if (next.status_code) {
      params.set("status_code", next.status_code);
    }
    navigate(`/events${params.toString() ? `?${params.toString()}` : ""}`);
  }

  function selectEvent(eventID: string) {
    const params = new URLSearchParams();
    if (filters.world_model_id) {
      params.set("world_model_id", filters.world_model_id);
    }
    if (filters.path) {
      params.set("path", filters.path);
    }
    if (filters.source_ip) {
      params.set("source_ip", filters.source_ip);
    }
    if (filters.status_code) {
      params.set("status_code", filters.status_code);
    }
    params.set("event_id", eventID);
    navigate(`/events?${params.toString()}`);
  }

  return (
    <div className="stack">
      <PageHeader
        title="Event Log"
        description="Filter live event telemetry, inspect request details, and review metadata for suspicious activity."
      />
      <Panel title="Filters" subtitle="Basic filtering against /api/events">
        <EventFilters models={models} onApply={applyFilters} onReset={() => navigate("/events")} values={filters} />
      </Panel>
      <div className="split-layout">
        <Panel title="Events" subtitle="Recent live events">
          <EventList events={events} onSelect={selectEvent} selectedEventID={selectedEventID} />
        </Panel>
        <Panel title="Event detail" subtitle="Selected record from /api/events/{id}">
          <EventDetail event={selectedEvent} />
        </Panel>
      </div>
    </div>
  );
}
