import type { Trend } from "./api";

const RADAR_KEY_STORAGE = "trendinary.radar.key.v1";
const LEGACY_STORAGE = "trendinary.following.v1";
const LEGACY_MIGRATED = "trendinary.following.v1.migrated";

export type FollowKind = "trend" | "topic" | "entity";

export type FollowRecord = {
  id: string;
  kind: FollowKind;
  value: string;
  display_name: string;
  created_at: string;
};

export type FollowAlertKind = "lifecycle" | "velocity" | "corroboration" | "resurfacing";

export type FollowAlert = {
  id: string;
  follow_id: string;
  trend_key: string;
  slug: string;
  name: string;
  kind: FollowAlertKind;
  title: string;
  body: string;
  created_at: string;
  read: boolean;
};

export type RadarSensitivity = "early" | "balanced" | "quiet";

export type RadarPreferences = {
  sensitivity: RadarSensitivity;
  lifecycle: boolean;
  velocity: boolean;
  corroboration: boolean;
  resurfacing: boolean;
};

export type FollowingState = {
  preferences: RadarPreferences;
  follows: FollowRecord[];
  alerts: FollowAlert[];
};

type Envelope<T> = { data: T };
type RadarCreated = { sync_key: string; state: FollowingState };

type LegacyState = {
  version?: number;
  follows?: Array<{ slug?: string; name?: string }>;
};

const emptyPreferences = (): RadarPreferences => ({
  sensitivity: "balanced",
  lifecycle: true,
  velocity: true,
  corroboration: true,
  resurfacing: true,
});

export const emptyFollowingState = (): FollowingState => ({
  preferences: emptyPreferences(),
  follows: [],
  alerts: [],
});

class FollowingAPIError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

let memoryRadarKey = "";
let storageUnavailable = false;
let radarPromise: Promise<{ key: string; state: FollowingState }> | undefined;

function storageGet(key: string) {
  if (storageUnavailable || typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(key);
  } catch {
    storageUnavailable = true;
    return null;
  }
}

function storageSet(key: string, value: string) {
  if (storageUnavailable || typeof window === "undefined") return;
  try {
    window.localStorage.setItem(key, value);
  } catch {
    storageUnavailable = true;
  }
}

function storageRemove(key: string) {
  if (storageUnavailable || typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(key);
  } catch {
    storageUnavailable = true;
  }
}

function radarKey() {
  return memoryRadarKey || storageGet(RADAR_KEY_STORAGE) || "";
}

function rememberRadarKey(value: string) {
  memoryRadarKey = value.trim();
  storageSet(RADAR_KEY_STORAGE, memoryRadarKey);
}

async function request<T>(path: string, options: RequestInit = {}, key = radarKey()): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (options.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (key) headers.set("Authorization", `Bearer ${key}`);
  const response = await fetch(path, { ...options, headers });
  if (!response.ok) {
    let detail = `Trendinary API ${response.status}`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) detail = payload.error;
    } catch {
      // Keep the status-derived message.
    }
    throw new FollowingAPIError(response.status, detail);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

async function createRadar(): Promise<RadarCreated> {
  const payload = await request<Envelope<RadarCreated>>("/api/v1/following/radar", { method: "POST" }, "");
  rememberRadarKey(payload.data.sync_key);
  return payload.data;
}

async function stateForKey(key: string): Promise<FollowingState> {
  return (await request<Envelope<FollowingState>>("/api/v1/following/state", {}, key)).data;
}

