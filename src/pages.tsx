import { createSignal, For, Show } from "solid-js";
import { useParams } from "@solidjs/router";
import * as stylex from "@stylexjs/stylex";
import { fetchTrend, fetchTrends, type Trend } from "./api";
import { missed } from "./data";
import { styles } from "./styles.stylex";

const sx = stylex.attrs;

function message(reason: unknown) {
  return reason instanceof Error ? reason.message : "Something went wrong.";
}

function useTrendList() {
  const [items, setItems] = createSignal<Trend[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string>();

  void fetchTrends()
    .then((value) => setItems(value))
    .catch((reason) => setError(message(reason)))
    .finally(() => setLoading(false));

  return { items, loading, error };
}

function TrendRows(props: { items: Trend[] }) {
  return (
    <div {...sx(styles.trendList)}>
      <For each={props.items}>
        {(trend) => (
          <a href={`/trend/${trend.slug}`} {...sx(styles.trendRow)}>
            <div {...sx(styles.rank)}>{String(trend.rank).padStart(2, "0")}</div>
            <div>
              <div {...sx(styles.trendName)}>{trend.name}</div>
              <div {...sx(styles.trendMeta)}>{trend.category} · {trend.status}</div>
            </div>
            <div {...sx(styles.change)}>{trend.change}</div>
            <div {...sx(styles.score)}>{trend.score}</div>
            <div {...sx(styles.reason)}>{trend.reason}</div>
            <div {...sx(styles.arrow)}>↗</div>
          </a>
        )}
      </For>
    </div>
  );
}

function ProductTabs() {
  return (
    <div {...sx(styles.tabs)}>
      <a {...sx(styles.tab)} href="/">Now</a>
      <a {...sx(styles.tab)} href="/peep">PEEP 👀</a>
      <a {...sx(styles.tab)} href="/fomo">FOMO</a>
      <a {...sx(styles.tab)} href="/following">Following</a>
      <a {...sx(styles.tab)} href="/trend/at-protocol">WTF?</a>
    </div>
  );
}

function LoadingScoreboard() {
  return <div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>SCAN IN PROGRESS</div><h2 {...sx(styles.sectionTitle)}>Reading the internet…</h2></div>;
}

function ApiError(props: { value?: string }) {
  return <Show when={props.value}>{(value) => <div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>SCAN INTERRUPTED</div><h2 {...sx(styles.sectionTitle)}>{value()}</h2></div>}</Show>;
}

export function HomePage() {
  const trends = useTrendList();

  return (
    <>
      <section {...sx(styles.hero)}>
        <div>
          <div {...sx(styles.eyebrow)}>LIVE INTERNET SCOREBOARD · API V1</div>
          <h1 {...sx(styles.heroTitle)}>Know what's <span {...sx(styles.heroAccent)}>happening.</span><br />Know why.</h1>
          <p {...sx(styles.heroCopy)}>Trendinary scans public signals across the web to find what is accelerating, explain why it matters, and show you what everyone else is about to talk about.</p>
        </div>
        <div {...sx(styles.statusCard)}>
          <div {...sx(styles.statusLabel)}>Internet temperature</div>
          <div {...sx(styles.statusValue)}>VERY ONLINE</div>
          <div {...sx(styles.statusSub)}>The UI is now backed by the Go API. Live ingestion and calculated scores are the next data-engine milestone.</div>
        </div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Happening now</h2><p {...sx(styles.sectionCopy)}>Ranked by attention × velocity × breadth × novelty.</p></div>
          <div {...sx(styles.eyebrow)}>{trends.loading() ? "SCANNING" : "API CONNECTED"}</div>
        </div>
        <ApiError value={trends.error()} />
        <Show when={!trends.loading()} fallback={<LoadingScoreboard />}>
          <TrendRows items={trends.items()} />
        </Show>
      </section>
      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}><h2 {...sx(styles.sectionTitle)}>How Trendinary thinks</h2></div>
        <div {...sx(styles.grid3)}>
          <div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>SCAN</div><h3 {...sx(styles.cardTitle)}>Spot the signal</h3><p {...sx(styles.cardCopy)}>Detect unusual acceleration before a topic is obviously mainstream.</p></div>
          <div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>WTF?</div><h3 {...sx(styles.cardTitle)}>Explain the moment</h3><p {...sx(styles.cardCopy)}>Turn fragmented posts, headlines, clips, and searches into one grounded explanation.</p></div>
          <div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>LORE</div><h3 {...sx(styles.cardTitle)}>Keep the history</h3><p {...sx(styles.cardCopy)}>Every trend becomes a living entry in the internet's memory instead of disappearing with the feed.</p></div>
        </div>
      </section>
    </>
  );
}

