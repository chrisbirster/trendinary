import { fetchTrend, fetchTrends, type Trend } from "./api";

const STORAGE_KEY = "trendinary.following.v1";
const MAX_ALERTS = 100;

export type FollowBaseline = {
  status: Trend["status"];
  score: number;
  velocity: number;
  source_breadth: number;
  source_count: number;
  observed_at: string;
};

export type FollowRecord = {
  slug: string;
  trend_key?: string;
  name: string;
  followed_at: string;
  baseline?: FollowBaseline;
};

export type FollowAlertKind = "lifecycle" | "velocity" | "corroboration" | "resurfacing";

export type FollowAlert = {
  id: string;
  fingerprint: string;
  slug: string;
  trend_key?: string;
  name: string;
  kind: FollowAlertKind;
  title: string;
  body: string;
  created_at: string;
  read: boolean;
};

export type FollowingState = {
  version: 1;
  follows: FollowRecord[];
  alerts: FollowAlert[];
};

const emptyState = (): FollowingState => ({ version: 1, follows: [], alerts: [] });

function cloneState(state: FollowingState): FollowingState {
  return {
    version: 1,
    follows: state.follows.map((follow) => ({
      ...follow,
      baseline: follow.baseline ? { ...follow.baseline } : undefined,
    })),
    alerts: state.alerts.map((alert) => ({ ...alert })),
  };
}

let memoryState = emptyState();
let storageDisabledForSession = false;

function hasStorage() {
  if (storageDisabledForSession) return false;
  try {
    return typeof window !== "undefined" && "localStorage" in window;
  } catch {
    storageDisabledForSession = true;
    return false;
  }
}

export function loadFollowingState(): FollowingState {
  if (!hasStorage()) return cloneState(memoryState);
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      memoryState = emptyState();
      return cloneState(memoryState);
    }
    const value = JSON.parse(raw) as Partial<FollowingState>;
    if (value.version !== 1 || !Array.isArray(value.follows) || !Array.isArray(value.alerts)) {
      memoryState = emptyState();
      return cloneState(memoryState);
    }
    memoryState = { version: 1, follows: value.follows, alerts: value.alerts };
    return cloneState(memoryState);
  } catch {
    storageDisabledForSession = true;
    return cloneState(memoryState);
  }
}

function persist(state: FollowingState) {
  memoryState = cloneState(state);
  if (!hasStorage()) return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(memoryState));
  } catch {
    // Some private-browsing/storage policies allow reads but reject writes.
    // Stay on the in-memory state for the rest of this page session so later
    // actions cannot reload stale persisted data and erase the user's radar.
    storageDisabledForSession = true;
  }
}

function uniqueSourceCount(trend: Trend) {
  const identities = new Set<string>();
  for (const source of trend.sources ?? []) {
    const identity = (source.domain || source.name || source.url || "").trim().toLowerCase();
    if (identity) identities.add(identity);
  }
  return identities.size;
}

function baselineFor(trend: Trend): FollowBaseline {
  return {
    status: trend.status,
    score: trend.score,
    velocity: trend.quality?.velocity ?? 0,
    source_breadth: trend.quality?.source_breadth ?? 0,
    source_count: uniqueSourceCount(trend),
    observed_at: new Date().toISOString(),
  };
}

export function isFollowing(slug: string) {
  return loadFollowingState().follows.some((follow) => follow.slug === slug);
}

export function followTrend(trend: Trend) {
  const state = loadFollowingState();
  const existing = state.follows.find((follow) => follow.slug === trend.slug);
  const next: FollowRecord = {
    slug: trend.slug,
    trend_key: trend.id,
    name: trend.name,
    followed_at: existing?.followed_at ?? new Date().toISOString(),
    baseline: baselineFor(trend),
  };
  state.follows = [next, ...state.follows.filter((follow) => follow.slug !== trend.slug)];
  persist(state);
  return cloneState(memoryState);
}

export function unfollowTrend(slug: string) {
  const state = loadFollowingState();
  state.follows = state.follows.filter((follow) => follow.slug !== slug);
  persist(state);
  return cloneState(memoryState);
}

export function markFollowingAlertsRead() {
  const state = loadFollowingState();
  state.alerts = state.alerts.map((alert) => ({ ...alert, read: true }));
  persist(state);
  return cloneState(memoryState);
}

export function clearFollowingAlerts() {
  const state = loadFollowingState();
  state.alerts = [];
  persist(state);
  return cloneState(memoryState);
}

function lifecycleRank(status: Trend["status"]) {
  switch (status) {
    case "EMERGING": return 1;
    case "RISING": return 2;
    case "BREAKING": return 3;
    case "PEAKING": return 3;
    case "RESURFACING": return 2;
    case "COOLING": return 0;
  }
}

function makeAlert(follow: FollowRecord, trend: Trend, kind: FollowAlertKind, fingerprint: string, title: string, body: string): FollowAlert {
  const now = new Date().toISOString();
  return {
    id: `${follow.slug}:${kind}:${Date.now()}:${Math.random().toString(36).slice(2, 8)}`,
    fingerprint,
    slug: follow.slug,
    trend_key: trend.id ?? follow.trend_key,
    name: trend.name || follow.name,
    kind,
    title,
    body,
    created_at: now,
    read: false,
  };
}

