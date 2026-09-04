import { createSignal, For, Show } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import {
  fetchSourceRuns,
  fetchSources,
  fetchTrendSourceHealth,
  ingestSource,
  setSourceEnabled,
  type EditorialSource,
  type IngestionRun,
} from "./admin-api";
import {
  fetchSourceAnalytics,
  type SourceAnalyticsReport,
  type SourceContribution,
  type SourceReliabilityStatus,
} from "./source-intelligence-api";
import { adminStyles as styles } from "./admin.stylex";

const sx = stylex.attrs;

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function SourceShell(props: { children: unknown }) {
  const path = window.location.pathname;
  const links = [
    ["Inbox", "/admin/inbox"],
    ["Queue", "/admin/queue"],
    ["Notes", "/admin/notes"],
    ["Issues", "/admin/issues"],
    ["Sources", "/admin/sources"],
    ["Quality", "/admin/quality"],
    ["Trash", "/admin/trash"],
  ];
  return (
    <div {...sx(styles.page)}>
      <div {...sx(styles.shell)}>
        <aside {...sx(styles.sidebar)}>
          <div {...sx(styles.brand)}>Trendinary</div>
          <div {...sx(styles.adminLabel)}>Private editorial admin</div>
          <nav {...sx(styles.nav)}>
            <For each={links}>{([label, href]) => <a href={href} {...sx(styles.navLink, path === href ? styles.navActive : undefined)}>{label}</a>}</For>
          </nav>
          <a href="/" {...sx(styles.backLink)}>← Public Trendinary</a>
        </aside>
        <main {...sx(styles.main)}>
          <header {...sx(styles.topbar)}>
            <div>
              <div {...sx(styles.eyebrow)}>Discovery operations</div>
              <h1 {...sx(styles.title)}>Sources</h1>
              <p {...sx(styles.copy)}>Everything Trendinary listens to, whether it is healthy, and whether it actually contributes useful early evidence.</p>
            </div>
          </header>
          {props.children as never}
        </main>
      </div>
    </div>
  );
}