export function PeepPage() {
  const trends = useTrendList();
  const emerging = () => trends.items().filter((trend: Trend) => trend.status === "EMERGING" || trend.status === "RISING").slice(0, 4);

  return (
    <>
      <section {...sx(styles.hero)}>
        <div><div {...sx(styles.eyebrow)}>PEEP 👀 · EARLY SIGNALS</div><h1 {...sx(styles.heroTitle)}>See it <span {...sx(styles.heroAccent)}>before</span> it blows up.</h1><p {...sx(styles.heroCopy)}>Small topics with abnormal velocity. This is where Trendinary gets weird—and useful.</p></div>
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>PEEP rule</div><div {...sx(styles.statusValue)}>WATCH THE SLOPE</div><div {...sx(styles.statusSub)}>Absolute popularity is late. PEEP prioritizes unusual acceleration and cross-community spread.</div></div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}><div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>Emerging now</h2><p {...sx(styles.sectionCopy)}>Low baseline. High acceleration. Maximum “what is this?” energy.</p></div></div><ApiError value={trends.error()} /><Show when={!trends.loading()} fallback={<LoadingScoreboard />}><TrendRows items={emerging()} /></Show></section>
      <section {...sx(styles.section)}><div {...sx(styles.grid3)}><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>WHY PEEP EXISTS</div><h3 {...sx(styles.cardTitle)}>Popularity is late.</h3><p {...sx(styles.cardCopy)}>A leaderboard tells you what already won. PEEP focuses on what is suddenly moving much faster than normal.</p></div><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>SIGNAL</div><h3 {...sx(styles.cardTitle)}>Crossing communities</h3><p {...sx(styles.cardCopy)}>A topic gets more interesting when it jumps from one community into several unrelated ones.</p></div><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>NOISE FILTER</div><h3 {...sx(styles.cardTitle)}>Not every spike matters.</h3><p {...sx(styles.cardCopy)}>Trendinary should separate coordinated spam and recurring chatter from genuine unusual attention.</p></div></div></section>
    </>
  );
}

export function FomoPage() {
  return (
    <>
      <section {...sx(styles.hero)}>
        <div><div {...sx(styles.eyebrow)}>FOMO · CATCH ME UP</div><h1 {...sx(styles.heroTitle)}>You logged off.<br /><span {...sx(styles.heroAccent)}>We kept score.</span></h1><p {...sx(styles.heroCopy)}>The smallest possible briefing on everything that mattered while you were gone.</p></div>
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Since your last visit</div><div {...sx(styles.statusValue)}>3 BIG THINGS</div><div {...sx(styles.statusSub)}>FOMO remains a prototype briefing until accounts and visit history land.</div></div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>What you missed</h2><p {...sx(styles.sectionCopy)}>A briefing, not another infinite feed.</p></div></div>
        <For each={missed}>{(item) => <div {...sx(styles.fomoRow)}><div {...sx(styles.fomoTime)}>{item.peak}</div><div><div {...sx(styles.trendName)}>{item.name}</div><p {...sx(styles.cardCopy)}>{item.summary}</p></div><a {...sx(styles.headerAction)} href="/trend/at-protocol">Get the lore →</a></div>}</For>
      </section>
    </>
  );
}

export function FollowingPage() {
  return (
    <>
      <section {...sx(styles.hero)}>
        <div><div {...sx(styles.eyebrow)}>FOLLOWING · YOUR RADAR</div><h1 {...sx(styles.heroTitle)}>Tell me when <span {...sx(styles.heroAccent)}>something changes.</span></h1><p {...sx(styles.heroCopy)}>Follow people, companies, technologies, games, teams, memes, or weird phrases. Trendinary only bothers you when the baseline actually changes.</p></div>
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Radar status</div><div {...sx(styles.statusValue)}>QUIET</div><div {...sx(styles.statusSub)}>Accounts and persistent follows are not wired yet. The product rule stays: quiet is valid output.</div></div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}><div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>YOUR INTERNET, WITHOUT THE FEED</div><h2 {...sx(styles.sectionTitle)}>Follow your first topic.</h2><p {...sx(styles.heroCopy)} style={{ margin: "12px auto 22px" }}>Try AT Protocol, SolidJS, an NFL team, a company, or your favorite game. We'll build the radar view here.</p><a {...sx(styles.followButton)} href="/trend/at-protocol">Follow AT Protocol</a></div></section>
    </>
  );
}

