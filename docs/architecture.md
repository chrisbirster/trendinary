# Trendinary architecture

## Runtime shape

Trendinary ships as one stateless Go binary behind a thin Cloudflare edge, with Turso/libSQL as the durable production database.

```text
Browser
  |
  v
Cloudflare DNS + Worker edge (SST)
  |
  v
Fly.io (stateless)
  |
  v
Go process
  |-- HTTP/API server
  |-- private editorial/admin API
  |-- periodic multi-source trend scanner
  `-- continuous ATProto Jetstream collector
       |
       +--> bounded recent-signal window
       `--> Turso/libSQL durable state
```

The HTTP server exposes:

```text
/api/v1/*         public versioned JSON API
/api/v1/admin/*   private editorial JSON API
/admin/*          private Solid admin routes
/*                embedded Vite/Solid 2 SPA
```

Vite builds the Solid 2 application into `internal/web/dist`. Go's `embed` package packages that directory into the production binary. Solid Router owns client-side navigation; the Go static handler falls back to `index.html` for non-API routes.

The private admin wrapper executes before the SPA. `/admin/*` and `/api/v1/admin/*` therefore require server-side authorization rather than depending on hidden frontend navigation.

## Database schema boundary

Atlas is the only component allowed to create or evolve the application schema. The desired state is `schema/trendinary.sql` and Atlas environments live in `atlas.hcl`.

```text
explicit reset (optional) -> Atlas schema apply -> Trendinary runtime
```

Normal runtime opens an existing database through a schema-managed connection, performs a read-only compatibility check, and then does application data work. It never runs migrations. Blank or stale databases fail startup with `database schema is not migrated`.

`trendinary db reset` is the only destructive application command. It uses an administrative connection, drops only the explicitly declared Trendinary-owned tables plus the obsolete reset ledger, and exits without recreating schema. Unknown/provider-owned database objects are outside that reset boundary.

This separation applies to local SQLite, local libSQL release tests, and production Turso.

## Local development

Prepare the default `trendinary.db` with Atlas before starting the API:

```bash
npm run db:schema:local:plan
npm run db:schema:local:apply
npm run dev:api
```

Run Vite in another terminal:

```bash
npm run dev
```

Vite proxies `/api` to `http://127.0.0.1:8080`.

Without Turso environment variables, local development uses `trendinary.db` through `modernc.org/sqlite`. Tests use isolated SQLite/libSQL databases prepared from the same desired schema. Application startup never bootstraps those databases itself.

Set `TRENDINARY_JETSTREAM_DISABLED=1` when local development should not connect to the live AT Protocol stream.

Enable local admin access with:

```bash
TRENDINARY_ADMIN_PASSWORD='a-long-private-password' npm run dev:api
```

To use Turso intentionally instead:

```bash
export TURSO_DATABASE_URL='libsql://...turso.io'
export TURSO_AUTH_TOKEN='...'
npm run db:schema:plan
npm run db:schema:apply
npm run dev:api
```

## Production build

```bash
npm run build:web
npm run build:server
```

The Dockerfile performs the same sequence in separate Node and Go build stages and emits the Fly production image. Go module lock files are verified by CI before the application build.

## Production schema/deploy topology

Production schema application is outside the Fly application lifecycle:

```text
release preflight: Atlas dry-run
          ↓
published vX.Y.Z release
          ↓
Atlas schema apply to Turso
          ↓
Fly deploy
          ↓
Cloudflare deploy
          ↓
public production smoke
```

Production schema/deploy jobs are serialized. Fly Machines therefore never race each other on DDL, and a new application release starts only after the target schema is prepared.

## Public edge

SST owns the Cloudflare Worker and the `trendinary.com` custom domain. The worker is intentionally a thin reverse proxy to `https://trendinary.fly.dev`.

The edge owns concerns that belong before the Go application:

- immutable asset caching;
- rate limiting;
- bot / abuse controls;
- request normalization;
- coarse API caching where freshness rules permit it;
- edge observability.

Cloudflare KV is reserved for durable edge metadata/configuration rather than sub-minute trend state. R2 is the long-lived raw/provenance archive target. Business logic, trend scoring, clustering, source analysis, editorial logic, and the public/private APIs stay in Go.

## Public API contract

The current API namespace is `/api/v1`.

Important public routes include:

