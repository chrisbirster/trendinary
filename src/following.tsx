import { createSignal, For, onCleanup, Show } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import { fetchTrend, fetchTrends, type Trend } from "./api";
import {
  addFollow,
  clearFollowingAlerts,
  currentRadarKey,
  disableWebPush,
  emptyFollowingState,
  enableWebPush,
  followTrend,
  importRadarKey,
  loadFollowingState,
  markFollowingAlertsRead,
  refreshFollowing,
  startFollowingMonitor,
  startFreshRadar,
  type FollowKind,
  type PushState,
  type RadarPreferences,
  type RadarSensitivity,
  unfollow,
  updateRadarPreferences,
  webPushState,
} from "./following-store";
import { styles } from "./styles.stylex";

const sx = stylex.attrs;

function message(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function ProductTabs() {
  return (
    <div {...sx(styles.tabs)}>
      <a {...sx(styles.tab)} href="/">Now</a>
      <a {...sx(styles.tab)} href="/peep">PEEP 👀</a>
      <a {...sx(styles.tab)} href="/fomo">FOMO</a>
      <a {...sx(styles.tab)} href="/following">Following</a>
    </div>
  );
}

function relativeTime(value: string) {
  const at = new Date(value).getTime();
  if (!Number.isFinite(at)) return value;
  const minutes = Math.max(0, Math.floor((Date.now() - at) / 60_000));
  if (minutes < 1) return "now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function maskedKey(value: string) {
  if (!value) return "creating…";
  return `${value.slice(0, 8)}…${value.slice(-6)}`;
}

export function FollowingPage() {
  const [state, setState] = createSignal(emptyFollowingState());
  const [trends, setTrends] = createSignal<Trend[]>([]);
  const [ready, setReady] = createSignal(false);
  const [error, setError] = createSignal<string>();
  const [working, setWorking] = createSignal(false);
  const [push, setPush] = createSignal<PushState>("disabled");
  const [radarKey, setRadarKey] = createSignal(currentRadarKey());
  const [importKey, setImportKey] = createSignal("");
  const [topic, setTopic] = createSignal("");
  const [entity, setEntity] = createSignal("");
  const [copied, setCopied] = createSignal(false);

  const isFollowed = (slug: string) => state().follows.find((follow) => follow.kind === "trend" && follow.value === slug);
  const unread = () => state().alerts.filter((alert) => !alert.read).length;

  const initialize = async () => {
    try {
      const [nextState, live, pushState] = await Promise.all([
        loadFollowingState(),
        fetchTrends(),
        webPushState(),
      ]);
      setState(nextState);
      setTrends(live);
      setPush(pushState);
      setRadarKey(currentRadarKey());
      const slug = new URLSearchParams(window.location.search).get("follow")?.trim();
      if (slug && !nextState.follows.some((follow) => follow.kind === "trend" && follow.value === slug)) {
        const trend = await fetchTrend(slug);
        setState(await followTrend(trend));
        window.history.replaceState(null, "", "/following");
      }
    } catch (reason) {
      setError(message(reason));
    } finally {
      setReady(true);
    }
  };

  void initialize();
  const stopMonitor = startFollowingMonitor((next) => {
    setState(next);
    setRadarKey(currentRadarKey());
  });
  onCleanup(stopMonitor);

  const perform = async (operation: () => Promise<ReturnType<typeof state>>) => {
    if (working()) return;
    setWorking(true);
    setError(undefined);
    try {
      setState(await operation());
      setRadarKey(currentRadarKey());
    } catch (reason) {
      setError(message(reason));
    } finally {
      setWorking(false);
    }
  };

  const toggleTrend = (trend: Trend) => {
    const existing = isFollowed(trend.slug);
    void perform(() => existing ? unfollow(existing.id) : followTrend(trend));
  };

  const addNamedFollow = (kind: FollowKind, value: string, clear: () => void) => {
    const normalized = value.trim();
    if (!normalized) return;
    void perform(async () => {
      const next = await addFollow(kind, normalized, normalized);
      clear();
      return next;
    });
  };

  const setSensitivity = (sensitivity: RadarSensitivity) => {
    void perform(() => updateRadarPreferences({ ...state().preferences, sensitivity }));
  };

  const togglePreference = (key: keyof Omit<RadarPreferences, "sensitivity">) => {
    void perform(() => updateRadarPreferences({ ...state().preferences, [key]: !state().preferences[key] }));
  };

  const copyKey = async () => {
    const value = currentRadarKey();
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setError("Clipboard access was blocked. Select the Radar Key from your password manager/import flow instead.");
    }
  };

  const importExisting = () => {
    const value = importKey().trim();
    if (!value) return;
    void perform(async () => {
      const next = await importRadarKey(value);
      setImportKey("");
      setRadarKey(currentRadarKey());
      setPush(await webPushState());
      return next;
    });
  };

  const newRadar = () => {
    void perform(async () => {
      const next = await startFreshRadar();
      setRadarKey(currentRadarKey());
      setPush(await webPushState());
      return next;
    });
  };

  const togglePush = async () => {
    if (working()) return;
    setWorking(true);
    setError(undefined);
    try {
      setPush(push() === "enabled" ? await disableWebPush() : await enableWebPush());
    } catch (reason) {
      setError(message(reason));
    } finally {
      setWorking(false);
    }
  };

  const refresh = () => void perform(() => refreshFollowing());

  return (
    <>
      <section {...sx(styles.hero)}>
        <div>
          <div {...sx(styles.eyebrow)}>FOLLOWING · CROSS-DEVICE RADAR</div>
          <h1 {...sx(styles.heroTitle)}>Tell me when <span {...sx(styles.heroAccent)}>something changes.</span></h1>
          <p {...sx(styles.heroCopy)}>Trendinary now watches your radar on the server even when every browser is closed. No account or email is required: a private Radar Key syncs follows, alert settings, and the inbox across your devices.</p>
        </div>
        <div {...sx(styles.statusCard)}>
          <div {...sx(styles.statusLabel)}>Radar status</div>
          <div {...sx(styles.statusValue)}>{state().follows.length ? `${state().follows.length} FOLLOWED` : "QUIET"}</div>
          <div {...sx(styles.statusSub)}>{unread()} unread · Web Push {push()} · {state().preferences.sensitivity} sensitivity</div>
          <button {...sx(styles.followButton)} type="button" disabled={working() || push() === "unsupported" || push() === "denied"} onClick={() => void togglePush()}>
            {push() === "enabled" ? "Disable push" : push() === "denied" ? "Push blocked" : push() === "unsupported" ? "Push unsupported" : "Enable closed-browser push"}
          </button>
        </div>
      </section>
      <ProductTabs />

      <Show when={error()}>{(value) => <section {...sx(styles.section)}><div {...sx(styles.emptyState)}>{value()}</div></section>}</Show>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Cross-device sync</h2><p {...sx(styles.sectionCopy)}>Your Radar Key is the credential. Trendinary stores only its one-way SHA-256 identity, so the original key cannot be recovered from the server.</p></div>
        </div>
        <div {...sx(styles.grid3)}>
          <article {...sx(styles.card)}>
            <div {...sx(styles.cardKicker)}>THIS RADAR</div>
            <h3 {...sx(styles.cardTitle)}>{maskedKey(radarKey())}</h3>
            <p {...sx(styles.cardCopy)}>Treat the full Radar Key like a password. Anyone with it can read or change this radar.</p>
            <div {...sx(styles.chips)}><button {...sx(styles.smallButton)} type="button" onClick={() => void copyKey()}>{copied() ? "Copied ✓" : "Copy Radar Key"}</button><button {...sx(styles.smallButton)} type="button" onClick={newRadar}>Start fresh radar</button></div>
          </article>
          <article {...sx(styles.card)}>
            <div {...sx(styles.cardKicker)}>USE ANOTHER DEVICE'S RADAR</div>
            <h3 {...sx(styles.cardTitle)}>Import Radar Key</h3>
            <input {...sx(styles.askInput)} type="password" autocomplete="off" value={importKey()} onInput={(event) => setImportKey(event.currentTarget.value)} placeholder="Paste Radar Key" />
            <button {...sx(styles.smallButton)} type="button" disabled={!importKey().trim() || working()} onClick={importExisting}>Import and sync</button>
          </article>
          <article {...sx(styles.card)}>
            <div {...sx(styles.cardKicker)}>MIGRATION</div>
            <h3 {...sx(styles.cardTitle)}>v0.4 follows come with you.</h3>
            <p {...sx(styles.cardCopy)}>The first v0.5 radar on this browser automatically copies existing device-local trend follows to Turso. The old local record is retained if migration is interrupted so it can retry safely.</p>
          </article>
        </div>
      </section>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Alert tuning</h2><p {...sx(styles.sectionCopy)}>Choose how early Trendinary should bother you and which kinds of material change matter.</p></div>
        </div>
        <div {...sx(styles.card)}>
          <div {...sx(styles.cardKicker)}>SENSITIVITY</div>
          <div {...sx(styles.chips)}>
            <For each={["early", "balanced", "quiet"] as RadarSensitivity[]}>{(value) => <button {...sx(state().preferences.sensitivity === value ? styles.followButton : styles.smallButton)} type="button" onClick={() => setSensitivity(value)}>{value.toUpperCase()}</button>}</For>
          </div>
          <div {...sx(styles.cardKicker)}>ALERT TYPES</div>
          <div {...sx(styles.chips)}>
            <button {...sx(state().preferences.lifecycle ? styles.followButton : styles.smallButton)} type="button" onClick={() => togglePreference("lifecycle")}>Lifecycle {state().preferences.lifecycle ? "ON" : "OFF"}</button>
            <button {...sx(state().preferences.velocity ? styles.followButton : styles.smallButton)} type="button" onClick={() => togglePreference("velocity")}>Acceleration {state().preferences.velocity ? "ON" : "OFF"}</button>
            <button {...sx(state().preferences.corroboration ? styles.followButton : styles.smallButton)} type="button" onClick={() => togglePreference("corroboration")}>Corroboration {state().preferences.corroboration ? "ON" : "OFF"}</button>
            <button {...sx(state().preferences.resurfacing ? styles.followButton : styles.smallButton)} type="button" onClick={() => togglePreference("resurfacing")}>Resurfacing {state().preferences.resurfacing ? "ON" : "OFF"}</button>
          </div>
        </div>
      </section>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Alert inbox</h2><p {...sx(styles.sectionCopy)}>This inbox is durable and shared by every device using the Radar Key. The server evaluates the radar once per minute even with no browser open.</p></div>
          <div {...sx(styles.chips)}>
            <button {...sx(styles.smallButton)} type="button" disabled={working()} onClick={refresh}>{working() ? "Syncing…" : "Sync now"}</button>
            <Show when={unread() > 0}><button {...sx(styles.smallButton)} type="button" onClick={() => void perform(markFollowingAlertsRead)}>Mark read</button></Show>
            <Show when={state().alerts.length > 0}><button {...sx(styles.smallButton)} type="button" onClick={() => void perform(clearFollowingAlerts)}>Clear</button></Show>
          </div>
        </div>
        <Show when={state().alerts.length > 0} fallback={<div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>QUIET IS VALID OUTPUT</div><h3 {...sx(styles.cardTitle)}>Nothing material changed.</h3><p {...sx(styles.cardCopy)}>Trendinary will surface acceleration, lifecycle transitions, resurfacing, or broader independent-source corroboration here.</p></div>}>
          <div {...sx(styles.trendList)}>
            <For each={state().alerts}>{(alert) => (
              <a href={`/trend/${alert.slug}`} {...sx(styles.trendRow)}>
                <div {...sx(styles.rank)}>{alert.read ? "·" : "●"}</div>
                <div><div {...sx(styles.trendName)}>{alert.title}</div><div {...sx(styles.trendMeta)}>{alert.kind.toUpperCase()} · {relativeTime(alert.created_at)}</div></div>
                <div {...sx(styles.reason)}>{alert.body}</div>
                <div {...sx(styles.arrow)}>↗</div>
              </a>
            )}</For>
          </div>
        </Show>
      </section>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>Your radar</h2><p {...sx(styles.sectionCopy)}>Follow an exact trend, a recurring entity, or a broader topic. Entity/topic follows can match future trend clusters that did not exist when you followed them.</p></div></div>
        <Show when={ready()} fallback={<div {...sx(styles.emptyState)}>Loading radar…</div>}>
          <Show when={state().follows.length > 0} fallback={<div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>NO FOLLOWS YET</div><h3 {...sx(styles.cardTitle)}>Pick a live trend or add a topic/entity below.</h3></div>}>
            <div {...sx(styles.grid3)}>
              <For each={state().follows}>{(follow) => (
                <article {...sx(styles.card)}>
                  <div><div {...sx(styles.cardKicker)}>{follow.kind}</div><h3 {...sx(styles.cardTitle)}>{follow.display_name}</h3><p {...sx(styles.cardCopy)}>Watching <strong>{follow.value}</strong> since {relativeTime(follow.created_at)}.</p></div>
                  <div {...sx(styles.chips)}><Show when={follow.kind === "trend"}><a {...sx(styles.chip)} href={`/trend/${follow.value}`}>Open trend</a></Show><button {...sx(styles.smallButton)} type="button" onClick={() => void perform(() => unfollow(follow.id))}>Unfollow</button></div>
                </article>
              )}</For>
            </div>
          </Show>
        </Show>
      </section>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>Follow a topic or entity</h2><p {...sx(styles.sectionCopy)}>Topic follows search category/name/reason/aliases. Entity follows stay tighter to trend names and aliases.</p></div></div>
        <div {...sx(styles.grid3)}>
          <article {...sx(styles.card)}>
            <div {...sx(styles.cardKicker)}>TOPIC</div><h3 {...sx(styles.cardTitle)}>Watch a subject</h3>
            <input {...sx(styles.askInput)} value={topic()} onInput={(event) => setTopic(event.currentTarget.value)} placeholder="AI, cybersecurity, space…" />
            <button {...sx(styles.smallButton)} type="button" disabled={!topic().trim() || working()} onClick={() => addNamedFollow("topic", topic(), () => setTopic(""))}>Follow topic</button>
          </article>
          <article {...sx(styles.card)}>
            <div {...sx(styles.cardKicker)}>ENTITY</div><h3 {...sx(styles.cardTitle)}>Watch a named thing</h3>
            <input {...sx(styles.askInput)} value={entity()} onInput={(event) => setEntity(event.currentTarget.value)} placeholder="OpenAI, Audacity, Artemis…" />
            <button {...sx(styles.smallButton)} type="button" disabled={!entity().trim() || working()} onClick={() => addNamedFollow("entity", entity(), () => setEntity(""))}>Follow entity</button>
          </article>
          <article {...sx(styles.card)}>
            <div {...sx(styles.cardKicker)}>HOW MATCHING WORKS</div><h3 {...sx(styles.cardTitle)}>Future clusters count.</h3><p {...sx(styles.cardCopy)}>Unlike an exact trend follow, a topic or entity remains active when today's trend cools. A new matching stable trend can establish its own baseline and alert later.</p>
          </article>
        </div>
      </section>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>Follow something live</h2><p {...sx(styles.sectionCopy)}>The server captures the first matched observation as a baseline, so existing popularity does not create a fake alert.</p></div></div>
        <Show when={ready()} fallback={<div {...sx(styles.emptyState)}>Loading live trends…</div>}>
          <div {...sx(styles.grid3)}>
            <For each={trends()}>{(trend) => (
              <article {...sx(styles.card)}>
                <div><div {...sx(styles.cardKicker)}>{trend.status} · SCORE {trend.score}</div><h3 {...sx(styles.cardTitle)}>{trend.name}</h3><p {...sx(styles.cardCopy)}>{trend.reason}</p></div>
                <div {...sx(styles.chips)}><a {...sx(styles.chip)} href={`/trend/${trend.slug}`}>Details</a><button {...sx(styles.followButton)} type="button" disabled={working()} onClick={() => toggleTrend(trend)}>{isFollowed(trend.slug) ? "Following ✓" : "Follow"}</button></div>
              </article>
            )}</For>
          </div>
        </Show>
      </section>
    </>
  );
}
