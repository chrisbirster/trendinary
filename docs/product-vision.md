# Trendinary — Product Vision

## One sentence

**Trendinary is the live dictionary of the internet: a continuously updating map of what is capturing attention, why it is happening, how it is spreading, and what you need to know about it.**

## The problem

The internet does not have one front page anymore. A story can begin in a developer community, mutate on Reddit, become a meme on TikTok, get debated on Bluesky, become a YouTube explainer, spike in search, and only then become conventional news.

Most products show one slice of that movement:

- search engines answer questions you already know to ask;
- social networks show what is popular inside their own network;
- news aggregators show what publishers chose to cover;
- trend dashboards often show keywords without enough context;
- AI answer engines explain topics, but generally wait for the user to provide the question.

Trendinary starts one step earlier:

> **What should I know about right now?**

## The mission

Trendinary tries to build a real-time model of internet attention.

It watches public signals, identifies unusual acceleration, clusters related activity into coherent trends, explains the event, reconstructs how it spread, preserves the history, and lets the user explore the trend conversationally.

The product loop is:

**DISCOVER → UNDERSTAND → EXPLORE → FOLLOW**

The consumer-facing language is intentionally more memorable:

- **SCAN** — detect the signal.
- **PEEP** — surface early, weird, emerging signals.
- **WTF?** — explain why something is trending.
- **VIBE** — describe the shape and tone of the conversation.
- **LORE** — preserve the backstory and historical context.
- **FOMO** — catch the user up on what mattered while they were gone.

## The Trendinary object

Trendinary should not think primarily in articles or posts. It thinks in **trend objects**.

A trend can represent a person, company, product, event, meme, phrase, technology, game, movie, sports story, political event, or broader conversation cluster.

Each object can contain:

- Trendinary Score
- lifecycle state: Emerging, Rising, Breaking, Peaking, Cooling, Resurfacing
- attention velocity
- source breadth
- novelty
- short explanation
- trend start and peak times
- source communities
- timeline
- origin / propagation path
- related entities and trends
- Vibe summary
- Lore / evergreen context
- citations and source links
- conversational Ask Trendinary context

## Trendinary Score

The score should represent more than raw popularity.

A useful conceptual model is:

**attention × velocity × breadth × novelty**

The system should reward unusual acceleration and cross-community spread. A celebrity with permanent high volume may be less “trendy” than an obscure project whose attention is increasing 3,000% in an hour.

## Product surfaces

### Now

The live internet scoreboard. It answers: **What is happening?**

### PEEP

Early signals with low baseline volume and unusually high acceleration. It answers: **What might everyone be talking about next?**

### FOMO

A bounded briefing of the important things that happened while the user was away. It intentionally avoids becoming another infinite feed.

### Following

A personal radar. Users follow topics, people, companies, technologies, games, sports teams, or phrases. Trendinary alerts on meaningful deviation from the normal baseline rather than every mention.

### Trend detail

A living page for a trend with four signature interactions:

- **WTF?** — Why is this trending?
- **LORE** — What is the backstory?
- **VIBE** — What does the conversation feel like?
- **TIMELINE** — How did it spread?

It also provides **Ask Trendinary**, a contextual answer interface grounded in the trend and its sources.

## AI's role

AI should not decide what is trending by inventing stories or mass-producing articles.

Internet activity produces the signal. AI acts as the analyst:

- cluster related activity;
- summarize the event;
- explain why the spike occurred;
- distinguish old context from new developments;
- summarize competing interpretations;
- build timelines;
- answer questions with citations.

The signal should be evidence-first and the explanation should be source-grounded.

## Long-term opportunity

The most interesting long-term artifact is not the daily feed. It is the accumulated historical dataset.

Trendinary can become a searchable history of internet attention: when ideas appeared, how memes spread, what dominated a category during a month or year, which communities noticed something first, and how different events connected.

That turns a disposable trend app into a continuously growing knowledge graph of internet culture.
