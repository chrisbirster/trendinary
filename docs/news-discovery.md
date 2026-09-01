# News discovery strategy

Trendinary uses multiple low-cost/open discovery paths instead of making one paid aggregator the source of truth.

## Active public discovery layers

```text
ATProto Jetstream / Bluesky
Hacker News
GitHub
RSS / Atom feeds
Wikipedia pageviews
YouTube (optional API key)
GDELT DOC
NewsData (optional free quota)
```

Direct Reddit API ingestion was removed from the active v0.2 runtime. Trendinary does not add a Reddit HTML scraper intended to bypass commercial API/access requirements. Reddit-adjacent discoveries may arrive indirectly through TechURLs or original publishers.

## GDELT

`internal/ingest/gdelt` uses the GDELT DOC 2.0 ArticleList JSON API.

Defaults:

- one broad technology/internet query
- one-hour search window
- newest-first ordering
- up to 250 records
- 10-minute in-process cache
- explicit Trendinary User-Agent

Environment:

```bash
TRENDINARY_GDELT_DISABLED=1
TRENDINARY_GDELT_QUERY='(technology OR "artificial intelligence" OR cybersecurity OR startup OR science OR gaming)'
TRENDINARY_GDELT_LIMIT=250
```

GDELT is a discovery/index source. Trendinary normalizes the returned original article URL and publisher domain into its `Signal` model.

## NewsData

NewsData is optional and only activated when an API key is configured:

```bash
TRENDINARY_NEWSDATA_API_KEY='...'
TRENDINARY_NEWSDATA_DAILY_CALLS=200
TRENDINARY_NEWSDATA_CATEGORY='technology'
```

The free-tier request size is fixed at 10 in Trendinary. The adapter follows `nextPage` tokens and maintains a small rolling in-memory discovery cache.

### Durable quota pacing

`history.ReserveDailyQuota` stores request usage in SQLite before each network request. Failed requests still count locally. This prevents a restart from resetting Trendinary's own daily accounting.

Instead of immediately exhausting the allowance, the next-call time is distributed across the remaining UTC day. If Trendinary starts halfway through a day with the full allowance unused, the cadence automatically tightens so it can still make useful use of the remaining quota.

Every successfully normalized NewsData page is persisted to `signals`, even when those articles never become a public trend. That makes the quota useful for future replay/calibration.

### Corroboration-only rule

The free NewsData feed is treated as delayed corroboration. A cluster containing only `newsdata:*` signals cannot independently pass the Detection Quality v2 candidate gate. NewsData can strengthen a cluster discovered by a fresher source.

## RSS / Atom

`TRENDINARY_RSS_FEEDS` accepts a comma-separated list of publisher feeds. RSS/Atom is preferred when publishers expose it because it avoids a central API quota and preserves the original publisher URL.

```bash
TRENDINARY_RSS_FEEDS='https://example.com/feed.xml,https://another.example/rss'
TRENDINARY_RSS_LIMIT=50
```

## Source identity

A discovery adapter is not automatically the publisher. For news aggregators/indexes such as GDELT and NewsData, the public `Signal.Source` is the original publisher/domain whenever that can be determined. This allows cross-publisher breadth to remain meaningful.

## Candidate quality

Detection Quality v2 applies source/author flood limits, duplicate-text suppression, entity-aware cluster merging, and stronger candidate gating before historical scoring. Adding more news sources must increase corroboration rather than turn every standalone headline into a trend.
