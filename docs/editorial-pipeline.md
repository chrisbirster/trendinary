# Private editorial pipeline

Trendinary has two related but deliberately separate products:

- public Trendinary answers **what is trending?**
- private `/admin` answers **what should I spend my time reading, watching, or listening to?**

Personal editorial actions never affect the public Trendinary Score.

## Pipeline

```text
discovery
  -> normalization / conservative URL dedupe
  -> optional metadata enrichment
  -> deterministic editorial scoring
  -> /admin/inbox
  -> queue / open / consume / save / reject
  -> personal note + worth-sharing reaction
  -> newsletter issue assembly
```

## Storage

The editorial domain shares the process-wide SQLite connection and writer policy with trend history, but has separate tables and APIs. Important tables include:

- `editorial_sources`
- `content_items`
- `content_discoveries`
- `content_notes`
- `ingestion_runs`
- `newsletter_issues`
- `newsletter_issue_items`

Rejected content is retained because it is useful future preference evidence.

## Editorial score

The v0.2 score is intentionally deterministic and inspectable. It uses neutral defaults when Trendinary does not yet have real personalization evidence.

```text
personal relevance  30%
novelty             20%
source quality      15%
trend signal        15%
freshness           10%
serendipity         10%
```

Serendipity is explicit rather than pretending the recommendation system has learned a perfect preference model. The UI may reserve roughly 10-20% of attention for material outside the obvious interest profile.

## Authorization

`/admin/*` and `/api/v1/admin/*` are guarded by the Go server. Hiding frontend navigation is not security.

The initial guard uses HTTP Basic authentication with:

```bash
TRENDINARY_ADMIN_PASSWORD='a-long-private-password'
```

When that variable is absent, admin access is unavailable. Public Trendinary routes remain independent.

## Manual ingestion

TechURLs uses the same ingestion service from the CLI and HTTP admin action:

```bash
go run ./cmd/trendinary ingest techurls
# or, with a built binary
trendinary ingest techurls
```

The source can later be invoked by a scheduler without changing ingestion logic.

## Newsletter assembly

The current issue builder is editorial assembly only. It can organize saved/consumed/noted material into section data, but it does not send newsletters and must not fabricate personal opinions. Source material, personal notes, and generated structural suggestions remain distinguishable.
