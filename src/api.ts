export type BiasLabel =
  | "left"
  | "lean-left"
  | "center"
  | "lean-right"
  | "right"
  | "mixed"
  | "not-rated";

export type BiasAssessment = {
  label: BiasLabel;
  provider: string;
  score?: number;
  confidence?: string;
  scope?: string;
  methodology_url?: string;
  rating_url?: string;
  as_of?: string;
};

export type TrendSource = {
  name: string;
  domain?: string;
  url?: string;
  bias?: BiasAssessment;
};

export type TimelineEvent = {
  time: string;
  label: string;
  text: string;
};

export type Trend = {
  slug: string;
  rank: number;
  name: string;
  category: string;
  score: number;
  change: string;
  status: "BREAKING" | "RISING" | "EMERGING" | "PEAKING" | "COOLING" | "RESURFACING";
  reason: string;
  started: string;
  vibe: string;
  sources: TrendSource[];
  timeline?: TimelineEvent[];
  lore?: string;
  why?: string;
};

export type ScoreBreakdown = {
  version: string;
  score: number;
  attention: number;
  velocity: number;
  source_breadth: number;
  community_breadth: number;
  novelty: number;
  confidence: number;
};

export type RawTrendMetrics = {
  signal_count: number;
  source_count: number;
  community_count: number;
  raw_attention: number;
  raw_engagement: number;
};

export type TrendSnapshot = {
  trend_key: string;
  observed_at: string;
  lifecycle: Trend["status"];
  score: ScoreBreakdown;
  raw: RawTrendMetrics;
};

type Envelope<T> = { data: T };

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(`Trendinary API ${response.status}`);
  return (await response.json()) as T;
}

export async function fetchTrends(): Promise<Trend[]> {
  return (await getJSON<Envelope<Trend[]>>("/api/v1/trends")).data;
}

export async function fetchTrend(slug: string): Promise<Trend> {
  return (await getJSON<Envelope<Trend>>(`/api/v1/trends/${encodeURIComponent(slug)}`)).data;
}

export async function fetchTrendHistory(slug: string, limit = 48): Promise<TrendSnapshot[]> {
  return (
    await getJSON<Envelope<TrendSnapshot[]>>(
      `/api/v1/trends/${encodeURIComponent(slug)}/history?limit=${encodeURIComponent(String(limit))}`,
    )
  ).data;
}

export async function fetchSource(domain: string): Promise<TrendSource> {
  return (await getJSON<Envelope<TrendSource>>(`/api/v1/sources/${encodeURIComponent(domain)}`)).data;
}
