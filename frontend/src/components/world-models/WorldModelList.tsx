import { formatDateTime } from "../../app/format";
import { EmptyState } from "../layout/EmptyState";
import type { WorldModelSummary } from "../../types/worldModels";

interface WorldModelListProps {
  items: WorldModelSummary[];
  selectedID?: string;
  onSelect: (id: string) => void;
  onCreateNew: () => void;
}

export function WorldModelList({ items, selectedID, onSelect, onCreateNew }: WorldModelListProps) {
  return (
    <div className="stack">
      <button className="button button--primary" onClick={onCreateNew} type="button">
        New world model
      </button>
      {items.length === 0 ? (
        <EmptyState title="No world models yet" message="Create the first world model to unlock generation flows." />
      ) : (
        <div className="list">
          {items.map((item) => (
            <button
              key={item.id}
              className={`list-card${selectedID === item.id ? " is-selected" : ""}`}
              onClick={() => onSelect(item.id)}
              type="button"
            >
              <div className="list-card__title-row">
                <strong>{item.name}</strong>
                <span className="list-card__meta">{item.id}</span>
              </div>
              <p>{item.description || "No description provided."}</p>
              <dl className="inline-metrics">
                <div>
                  <dt>Departments</dt>
                  <dd>{item.departmentCount}</dd>
                </div>
                <div>
                  <dt>Employees</dt>
                  <dd>{item.employeeCount}</dd>
                </div>
                <div>
                  <dt>Projects</dt>
                  <dd>{item.projectCount}</dd>
                </div>
                <div>
                  <dt>Themes</dt>
                  <dd>{item.documentThemeCount}</dd>
                </div>
              </dl>
              <span className="list-card__timestamp">Updated {formatDateTime(item.updatedAt)}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
