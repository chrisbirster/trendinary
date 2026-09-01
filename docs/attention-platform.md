# Attention Platform v1

This train turns Trendinary's Jetstream foundation into a cross-network attention and explanation system.

## Discovery

Current discovery sources:

- Hacker News
- AT Protocol / Bluesky Jetstream v2
- GitHub repository search
- Reddit OAuth Data API when approved credentials are configured

All sources normalize into the same `Signal` model before clustering and scoring.

## Candidate quality

- Jetstream-only clusters require at least two distinct authors.
- The recent ATProto working set remains bounded by TTL and capacity.
- Lexical clustering is capped to the newest 1,500 streaming signals per pass.
- At most the strongest 100 candidate clusters proceed to scoring.
- Candidate Bluesky posts are selectively hydrated through AppView for engagement counters; the firehose itself remains cheap.

## Identity

Lexical wording is not the public identity of a trend. SQLite persists stable trend entities, aliases, canonical terms, and slugs. A later scan can reconnect to the same entity even when the cluster wording changes.

## Source Lens

Political leaning is evidence-backed source metadata, not an inferred verdict. Perspective Mix counts rated and unrated sources separately. Political leaning is explicitly separate from factual reliability, article stance, and claim accuracy.

## Propagation

Trendinary records source observations over time and exposes an ordered propagation path. The path is based on observed timestamps, not generated narrative order.

## Explanation

`grounded-deterministic-v1` builds WTF, What Changed, Lore, evidence, and Ask responses from the measured trend object and citable source bundle. It does not require an LLM and does not invent facts outside the evidence bundle. A future model-backed implementation can use the same response contract while remaining citation constrained.

## API

- `GET /api/v1/health/streams`
- `GET /api/v1/trends/:slug/history`
- `GET /api/v1/trends/:slug/propagation`
- `GET /api/v1/trends/:slug/explanation`
- `POST /api/v1/trends/:slug/ask`

## Environment

- `TRENDINARY_GITHUB_DISABLED=1` disables GitHub discovery.
- `TRENDINARY_GITHUB_TOKEN` increases GitHub API headroom.
- `TRENDINARY_GITHUB_LIMIT` controls GitHub candidates.
- `TRENDINARY_REDDIT_DISABLED=1` disables Reddit discovery.
- `TRENDINARY_REDDIT_CLIENT_ID` and `TRENDINARY_REDDIT_CLIENT_SECRET` enable Reddit OAuth discovery.
- `TRENDINARY_REDDIT_USER_AGENT` configures the required Reddit user agent.
- `TRENDINARY_REDDIT_LIMIT` controls Reddit candidates.
