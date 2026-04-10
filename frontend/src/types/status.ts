export interface StatusResponse {
  service: {
    name: string;
    version: string;
  };
  database: {
    ready: boolean;
    path: string;
  };
  storage: {
    root: string;
    generated_assets_dir: string;
  };
  provider: {
    mode: string;
    ready: boolean;
    base_url?: string;
    model?: string;
  };
  counts: {
    assets: number;
    recent_events: number;
  };
  recent_events: Array<{
    id: string;
    event_type: string;
    path: string;
    source_ip: string;
    timestamp: string;
  }>;
  latest_job?: {
    id: string;
    world_model_id: string;
    status: string;
    completed_at?: string;
    asset_count: number;
  };
}
