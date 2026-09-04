import { createSignal, For, onCleanup, Show } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import { fetchTrend, fetchTrends, type Trend } from "./api";
import {
  clearFollowingAlerts,
  followTrend,
  isFollowing,
  loadFollowingState,
  markFollowingAlertsRead,
  refreshFollowing,
  requestBrowserFollowingAlerts,
  startFollowingMonitor,
  unfollowTrend,
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

function notificationStatus() {
  if (typeof Notification === "undefined") return "unsupported";
  return Notification.permission;
}

export function FollowingPage() {
  const [state, setState] = createSignal(loadFollowingState());
  const [trends, setTrends] = createSignal<Trend[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string>();
  const [permission, setPermission] = createSignal(notificationStatus());
  const [refreshing, setRefreshing] = createSignal(false);

  const loadTrends = async () => {
    try {
      setTrends(await fetchTrends());
    } catch (reason) {
      setError(message(reason));
    } finally {
      setLoading(false);
    }
  };

  const bootstrapQueryFollow = async () => {
    const slug = new URLSearchParams(window.location.search).get("follow")?.trim();
    if (!slug || isFollowing(slug)) return;
    try {
      const trend = await fetchTrend(slug);
      setState(followTrend(trend));
      window.history.replaceState(null, "", "/following");
    } catch (reason) {
      setError(message(reason));
    }
  };

  void loadTrends();
  void bootstrapQueryFollow();
  const stopMonitor = startFollowingMonitor(setState);
  onCleanup(stopMonitor);

  const toggle = (trend: Trend) => {
    setState(isFollowing(trend.slug) ? unfollowTrend(trend.slug) : followTrend(trend));
  };

  const remove = (slug: string) => setState(unfollowTrend(slug));

  const refresh = async () => {
    if (refreshing()) return;
    setRefreshing(true);
    try {
      setState(await refreshFollowing());
    } finally {
      setRefreshing(false);
    }
  };

  const enableNotifications = async () => {
    const result = await requestBrowserFollowingAlerts();
    setPermission(result);
  };

  const unread = () => state().alerts.filter((alert) => !alert.read).length;

  return (
    <>
      <section {...sx(styles.hero)}>
        <div>
          <div {...sx(styles.eyebrow)}>FOLLOWING · DEVICE-LOCAL RADAR</div>
          <h1 {...sx(styles.heroTitle)}>Tell me when <span {...sx(styles.heroAccent)}>something changes.</span></h1>
          <p {...sx(styles.heroCopy)}>Follow a trend without creating an account. Trendinary watches lifecycle, acceleration, resurfacing, and independent-source corroboration and stays quiet when nothing materially changes.</p>
        </div>
        <div {...sx(styles.statusCard)}>
          <div {...sx(styles.statusLabel)}>Radar status</div>
          <div {...sx(styles.statusValue)}>{state().follows.length ? `${state().follows.length} FOLLOWED` : "QUIET"}</div>
          <div {...sx(styles.statusSub)}>{unread()} unread alert{unread() === 1 ? "" : "s"} · browser notifications {permission()}</div>
          <Show when={permission() !== "granted" && permission() !== "unsupported"}>
            <button {...sx(styles.smallButton)} type="button" onClick={() => void enableNotifications()}>Enable browser alerts</button>
          </Show>
        </div>
      </section>
      <ProductTabs />

      <Show when={error()}>{(value) => <section {...sx(styles.section)}><div {...sx(styles.emptyState)}>{value()}</div></section>}</Show>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div>
            <h2 {...sx(styles.sectionTitle)}>Alert inbox</h2>
            <p {...sx(styles.sectionCopy)}>Alerts are generated from changes since this browser's last observed baseline. Browser notifications work while Trendinary is open; the in-app inbox is the durable device-local record.</p>
          </div>
          <div {...sx(styles.chips)}>
            <button {...sx(styles.smallButton)} type="button" disabled={refreshing()} onClick={() => void refresh()}>{refreshing() ? "Checking…" : "Check now"}</button>
            <Show when={unread() > 0}><button {...sx(styles.smallButton)} type="button" onClick={() => setState(markFollowingAlertsRead())}>Mark read</button></Show>
            <Show when={state().alerts.length > 0}><button {...sx(styles.smallButton)} type="button" onClick={() => setState(clearFollowingAlerts())}>Clear</button></Show>
          </div>
        </div>
        <Show when={state().alerts.length > 0} fallback={<div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>QUIET IS VALID OUTPUT</div><h3 {...sx(styles.cardTitle)}>Nothing material changed.</h3><p {...sx(styles.cardCopy)}>Trendinary will surface acceleration, lifecycle transitions, resurfacing, or broader corroboration here.</p></div>}>
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
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Your radar</h2><p {...sx(styles.sectionCopy)}>Stored only in this browser for v0.4.0. No account, email address, or cross-device profile is required.</p></div>
        </div>
        <Show when={state().follows.length > 0} fallback={<div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>NO FOLLOWS YET</div><h3 {...sx(styles.cardTitle)}>Pick something from the live scoreboard below.</h3></div>}>
          <div {...sx(styles.grid3)}>
            <For each={state().follows}>{(follow) => (
              <article {...sx(styles.card)}>
                <div><div {...sx(styles.cardKicker)}>{follow.baseline?.status ?? "FOLLOWING"}</div><h3 {...sx(styles.cardTitle)}>{follow.name}</h3><p {...sx(styles.cardCopy)}>Last baseline: score {follow.baseline?.score ?? "—"} · velocity {Math.round((follow.baseline?.velocity ?? 0) * 100)}% · {follow.baseline?.source_count ?? 0} sources</p></div>
                <div {...sx(styles.chips)}><a {...sx(styles.chip)} href={`/trend/${follow.slug}`}>Open trend</a><button {...sx(styles.smallButton)} type="button" onClick={() => remove(follow.slug)}>Unfollow</button></div>
              </article>
            )}</For>
          </div>
        </Show>
      </section>

      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>Follow something live</h2><p {...sx(styles.sectionCopy)}>The initial baseline is captured when you follow it, so existing popularity does not create a fake alert.</p></div></div>
        <Show when={!loading()} fallback={<div {...sx(styles.emptyState)}>Loading live trends…</div>}>
          <div {...sx(styles.grid3)}>
            <For each={trends()}>{(trend) => (
              <article {...sx(styles.card)}>
                <div><div {...sx(styles.cardKicker)}>{trend.status} · SCORE {trend.score}</div><h3 {...sx(styles.cardTitle)}>{trend.name}</h3><p {...sx(styles.cardCopy)}>{trend.reason}</p></div>
                <div {...sx(styles.chips)}><a {...sx(styles.chip)} href={`/trend/${trend.slug}`}>Details</a><button {...sx(styles.followButton)} type="button" onClick={() => toggle(trend)}>{isFollowing(trend.slug) ? "Following ✓" : "Follow"}</button></div>
              </article>
            )}</For>
          </div>
        </Show>
      </section>
    </>
  );
}
