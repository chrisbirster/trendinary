import { For } from "solid-js";
import { useParams } from "@solidjs/router";
import * as stylex from "@stylexjs/stylex";
import { emerging, missed, timeline, trends } from "./data";
import { styles } from "./styles.stylex";

const sx = stylex.attrs;

function TrendRows(props: { items?: typeof trends }) {
  const items = () => props.items ?? trends;
  return (
    <div {...sx(styles.trendList)}>
      <For each={items()}>
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

export function HomePage() {
  return (
    <>
      <section {...sx(styles.hero)}>
        <div>
          <div {...sx(styles.eyebrow)}>LIVE INTERNET SCOREBOARD · 21:24 ET</div>
          <h1 {...sx(styles.heroTitle)}>Know what's <span {...sx(styles.heroAccent)}>happening.</span><br />Know why.</h1>
          <p {...sx(styles.heroCopy)}>Trendinary scans public signals across the web to find what is accelerating, explain why it matters, and show you what everyone else is about to talk about.</p>
        </div>
        <div {...sx(styles.statusCard)}>
          <div {...sx(styles.statusLabel)}>Internet temperature</div>
          <div {...sx(styles.statusValue)}>VERY ONLINE</div>
          <div {...sx(styles.statusSub)}>18 major trend clusters · 4 weird things emerging · news velocity +23% in the last hour.</div>
        </div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}>
        <div {...sx(styles.sectionHeader)}>
          <div><h2 {...sx(styles.sectionTitle)}>Happening now</h2><p {...sx(styles.sectionCopy)}>Ranked by attention × velocity × breadth × novelty.</p></div>
          <div {...sx(styles.eyebrow)}>AUTO-REFRESHING</div>
        </div>
        <TrendRows />
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
  return (
    <>
      <section {...sx(styles.hero)}>
        <div><div {...sx(styles.eyebrow)}>PEEP 👀 · EARLY SIGNALS</div><h1 {...sx(styles.heroTitle)}>See it <span {...sx(styles.heroAccent)}>before</span> it blows up.</h1><p {...sx(styles.heroCopy)}>Small topics with abnormal velocity. This is where Trendinary gets weird—and useful.</p></div>
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Best early signal</div><div {...sx(styles.statusValue)}>+2,970%</div><div {...sx(styles.statusSub)}>“That Blue Chair” is tiny in absolute volume, but its spread rate is currently the fastest thing we can see.</div></div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}><div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>Emerging now</h2><p {...sx(styles.sectionCopy)}>Low baseline. High acceleration. Maximum “what is this?” energy.</p></div></div><TrendRows items={emerging} /></section>
      <section {...sx(styles.section)}><div {...sx(styles.grid3)}><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>WHY PEEP EXISTS</div><h3 {...sx(styles.cardTitle)}>Popularity is late.</h3><p {...sx(styles.cardCopy)}>A leaderboard tells you what already won. PEEP focuses on the slope: what is suddenly moving much faster than normal.</p></div><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>SIGNAL</div><h3 {...sx(styles.cardTitle)}>Crossing communities</h3><p {...sx(styles.cardCopy)}>A topic gets more interesting when it jumps from one community into several unrelated ones.</p></div><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>NOISE FILTER</div><h3 {...sx(styles.cardTitle)}>Not every spike matters.</h3><p {...sx(styles.cardCopy)}>Trendinary should separate coordinated spam and recurring chatter from genuine unusual attention.</p></div></div></section>
    </>
  );
}

export function FomoPage() {
  return (
    <>
      <section {...sx(styles.hero)}>
        <div><div {...sx(styles.eyebrow)}>FOMO · CATCH ME UP</div><h1 {...sx(styles.heroTitle)}>You logged off.<br /><span {...sx(styles.heroAccent)}>We kept score.</span></h1><p {...sx(styles.heroCopy)}>The smallest possible briefing on everything that mattered while you were gone.</p></div>
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Since your last visit</div><div {...sx(styles.statusValue)}>3 BIG THINGS</div><div {...sx(styles.statusSub)}>Plus 7 stories that looked important for twenty minutes and then vanished. We left those out.</div></div>
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
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Radar status</div><div {...sx(styles.statusValue)}>QUIET</div><div {...sx(styles.statusSub)}>Nothing abnormal in your followed topics right now. That's a feature.</div></div>
      </section>
      <ProductTabs />
      <section {...sx(styles.section)}><div {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>YOUR INTERNET, WITHOUT THE FEED</div><h2 {...sx(styles.sectionTitle)}>Follow your first topic.</h2><p {...sx(styles.heroCopy)} style={{ margin: "12px auto 22px" }}>Try AT Protocol, SolidJS, an NFL team, a company, or your favorite game. We'll build the radar view here.</p><a {...sx(styles.followButton)} href="/trend/at-protocol">Follow AT Protocol</a></div></section>
    </>
  );
}

export function TrendPage() {
  const params = useParams();
  const trend = () => trends.find((item) => item.slug === params.slug) ?? trends[0];
  return (
    <>
      <section {...sx(styles.detailHero)}>
        <div><div {...sx(styles.eyebrow)}>{trend().category} · {trend().status} · STARTED {trend().started}</div><h1 {...sx(styles.detailTitle)}>{trend().name}</h1><p {...sx(styles.heroCopy)}>{trend().reason}</p><div {...sx(styles.chips)}><For each={trend().sources}>{(source) => <span {...sx(styles.chip)}>{source}</span>}</For></div></div>
        <div {...sx(styles.statusCard)}><div {...sx(styles.statusLabel)}>Trendinary score</div><div {...sx(styles.scoreBig)}>{trend().score}</div><div {...sx(styles.change)} style={{ "text-align": "left", "margin-top": "8px" }}>{trend().change} velocity</div><div {...sx(styles.statusSub)} style={{ "margin-top": "14px" }}>VIBE: {trend().vibe}</div></div>
      </section>
      <div {...sx(styles.actionGrid)}><a {...sx(styles.actionCard)} href="#wtf">WTF?<span {...sx(styles.actionLabel)}>Why's this trending?</span></a><a {...sx(styles.actionCard)} href="#lore">LORE<span {...sx(styles.actionLabel)}>Give me the backstory.</span></a><a {...sx(styles.actionCard)} href="#vibe">VIBE<span {...sx(styles.actionLabel)}>What does it feel like?</span></a><a {...sx(styles.actionCard)} href="#timeline">TIMELINE<span {...sx(styles.actionLabel)}>How did it spread?</span></a></div>
      <section {...sx(styles.twoCol)}>
        <div {...sx(styles.whyBox)} id="wtf"><div {...sx(styles.eyebrow)}>WTF? · WHY'S THIS TRENDING?</div><h2 {...sx(styles.whyTitle)}>The open social web is having another moment.</h2><p {...sx(styles.whyCopy)}>A burst of app launches and developer discussion has pushed AT Protocol back into the spotlight. The notable part isn't one viral post—it is the same subject accelerating across developer communities, social feeds, and technology news at roughly the same time. Trendinary groups those fragments into a single trend instead of making you reconstruct the story yourself.</p></div>
        <div {...sx(styles.card)} id="vibe"><div {...sx(styles.cardKicker)}>VIBE CHECK</div><h3 {...sx(styles.cardTitle)}>{trend().vibe}</h3><p {...sx(styles.cardCopy)}>Developers sound optimistic about interoperability. Skeptics are asking whether open protocols can translate into mainstream consumer products. Everyone else mostly wants to know what AT Protocol actually is.</p></div>
      </section>
      <section {...sx(styles.section)} id="timeline"><div {...sx(styles.sectionHeader)}><div><h2 {...sx(styles.sectionTitle)}>How it spread</h2><p {...sx(styles.sectionCopy)}>A reconstructed attention timeline across public sources.</p></div></div><div {...sx(styles.timeline)}><For each={timeline}>{(item) => <div {...sx(styles.timelineItem)}><div {...sx(styles.timelineTime)}>{item[0]}</div><div><div {...sx(styles.timelineTitle)}>{item[1]}</div><div {...sx(styles.timelineCopy)}>{item[2]}</div></div></div>}</For></div></section>
      <section {...sx(styles.section)} id="lore"><div {...sx(styles.twoCol)}><div {...sx(styles.card)}><div {...sx(styles.cardKicker)}>THE LORE</div><h3 {...sx(styles.cardTitle)}>The context that existed before today's spike.</h3><p {...sx(styles.cardCopy)}>Trendinary pages persist after the spike ends. Over time this area becomes the evergreen history of the subject: what it is, the important prior moments, previous peaks, and the people or products connected to it.</p></div><div {...sx(styles.ask)}><div {...sx(styles.cardKicker)}>ASK TRENDINARY</div><input {...sx(styles.askInput)} placeholder={`Ask anything about ${trend().name}…`} /><div {...sx(styles.askSuggestions)}><button {...sx(styles.smallButton)}>Explain like I'm five</button><button {...sx(styles.smallButton)}>Why do developers care?</button><button {...sx(styles.smallButton)}>Show the skeptical take</button></div></div></div></section>
    </>
  );
}

export function NotFoundPage() {
  return <section {...sx(styles.emptyState)}><div {...sx(styles.eyebrow)}>404 · LOST IN THE FEED</div><h1 {...sx(styles.heroTitle)}>This trend died.</h1><a {...sx(styles.followButton)} href="/">Back to now</a></section>;
}
