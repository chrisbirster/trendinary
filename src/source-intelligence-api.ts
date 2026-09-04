import type { TrendSourceStatus } from "./admin-api";

export type SourceReliabilityStatus = TrendSourceStatus & {
  attempts?: number;
  successes?: number;
  signals_produced?: number;
  last_duration?: number;
  average_duration?: number;
  http_requests?: number;
  not_modified?: number;
};

export type SourceContribution = {
  key: string;
  name: string;
  signals: number;
  trends: number;
  first_hits: number;
  solo_trends: number;
  average_lead_to_breaking_minutes: number;
};

export type SourceAnalyticsReport = {
  since: string;
  publishers: SourceContribution[];
  channels: SourceContribution[];
};

export type ReplayReport = {
  corpus: string;
  signals: number;
  expected_pairs: number;
  predicted_pairs: number;
  true_positive_pairs: number;
  precision: number;
  recall: number;
};

export type CalibrationCandidate = {
  threshold: number;
  f1: number;
  replay: ReplayReport;
};

export type CalibrationV1 = {
  ready: boolean;
  required_labels: number;
  labels: number;
  current_cluster_threshold: number;
  recommended_cluster_threshold?: number;
  recommended_min_score: number;
  current_replay: ReplayReport;
  best_replay?: CalibrationCandidate;
  note: string;
};

type Envelope<T> = { data: T };

async function adminGet<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    let detail = `Admin API ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) detail = payload.error;
    } catch {
      // Keep the HTTP fallback.
    }
    throw new Error(detail);
  }
  return ((await response.json()) as Envelope<T>).data;
}

export function fetchSourceAnalytics(window = "168h") {
  return adminGet<SourceAnalyticsReport>(`/api/v1/admin/sources/analytics?window=${encodeURIComponent(window)}`);
}

export function fetchCalibrationV1() {
  return adminGet<CalibrationV1>("/api/v1/admin/quality/calibration");
}