- `GET /api/v1/healthz`
- `GET /api/v1/health/streams`
- `GET /api/v1/trends`
- `GET /api/v1/trends/:slug`
- `GET /api/v1/trends/:slug/history`
- `GET /api/v1/trends/:slug/propagation`
- `GET /api/v1/trends/:slug/explanation`
- `POST /api/v1/trends/:slug/ask`
- `GET /api/v1/sources/:domain`
- `GET /api/v1/methodology/score`
- `GET /api/v1/methodology/bias`

The API is versioned so the Solid app, future native clients, and third-party consumers can evolve independently.

## Private editorial API

The private admin API supports content inbox/queue/notes/trash state, enrichment, TechURLs ingestion/source status, and newsletter issue assembly. Domain logic lives in `internal/editorial`; HTTP handlers only translate requests/responses.

See `docs/editorial-pipeline.md` and `docs/sources/techurls.md`.

## Discovery and scoring pipeline

```text
ATProto Jetstream / Bluesky ---------+
Hacker News -------------------------|
GitHub ------------------------------|
RSS / Atom --------------------------|
Wikipedia pageviews -----------------+--> normalized Signal
YouTube (optional) ------------------|
GDELT DOC ---------------------------|
NewsData (optional quota) -----------+
                                           |
                                           v
                                  noise/flood controls
                                           |
                                           v
                             entity-aware candidate merge
                                           |
                                           v
                               stronger candidate gate
                                           |
                                  Bluesky enrichment
                                           |
                                           v
                                stable trend identity
                                           |
                                           v
                               historical Turso baseline
                                           |
                                           v
                           Trendinary Score + lifecycle
                                           |
                        +------------------+------------------+
                        |                  |                  |
                        v                  v                  v
                       NOW                PEEP             history /
                                                          propagation
```

NewsData's free feed is deliberately corroborative-only: a NewsData-only cluster cannot independently become a public trend. GDELT searches a short recent window and is cached so the two-minute scanner does not translate into a request every two minutes.

Direct Reddit API ingestion is not part of the active runtime. Trendinary does not add a Reddit HTML-scraping bypass; Reddit-adjacent material can enter through TechURLs or original-publisher discovery.

See `docs/news-discovery.md`.

## AT Protocol stream correctness

Jetstream v2 is filtered to `app.bsky.feed.post` commit events. Creates, updates, and deletes are folded into durable signal state and the bounded recent window.

For every event batch:

1. persist signal creates/updates/deletes to Turso;
2. update the bounded in-memory working window;
3. persist `Batch.LastCursor()` to Turso.

Only advancing the cursor after state succeeds makes replay idempotent and avoids gaps after process crashes. A saved cursor reconnects through Jetstream archive replay and then cuts over to the live tail.

See `docs/jetstream.md` for operational details.

## Persistence

Turso/libSQL is the production operational source of truth. Fly has no database volume and can be replaced/restarted without moving durable application state. The Go process uses the remote libSQL `database/sql` driver, and the private editorial and Following stores share the same durable SQL handle while remaining separate application domains.

Public/history storage includes:

- normalized source signals;
- trend snapshots;
- raw observation metrics used for baseline recalibration;
- stable trend entities/aliases;
- propagation observations;
- score-model version and normalized score inputs;
- durable Jetstream cursors;
- durable daily API-quota reservations.

Private/editorial/personal-radar storage includes:

- discovery sources;
- normalized/deduplicated content items;
- discovery occurrences;
- personal editorial state/timestamps;
- notes and worth-sharing reactions;
- ingestion runs;
- newsletter issues and issue items;
- Radar follows, baselines, alerts, preferences, and push subscriptions.

The in-memory recent-signal store remains bounded and disposable; it exists only to make current-window clustering cheap.

Production sets `TRENDINARY_REQUIRE_TURSO=1`. If `TURSO_DATABASE_URL` or `TURSO_AUTH_TOKEN` is missing, the origin refuses to start rather than silently falling back to ephemeral local SQLite.

## Recovery and provenance

Database recovery is a Turso concern, not a Fly-container concern. Use Turso point-in-time recovery and Turso database export/dump tooling for production database recovery or offline snapshots.

`trendinary backup <destination.db>` remains available only when running against local SQLite; it is intentionally rejected for a Turso-backed process.

R2 remains the long-lived target for raw source payloads, provenance/replay fixtures, and generated exports. It is not required to keep the production relational database durable.

See `docs/turso.md`, `docs/local-release-gate.md`, and `docs/release-process.md` for operational details.
