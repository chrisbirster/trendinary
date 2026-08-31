# Trendinary architecture

## Runtime shape

Trendinary ships as one Go binary.

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
Go HTTP server
  |-- /api/v1/*       versioned JSON API
  `-- /*              embedded Vite/Solid 2 SPA
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

## Production build

```bash
npm run build:web
npm run build:server
```

The Dockerfile performs the same sequence in separate Node and Go build stages and emits a small production image.

## Public edge

SST owns the Cloudflare Worker and the `trendinary.com` custom domain. The worker currently behaves as a deliberately thin reverse proxy to `https://trendinary.fly.dev`.

The edge is the future home for concerns that belong before the Go application:

- immutable asset caching
- rate limiting
- bot / abuse controls
- request normalization
- coarse API caching where freshness rules permit it
- edge observability

Business logic, trend scoring, clustering, source analysis, and the public API stay in Go.

## API contract

The first API namespace is `/api/v1`.

Current routes:

- `GET /api/v1/healthz`
- `GET /api/v1/trends`
- `GET /api/v1/trends/:slug`
- `GET /api/v1/sources/:domain`
- `GET /api/v1/methodology/bias`

The API is intentionally versioned before live ingestion begins so the Solid app, future native clients, and third-party consumers can evolve independently.

## Data pipeline target

```text
Source adapters
    |
    v
Raw signals ---> raw archive
    |
    v
Normalization / entity extraction
    |
    v
Cluster engine
    |
    v
Trend snapshots + baselines
    |
    +--> Trendinary Score / lifecycle
    +--> WTF? grounded explanation
    +--> LORE durable context
    +--> VIBE conversation shape
    `--> FOMO / alerts / Following
```

The first live adapters should be Hacker News and AT Protocol because both expose clean public APIs and are highly relevant to Trendinary's early tech/internet audience.

## Persistence

The current branch uses an in-memory store so the HTTP/API boundary can stabilize first. Persistent storage is the next backend milestone. Raw source events, normalized signals, trend snapshots, users/follows, generated explanations, and provenance must be stored separately enough that any public explanation can be traced back to evidence.
