import { createSignal, For, Show } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import { fetchFomo, fetchPeep, type FomoItem, type Trend } from "./api";
import { styles } from "./styles.stylex";

const sx = stylex.attrs;

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

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function percent(value: number) {
  return `${Math.round(value * 100)}%`;
}

function relativeTime(value: string) {
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return value;
  const minutes = Math.max(0, Math.round((Date.now() - timestamp) / 60000));
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

function PeepRows(props: { items: Trend[] }) {
  return (
    <div {...sx(styles.trendList)}>
      <For each={props.items}>
        {(trend) => (
          <a href={`/trend/${trend.slug}`} {...sx(styles.trendRow)}>
            <div {...sx(styles.rank)}>{String(trend.rank).padStart(2, "0")}</div>
            <div>
              <div {...sx(styles.trendName)}>{trend.name}</div>
              <div {...sx(styles.trendMeta)}>{trend.status} · PEEP {trend.quality.peep_score}</div>
            </div>
            <div {...sx(styles.change)}>{percent(trend.quality.velocity)}</div>
            <div {...sx(styles.score)}>{trend.quality.peep_score}</div>
            <div {...sx(styles.reason)}>{(trend.quality.why_watching ?? []).join(" · ") || trend.reason}</div>
            <div {...sx(styles.arrow)}>↗</div>
          </a>
        )}
      </For>
    </div>
  );
}

export function SignalPeepPage() {
  const [items, setItems] = createSignal<Trend[]>([]);
  const [error, setError] = createSignal<string>();
  const [loading, setLoading] = createSignal(true);

  void fetchPeep()
    .then(setItems)
    .catch((reason) => setError(errorMessage(reason)))
    .finally(() => setLoading(false));

  return (
    <>
      <section {...sx(styles.hero)}>
        <div>
          <div {...sx(styles.eyebrow)}>PEEP 👀 · SIGNAL QUALITY V3</div>
          <h1 {...sx(styles.heroTitle)}>Watch the <span {...sx(styles.heroAccent)}>slope.</span></h1>
          <p {...sx(styles.heroCopy)}>PEEP now has its own score: velocity, independent-source breadth, community spread, and novelty, confidence-gated so raw fame cannot dominate.</p>
        </div>
        <div {...sx(styles.statusCard)}>
          <div {...sx(styles.statusLabel)}>PEEP score</div>
          <div {...sx(styles.statusValue)}>EARLY ≠ POPULAR</div>
          <div {...sx(styles.statusSub)}>The main Trendinary Score ranks importance. PEEP ranks “this is moving strangely fast and spreading.”</div>
        </div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Emerging now</h2><p {...sx(styles.sectionCopy)}>Rising and emerging trends ranked by early-signal quality, not their main leaderboard rank.</p></div>
          <div {...sx(styles.eyebrow)}>{loading() ? "MEASURING SLOPE" : "LIVE PEEP SCORE"}</div>
        </div>
        <Show when={error()}>{(value) => <div {...sx(styles.emptyState)}>{value()}</div>}</Show>
        <Show when={!loading()} fallback={<div {...sx(styles.emptyState)}>Measuring abnormal acceleration…</div>}>
          <Show when={items().length > 0} fallback={<div {...sx(styles.emptyState)}>No current trend has crossed the PEEP early-signal gate. Quiet is valid output.</div>}>
            <PeepRows items={items()} />
          </Show>
        </Show>
      </section>
      <section {...sx(styles.section)}>
        <div {...sx(styles.grid3)}>
          <div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>SLOPE</div><h3 {...sx(styles.cardTitle)}>Velocity first</h3><p {...sx(styles.cardCopy)}>A small topic moving ten times faster than normal is more interesting here than a giant topic behaving normally.</p></div>
          <div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>BREADTH</div><h3 {...sx(styles.cardTitle)}>Independent pickup</h3><p {...sx(styles.cardCopy)}>PEEP rewards movement across sources and communities instead of one loud account or one flooded network.</p></div>
          <div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>CONFIDENCE</div><h3 {...sx(styles.cardTitle)}>Quiet is allowed</h3><p {...sx(styles.cardCopy)}>Low-confidence spikes are suppressed rather than padded into a feed just to make the page look busy.</p></div>
        </div>
      </section>
    </>
  );
}

function FomoRow(props: { item: FomoItem }) {
  return (
    <div {...sx(styles.fomoRow)}>
      <div {...sx(styles.fomoTime)}>{relativeTime(props.item.last_seen)}</div>
      <div>
        <div {...sx(styles.trendName)}>{props.item.name}</div>
        <p {...sx(styles.cardCopy)}>{props.item.summary}</p>
        <div {...sx(styles.trendMeta)}>PEAK {props.item.peak_score} · velocity {percent(props.item.max_velocity)} · breadth {percent(props.item.source_breadth)}</div>
      </div>
      <a {...sx(styles.headerAction)} href={`/trend/${props.item.slug}`}>Open lore →</a>
    </div>
  );
}

export function SignalFomoPage() {
  const [items, setItems] = createSignal<FomoItem[]>([]);
  const [error, setError] = createSignal<string>();
  const [loading, setLoading] = createSignal(true);

  void fetchFomo(24, 7)
    .then(setItems)
    .catch((reason) => setError(errorMessage(reason)))
    .finally(() => setLoading(false));

  return (
    <>
      <section {...sx(styles.hero)}>
        <div>
          <div {...sx(styles.eyebrow)}>FOMO · HISTORY-BACKED CATCH-UP</div>
          <h1 {...sx(styles.heroTitle)}>Seven things.<br /><span {...sx(styles.heroAccent)}>Then you're done.</span></h1>
          <p {...sx(styles.heroCopy)}>FOMO is now generated from persisted trend history. It chooses the strongest moments in the last 24 hours and stops instead of becoming another feed.</p>
        </div>
        <div {...sx(styles.statusCard)}>
          <div {...sx(styles.statusLabel)}>Briefing window</div>
          <div {...sx(styles.statusValue)}>LAST 24 HOURS</div>
          <div {...sx(styles.statusSub)}>Finite by design: maximum seven trends, ranked by historical peak score and acceleration.</div>
        </div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>What mattered</h2><p {...sx(styles.sectionCopy)}>A catch-up briefing derived from actual stored observations.</p></div>
          <div {...sx(styles.eyebrow)}>{loading() ? "BUILDING BRIEF" : `${items().length} ITEMS`}</div>
        </div>
        <Show when={error()}>{(value) => <div {...sx(styles.emptyState)}>{value()}</div>}</Show>
        <Show when={!loading()} fallback={<div {...sx(styles.emptyState)}>Reading the last 24 hours…</div>}>
          <Show when={items().length > 0} fallback={<div {...sx(styles.emptyState)}>Nothing strong enough to make the briefing yet.</div>}>
            <For each={items()}>{(item) => <FomoRow item={item} />}</For>
          </Show>
        </Show>
      </section>
    </>
  );
}
