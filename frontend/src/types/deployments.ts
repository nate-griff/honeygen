export interface Deployment {
  id: string;
  generation_job_id: string;
  world_model_id: string;
  protocol: string;
  port: number;
  root_path: string;
  status: string;
  share_name?: string;
  mount_path?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDeploymentRequest {
  generation_job_id: string;
  world_model_id: string;
  protocol: string;
  port: number;
  root_path: string;
}
