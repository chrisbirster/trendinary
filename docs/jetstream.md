# AT Protocol Jetstream discovery

Trendinary uses the official Bluesky Jetstream v2 Go client as a continuous AT Protocol discovery source.

## Scope

The stream consumes commit events for:

```text
app.bsky.feed.post
```

Identity/account events and other collections can be added when they directly improve trend discovery.

## Runtime flow

```text
Jetstream v2
  -> app.bsky.feed.post commits
  -> create / update / delete folding
  -> normalized Trendinary Signal
  -> Turso/libSQL durable signal state
  -> bounded recent-signal window
  -> durable Turso Jetstream cursor
  -> periodic multi-source scanner
  -> cross-network discovery merge
  -> noise controls + bounded clustering
  -> selective Bluesky hydration/profile resolution
  -> historical baseline
  -> Trendinary Score + lifecycle
  -> /api/v1/trends
```

The HTTP server, Jetstream collector, and periodic scorer run independently in the same Go process. A recoverable stream disconnect does not take the website or API down.

## Cursor correctness

The stream cursor is stored in the durable SQL database under:

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

Because production cursor state lives in Turso rather than on a Fly disk, replacing a Fly machine does not reset stream replay position.

## Recent signal window

Turso is the durable record; clustering does not query the entire signal table on every score pass. Jetstream writes a bounded in-memory working set instead.

Defaults:

- maximum entries: `50,000`
- TTL: `30m`

The window uses a timestamp min-heap for expiry and capacity eviction, making normal stream ingestion O(log n) instead of scanning the whole window for every post. Deletes and updates are reflected immediately, and stale heap nodes from updates cannot evict newer versions of a record.

The current lexical/entity-aware clusterer performs pairwise similarity checks over a bounded sample. Each scoring pass therefore uses only the newest `1,500` streaming signals from the larger working window, plus polling-source signals. This bounds the expensive phase while preserving the larger buffer for current attention context.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `TRENDINARY_JETSTREAM_DISABLED` | unset | Set to `1` to disable continuous ATProto ingestion. |
| `TRENDINARY_JETSTREAM_HOST` | `https://jetstream.us-east.bsky.network` | Jetstream v2 service endpoint. |
| `TRENDINARY_JETSTREAM_BATCH_SIZE` | `128` | Maximum events folded per client batch. |
| `TRENDINARY_RECENT_SIGNAL_LIMIT` | `50000` | Maximum signals retained in the in-memory discovery window. |
| `TRENDINARY_RECENT_SIGNAL_TTL` | `30m` | Age limit for the in-memory discovery window. |
| `TRENDINARY_SCAN_INTERVAL` | `2m` | Periodic scoring cadence. |
| `TURSO_DATABASE_URL` | unset | Production libSQL database URL. |
| `TURSO_AUTH_TOKEN` | unset | Production database auth token. |

## Current limitations

Jetstream commit events provide the post record, but continuously changing like/repost/reply counters still come from selective Bluesky AppView hydration rather than the stream itself. Stream-first topics initially score from signal count, velocity, author/community breadth, novelty, and cross-source spread.

DID/handle/profile resolution and candidate engagement hydration are implemented, but richer account-state folding, record facets, quoted-post context, and raw event replay samples remain follow-on opportunities.

Next ATProto improvements include:

- account-state folding/purge semantics;
- richer record facets/links/quoted-post context;
- raw event archive samples in R2 for replay/debugging;
- indexed/semantic candidate generation so more of the live window can participate efficiently.