async function migrateLegacyFollows(key: string) {
  if (storageGet(LEGACY_MIGRATED) === "1") return;
  const raw = storageGet(LEGACY_STORAGE);
  if (!raw) {
    storageSet(LEGACY_MIGRATED, "1");
    return;
  }
  try {
    const legacy = JSON.parse(raw) as LegacyState;
    for (const follow of legacy.follows ?? []) {
      const slug = follow.slug?.trim();
      if (!slug) continue;
      await request<Envelope<FollowRecord>>(
        "/api/v1/following/follows",
        {
          method: "POST",
          body: JSON.stringify({ kind: "trend", value: slug, display_name: follow.name || slug }),
        },
        key,
      );
    }
    storageSet(LEGACY_MIGRATED, "1");
  } catch {
    // Migration is intentionally retryable. Never delete the v0.4 local data
    // merely because the first v0.5 server sync happened during an outage.
  }
}

async function existingPushSubscription() {
  if (typeof window === "undefined" || !("serviceWorker" in navigator) || !("PushManager" in window)) return undefined;
  const registration = await navigator.serviceWorker.getRegistration("/");
  return registration?.pushManager.getSubscription();
}

async function bindSubscriptionToRadar(key: string, subscription: PushSubscription) {
  const serialized = subscription.toJSON();
  if (!serialized.endpoint || !serialized.keys?.p256dh || !serialized.keys.auth) return;
  await request<Envelope<{ id: string }>>(
    "/api/v1/following/push/subscription",
    {
      method: "PUT",
      body: JSON.stringify({ endpoint: serialized.endpoint, p256dh: serialized.keys.p256dh, auth: serialized.keys.auth }),
    },
    key,
  );
}

async function rebindExistingPush(key: string) {
  const subscription = await existingPushSubscription();
  if (subscription) await bindSubscriptionToRadar(key, subscription);
}

async function ensureRadarInternal(): Promise<{ key: string; state: FollowingState }> {
  let key = radarKey();
  if (key) {
    try {
      const state = await stateForKey(key);
      await migrateLegacyFollows(key);
      return { key, state: await stateForKey(key) };
    } catch (reason) {
      if (!(reason instanceof FollowingAPIError) || (reason.status !== 401 && reason.status !== 404)) throw reason;
      memoryRadarKey = "";
      storageRemove(RADAR_KEY_STORAGE);
      key = "";
    }
  }
  const created = await createRadar();
  key = created.sync_key;
  await migrateLegacyFollows(key);
  return { key, state: await stateForKey(key) };
}

export function ensureRadar(): Promise<{ key: string; state: FollowingState }> {
  if (!radarPromise) {
    radarPromise = ensureRadarInternal().finally(() => {
      radarPromise = undefined;
    });
  }
  return radarPromise;
}

export async function loadFollowingState(): Promise<FollowingState> {
  return (await ensureRadar()).state;
}

export function currentRadarKey() {
  return radarKey();
}

export async function importRadarKey(value: string): Promise<FollowingState> {
  const key = value.trim();
  if (!key) throw new Error("Radar Key is required.");
  const state = await stateForKey(key);
  radarPromise = undefined;
  rememberRadarKey(key);
  await rebindExistingPush(key);
  return state;
}

export async function startFreshRadar(): Promise<FollowingState> {
  radarPromise = undefined;
  memoryRadarKey = "";
  storageRemove(RADAR_KEY_STORAGE);
  const created = await createRadar();
  await rebindExistingPush(created.sync_key);
  return created.state;
}

export async function addFollow(kind: FollowKind, value: string, displayName?: string): Promise<FollowingState> {
  const { key } = await ensureRadar();
  await request<Envelope<FollowRecord>>(
    "/api/v1/following/follows",
    {
      method: "POST",
      body: JSON.stringify({ kind, value, display_name: displayName || value }),
    },
    key,
  );
  return stateForKey(key);
}

export async function followTrend(trend: Trend) {
  return addFollow("trend", trend.slug, trend.name);
}

export async function followEntity(value: string) {
  return addFollow("entity", value, value);
}

export async function followTopic(value: string) {
  return addFollow("topic", value, value);
}

export async function unfollow(id: string): Promise<FollowingState> {
  const { key } = await ensureRadar();
  await request<void>(`/api/v1/following/follows/${encodeURIComponent(id)}`, { method: "DELETE" }, key);
  return stateForKey(key);
}

