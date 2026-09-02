import type { Trend } from "./api";

export type QualityLabel =
  | "real-trend"
  | "noise"
  | "duplicate"
  | "interesting-too-early"
  | "detected-too-late"
  | "bad-cluster"
  | "wrong-canonical-name";

export type QualityFeedback = {
  id: string;
  trend_key: string;
  trend_slug: string;
  trend_name: string;
  label: QualityLabel;
  note?: string;
  score: number;
  lifecycle?: string;
  created_at: string;
};

export type QualityReport = {
  labels: number;
  real_trends: number;
  noise: number;
  duplicates: number;
  interesting_too_early: number;
  detected_too_late: number;
  bad_clusters: number;
  wrong_names: number;
  precision_proxy: number;
  top_10_precision: number;
  top_10_evaluated: number;
  top_25_precision: number;
  top_25_evaluated: number;
  false_positive_rate: number;
  duplicate_cluster_rate: number;
  early_hit_rate: number;
  cluster_health: number;
  naming_health: number;
  recommended_min_score: number;
  positive_mean_score: number;
  noise_mean_score: number;
  average_source_breadth: number;
  average_source_count: number;
  average_lead_to_breaking_minutes: number;
  average_emerging_to_rising_minutes: number;
  average_rising_to_breaking_minutes: number;
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

type Envelope<T> = { data: T };

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "same-origin",
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    let message = `Quality API ${response.status}`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Keep the HTTP fallback.
    }
    throw new Error(message);
  }
  return (await response.json()) as T;
}

export async function fetchQualityFeedback(limit = 100) {
  return (await request<Envelope<QualityFeedback[]>>(`/api/v1/admin/quality/feedback?limit=${limit}`)).data;
}

export async function fetchQualityReport() {
  return (await request<Envelope<QualityReport>>("/api/v1/admin/quality/report")).data;
}

export async function fetchQualityReplay(threshold = 0.56) {
  return (await request<Envelope<ReplayReport>>(`/api/v1/admin/quality/replay?threshold=${threshold}`)).data;
}

export async function labelTrend(trend: Trend, label: QualityLabel, note = "") {
  return (
    await request<Envelope<QualityFeedback>>(
      `/api/v1/admin/quality/trends/${encodeURIComponent(trend.slug)}/feedback`,
      {
        method: "POST",
        body: JSON.stringify({
          trend_key: trend.id ?? trend.slug,
          trend_name: trend.name,
          label,
          note,
          score: trend.score,
          lifecycle: trend.status,
        }),
      },
    )
  ).data;
}