function detectAlerts(follow: FollowRecord, trend: Trend): FollowAlert[] {
  const previous = follow.baseline;
  if (!previous) return [];

  const alerts: FollowAlert[] = [];
  const sourceCount = uniqueSourceCount(trend);
  const velocity = trend.quality?.velocity ?? 0;
  const breadth = trend.quality?.source_breadth ?? 0;

  if (trend.status === "RESURFACING" && previous.status !== "RESURFACING") {
    alerts.push(makeAlert(
      follow,
      trend,
      "resurfacing",
      `${follow.slug}:resurfacing:${trend.status}`,
      `${trend.name} is resurfacing`,
      `Trendinary detected a new burst after the topic had cooled. Score ${trend.score}.`,
    ));
  } else if (lifecycleRank(trend.status) > lifecycleRank(previous.status) && (trend.status === "RISING" || trend.status === "BREAKING" || trend.status === "PEAKING")) {
    alerts.push(makeAlert(
      follow,
      trend,
      "lifecycle",
      `${follow.slug}:lifecycle:${trend.status}`,
      `${trend.name} moved to ${trend.status.toLowerCase()}`,
      `Lifecycle changed from ${previous.status} to ${trend.status}. Score ${trend.score}.`,
    ));
  }

  const velocityDelta = velocity - previous.velocity;
  const scoreDelta = trend.score - previous.score;
  if ((velocity >= 0.55 && velocityDelta >= 0.20) || scoreDelta >= 15) {
    const bucket = Math.floor(velocity * 10);
    alerts.push(makeAlert(
      follow,
      trend,
      "velocity",
      `${follow.slug}:velocity:${bucket}:${Math.floor(trend.score / 10)}`,
      `${trend.name} accelerated`,
      `Velocity rose ${Math.round(velocityDelta * 100)} points and the score moved ${scoreDelta >= 0 ? "+" : ""}${scoreDelta}.`,
    ));
  }

  const sourceDelta = sourceCount - previous.source_count;
  const breadthDelta = breadth - previous.source_breadth;
  if (sourceCount >= 2 && (sourceDelta >= 2 || breadthDelta >= 0.25)) {
    alerts.push(makeAlert(
      follow,
      trend,
      "corroboration",
      `${follow.slug}:sources:${sourceCount}:${Math.floor(breadth * 10)}`,
      `${trend.name} crossed into more independent sources`,
      `Evidence now spans ${sourceCount} independent publisher/source identities; breadth is ${Math.round(breadth * 100)}%.`,
    ));
  }

  return alerts;
}

function maybeNotify(alert: FollowAlert) {
  if (typeof window === "undefined" || typeof Notification === "undefined") return;
  if (Notification.permission !== "granted") return;
  if (typeof document !== "undefined" && document.visibilityState === "visible") return;
  try {
    new Notification(alert.title, { body: alert.body, tag: alert.fingerprint });
  } catch {
    // In-app alerts remain authoritative when browser notifications are unavailable.
  }
}

export async function requestBrowserFollowingAlerts() {
  if (typeof Notification === "undefined") return "unsupported" as const;
  if (Notification.permission === "granted") return "granted" as const;
  if (Notification.permission === "denied") return "denied" as const;
  try {
    return await Notification.requestPermission();
  } catch {
    return "denied" as const;
  }
}

// The live leaderboard is fetched once per monitor pass. Follows that have
// fallen off the published leaderboard fall back to the trend-detail endpoint.
// This keeps 20 follows from becoming 20 requests every minute while retaining
// coverage for cooling/resurfacing topics that are no longer top-ranked.
export async function refreshFollowing(
  fetcher: (slug: string) => Promise<Trend> = fetchTrend,
  listFetcher: () => Promise<Trend[]> = fetchTrends,
) {
  const state = loadFollowingState();
  if (state.follows.length === 0) return state;

  const liveBySlug = new Map<string, Trend>();
  try {
    for (const trend of await listFetcher()) liveBySlug.set(trend.slug, trend);
  } catch {
    // Individual detail lookups below remain a valid fallback.
  }

  const knownFingerprints = new Set(state.alerts.map((alert) => alert.fingerprint));
  const newAlerts: FollowAlert[] = [];
  const nextFollows = await Promise.all(state.follows.map(async (follow) => {
    try {
      const trend = liveBySlug.get(follow.slug) ?? await fetcher(follow.slug);
      for (const alert of detectAlerts(follow, trend)) {
        if (knownFingerprints.has(alert.fingerprint)) continue;
        knownFingerprints.add(alert.fingerprint);
        newAlerts.push(alert);
      }
      return {
        ...follow,
        trend_key: trend.id ?? follow.trend_key,
        name: trend.name || follow.name,
        baseline: baselineFor(trend),
      };
    } catch {
      // A temporary API failure must not delete a user's follow or baseline.
      return follow;
    }
  }));

  state.follows = nextFollows;
  state.alerts = [...newAlerts.reverse(), ...state.alerts].slice(0, MAX_ALERTS);
  persist(state);
  for (const alert of newAlerts) maybeNotify(alert);
  return cloneState(memoryState);
}

export function startFollowingMonitor(onUpdate: (state: FollowingState) => void, intervalMs = 60_000) {
  if (typeof window === "undefined") return () => undefined;
  let stopped = false;
  let running = false;

  const run = async () => {
    if (stopped || running) return;
    running = true;
    try {
      onUpdate(await refreshFollowing());
    } finally {
      running = false;
    }
  };

  void run();
  const timer = window.setInterval(() => void run(), Math.max(30_000, intervalMs));
  const onVisibility = () => {
    if (document.visibilityState === "visible") void run();
  };
  document.addEventListener("visibilitychange", onVisibility);

  return () => {
    stopped = true;
    window.clearInterval(timer);
    document.removeEventListener("visibilitychange", onVisibility);
  };
}
