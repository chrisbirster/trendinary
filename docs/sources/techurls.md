# TechURLs discovery source

TechURLs is Trendinary's first private editorial discovery adapter.

## What Trendinary consumes

For each usable linked item Trendinary stores or derives:

- headline
- actual outbound destination URL
- TechURLs section/source name
- relative source age such as `3h`
- discovery timestamp
- destination hostname
- conservatively normalized/canonical URL
- coarse content type

TechURLs is the **discovery source**, not the original publisher. Trendinary preserves the outbound destination rather than treating a TechURLs link as the canonical article.

## What Trendinary deliberately does not consume

- full copyrighted article bodies
- TechURLs redirect URLs when a direct outbound URL is available
- hidden/private TechURLs state
- aggressive fuzzy headline deduplication

Destination enrichment is lightweight metadata only and is allowed to fail without blocking the inbox.

## Parser assumptions

The adapter looks for section-like headings/source labels and link rows containing a relative-age token near an absolute outbound HTTP(S) link. It does not require one exact DOM tree. A malformed item is counted and skipped rather than aborting the entire fetch.

Fixture tests cover representative sections, relative ages, outbound URLs, content types, malformed entries, URL normalization, duplicate ingestion, editorial state changes, notes, and ingestion-run accounting.

## URL dedupe

Normalization is deliberately conservative:

1. lowercase hostname
2. remove fragments
3. remove obvious tracking parameters such as `utm_*`/common click IDs
4. preserve meaningful query parameters
5. prefer explicit destination canonical metadata when enrichment confidently finds it

The normalized canonical URL is the primary content identity. Discoveries remain separate records so one content item may be found multiple times without duplicating the item itself.

## Ingestion metrics

Each run records:

- sections seen
- items seen
- items inserted
- items updated
- duplicates
- malformed items
- errors/status
- started/completed timestamps

These are visible from `/admin/sources` and its run history.

## Failure behavior

- explicit Trendinary User-Agent
- bounded HTTP timeout
- no tight polling loop
- one malformed item does not fail the run
- enrichment failure does not reject the discovery
- raw HTML archival is optional and does not gate ingestion

## Manual ingestion

```bash
trendinary ingest techurls
```

The equivalent admin action is:

```text
POST /api/v1/admin/sources/techurls/ingest
```

## Disable source

The source has persisted enabled/disabled state managed from `/admin/sources`. Disabling it prevents ingestion while preserving existing discoveries, notes, rejects, and issue references.

## Reddit-adjacent discovery

Trendinary does not depend on direct Reddit API access and does not add a Reddit HTML scraper as a payment/approval bypass. When TechURLs surfaces Reddit-adjacent technology stories, Trendinary can use those as discovery pointers while retaining freshness penalties and the original destination/source semantics.
