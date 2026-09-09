import { ensureRadar } from "./following-store";

export type AlertFeedbackRating = "useful" | "noise" | "too_late";

export type AlertContext = {
  alert_id: string;
  lifecycle_before: string;
  lifecycle_after: string;
  score_before: number;
  score_after: number;
  velocity_before: number;
  velocity_after: number;
  source_count_before: number;
  source_count_after: number;
  source_breadth_before: number;
  source_breadth_after: number;
  observed_at: string;
};

export type RadarBriefingItem = {
  trend_key: string;
  slug: string;
  name: string;
  latest_at: string;
  alert_count: number;
  kinds: string[];
  changes: string[];
  latest_context?: AlertContext;
};

export type RadarBriefing = {
  since: string;
  generated_at: string;
  quiet: boolean;
  items: RadarBriefingItem[];
};

export type RadarQuality = {
  total_alerts: number;
  rated: number;
  useful: number;
  noise: number;
  too_late: number;
  useful_rate: number;
  push_attempts: number;
  delivered_alerts: number;
  failed_deliveries: number;
};

type Envelope<T> = { data: T };

async function radarRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { key } = await ensureRadar();
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  headers.set("Authorization", `Bearer ${key}`);
  if (options.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(path, { ...options, headers });
  if (!response.ok) {
    let detail = `Trendinary API ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) detail = payload.error;
    } catch {
      // Keep status-derived error.
    }
    throw new Error(detail);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export async function loadRadarFeedback(): Promise<Record<string, AlertFeedbackRating>> {
  return (await radarRequest<Envelope<Record<string, AlertFeedbackRating>>>("/api/v1/following/alerts/feedback")).data;
}

export async function rateRadarAlert(alertID: string, rating: AlertFeedbackRating): Promise<AlertFeedbackRating> {
  const payload = await radarRequest<Envelope<{ rating: AlertFeedbackRating }>>(
    `/api/v1/following/alerts/${encodeURIComponent(alertID)}/feedback`,
    { method: "POST", body: JSON.stringify({ rating }) },
  );
  return payload.data.rating;
}

export async function loadRadarQuality(): Promise<RadarQuality> {
  return (await radarRequest<Envelope<RadarQuality>>("/api/v1/following/quality")).data;
}

export async function loadRadarBriefing(): Promise<RadarBriefing> {
  return (await radarRequest<Envelope<RadarBriefing>>("/api/v1/following/briefing")).data;
}

export async function markRadarBriefingSeen(): Promise<void> {
  await radarRequest<void>("/api/v1/following/briefing/seen", { method: "POST" });
}
