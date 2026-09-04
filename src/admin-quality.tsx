import { createSignal, For, Show } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import { fetchTrends, type Trend } from "./api";
import { adminStyles as styles } from "./admin.stylex";
import {
  fetchQualityFeedback,
  fetchQualityReplay,
  fetchQualityReport,
  labelTrend,
  type QualityFeedback,
  type QualityLabel,
  type QualityReport,
  type ReplayReport,
} from "./quality-api";
import { fetchCalibrationV1, type CalibrationV1 } from "./source-intelligence-api";

const sx = stylex.attrs;

const labels: Array<[QualityLabel, string]> = [
  ["real-trend", "Real trend"],
  ["interesting-too-early", "Good early catch"],
  ["detected-too-late", "Too late"],
  ["noise", "Noise"],
  ["duplicate", "Duplicate"],
  ["bad-cluster", "Bad cluster"],
  ["wrong-canonical-name", "Wrong name"],
];

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function percent(value: number) {
  return `${Math.round(value * 100)}%`;
}

function measuredPercent(value: number, sample: number) {
  return sample > 0 ? percent(value) : "—";
}

function duration(minutes: number) {
  if (minutes <= 0) return "—";
  if (minutes < 60) return `${Math.round(minutes)}m`;
  const hours = Math.floor(minutes / 60);
  const remainder = Math.round(minutes % 60);
  return remainder ? `${hours}h ${remainder}m` : `${hours}h`;
}

function QualityShell(props: { children: unknown }) {
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
            <For each={links}>{([label, href]) => <a href={href} {...sx(styles.navLink, href === "/admin/quality" ? styles.navActive : undefined)}>{label}</a>}</For>
          </nav>
          <a href="/" {...sx(styles.backLink)}>← Public Trendinary</a>
        </aside>
        <main {...sx(styles.main)}>
          <header {...sx(styles.topbar)}>
            <div>
              <div {...sx(styles.eyebrow)}>Signal quality v3</div>
              <h1 {...sx(styles.title)}>Teach the detector what “good” means.</h1>
              <p {...sx(styles.copy)}>Label real production trends, noise, timing misses, duplicate clusters, and naming mistakes. Trendinary turns those labels into measurable calibration and replay benchmarks.</p>
            </div>
          </header>
          {props.children as never}
        </main>
      </div>
    </div>
  );
}

function Metric(props: { label: string; value: string | number; copy?: string }) {
  return (
    <article {...sx(styles.card)}>
      <div>
        <div {...sx(styles.meta)}>{props.label}</div>
        <Show when={props.copy}><div {...sx(styles.description)}>{props.copy}</div></Show>
      </div>
      <div>
        <div {...sx(styles.score)}>{props.value}</div>
        <div {...sx(styles.scoreLabel)}>quality signal</div>
      </div>
    </article>
  );
}

