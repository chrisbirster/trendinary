# Trendinary architecture

## Runtime shape

Trendinary ships as one Go binary behind a thin Cloudflare edge.

```text
Browser
  |
  v
Cloudflare DNS + Worker edge (SST)
  |
  v
Fly.io
  |
  v
Go process
  |-- HTTP/API server
  |-- private editorial/admin API
  |-- periodic multi-source trend scanner
  `-- continuous ATProto Jetstream collector
       |
       +--> bounded recent-signal window
       `--> SQLite on Fly volume
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

## Local development

Run the API:

```bash
npm run dev:api
```

Run Vite in another terminal:

```bash
npm run dev
```

Vite proxies `/api` to `http://127.0.0.1:8080`.

Set `TRENDINARY_JETSTREAM_DISABLED=1` when local development should not connect to the live AT Protocol stream.

Enable local admin access with:

```bash
TRENDINARY_ADMIN_PASSWORD='a-long-private-password' npm run dev:api
```

## Production build

```bash
npm run build:web
npm run build:server
```

The Dockerfile performs the same sequence in separate Node and Go build stages and emits the Fly production image. Go module lock files are verified by CI before the application build.

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
                              historical SQLite baseline
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

Direct Reddit API ingestion is not part of the active v0.2 runtime. Trendinary does not add a Reddit HTML-scraping bypass; Reddit-adjacent material can enter through TechURLs or original-publisher discovery.

See `docs/news-discovery.md`.

## AT Protocol stream correctness

Jetstream v2 is filtered to `app.bsky.feed.post` commit events. Creates, updates, and deletes are folded into durable signal state and the bounded recent window.

For every event batch:

1. persist signal creates/updates/deletes;
2. update the bounded in-memory working window;
3. persist `Batch.LastCursor()`.

Only advancing the cursor after state succeeds makes replay idempotent and avoids gaps after process crashes. A saved cursor reconnects through Jetstream archive replay and then cuts over to the live tail.

See `docs/jetstream.md` for operational details.

## Persistence

SQLite is the operational source of truth on the Fly volume at `/data/trendinary.db` in production. It stores public trend state and private editorial state in separate domain tables while sharing one connection/writer policy.

Public/history storage includes:

- normalized source signals;
- trend snapshots;
- raw observation metrics used for baseline recalibration;
- stable trend entities/aliases;
- propagation observations;
- score-model version and normalized score inputs;
- durable Jetstream cursors;
- durable daily API-quota reservations.

Private editorial storage includes:

- discovery sources;
- normalized/deduplicated content items;
- discovery occurrences;
- personal editorial state/timestamps;
- notes and worth-sharing reactions;
- ingestion runs;
- newsletter issues and issue items.

SQLite runs in WAL mode with a single application writer. The in-memory recent-signal store is deliberately bounded and disposable; it exists only to make current-window clustering cheap.

## Backups and provenance

`trendinary backup <destination.db>` uses SQLite `VACUUM INTO` to produce a transactionally consistent standalone snapshot rather than copying a live WAL database.

`.github/workflows/backup.yml` runs manually or daily:

1. execute the backup command inside the Fly machine against the mounted `/data` volume;
2. transfer the snapshot with Fly SFTP;
3. verify it is non-empty and print its SHA-256;
4. upload it to the configured Cloudflare R2 bucket via the R2 S3-compatible endpoint;
5. remove the temporary snapshot from Fly.

The workflow expects `FLY_API_TOKEN`, R2 access-key secrets, `CLOUDFLARE_DEFAULT_ACCOUNT_ID`, and repository variable `TRENDINARY_R2_BUCKET`.

R2 remains the long-lived target for raw source payloads, provenance/replay fixtures, exports, and SQLite backups.
