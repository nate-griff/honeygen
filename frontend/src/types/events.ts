export interface EventRecord {
  id: string;
  asset_id?: string;
  world_model_id?: string;
  event_type: string;
  method: string;
  query: string;
  path: string;
  source_ip: string;
  user_agent: string;
  referer: string;
  status_code: number;
  bytes_sent: number;
  timestamp: string;
  level: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}