function TrendLabelCard(props: { trend: Trend; onLabeled: () => void; latest?: QualityFeedback }) {
  const [note, setNote] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal<string>();

  const save = async (label: QualityLabel) => {
    if (busy()) return;
    setBusy(true);
    setError(undefined);
    try {
      await labelTrend(props.trend, label, note());
      setNote("");
      props.onLabeled();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };

  return (
    <article {...sx(styles.card)}>
      <div>
        <div {...sx(styles.meta)}>
          <span>#{props.trend.rank}</span><span>{props.trend.status}</span><span>score {props.trend.score}</span><span>PEEP {props.trend.quality.peep_score}</span>
          <Show when={props.latest}><span>latest: {props.latest?.label}</span></Show>
        </div>
        <h2 {...sx(styles.headline)}>{props.trend.name}</h2>
        <p {...sx(styles.description)}>{props.trend.reason}</p>
        <div {...sx(styles.why)}>Watching because: {(props.trend.quality.why_watching ?? []).join(" · ") || "quality metrics still accumulating"}</div>
        <div {...sx(styles.toolbar)}>
          <input {...sx(styles.select)} value={note()} onInput={(event) => setNote(event.currentTarget.value)} placeholder="Optional calibration note" />
        </div>
        <div {...sx(styles.actions)}>
          <For each={labels}>{([label, title]) => <button disabled={busy()} {...sx(styles.button, label === "real-trend" || label === "interesting-too-early" ? styles.buttonAccent : undefined)} onClick={() => void save(label)}>{title}</button>}</For>
        </div>
        <Show when={error()}>{(value) => <div {...sx(styles.why)}>{value()}</div>}</Show>
      </div>
      <div>
        <div {...sx(styles.score)}>{props.trend.quality.peep_score}</div>
        <div {...sx(styles.scoreLabel)}>PEEP score</div>
      </div>
    </article>
  );
}

export function AdminQualityPage() {
  const [trends, setTrends] = createSignal<Trend[]>([]);
  const [feedback, setFeedback] = createSignal<QualityFeedback[]>([]);
  const [report, setReport] = createSignal<QualityReport>();
  const [replay, setReplay] = createSignal<ReplayReport>();
  const [calibration, setCalibration] = createSignal<CalibrationV1>();
  const [error, setError] = createSignal<string>();
  const [loading, setLoading] = createSignal(true);

  const load = () => {
    setLoading(true);
    setError(undefined);
    void Promise.all([fetchTrends(), fetchQualityFeedback(), fetchQualityReport(), fetchQualityReplay(), fetchCalibrationV1()])
      .then(([trendValues, feedbackValues, reportValue, replayValue, calibrationValue]) => {
        setTrends(trendValues);
        setFeedback(feedbackValues);
        setReport(reportValue);
        setReplay(replayValue);
        setCalibration(calibrationValue);
      })
      .catch((reason) => setError(errorMessage(reason)))
      .finally(() => setLoading(false));
  };
  load();

  const latestFor = (trend: Trend) => feedback().find((value) => value.trend_key === (trend.id ?? trend.slug));
  const labelQueue = () => [...trends()].sort((left, right) => Number(Boolean(latestFor(left))) - Number(Boolean(latestFor(right))));
  const unlabeledLive = () => trends().filter((trend) => !latestFor(trend)).length;

  return (
    <QualityShell>
      <Show when={error()}>{(value) => <div {...sx(styles.why)}>{value()}</div>}</Show>
      <Show when={calibration()}>
        {(value) => (
          <article {...sx(styles.card)}>
            <div>
              <div {...sx(styles.meta)}>CALIBRATION V1 · {value().labels}/{value().required_labels} DISTINCT HUMAN-LABELED TRENDS</div>
              <h2 {...sx(styles.headline)}>{value().ready ? "Replay calibration is ready." : "Keep labeling before tuning production."}</h2>
              <p {...sx(styles.description)}>{value().note}</p>
              <div {...sx(styles.why)}>Current cluster threshold {value().current_cluster_threshold.toFixed(2)} · public score gate {value().recommended_min_score}</div>
            </div>
            <div>
              <div {...sx(styles.score)}>{value().ready && value().recommended_cluster_threshold !== undefined ? value().recommended_cluster_threshold.toFixed(2) : Math.round((value().labels / value().required_labels) * 100) + "%"}</div>
              <div {...sx(styles.scoreLabel)}>{value().ready ? "replay threshold" : "labeling progress"}</div>
            </div>
          </article>
        )}
      </Show>
      <Show when={report()}>
        {(value) => (
          <>
            <div {...sx(styles.meta)}>{value().labels} stable trends labeled · {unlabeledLive()} currently published trends still need a label · recommended public gate {value().recommended_min_score}</div>
            <Metric label="Top 10 precision" value={measuredPercent(value().top_10_precision, value().top_10_evaluated)} copy={`${value().top_10_evaluated} scored, precision-eligible labels in the highest-ranked sample.`} />
            <Metric label="Top 25 precision" value={measuredPercent(value().top_25_precision, value().top_25_evaluated)} copy={`${value().top_25_evaluated} scored, precision-eligible labels in the highest-ranked sample.`} />
            <Metric label="Precision proxy" value={percent(value().precision_proxy)} copy="Usable trend labels divided by usable + noise/duplicate/bad-cluster labels." />
            <Metric label="False-positive rate" value={percent(value().false_positive_rate)} copy="Noise, duplicate, and bad-cluster outcomes among precision-eligible labels." />
            <Metric label="Duplicate-cluster rate" value={percent(value().duplicate_cluster_rate)} copy="How often a stable candidate was labeled as a duplicate cluster." />
            <Metric label="Early hit rate" value={percent(value().early_hit_rate)} copy="Good early catches versus trends you marked detected too late." />
            <Metric label="Average source breadth" value={value().average_source_count > 0 ? value().average_source_count.toFixed(1) : "—"} copy={value().average_source_count > 0 ? `${percent(value().average_source_breadth)} normalized breadth across positively labeled trends.` : "History-backed source breadth appears once labeled trends have persisted snapshots."} />
            <Metric label="Lead time to Breaking" value={duration(value().average_lead_to_breaking_minutes)} copy="Average time from first Trendinary observation to the first BREAKING snapshot." />
            <Metric label="Emerging → Rising" value={duration(value().average_emerging_to_rising_minutes)} copy="Average observed lifecycle time for positively labeled trends that reached both states." />
            <Metric label="Rising → Breaking" value={duration(value().average_rising_to_breaking_minutes)} copy="Average observed lifecycle time for positively labeled trends that reached both states." />
            <Metric label="Cluster health" value={percent(value().cluster_health)} copy="Penalty comes from duplicate and bad-cluster labels." />
            <Metric label="Naming health" value={percent(value().naming_health)} copy="How often stable canonical naming survives human review." />
          </>
        )}
      </Show>
      <Show when={replay()}>
        {(value) => (
          <article {...sx(styles.card)}>
            <div>
              <div {...sx(styles.meta)}>DETERMINISTIC REPLAY · {value().corpus || "human corpus warming up"}</div>
              <h2 {...sx(styles.headline)}>Cluster benchmark</h2>
              <p {...sx(styles.description)}>The same persisted signals that formed labeled production trends are replayed through the entity-aware clusterer at the current comparison threshold.</p>
              <div {...sx(styles.why)}>{value().signals} signals · {value().expected_pairs} expected pairs · {value().predicted_pairs} predicted pairs</div>
            </div>
            <div><div {...sx(styles.score)}>{value().signals ? percent(value().precision) : "—"}</div><div {...sx(styles.scoreLabel)}>replay precision</div></div>
          </article>
        )}
      </Show>
      <div {...sx(styles.topbar)}>
        <div><div {...sx(styles.eyebrow)}>Live calibration queue · unlabeled first</div><h2 {...sx(styles.headline)}>Label what the detector is publishing.</h2></div>
        <button {...sx(styles.button)} disabled={loading()} onClick={load}>{loading() ? "Refreshing…" : "Refresh"}</button>
      </div>
      <Show when={!loading()} fallback={<div {...sx(styles.why)}>Loading live Trendinary output…</div>}>
        <For each={labelQueue()}>{(trend) => <TrendLabelCard trend={trend} latest={latestFor(trend)} onLabeled={load} />}</For>
      </Show>
    </QualityShell>
  );
}
