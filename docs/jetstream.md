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
  -> clustering
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

1. persist creates/updates and upstream deletes;
2. update the bounded recent window;
3. persist `Batch.LastCursor()`.

The state operations are idempotent. If the process crashes after writing signals but before advancing the cursor, the batch is replayed safely.

On first boot, Trendinary starts from Jetstream's current live tip. Once a cursor exists, reconnects use Jetstream v2 archive replay from that cursor and then cut over to the live tail. This avoids intentionally replaying the entire historical AT Protocol archive on a brand-new installation while preserving gap-free restarts.

## Recent signal window

SQLite is the durable record; clustering does not query the entire signal table on every score pass. Jetstream writes a bounded in-memory working set instead.

Defaults:

- maximum entries: `50,000`
- TTL: `30m`

Both TTL pruning and capacity eviction are deterministic. Deletes and updates are reflected immediately.

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

Next ATProto improvements:

- DID-to-handle identity cache;
- selective engagement hydration for candidate clusters;
- richer record facets/links/quoted-post context;
- stream health/cursor observability endpoint;
- raw event archive samples in R2 for replay/debugging.