export function TrendPage() {
  const params = useParams();
  const [trend, setTrend] = createSignal<Trend>();
  const [error, setError] = createSignal<string>();
  const slug = params.slug;

  if (slug) {
    void fetchTrend(slug)
      .then((value) => setTrend(value))
      .catch((reason) => setError(message(reason)));
  } else {
    setError("Missing trend slug.");
  }

  return (
    <>
      <ApiError value={error()} />
      <Show when={trend()} fallback={<LoadingScoreboard />}>
        {(current) => (
          <>
            <section {...sx(styles.detailHero)}>
              <div><div {...sx(styles.eyebrow)}>{current().category} · {current().status} · STARTED {current().started}</div><h1 {...sx(styles.detailTitle)}>{current().name}</h1><p {...sx(styles.heroCopy)}>{current().reason}</p><div {...sx(styles.chips)}><For each={current().sources}>{(source) => <a href={source.url} {...sx(styles.chip)}>{source.name}{source.bias ? ` · ${source.bias.label.toUpperCase()}` : ""}</a>}</For></div></div>
              <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Trendinary score</div><div {...sx(styles.scoreBig)}>{current().score}</div><div {...sx(styles.change)} style={{ "text-align": "left", "margin-top": "8px" }}>{current().change} velocity</div><div {...sx(styles.statusSub)} style={{ "margin-top": "14px" }}>VIBE: {current().vibe}</div></div>
            </section>
            <div {...sx(styles.actionGrid)}><a {...sx(styles.actionCard)} href="#wtf">WTF?<span {...sx(styles.actionLabel)}>Why's this trending?</span></a><a {...sx(styles.actionCard)} href="#lore">LORE<span {...sx(styles.actionLabel)}>Give me the backstory.</span></a><a {...sx(styles.actionCard)} href="#vibe">VIBE<span {...sx(styles.actionLabel)}>What does it feel like?</span></a><a {...sx(styles.actionCard)} href="#timeline">TIMELINE<span {...sx(styles.actionLabel)}>How did it spread?</span></a></div>
            <section {...sx(styles.twoCol)}>
              <div {...sx(styles.whyBox)} id="wtf"><div {...sx(styles.eyebrow)}>WTF? · WHY'S THIS TRENDING?</div><h2 {...sx(styles.whyTitle)}>{current().reason}</h2><p {...sx(styles.whyCopy)}>{current().why ?? "Trendinary has detected the cluster, but a sourced explanation has not been generated yet."}</p></div>
              <div {...sx(styles.card)} id="vibe"><div {...sx(styles.cardKicker)}>VIBE CHECK</div><h3 {...sx(styles.cardTitle)}>{current().vibe}</h3><p {...sx(styles.cardCopy)}>VIBE is intentionally qualitative. It summarizes the shape of the conversation without pretending sentiment percentages are objective measurements.</p></div>
            </section>
            <section {...sx(styles.section)} id="timeline"><div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>How it spread</h2><p {...sx(styles.sectionCopy)}>A reconstructed attention timeline across public sources.</p></div></div><div {...sx(styles.timeline)}><For each={current().timeline ?? []}>{(item) => <div {...sx(styles.timelineItem)}><div {...sx(styles.timelineTime)}>{item.time}</div><div><div {...sx(styles.timelineTitle)}>{item.label}</div><div {...sx(styles.timelineCopy)}>{item.text}</div></div></div>}</For></div></section>
            <section {...sx(styles.section)} id="lore"><div {...sx(styles.twoCol)}><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>THE LORE</div><h3 {...sx(styles.cardTitle)}>The context that existed before today's spike.</h3><p {...sx(styles.cardCopy)}>{current().lore ?? "Lore is being assembled from durable, sourced context."}</p></div><div {...sx(styles.ask)}><div {...sx(styles.cardKicker)}>ASK TRENDINARY</div><input {...sx(styles.askInput)} placeholder={`Ask anything about ${current().name}…`} /><div {...sx(styles.askSuggestions)}><button {...sx(styles.smallButton)}>Explain like I'm five</button><button {...sx(styles.smallButton)}>Why should I care?</button><button {...sx(styles.smallButton)}>Show the skeptical take</button></div></div></div></section>
            <section {...sx(styles.section)}><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>SOURCE LENS</div><h3 {...sx(styles.cardTitle)}>Bias is metadata, not a verdict.</h3><p {...sx(styles.cardCopy)}>When a news source has an evidence-backed political-lean assessment, Trendinary will show the provider, confidence, scope, and methodology. Unrated sources stay unrated; article stance and source-level leaning remain separate concepts.</p></div></section>
          </>
        )}
      </Show>
    </>
  );
}

export function NotFoundPage() {
  return <section {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>404 · LOST IN THE FEED</div><h1 {...sx(styles.heroTitle)}>This trend died.</h1><a {...sx(styles.followButton)} href="/">Back to now</a></section>;
}
