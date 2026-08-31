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

export async function fetchSource(domain: string): Promise<TrendSource> {
  return (await getJSON<Envelope<TrendSource>>(`/api/v1/sources/${encodeURIComponent(domain)}`)).data;
}
