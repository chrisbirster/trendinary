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
  |-- periodic trend scanner
  `-- continuous ATProto Jetstream collector
       |
       +--> bounded recent-signal window
       `--> SQLite on Fly volume
```

The HTTP server exposes:

```text
/api/v1/*   versioned JSON API
/*          embedded Vite/Solid 2 SPA
```

Vite builds the Solid 2 application into `internal/web/dist`. Go's `embed` package packages that directory into the production binary. Solid Router owns client-side navigation; the Go static handler falls back to `index.html` for non-API routes.

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

Cloudflare KV is reserved for durable edge metadata/configuration rather than sub-minute trend state. R2 is the long-lived raw/provenance archive target. Business logic, trend scoring, clustering, source analysis, and the public API stay in Go.

## API contract

The current API namespace is `/api/v1`.

Important routes include:

- `GET /api/v1/healthz`
- `GET /api/v1/trends`
- `GET /api/v1/trends/:slug`
- `GET /api/v1/trends/:slug/history`
- `GET /api/v1/sources/:domain`
- `GET /api/v1/methodology/score`
- `GET /api/v1/methodology/bias`

The API is versioned so the Solid app, future native clients, and third-party consumers can evolve independently.

## Discovery and scoring pipeline

```text
Hacker News polling -----------+
                               |
ATProto Jetstream v2 ----------+--> normalized signals
  app.bsky.feed.post commits    |        |
                               |        v
                               |   bounded recent window
                               |        |
                               +--------+
                                        v
                                  cluster engine
                                        |
                          Bluesky search enrichment
                                        |
                                        v
                             historical SQLite baseline
                                        |
                                        v
                          Trendinary Score + lifecycle
                                        |
               +------------------------+----------------------+
               |                        |                      |
               v                        v                      v
             NOW                      PEEP                 trend history
```

Hacker News and Jetstream are peers at discovery time. Either may produce a trend independently. Bluesky search remains useful as a targeted enrichment layer for top candidate clusters.

## AT Protocol stream correctness

Jetstream v2 is filtered to `app.bsky.feed.post` commit events. Creates, updates, and deletes are folded into durable signal state and the bounded recent window.

For every event batch:

1. persist signal creates/updates/deletes;
2. update the bounded in-memory working window;
3. persist `Batch.LastCursor()`.

Only advancing the cursor after state succeeds makes replay idempotent and avoids gaps after process crashes. A saved cursor reconnects through Jetstream archive replay and then cuts over to the live tail.

See `docs/jetstream.md` for operational details.

## Persistence

SQLite is the operational source of truth on the Fly volume at `/data/trendinary.db` in production. It currently stores:

- normalized source signals;
- trend snapshots;
- raw observation metrics used for baseline recalibration;
- score-model version and normalized score inputs;
- durable stream cursors.

SQLite runs in WAL mode with a single application writer. The in-memory recent-signal store is deliberately bounded and disposable; it exists only to make current-window clustering cheap.

R2 remains the target for raw source payloads, provenance/replay fixtures, exports, and SQLite backups. User/follow/account storage can be introduced separately as those product features land.
