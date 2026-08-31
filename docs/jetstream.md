# AT Protocol Jetstream discovery

Trendinary uses the official Bluesky Jetstream v2 Go client as a continuous AT Protocol discovery source.

## Scope

The first stream intentionally consumes only commit events for:

```text
app.bsky.feed.post
```

Identity/account events and other collections can be added later when they directly improve trend discovery.

## Runtime flow

```text
Jetstream v2
  -> app.bsky.feed.post commits
  -> create / update / delete folding
  -> normalized Trendinary Signal
  -> SQLite durable signal state
  -> bounded recent-signal window
  -> durable Jetstream cursor
  -> periodic multi-source scanner
  -> HN + Jetstream discovery merge
  -> bounded clustering sample
  -> Bluesky search enrichment
  -> historical baseline
  -> Trendinary Score + lifecycle
  -> /api/v1/trends
```

The HTTP server, Jetstream collector, and periodic scorer run independently in the same Go process. A recoverable stream disconnect does not take the website or API down.

## Cursor correctness

The stream cursor is stored in SQLite under:

```text
atproto-jetstream-v2-posts
```

For each batch Trendinary applies state in this order:

1. fold repeated mutations for each record in wire order so the last mutation wins;
2. persist final creates/updates and upstream deletes;
3. update the bounded recent window;
4. persist `Batch.LastCursor()`.

The state operations are idempotent. If the process crashes after writing signals but before advancing the cursor, the batch is replayed safely.

On first boot, Trendinary starts from Jetstream's current live tip. Once a cursor exists, reconnects use Jetstream v2 archive replay from that cursor and then cut over to the live tail. This avoids intentionally replaying the entire historical AT Protocol archive on a brand-new installation while preserving gap-free restarts.

## Recent signal window

SQLite is the durable record; clustering does not query the entire signal table on every score pass. Jetstream writes a bounded in-memory working set instead.

Defaults:

- maximum entries: `50,000`
- TTL: `30m`

The window uses a timestamp min-heap for expiry and capacity eviction, making normal stream ingestion O(log n) instead of scanning the whole window for every post. Deletes and updates are reflected immediately, and stale heap nodes from updates cannot evict newer versions of a record.

The current v0.1 lexical clusterer performs pairwise similarity checks. Until candidate generation is indexed or semantic, each scoring pass therefore uses only the newest `1,500` streaming signals from the larger working window, plus the polling-source signals. This bounds the expensive phase while preserving the larger buffer for restart/replay context and future clustering strategies.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `TRENDINARY_JETSTREAM_DISABLED` | unset | Set to `1` to disable continuous ATProto ingestion. |
| `TRENDINARY_JETSTREAM_HOST` | `https://jetstream.us-east.bsky.network` | Jetstream v2 service endpoint. |
| `TRENDINARY_JETSTREAM_BATCH_SIZE` | `128` | Maximum events folded per client batch. |
| `TRENDINARY_RECENT_SIGNAL_LIMIT` | `50000` | Maximum signals retained in the in-memory discovery window. |
| `TRENDINARY_RECENT_SIGNAL_TTL` | `30m` | Age limit for the in-memory discovery window. |
| `TRENDINARY_SCAN_INTERVAL` | `2m` | Periodic scoring cadence. |

## Current limitations

Jetstream commit events provide the post record, but not the continuously changing like/repost/reply counters used by the Bluesky AppView. Stream-first topics therefore initially score from signal count, velocity, author/community breadth, novelty, and cross-source spread. Search enrichment can add engagement observations for top clusters.

The first collector also does not yet fold DID-level account/identity events. Recent post state naturally ages out of the working window, while durable account-state cleanup and handle hydration are follow-on work.

Next ATProto improvements:

- DID-to-handle identity cache;
- account-state folding/purge semantics;
- selective engagement hydration for candidate clusters;
- richer record facets/links/quoted-post context;
- stream health/cursor observability endpoint;
- raw event archive samples in R2 for replay/debugging;
- indexed/semantic clustering so the full live window can participate efficiently.
