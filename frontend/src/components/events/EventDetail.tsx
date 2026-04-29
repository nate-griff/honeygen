import { useState } from "react";
import { formatDateTime, formatFileSize } from "../../app/format";
import { getIPIntelligence } from "../../types/events";
import type { EventRecord, IPIntelligence } from "../../types/events";
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
          <IPIntelligenceSection intel={getIPIntelligence(event.metadata)} />
          <div>
            <h4>Metadata</h4>
            <pre className="preview-text">{JSON.stringify(event.metadata ?? {}, null, 2)}</pre>
          </div>
        </>
      )}
    </div>
  );
}

interface IPIntelligenceSectionProps {
  intel: IPIntelligence | null;
}

function IPIntelligenceSection({ intel }: IPIntelligenceSectionProps) {
  if (!intel) return null;

  const hasWhois = !!(intel.whois && (
    intel.whois.organization || intel.whois.network || intel.whois.country
  ));
  const hasGeo = !!(intel.geo && (
    intel.geo.city || intel.geo.region || intel.geo.country ||
    intel.geo.timezone || intel.geo.latitude != null || intel.geo.longitude != null
  ));

  return (
    <div>
      <h4>
        IP Intelligence{" "}
        <span className="muted" style={{ fontWeight: 400, fontSize: "0.85rem" }}>
          via {intel.source}
        </span>
      </h4>
      {hasWhois && (
        <>
          <p className="muted" style={{ margin: "0 0 0.4rem", fontSize: "0.8rem", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.06em" }}>
            WHOIS
          </p>
          <dl className="detail-grid detail-grid--compact" style={{ marginBottom: "0.75rem" }}>
            {intel.whois!.organization && (
              <div className="detail-grid__full">
                <dt>Organization</dt>
                <dd>{intel.whois!.organization}</dd>
              </div>
            )}
            {intel.whois!.network && (
              <div>
                <dt>Network</dt>
                <dd>{intel.whois!.network}</dd>
              </div>
            )}
            {intel.whois!.country && (
              <div>
                <dt>Country</dt>
                <dd>{intel.whois!.country}</dd>
              </div>
            )}
          </dl>
        </>
      )}
      {hasGeo && (
        <>
          <p className="muted" style={{ margin: "0 0 0.4rem", fontSize: "0.8rem", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.06em" }}>
            Geolocation
          </p>
          <dl className="detail-grid detail-grid--compact">
            {intel.geo!.city && (
              <div>
                <dt>City</dt>
                <dd>{intel.geo!.city}</dd>
              </div>
            )}
            {intel.geo!.region && (
              <div>
                <dt>Region</dt>
                <dd>{intel.geo!.region}</dd>
              </div>
            )}
            {intel.geo!.country && (
              <div>
                <dt>Country</dt>
                <dd>{intel.geo!.country}</dd>
              </div>
            )}
            {intel.geo!.timezone && (
              <div>
                <dt>Timezone</dt>
                <dd>{intel.geo!.timezone}</dd>
              </div>
            )}
            {intel.geo!.latitude != null && intel.geo!.longitude != null && (
              <div>
                <dt>Coordinates</dt>
                <dd>
                  {intel.geo!.latitude.toFixed(4)}, {intel.geo!.longitude.toFixed(4)}
                </dd>
              </div>
            )}
          </dl>
        </>
      )}
      {!hasWhois && !hasGeo && (
        <p className="muted" style={{ margin: 0 }}>
          Source: {intel.source} — no enrichment details available
        </p>
      )}
    </div>
  );
}
