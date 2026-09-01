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

export type ActorProfile = {
  did?: string;
  handle?: string;
  display_name?: string;
  avatar?: string;
};

export type TimelineEvent = {
  time: string;
  label: string;
  text: string;
};

export type PropagationHop = {
  source: TrendSource;
  first_seen: string;
  last_seen: string;
  signal_count: number;
  engagement: number;
};

export type PerspectiveMix = {
  rated_sources: number;
  total_sources: number;
  left: number;
  lean_left: number;
  center: number;
  lean_right: number;
  right: number;
  mixed: number;
  unrated: number;
  note: string;
};

export type Evidence = {
  source: TrendSource;
  title: string;
  url?: string;
  author?: string;
  published_at?: string;
};

export type Explanation = {
  summary: string;
  what_changed: string;
  lore: string;
  confidence: string;
  mode: string;
  evidence: Evidence[];
};

export type Trend = {
  id?: string;
  slug: string;
  aliases?: string[];
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
  top_voices?: ActorProfile[];
  propagation?: PropagationHop[];
  perspective?: PerspectiveMix;
  explanation?: Explanation;
  lore?: string;
  why?: string;
};

export type AskAnswer = {
  answer: string;
  mode: string;
  evidence: Evidence[];
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

export async function askTrend(slug: string, question: string): Promise<AskAnswer> {
  const response = await fetch(`/api/v1/trends/${encodeURIComponent(slug)}/ask`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ question }),
  });
  if (!response.ok) throw new Error(`Trendinary API ${response.status}`);
  return ((await response.json()) as Envelope<AskAnswer>).data;
}
