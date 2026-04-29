export interface IPWhois {
  organization?: string;
  network?: string;
  country?: string;
  raw?: string;
}

export interface IPGeo {
  country?: string;
  region?: string;
  city?: string;
  timezone?: string;
  latitude?: number;
  longitude?: number;
}

export interface IPIntelligence {
  source: string;
  whois?: IPWhois;
  geo?: IPGeo;
}

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

export function getIPIntelligence(metadata?: Record<string, unknown>): IPIntelligence | null {
  if (!metadata) return null;
  const raw = metadata.ip_intelligence;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null;
  const intel = raw as Record<string, unknown>;
  if (typeof intel.source !== "string") return null;
  return intel as unknown as IPIntelligence;
}
