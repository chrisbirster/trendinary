import { createSignal, For, Show } from "solid-js";
import { useParams } from "@solidjs/router";
import * as stylex from "@stylexjs/stylex";
import {
  askTrend,
  fetchTrend,
  fetchTrendHistory,
  type AskAnswer,
  type Evidence,
  type PerspectiveMix,
  type PropagationHop,
  type Trend,
  type TrendSnapshot,
} from "./api";
import { historyStyles } from "./history.stylex";
import { intelligenceStyles } from "./trend-intelligence.stylex";
import { styles } from "./styles.stylex";

const sx = stylex.attrs;

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function percent(value: number) {
  return `${Math.round(value * 100)}%`;
}

function formatTime(value: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function HistoryPanel(props: { items: TrendSnapshot[]; error?: string }) {
  const latest = () => props.items[props.items.length - 1];
  return (
    <div {...sx(historyStyles.panel)}>
      <Show
        when={props.items.length > 0}
        fallback={
          <div {...sx(historyStyles.empty)}>
            {props.error ?? "Trendinary has not accumulated enough observations for a momentum chart yet."}
          </div>
        }
      >
        <div {...sx(historyStyles.chart)} aria-label="Trendinary score history">
          <For each={props.items}>
            {(snapshot) => (
              <div
                {...sx(historyStyles.barSlot)}
                title={`${formatTime(snapshot.observed_at)} · score ${snapshot.score.score} · ${snapshot.lifecycle}`}
              >
                <div
                  {...sx(historyStyles.bar)}
                  style={{
                    height: `${Math.max(4, snapshot.score.score)}%`,
                    opacity: String(Math.max(0.35, snapshot.score.confidence)),
                  }}
                />
              </div>
            )}
          </For>
        </div>
        <div {...sx(historyStyles.footer)}>
          <span>{props.items.length} observations</span>
          <span>SCORE MODEL {latest()?.score.version ?? "—"}</span>
        </div>
        <Show when={latest()}>
          {(snapshot) => (
            <div {...sx(historyStyles.metrics)}>
              <div {...sx(historyStyles.metric)}>
                <div {...sx(historyStyles.metricLabel)}>Attention</div>
                <div {...sx(historyStyles.metricValue)}>{percent(snapshot().score.attention)}</div>
              </div>
              <div {...sx(historyStyles.metric)}>
                <div {...sx(historyStyles.metricLabel)}>Velocity</div>
                <div {...sx(historyStyles.metricValue)}>{percent(snapshot().score.velocity)}</div>
              </div>
              <div {...sx(historyStyles.metric)}>
                <div {...sx(historyStyles.metricLabel)}>Source breadth</div>
                <div {...sx(historyStyles.metricValue)}>{percent(snapshot().score.source_breadth)}</div>
              </div>
              <div {...sx(historyStyles.metric)}>
                <div {...sx(historyStyles.metricLabel)}>Confidence</div>
                <div {...sx(historyStyles.metricValue)}>{percent(snapshot().score.confidence)}</div>
              </div>
            </div>
          )}
        </Show>
      </Show>
    </div>
  );
}

function EvidenceGrid(props: { items?: Evidence[] }) {
  return (
    <Show when={(props.items?.length ?? 0) > 0}>
      <div {...sx(intelligenceStyles.evidenceGrid)}>
        <For each={props.items ?? []}>
          {(item) => (
            <a
              {...sx(intelligenceStyles.evidence)}
              href={item.url}
              target="_blank"
              rel="noreferrer"
            >
              <div {...sx(intelligenceStyles.evidenceSource)}>{item.source.name}</div>
              <div {...sx(intelligenceStyles.evidenceTitle)}>{item.title}</div>
              <Show when={item.author || item.published_at}>
                <div {...sx(intelligenceStyles.hopMeta)}>
                  {[item.author, item.published_at ? formatTime(item.published_at) : ""].filter(Boolean).join(" · ")}
                </div>
              </Show>
            </a>
          )}
        </For>
      </div>
    </Show>
  );
}

function PropagationPath(props: { hops?: PropagationHop[] }) {
  return (
    <Show
      when={(props.hops?.length ?? 0) > 0}
      fallback={<p {...sx(styles.cardCopy)}>Trendinary has not observed enough cross-source movement to reconstruct a propagation path yet.</p>}
    >
      <div {...sx(intelligenceStyles.path)}>
        <For each={props.hops ?? []}>
          {(hop, index) => (
            <>
              <div {...sx(intelligenceStyles.hop)}>
                <div {...sx(intelligenceStyles.hopName)}>{hop.source.name}</div>
                <div {...sx(intelligenceStyles.hopMeta)}>
                  First seen {formatTime(hop.first_seen)}<br />
                  {hop.signal_count} signal{hop.signal_count === 1 ? "" : "s"} · engagement {hop.engagement}
                </div>
              </div>
              <Show when={index() < (props.hops?.length ?? 0) - 1}>
                <div {...sx(intelligenceStyles.arrow)}>→</div>
              </Show>
            </>
          )}
        </For>
      </div>
    </Show>
  );
}

const perspectiveCells: Array<[keyof PerspectiveMix, string]> = [
  ["left", "Left"],
  ["lean_left", "Lean left"],
  ["center", "Center"],
  ["lean_right", "Lean right"],
  ["right", "Right"],
  ["mixed", "Mixed"],
  ["unrated", "Unrated"],
];

function PerspectivePanel(props: { value?: PerspectiveMix }) {
  return (
    <Show
      when={props.value}
      fallback={<p {...sx(styles.cardCopy)}>No evidence-backed source perspective data is available for this trend yet.</p>}
    >
      {(mix) => (
        <>
          <div {...sx(intelligenceStyles.perspective)}>
            <For each={perspectiveCells}>
              {([key, label]) => (
                <div {...sx(intelligenceStyles.perspectiveCell)}>
                  <div {...sx(intelligenceStyles.perspectiveValue)}>{String(mix()[key])}</div>
                  <div {...sx(intelligenceStyles.perspectiveLabel)}>{label}</div>
                </div>
              )}
            </For>
          </div>
          <p {...sx(styles.cardCopy)} style={{ "margin-top": "12px" }}>{mix().note}</p>
        </>
      )}
    </Show>
  );
}

function AskPanel(props: { trend: Trend }) {
  const [question, setQuestion] = createSignal("");
  const [answer, setAnswer] = createSignal<AskAnswer>();
  const [error, setError] = createSignal<string>();
  const [loading, setLoading] = createSignal(false);

  const submit = async (event: SubmitEvent) => {
    event.preventDefault();
    const value = question().trim();
    if (!value || loading()) return;
    setLoading(true);
    setError(undefined);
    try {
      setAnswer(await askTrend(props.trend.slug, value));
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setLoading(false);
    }
  };

  const askPreset = (value: string) => {
    setQuestion(value);
    void askTrend(props.trend.slug, value)
      .then((result) => {
        setAnswer(result);
        setError(undefined);
      })
      .catch((reason) => setError(errorMessage(reason)));
  };

  return (
    <div {...sx(styles.ask)}>
      <div {...sx(styles.cardKicker)}>ASK TRENDINARY</div>
      <form {...sx(intelligenceStyles.askForm)} onSubmit={submit}>
        <input
          {...sx(styles.askInput)}
          value={question()}
          onInput={(event) => setQuestion(event.currentTarget.value)}
          placeholder={`Ask anything grounded in the evidence for ${props.trend.name}…`}
        />
        <button {...sx(intelligenceStyles.askButton)} type="submit" disabled={loading()}>
          {loading() ? "ASKING…" : "ASK"}
        </button>
      </form>
      <div {...sx(styles.askSuggestions)}>
        <button {...sx(styles.smallButton)} type="button" onClick={() => askPreset("Why is this trending?")}>Why now?</button>
        <button {...sx(styles.smallButton)} type="button" onClick={() => askPreset("Who is driving this?")}>Who is driving it?</button>
        <button {...sx(styles.smallButton)} type="button" onClick={() => askPreset("What is the perspective mix?")}>Perspective mix</button>
      </div>
      <Show when={error()}>{(value) => <div {...sx(intelligenceStyles.answer)}>{value()}</div>}</Show>
      <Show when={answer()}>
        {(value) => (
          <>
            <div {...sx(intelligenceStyles.answer)}>
              {value().answer}
              <div {...sx(intelligenceStyles.answerMeta)}>MODE · {value().mode}</div>
            </div>
            <EvidenceGrid items={value().evidence} />
          </>
        )}
      </Show>
    </div>
  );
}

export function TrendPage() {
  const params = useParams();
  const [trend, setTrend] = createSignal<Trend>();
  const [history, setHistory] = createSignal<TrendSnapshot[]>([]);
  const [error, setError] = createSignal<string>();
  const [historyError, setHistoryError] = createSignal<string>();
  const slug = params.slug;

  if (slug) {
    void fetchTrend(slug)
      .then(setTrend)
      .catch((reason) => setError(errorMessage(reason)));
    void fetchTrendHistory(slug)
      .then(setHistory)
      .catch((reason) => setHistoryError(errorMessage(reason)));
  } else {
    setError("Missing trend slug.");
  }

  return (
    <>
      <Show when={error()}>
        {(value) => (
          <div {...sx(styles.emptyState)}>
            <div {...sx(styles.eyebrow)}>TREND LOOKUP FAILED</div>
            <h2 {...sx(styles.sectionTitle)}>{value()}</h2>
          </div>
        )}
      </Show>
      <Show
        when={trend()}
        fallback={<div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>SCAN IN PROGRESS</div><h2 {...sx(styles.sectionTitle)}>Loading trend intelligence…</h2></div>}
      >
        {(current) => (
          <>
            <section {...sx(styles.detailHero)}>
              <div>
                <div {...sx(styles.eyebrow)}>{current().category} · {current().status} · STARTED {current().started}</div>
                <h1 {...sx(styles.detailTitle)}>{current().name}</h1>
                <p {...sx(styles.heroCopy)}>{current().reason}</p>
                <div {...sx(styles.chips)}>
                  <For each={current().sources}>
                    {(source) => (
                      <a href={source.url} target="_blank" rel="noreferrer" {...sx(styles.chip)}>
                        {source.name}{source.bias ? ` · ${source.bias.label.toUpperCase()}` : ""}
                      </a>
                    )}
                  </For>
                </div>
              </div>
              <div {...sx(styles.statusCard)}>
                <div {...sx(styles.statusLabel)}>Trendinary score</div>
                <div {...sx(styles.scoreBig)}>{current().score}</div>
                <div {...sx(styles.change)} style={{ "text-align": "left", "margin-top": "8px" }}>{current().change} velocity</div>
                <div {...sx(styles.statusSub)} style={{ "margin-top": "14px" }}>VIBE: {current().vibe}</div>
                <Show when={current().id}>
                  <div {...sx(styles.statusSub)} style={{ "margin-top": "8px" }}>ENTITY: {current().id}</div>
                </Show>
              </div>
            </section>

            <div {...sx(styles.actionGrid)}>
              <a {...sx(styles.actionCard)} href="#wtf">WTF?<span {...sx(styles.actionLabel)}>Why's this trending?</span></a>
              <a {...sx(styles.actionCard)} href="#lore">LORE<span {...sx(styles.actionLabel)}>Give me the backstory.</span></a>
              <a {...sx(styles.actionCard)} href="#source-lens">SOURCE LENS<span {...sx(styles.actionLabel)}>Who is shaping this?</span></a>
              <a {...sx(styles.actionCard)} href="#propagation">PATH<span {...sx(styles.actionLabel)}>How did it spread?</span></a>
            </div>

            <section {...sx(styles.section)}>
              <div {...sx(styles.sectionHeader)}>
                <div><h2 {...sx(styles.sectionTitle)}>Momentum</h2><p {...sx(styles.sectionCopy)}>Persisted observations behind the current Trendinary Score.</p></div>
                <div {...sx(styles.eyebrow)}>SCORE HISTORY</div>
              </div>
              <HistoryPanel items={history()} error={historyError()} />
            </section>

            <section {...sx(styles.twoCol)} id="wtf">
              <div {...sx(styles.whyBox)}>
                <div {...sx(styles.eyebrow)}>WTF? · WHY'S THIS TRENDING?</div>
                <h2 {...sx(styles.whyTitle)}>{current().explanation?.summary ?? current().reason}</h2>
                <p {...sx(styles.whyCopy)}>{current().explanation?.what_changed ?? current().why ?? current().reason}</p>
                <Show when={current().explanation}>
                  {(value) => <div {...sx(intelligenceStyles.answerMeta)}>CONFIDENCE · {value().confidence} · {value().mode}</div>}
                </Show>
                <EvidenceGrid items={current().explanation?.evidence} />
              </div>
              <div {...sx(styles.card)}>
                <div {...sx(styles.cardKicker)}>TOP VOICES</div>
                <h3 {...sx(styles.cardTitle)}>Who is visibly driving this cluster?</h3>
                <div {...sx(intelligenceStyles.voices)}>
                  <For each={current().top_voices ?? []}>
                    {(voice) => <div {...sx(intelligenceStyles.voice)}>{voice.display_name || voice.handle || voice.did || "Unknown"}</div>}
                  </For>
                </div>
                <Show when={(current().top_voices?.length ?? 0) === 0}>
                  <p {...sx(styles.cardCopy)}>No resolved public profiles are attached to this trend yet.</p>
                </Show>
              </div>
            </section>

            <section {...sx(styles.section)} id="propagation">
              <div {...sx(styles.sectionHeader)}>
                <div><h2 {...sx(styles.sectionTitle)}>How it spread</h2><p {...sx(styles.sectionCopy)}>Observed source order—not an invented narrative.</p></div>
                <div {...sx(styles.eyebrow)}>PROPAGATION</div>
              </div>
              <PropagationPath hops={current().propagation} />
            </section>

            <section {...sx(styles.section)} id="source-lens">
              <div {...sx(styles.card)}>
                <div {...sx(styles.cardKicker)}>SOURCE LENS</div>
                <h3 {...sx(styles.cardTitle)}>Perspective mix</h3>
                <p {...sx(styles.cardCopy)}>Political leaning is source metadata, not a truth score. Reliability, article stance, and claim accuracy remain separate concepts.</p>
                <PerspectivePanel value={current().perspective} />
              </div>
            </section>

            <section {...sx(styles.section)} id="lore">
              <div {...sx(styles.twoCol)}>
                <div {...sx(styles.card)}>
                  <div {...sx(styles.cardKicker)}>THE LORE</div>
                  <h3 {...sx(styles.cardTitle)}>The context that survives today's spike.</h3>
                  <p {...sx(styles.cardCopy)}>{current().explanation?.lore ?? current().lore ?? "Trendinary has not accumulated durable context for this entity yet."}</p>
                  <Show when={(current().aliases?.length ?? 0) > 0}>
                    <div {...sx(styles.chips)} style={{ "margin-top": "14px" }}>
                      <For each={current().aliases}>{(alias) => <span {...sx(styles.chip)}>{alias}</span>}</For>
                    </div>
                  </Show>
                </div>
                <AskPanel trend={current()} />
              </div>
            </section>
          </>
        )}
      </Show>
    </>
  );
}
