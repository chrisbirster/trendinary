# Trendinary Top 20

Trendinary's primary product is a ranked chart of internet attention. The chart is intentionally closer to Billboard than to a binary classifier: rank answers **what is moving**, while confidence answers **how strongly the underlying event is corroborated**.

## Chart contract

A healthy production scan publishes positions `#1` through `#20` when at least 20 substantive candidates are available. Candidates are sorted by Trendinary Score v4 with deterministic slug tie-breaking.

Each entry exposes:

- current rank and score
- movement since the immediately previous chart: `▲ n`, `▼ n`, `—`, `NEW`, or `RE`
- peak rank
- total and consecutive scans on chart
- scans spent at `#1`
- confidence tier
- publisher, platform, and signal breadth

The chart may contain an `EMERGING` item with limited corroboration. That is not a factual-confidence claim; it means the attention signal is strong enough to chart but still needs more independent evidence.

## Confidence tiers

- `EMERGING` — substantive attention is present, but independent corroboration is limited.
- `CORROBORATED` — evidence spans at least two independent publishers/accounts or platforms.
- `CONFIRMED` — broad publisher and cross-platform corroboration with a strong confidence score.

Lifecycle (`RISING`, `BREAKING`, `COOLING`, etc.), chart rank, and confidence tier are separate dimensions.

## Provenance model

Trendinary distinguishes three levels that were previously conflated:

1. **Publisher/account** — who originated the evidence. Examples: `WIRED`, `ABC News`, `jlxc2001`, `opensourcevillain`, or a Bluesky handle.
2. **Platform** — the network on which the evidence appeared. Examples: Web, GitHub, Bluesky, Hacker News, YouTube, Wikipedia, or Google Trends.
3. **Discovery channel/feed** — how Trendinary found it. Examples: RSS, GDELT, NewsData, Hacker News, GitHub API, Bluesky Jetstream, or Google Trends RSS.

Multiple section feeds from the same publisher do not increase publisher breadth. GitHub and Bluesky are platforms; their owner/handle identities are counted as publishers/accounts for independent-evidence purposes.

## Duplicate evidence

Near-identical long-form metadata is fingerprinted before scoring. Mirrors and cloned repositories remain visible in provenance where useful, but duplicate copies do not count as independent corroboration and do not receive full scoring weight.

## Social noise

Low-information social utterances cannot seed Top 20 candidates. Generic single-word/name-like tokens such as `More`, `Yes`, `No`, and `Now` are excluded from named-entity evidence. Short social posts can still appear later as enrichment around an already-substantive candidate.

## Trendinary Score v4

Score v4 separates:

- attention
- velocity
- publisher breadth
- platform breadth
- novelty
- confidence

The API retains the older `source_breadth` and `community_breadth` fields as compatibility aliases for publisher breadth and platform breadth. New clients should prefer the explicit v4 fields.

## Google Trends

Trendinary uses Google's official **Trending Now RSS** (`https://trends.google.com/trending/rss?geo=US`) as an enabled public discovery/corroboration source. It is treated as search-interest evidence rather than absolute search volume.

Google also offers an official Google Trends API alpha. Access is approval-based, so the API source remains registered but disabled until credentials and approved access are available. Trendinary does not depend on scraped or unofficial Google Trends endpoints.

When the alpha API becomes available to this deployment, the intended role is to enrich/rerank existing candidate topics with consistently scaled search-interest history and geography, not to turn generic keywords into event identity by themselves.

## Source fleet

The scanner combines Hacker News, Bluesky Jetstream, GitHub repository discovery, Wikipedia pageviews, GDELT, the enabled RSS registry, Google Trends Trending Now, and optional credentialed APIs such as YouTube and NewsData. Runtime health and admin source analytics remain the operational views for source reliability and contribution.
