# Source intelligence

Trendinary separates three questions that should not be conflated:

1. **Is an adapter healthy?** — operational reliability of the polling/stream integration.
2. **Did a source contribute useful discovery evidence?** — observed timing and uniqueness in detected trends.
3. **Is a publisher factually reliable?** — a separate future/source-lens concern; it is not inferred from discovery performance.

## Fleet reliability

`/api/v1/health/streams` exposes process-lifetime diagnostics for public discovery adapters:

- attempts and successful polls
- current consecutive failures and last error
- last attempt, last success, and next scheduled poll
- cached signals and total signals produced by successful polls
- last and average request duration
- RSS HTTP request and `304 Not Modified` counts

These counters reset when the Trendinary process restarts. They are operational diagnostics, not historical publisher scores.

The private `/admin/sources` dashboard marks an enabled adapter as:

- `SCHEDULED` before its first successful poll
- `HEALTHY` after recent success
- `DEGRADED` while it has a current upstream error
- `STALE` when its last success is more than three cadences old (with a minimum thirty-minute tolerance)
- `NOT CONFIGURED` when the registry entry is intentionally inactive

## Contribution analytics

`GET /api/v1/admin/sources/analytics?window=168h` derives contribution from durable signals, trend memberships, and lifecycle snapshots.

For both original publishers and discovery channels it reports:

- unique signals observed
- stable trends with evidence from that source
- **first hits** — trends for which that source supplied the earliest persisted membership
- **solo trends** — trends whose evidence in the window came only from that publisher/channel
- average observed lead time from that source's first evidence to the trend's first `BREAKING` snapshot

Publisher and discovery channel are deliberately separate. A WIRED article submitted to Hacker News is publisher `wired.com`, discovery channel `hacker-news`.

First-hit and lead-time metrics describe Trendinary's observed discovery timing. They are not ratings of truth, political perspective, editorial quality, or factual reliability.

## Calibration v1

`GET /api/v1/admin/quality/calibration` is deliberately guarded.

Trendinary will not recommend a new clustering threshold until both conditions are met:

- at least **100 distinct human-labeled stable trends**
- at least **50 persisted replay signals** associated with those labels

Until then the endpoint reports collection progress only. No synthetic labels are generated.

Once the gate is satisfied, Calibration v1 replays the persisted human corpus across deterministic clustering thresholds from `0.30` through `0.70` in `0.02` steps. It selects the highest pairwise F1 result, using precision as a tie-breaker.

The result is a **recommendation only**. Trendinary never changes its production clustering or score weights automatically from calibration output; applying a recommendation requires a reviewed code change and normal release gates.