export async function updateRadarPreferences(preferences: RadarPreferences): Promise<FollowingState> {
  const { key } = await ensureRadar();
  await request<Envelope<RadarPreferences>>(
    "/api/v1/following/preferences",
    { method: "PATCH", body: JSON.stringify(preferences) },
    key,
  );
  return stateForKey(key);
}

export async function markFollowingAlertsRead(): Promise<FollowingState> {
  const { key } = await ensureRadar();
  await request<void>("/api/v1/following/alerts/read", { method: "POST" }, key);
  return stateForKey(key);
}

export async function clearFollowingAlerts(): Promise<FollowingState> {
  const { key } = await ensureRadar();
  await request<void>("/api/v1/following/alerts", { method: "DELETE" }, key);
  return stateForKey(key);
}

export async function refreshFollowing(): Promise<FollowingState> {
  const { key } = await ensureRadar();
  return (await request<Envelope<FollowingState>>("/api/v1/following/check", { method: "POST" }, key)).data;
}

export async function deleteCurrentRadar(): Promise<FollowingState> {
  const { key } = await ensureRadar();
  await request<void>("/api/v1/following/radar", { method: "DELETE" }, key);
  radarPromise = undefined;
  memoryRadarKey = "";
  storageRemove(RADAR_KEY_STORAGE);
  const created = await createRadar();
  await rebindExistingPush(created.sync_key);
  return created.state;
}

export function startFollowingMonitor(onUpdate: (state: FollowingState) => void, intervalMs = 60_000) {
  if (typeof window === "undefined") return () => undefined;
  let stopped = false;
  let running = false;
  const run = async () => {
    if (stopped || running) return;
    running = true;
    try {
      onUpdate(await loadFollowingState());
    } catch {
      // Keep the last good state during a temporary network outage.
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

function urlBase64ToUint8Array(value: string) {
  const padding = "=".repeat((4 - (value.length % 4)) % 4);
  const base64 = (value + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  return Uint8Array.from(raw, (character) => character.charCodeAt(0));
}

export type PushState = "enabled" | "disabled" | "denied" | "unsupported";

export async function webPushState(): Promise<PushState> {
  if (typeof window === "undefined" || !("serviceWorker" in navigator) || !("PushManager" in window) || typeof Notification === "undefined") return "unsupported";
  if (Notification.permission === "denied") return "denied";
  return (await existingPushSubscription()) ? "enabled" : "disabled";
}

export async function enableWebPush(): Promise<PushState> {
  if (typeof window === "undefined" || !("serviceWorker" in navigator) || !("PushManager" in window) || typeof Notification === "undefined") return "unsupported";
  const permission = Notification.permission === "granted" ? "granted" : await Notification.requestPermission();
  if (permission !== "granted") return permission === "denied" ? "denied" : "disabled";

  const { key } = await ensureRadar();
  await navigator.serviceWorker.register("/sw.js", { scope: "/" });
  const registration = await navigator.serviceWorker.ready;
  const publicKey = (await request<Envelope<{ public_key: string }>>("/api/v1/following/push/public-key", {}, "")).data.public_key;
  let subscription = await registration.pushManager.getSubscription();
  if (!subscription) {
    subscription = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(publicKey),
    });
  }
  await bindSubscriptionToRadar(key, subscription);
  return "enabled";
}

export async function disableWebPush(): Promise<PushState> {
  if (typeof window === "undefined" || !("serviceWorker" in navigator)) return "unsupported";
  const subscription = await existingPushSubscription();
  if (!subscription) return "disabled";
  const { key } = await ensureRadar();
  try {
    await request<void>(
      "/api/v1/following/push/subscription",
      { method: "DELETE", body: JSON.stringify({ endpoint: subscription.endpoint }) },
      key,
    );
  } finally {
    await subscription.unsubscribe();
  }
  return "disabled";
}