function relativeTime(value?: string) {
  if (!value) return "never";
  const delta = Date.now() - new Date(value).getTime();
  const minutes = Math.max(0, Math.round(delta / 60000));
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

function cadenceLabel(nanoseconds: number) {
  if (!nanoseconds) return "continuous";
  const minutes = Math.round(nanoseconds / 60_000_000_000);
  if (minutes < 60) return `${Math.max(1, minutes)}m`;
  const hours = minutes / 60;
  return Number.isInteger(hours) ? `${hours}h` : `${hours.toFixed(1)}h`;
}

function durationLabel(nanoseconds?: number) {
  if (!nanoseconds) return "—";
  const milliseconds = nanoseconds / 1_000_000;
  if (milliseconds < 1000) return `${Math.round(milliseconds)}ms`;
  return `${(milliseconds / 1000).toFixed(1)}s`;
}

function percent(numerator = 0, denominator = 0) {
  if (denominator <= 0) return "—";
  return `${Math.round((numerator / denominator) * 100)}%`;
}

function sourceState(source: SourceReliabilityStatus) {
  if (!source.enabled) return "NOT CONFIGURED";
  if (source.last_error) return "DEGRADED";
  if (source.last_success_at && source.cadence > 0) {
    const ageMs = Date.now() - new Date(source.last_success_at).getTime();
    const staleMs = Math.max(30 * 60_000, (source.cadence / 1_000_000) * 3);
    if (ageMs > staleMs) return "STALE";
  }
  if (source.last_success_at) return "HEALTHY";
  return "SCHEDULED";
}

function ContributionRows(props: { title: string; items: SourceContribution[] }) {
  return <section {...sx(styles.issueSection)}>
    <h2 {...sx(styles.sectionTitle)}>{props.title}</h2>
    <div {...sx(styles.list)}>
      <For each={props.items.slice(0, 12)}>{(item) =>
        <div {...sx(styles.sourceRow)}>
          <div><strong>{item.name}</strong><div {...sx(styles.copy)}>{item.key}</div></div>
          <span>{item.first_hits} first hits</span>
          <span>{item.trends} trends</span>
          <span>{item.signals} signals</span>
          <div>
            <span>{item.solo_trends} solo</span>
            <div {...sx(styles.copy)}>{item.average_lead_to_breaking_minutes > 0 ? `${Math.round(item.average_lead_to_breaking_minutes)}m avg lead` : "no BREAKING lead sample"}</div>
          </div>
        </div>
      }</For>
    </div>
  </section>;
}

export function AdminSourcesPage() {
  const [trendSources, setTrendSources] = createSignal<SourceReliabilityStatus[]>([]);
  const [analytics, setAnalytics] = createSignal<SourceAnalyticsReport>();
  const [editorialSources, setEditorialSources] = createSignal<EditorialSource[]>([]);
  const [runs, setRuns] = createSignal<IngestionRun[]>([]);
  const [busy, setBusy] = createSignal<string>();
  const [error, setError] = createSignal<string>();
  const [loading, setLoading] = createSignal(true);

  const load = async () => {
    setLoading(true);
    setError(undefined);
    try {
      const [health, editorial, contribution] = await Promise.all([
        fetchTrendSourceHealth(),
        fetchSources(),
        fetchSourceAnalytics("168h"),
      ]);
      setTrendSources((health.sources ?? []) as SourceReliabilityStatus[]);
      setEditorialSources(editorial);
      setAnalytics(contribution);
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setLoading(false);
    }
  };
  void load();

  const ingest = async (source: EditorialSource) => {
    setBusy(source.id);
    setError(undefined);
    try {
      await ingestSource(source.id);
      setRuns(await fetchSourceRuns(source.id));
      await load();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(undefined);
    }
  };

  const toggle = async (source: EditorialSource) => {
    setEditorialSources((values) => values.map((value) => value.id === source.id ? { ...value, enabled: !value.enabled } : value));
    try {
      setEditorialSources(await setSourceEnabled(source.id, !source.enabled));
    } catch (reason) {
      setError(errorMessage(reason));
      await load();
    }
  };

  const fleetAttempts = () => trendSources().reduce((sum, source) => sum + (source.attempts ?? 0), 0);
  const fleetSuccesses = () => trendSources().reduce((sum, source) => sum + (source.successes ?? 0), 0);
  const fleetSignals = () => trendSources().reduce((sum, source) => sum + (source.signals_produced ?? 0), 0);

  return <SourceShell>
    <Show when={error()}>{(value) => <div {...sx(styles.error)}>{value()}</div>}</Show>
    <Show when={!loading()} fallback={<div {...sx(styles.status)}>Loading source operations…</div>}>
      <section {...sx(styles.issueSection)}>
        <h2 {...sx(styles.sectionTitle)}>Fleet reliability · {trendSources().filter((source) => source.enabled).length} active</h2>
        <p {...sx(styles.copy)}>Process-lifetime counters: {fleetSuccesses()}/{fleetAttempts()} successful polls · {fleetSignals()} signals produced. RSS sources also report conditional-request reuse.</p>
        <div {...sx(styles.list)}>
          <For each={trendSources()}>{(source) =>
            <div {...sx(styles.sourceRow)}>
              <div>
                <strong>{source.name}</strong>
                <div {...sx(styles.copy)}>{source.kind} · {source.policy ?? "policy unspecified"}</div>
                <Show when={source.url}><div {...sx(styles.copy)}>{source.url}</div></Show>
                <Show when={source.terms_url}><a href={source.terms_url} target="_blank" rel="noreferrer" {...sx(styles.backLink)}>policy / docs ↗</a></Show>
              </div>
              <span>{sourceState(source)}</span>
              <div><span>every {cadenceLabel(source.cadence)}</span><div {...sx(styles.copy)}>avg {durationLabel(source.average_duration)}</div></div>
              <div><span>{percent(source.successes, source.attempts)} success</span><div {...sx(styles.copy)}>last {relativeTime(source.last_success_at)}</div></div>
              <div>
                <span>{source.cached_signals ?? 0} cached · {source.signals_produced ?? 0} produced</span>
                <Show when={(source.http_requests ?? 0) > 0}><div {...sx(styles.copy)}>{percent(source.not_modified, source.http_requests)} HTTP 304</div></Show>
                <Show when={source.last_error}><div {...sx(styles.error)}>{source.last_error}</div></Show>
                <Show when={source.next_run_at && source.enabled}><div {...sx(styles.copy)}>next {relativeTime(source.next_run_at).replace(" ago", "")}</div></Show>
              </div>
            </div>
          }</For>
        </div>
      </section>

      <Show when={analytics()}>{(value) => <>
        <ContributionRows title="Publisher contribution · last 7 days" items={value().publishers} />
        <ContributionRows title="Discovery-channel contribution · last 7 days" items={value().channels} />
      </>}</Show>

      <section {...sx(styles.issueSection)}>
        <h2 {...sx(styles.sectionTitle)}>Editorial discovery sources</h2>
        <p {...sx(styles.copy)}>Private reading and newsletter research. These feed Inbox → Queue → Notes → Worth Sharing and are separate from public trend scoring.</p>
        <For each={editorialSources()}>{(source) =>
          <div {...sx(styles.sourceRow)}>
            <div><strong>{source.name}</strong><div {...sx(styles.copy)}>{source.kind} · {source.url}</div></div>
            <span>{source.enabled ? "ENABLED" : "DISABLED"}</span>
            <span>last {relativeTime(source.last_successful_at)}</span>
            <span>{source.latest_item_count} items</span>
            <div {...sx(styles.actions)}>
              <button {...sx(styles.button)} onClick={() => void toggle(source)}>{source.enabled ? "Disable" : "Enable"}</button>
              <button disabled={!source.enabled || busy() === source.id} {...sx(styles.button, styles.buttonAccent)} onClick={() => void ingest(source)}>{busy() === source.id ? "Fetching…" : "Fetch now"}</button>
            </div>
          </div>
        }</For>
      </section>

      <Show when={runs().length}>
        <section {...sx(styles.issueSection)}>
          <h2 {...sx(styles.sectionTitle)}>Last editorial ingestion</h2>
          <For each={runs().slice(0, 5)}>{(run) => <div {...sx(styles.itemLine)}><span>{run.status} · {new Date(run.started_at).toLocaleString()}</span><span>{run.metrics.items_inserted} new · {run.metrics.duplicates} dupes · {run.metrics.malformed} malformed</span></div>}</For>
        </section>
      </Show>
    </Show>
  </SourceShell>;
}
