export type EditorialState = "inbox" | "queued" | "consumed" | "saved" | "rejected" | "archived";
export type ContentType = "article" | "video" | "podcast" | "repository" | "discussion" | "other";

export type ScoreComponents = {
  freshness: number;
  personal_relevance: number;
  novelty: number;
  source_quality: number;
  trend_signal: number;
  serendipity: number;
};

export type ContentNote = {
  content_item_id: string;
  note: string;
  worth_sharing?: boolean;
  created_at: string;
  updated_at: string;
};

export type EditorialItem = {
  id: string;
  canonical_url: string;
  original_url: string;
  title: string;
  publisher: string;
  publisher_domain: string;
  content_type: ContentType;
  description?: string;
  image_url?: string;
  author?: string;
  published_at?: string;
  discovered_at: string;
  updated_at: string;
  state: EditorialState;
  opened_at?: string;
  queued_at?: string;
  consumed_at?: string;
  saved_at?: string;
  rejected_at?: string;
  editorial_score: number;
  score_components: ScoreComponents;
  serendipity: boolean;
  why_interesting: string;
  enrichment_status: "pending" | "complete" | "partial" | "failed";
  enrichment_error?: string;
  discovery_source?: string;
  external_source_name?: string;
  source_age_text?: string;
  estimated_minutes?: number;
  note?: ContentNote;
};

export type EditorialSource = {
  id: string;
  name: string;
  kind: string;
  url: string;
  enabled: boolean;
  last_attempted_at?: string;
  last_successful_at?: string;
  latest_item_count: number;
  last_error?: string;
};

export type TrendSourceStatus = {
  id: string;
  name: string;
  kind: string;
  policy?: string;
  url?: string;
  terms_url?: string;
  enabled: boolean;
  cadence: number;
  last_attempt_at?: string;
  last_success_at?: string;
  next_run_at?: string;
  last_error?: string;
  failures: number;
  cached_signals: number;
};

export type RuntimeHealth = {
  stream: {
    enabled: boolean;
    connected: boolean;
    host?: string;
    last_event_at?: string;
    last_error?: string;
    reconnects: number;
  };
  scanner: {
    enabled: boolean;
    running: boolean;
    last_started_at?: string;
    last_success_at?: string;
    last_error?: string;
    signals: number;
    clusters: number;
    trends: number;
    warnings: number;
  };
  sources?: TrendSourceStatus[];
};

export type IngestionRun = {
  id: string;
  source_id: string;
  started_at: string;
  completed_at?: string;
  status: "running" | "success" | "partial" | "failed";
  metrics: {
    sections_seen: number;
    items_seen: number;
    items_inserted: number;
    items_updated: number;
    duplicates: number;
    malformed: number;
    errors: number;
  };
  error?: string;
};

export type IssueItem = {
  id: string;
  issue_id: string;
  content_item_id: string;
  section: "thinking" | "worth_your_time" | "question_source" | "misc";
  position: number;
  editor_note?: string;
  content?: EditorialItem;
};

export type NewsletterIssue = {
  id: string;
  title: string;
  status: "draft" | "ready" | "published" | "archived";
  issue_date?: string;
  intro?: string;
  question?: string;
  created_at: string;
  updated_at: string;
  items?: IssueItem[];
};

