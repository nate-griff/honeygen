import { StatusBadge } from "../layout/StatusBadge";
import type { WorldModelSummary } from "../../types/worldModels";

interface GenerationRunPanelProps {
  models: WorldModelSummary[];
  selectedWorldModelID: string;
  providerReady: boolean;
  isRunning: boolean;
  runError?: string;
  onSelectWorldModel: (id: string) => void;
  onRun: () => void;
}

export function GenerationRunPanel({
  models,
  selectedWorldModelID,
  providerReady,
  isRunning,
  runError,
  onSelectWorldModel,
  onRun,
}: GenerationRunPanelProps) {
  return (
    <div className="stack stack--compact">
      <div className="toolbar-row">
        <label className="toolbar-field">
          World model
          <select value={selectedWorldModelID} onChange={(event) => onSelectWorldModel(event.target.value)}>
            {models.map((model) => (
              <option key={model.id} value={model.id}>
                {model.name}
              </option>
            ))}
          </select>
        </label>
        <div className="toolbar-status">
          <span className="muted">Provider</span>
          <StatusBadge value={providerReady} />
        </div>
      </div>
      {runError ? <div className="error-alert">{runError}</div> : null}
      <button className="button button--primary" disabled={!selectedWorldModelID || isRunning} onClick={onRun} type="button">
        {isRunning ? "Running generation…" : "Run generation"}
      </button>
    </div>
  );
}
