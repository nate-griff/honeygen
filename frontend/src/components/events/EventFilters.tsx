import { useEffect, useState } from "react";
import type { WorldModelSummary } from "../../types/worldModels";

export interface EventFilterValues {
  world_model_id: string;
  path: string;
  source_ip: string;
  status_code: string;
}

interface EventFiltersProps {
  models: WorldModelSummary[];
  values: EventFilterValues;
  onApply: (values: EventFilterValues) => void;
  onReset: () => void;
}

export function EventFilters({ models, values, onApply, onReset }: EventFiltersProps) {
  const [draft, setDraft] = useState<EventFilterValues>(values);

  useEffect(() => {
    setDraft(values);
  }, [values]);

  return (
    <form
      className="form-grid"
      onSubmit={(event) => {
        event.preventDefault();
        onApply(draft);
      }}
    >
      <label>
        World model
        <select value={draft.world_model_id} onChange={(event) => setDraft((current) => ({ ...current, world_model_id: event.target.value }))}>
          <option value="">All world models</option>
          {models.map((model) => (
            <option key={model.id} value={model.id}>
              {model.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Path contains
        <input value={draft.path} onChange={(event) => setDraft((current) => ({ ...current, path: event.target.value }))} />
      </label>
      <label>
        Source IP
        <input value={draft.source_ip} onChange={(event) => setDraft((current) => ({ ...current, source_ip: event.target.value }))} />
      </label>
      <label>
        Status code
        <input value={draft.status_code} onChange={(event) => setDraft((current) => ({ ...current, status_code: event.target.value }))} />
      </label>
      <div className="button-row form-grid__full">
        <button className="button button--primary" type="submit">
          Apply filters
        </button>
        <button className="button button--ghost" onClick={onReset} type="button">
          Clear filters
        </button>
      </div>
    </form>
  );
}