type Envelope<T> = { data: T; meta?: Record<string, unknown> };

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
    let detail = `Admin API ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) detail = payload.error;
    } catch {
      // Keep the HTTP fallback.
    }
    throw new Error(detail);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export type InboxFilters = {
  type?: ContentType | "";
  period?: "" | "today" | "week";
  sort?: "recommended" | "newest" | "score";
  serendipity?: boolean;
};

function queryString(filters?: InboxFilters) {
  if (!filters) return "";
  const query = new URLSearchParams();
  if (filters.type) query.set("type", filters.type);
  if (filters.period) query.set("period", filters.period);
  if (filters.sort && filters.sort !== "recommended") query.set("sort", filters.sort);
  if (filters.serendipity) query.set("serendipity", "1");
  const value = query.toString();
  return value ? `?${value}` : "";
}

export async function fetchInbox(filters?: InboxFilters) {
  return (await request<Envelope<EditorialItem[]>>(`/api/v1/admin/inbox${queryString(filters)}`)).data;
}
export async function fetchQueue() { return (await request<Envelope<EditorialItem[]>>("/api/v1/admin/queue")).data; }
export async function fetchNotes() { return (await request<Envelope<EditorialItem[]>>("/api/v1/admin/notes")).data; }
export async function fetchTrash() { return (await request<Envelope<EditorialItem[]>>("/api/v1/admin/trash")).data; }
export async function fetchEditorialItem(id: string) { return (await request<Envelope<EditorialItem>>(`/api/v1/admin/content/${encodeURIComponent(id)}`)).data; }
export async function markOpened(id: string) { return (await request<Envelope<EditorialItem>>(`/api/v1/admin/content/${encodeURIComponent(id)}/open`, { method: "POST" })).data; }
export async function enrichItem(id: string) { return (await request<Envelope<EditorialItem>>(`/api/v1/admin/content/${encodeURIComponent(id)}/enrich`, { method: "POST" })).data; }
export async function setEditorialState(id: string, state: EditorialState) {
  return (await request<Envelope<EditorialItem>>(`/api/v1/admin/content/${encodeURIComponent(id)}/state`, { method: "PATCH", body: JSON.stringify({ state }) })).data;
}
export async function putNote(id: string, note: string, worthSharing?: boolean) {
  return (await request<Envelope<ContentNote>>(`/api/v1/admin/content/${encodeURIComponent(id)}/note`, { method: "PUT", body: JSON.stringify({ note, worth_sharing: worthSharing }) })).data;
}
export async function fetchSources() { return (await request<Envelope<EditorialSource[]>>("/api/v1/admin/sources")).data; }
export async function fetchTrendSourceHealth() { return request<RuntimeHealth>("/api/v1/health/streams"); }
export async function setSourceEnabled(id: string, enabled: boolean) {
  return (await request<Envelope<EditorialSource[]>>(`/api/v1/admin/sources/${encodeURIComponent(id)}`, { method: "PATCH", body: JSON.stringify({ enabled }) })).data;
}
export async function ingestSource(id: string) {
  return (await request<Envelope<IngestionRun>>(`/api/v1/admin/sources/${encodeURIComponent(id)}/ingest`, { method: "POST" })).data;
}
export async function fetchSourceRuns(id: string) { return (await request<Envelope<IngestionRun[]>>(`/api/v1/admin/sources/${encodeURIComponent(id)}/runs`)).data; }
export async function fetchIssues() { return (await request<Envelope<NewsletterIssue[]>>("/api/v1/admin/issues")).data; }
export async function fetchIssue(id: string) { return (await request<Envelope<NewsletterIssue>>(`/api/v1/admin/issues/${encodeURIComponent(id)}`)).data; }
export async function fetchIssueSuggestions() { return (await request<Envelope<EditorialItem[]>>("/api/v1/admin/issues/suggestions")).data; }
export async function createIssue(input: Partial<NewsletterIssue>) { return (await request<Envelope<NewsletterIssue>>("/api/v1/admin/issues", { method: "POST", body: JSON.stringify(input) })).data; }
export async function updateIssue(id: string, input: Partial<NewsletterIssue>) { return (await request<Envelope<NewsletterIssue>>(`/api/v1/admin/issues/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(input) })).data; }
export async function addIssueItem(issueId: string, input: Partial<IssueItem>) { return (await request<Envelope<IssueItem>>(`/api/v1/admin/issues/${encodeURIComponent(issueId)}/items`, { method: "POST", body: JSON.stringify(input) })).data; }
export async function removeIssueItem(issueId: string, itemId: string) { return request<void>(`/api/v1/admin/issues/${encodeURIComponent(issueId)}/items/${encodeURIComponent(itemId)}`, { method: "DELETE" }); }
