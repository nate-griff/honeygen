export interface GenerationLogEntry {
  time: string;
  level: string;
  message: string;
  path?: string;
  category?: string;
}

export interface GenerationSummary {
  manifest_count: number;
  asset_count: number;
  categories?: string[];
  logs?: GenerationLogEntry[];
}

export interface GenerationJob {
  id: string;
  world_model_id: string;
  status: string;
  started_at?: string;
  completed_at?: string;
  error_message?: string;
  summary: GenerationSummary;
  created_at: string;
  updated_at: string;
}
