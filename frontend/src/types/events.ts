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
  if (hasInvalidNestedGeoCoordinates(intel)) return null;

  const geoRecord = normalizeGeoRecord(intel);
  const whoisRecord = normalizeWhoisRecord(intel);

  return {
    source: intel.source,
    ...(whoisRecord ? { whois: whoisRecord } : {}),
    ...(geoRecord ? { geo: geoRecord } : {}),
  };
}

function hasInvalidNestedGeoCoordinates(intel: Record<string, unknown>): boolean {
  const rawGeo = intel.geo;
  if (!rawGeo || typeof rawGeo !== "object" || Array.isArray(rawGeo)) {
    return false;
  }

  const geo = rawGeo as Record<string, unknown>;
  return (
    (geo.latitude !== undefined && typeof geo.latitude !== "number") ||
    (geo.longitude !== undefined && typeof geo.longitude !== "number")
  );
}

function normalizeGeoRecord(intel: Record<string, unknown>): IPGeo | null {
  const rawGeo = intel.geo;
  const geo =
    rawGeo && typeof rawGeo === "object" && !Array.isArray(rawGeo)
      ? (rawGeo as Record<string, unknown>)
      : intel;

  const latitude = geo.latitude;
  const longitude = geo.longitude;
  if ((latitude !== undefined && typeof latitude !== "number") || (longitude !== undefined && typeof longitude !== "number")) {
    return null;
  }

  const normalized: IPGeo = {};
  if (typeof geo.country === "string") normalized.country = geo.country;
  if (typeof geo.region === "string") normalized.region = geo.region;
  if (typeof geo.city === "string") normalized.city = geo.city;
  if (typeof geo.timezone === "string") normalized.timezone = geo.timezone;
  if (typeof latitude === "number") normalized.latitude = latitude;
  if (typeof longitude === "number") normalized.longitude = longitude;

  return Object.keys(normalized).length > 0 ? normalized : null;
}

function normalizeWhoisRecord(intel: Record<string, unknown>): IPWhois | null {
  const rawWhois = intel.whois;
  const whois =
    rawWhois && typeof rawWhois === "object" && !Array.isArray(rawWhois)
      ? (rawWhois as Record<string, unknown>)
      : intel;

  const normalized: IPWhois = {};
  if (typeof whois.organization === "string") normalized.organization = whois.organization;
  if (typeof whois.network === "string") normalized.network = whois.network;
  if (typeof whois.country === "string" && rawWhois) normalized.country = whois.country;
  if (typeof whois.raw === "string") normalized.raw = whois.raw;

  return Object.keys(normalized).length > 0 ? normalized : null;
}
