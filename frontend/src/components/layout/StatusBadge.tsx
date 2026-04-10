import { humanizeLabel } from "../../app/format";

interface StatusBadgeProps {
  value: string | boolean;
}

export function StatusBadge({ value }: StatusBadgeProps) {
  const normalized = typeof value === "boolean" ? (value ? "ready" : "not_ready") : value;
  const tone =
    normalized === "completed" || normalized === "ready"
      ? "success"
      : normalized === "running" || normalized === "pending"
        ? "warning"
        : normalized === "failed" || normalized === "not_ready"
          ? "danger"
          : "neutral";

  return <span className={`status-badge status-badge--${tone}`}>{humanizeLabel(normalized)}</span>;
}
