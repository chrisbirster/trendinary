# Trendinary

**The live dictionary of the internet.**

Trendinary watches public internet signals, detects unusual acceleration, clusters related observations into stable trend entities, explains why they are moving, and tracks how attention propagates across sources.

It also contains a private editorial system under `/admin` for deciding what is worth reading/watching/listening to, calibrating trend detection quality, and assembling saved material into newsletter drafts.

## Stack

- Go 1.27 HTTP/API server
- Turso/libSQL as the production durable database
- local SQLite (`modernc.org/sqlite`) for development and tests
- Atlas for declarative SQLite/libSQL schema management
- SolidJS 2 + Solid Router
- StyleX
- Vite 8 + TypeScript
- embedded SPA via Go `embed`
- stateless Fly.io origin
- Cloudflare Worker/DNS/KV/R2 managed through SST

## Branch and release flow

```text
feature/* -> dev -> main -> vX.Y.Z -> production
```

Feature work targets `dev`. Normal releases are `dev -> main` pull requests. `main` is release-only except documented hotfixes.

## Public product

- `/` — NOW live scoreboard
- `/peep` — early/accelerating signals ranked by a dedicated PEEP score
- `/fomo` — finite, history-backed catch-up briefing
- `/following` — personal-radar product surface
- `/trend/:slug` — stable trend intelligence page with momentum, timed propagation, perspective, evidence, WTF/LORE, and Ask

Current discovery sources include Hacker News, ATProto Jetstream/Bluesky, GitHub, configurable RSS/Atom, Wikipedia pageviews, GDELT DOC, optional YouTube, and optional quota-aware NewsData.

Direct Reddit API ingestion is not an active dependency.

## Signal Quality v3

Trendinary Score v3 keeps the 0–100 public score but shifts weight from absolute attention toward unexpected acceleration and independent-source spread:

```text
attention          14%
velocity           34%
source breadth     19%
community breadth  13%
novelty            12%
confidence           8%
```

PEEP has a separate early-signal score so a small topic moving abnormally fast can outrank a famous topic behaving normally. It emphasizes velocity, source breadth, community breadth, and novelty, then confidence-gates the result.

The scanner also persists the exact signal membership of each stable trend. Private human labels can therefore be replayed through the deterministic entity-aware clusterer instead of evaluating against reconstructed summaries.

See `docs/signal-quality-v3.md`.

## Private editorial admin

Everything under `/admin/*` and `/api/v1/admin/*` is private and enforced by the Go server.

Initial local/admin auth:

```bash
export TRENDINARY_ADMIN_PASSWORD='use-a-long-private-password'
```

Important routes:

- `/admin/inbox`
- `/admin/queue`
- `/admin/notes`
- `/admin/issues`
- `/admin/sources`
- `/admin/quality` — label live trends and inspect detection/replay quality
- `/admin/trash`

The quality workspace accepts these durable labels:

- real trend
- noise
- duplicate
- interesting / caught early
- detected too late
- bad cluster
- wrong canonical name

It reports a precision proxy, early-hit rate, cluster health, naming health, a score-threshold recommendation, and deterministic replay precision/recall. These metrics are for detection calibration, not engagement optimization.

TechURLs is the first private editorial discovery adapter.

Manual ingestion:

```bash
go run ./cmd/trendinary ingest techurls
```

A local SQLite database can still produce a consistent snapshot:

```bash
go run ./cmd/trendinary backup ./trendinary-backup.db
```

That command is intentionally unavailable when the process is connected to Turso; production recovery uses Turso point-in-time recovery/export tooling.

Print current historical calibration:

```bash
go run ./cmd/trendinary calibration
```

## News discovery configuration

GDELT is enabled by default unless explicitly disabled:

```bash
export TRENDINARY_GDELT_DISABLED=1
export TRENDINARY_GDELT_QUERY='(technology OR "artificial intelligence" OR cybersecurity OR startup OR science OR gaming)'
export TRENDINARY_GDELT_LIMIT=250
```

NewsData activates only when a key is present. Its daily calls are durably paced in the shared SQL database rather than burned immediately:

```bash
export TRENDINARY_NEWSDATA_API_KEY='...'
export TRENDINARY_NEWSDATA_DAILY_CALLS=200
export TRENDINARY_NEWSDATA_CATEGORY='technology'
```

RSS/Atom feeds:

```bash
export TRENDINARY_RSS_FEEDS='https://example.com/feed.xml,https://another.example/rss'
```

Optional YouTube discovery:

```bash
export TRENDINARY_YOUTUBE_API_KEY='...'
```

See `docs/news-discovery.md` for source semantics and quota behavior.

## Local development

Install the exact committed dependency graph and make sure the Atlas CLI is available:

```bash
npm ci
```

Prepare the default local SQLite database from the declarative schema before starting the application:

```bash
npm run db:schema:local:plan
npm run db:schema:local:apply
```

Then run the Go API:

```bash
npm run dev:api
```

Normal application startup never creates or upgrades schema. If `trendinary.db` is blank or stale, startup fails with `database schema is not migrated` until Atlas has prepared it.

Run Vite separately:

```bash
npm run dev
```

Vite proxies `/api` to the local Go server.

To avoid connecting to ATProto during local UI work:

```bash
TRENDINARY_JETSTREAM_DISABLED=1 npm run dev:api
```

To intentionally use Turso during development, set `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`, review `npm run db:schema:plan`, apply it with `npm run db:schema:apply`, and then start the API.

## Verification

```bash
npm run verify
```

For the production-topology release gate, including real libSQL, Atlas, two application containers, and browser E2E:

```bash
npm run verify:release
```

See `docs/local-release-gate.md` for the complete gate and `docs/dependencies.md` for dependency and supply-chain policy.

## Production persistence

Fly is stateless. Production requires:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

`fly.toml` sets `TRENDINARY_REQUIRE_TURSO=1`, so production startup fails closed if Turso is not configured. No Fly volume is required.

Atlas is the sole owner of application schema creation and evolution. The production release path plans the desired schema before a release tag is created, then applies it to Turso before the new Fly application release starts. Trendinary application Machines only verify the prepared schema and perform normal data operations.

Turso stores public trend history, stable identities, signal memberships, human quality labels, Jetstream cursors, quota state, and the private editorial database. R2 remains the archive target for raw source provenance/replay payloads rather than the primary database.

See `docs/turso.md` and `docs/architecture.md`.

## Documentation

- `docs/product-vision.md`
- `docs/product-language.md`
- `docs/architecture.md`
- `docs/attention-platform.md`
- `docs/trend-score.md`
- `docs/signal-quality-v3.md`
- `docs/news-discovery.md`
- `docs/editorial-pipeline.md`
- `docs/sources/techurls.md`
- `docs/turso.md`
- `docs/local-release-gate.md`
- `docs/release-process.md`
- `docs/dependencies.md`
