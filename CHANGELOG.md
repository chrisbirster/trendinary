# Changelog

All notable Trendinary changes are tracked here. Releases use Semantic Versioning and Git tags prefixed with `v`.

## [Unreleased]


## [0.6.5] - 2026-09-14

### Changed

- Discovery startup now warms configured RSS, GDELT, YouTube, and NewsData sources immediately while preserving normal steady-state cadence and jitter after the first scan.
- Source discovery now uses per-source timeout budgets, including longer windows for GDELT, NewsData, and YouTube instead of forcing every source through the same eight-second deadline.
- Semantic clustering now precomputes token/entity sets and avoids per-pair allocation churn while remaining context-cancellable, cutting the validated 500-signal staging scan from deadline exhaustion to roughly four seconds.
- The Top 20 shortlist now log-compresses raw engagement and limits any single non-web platform to about 20 percent of the shortlist when other candidates are available, with a bounded reserve for backfill.
- Managed production runtime startup is now read-only with respect to schema/seed provisioning; durable TechURLs seed data moved into Goose migration `00005_runtime_seed_data.sql`, and Web Push no longer persists a generated VAPID key during managed startup.

### Fixed

- Scanner discovery no longer hangs waiting for a source that ignores context cancellation; the parent scan deadline can terminate discovery cleanly.
- YouTube view counts can no longer overwhelm the pre-score shortlist and crowd out corroborated RSS, Hacker News, GitHub, and news signals.
- Top 20 finalization now keeps a five-candidate reserve available until persistence/snapshot work completes, allowing failed candidates to be backfilled instead of producing 18 or 19 chart entries.
- Production startup no longer performs the editorial source UPSERT or conditional Web Push key INSERT that caused Turso write-block crash loops even when scanner and Jetstream producers were disabled.
- Added regression coverage for cancellable clustering, source deadline handling, shortlist diversity, Top 20 backfill, and managed-database startup write safety.


## [0.6.4] - 2026-09-13

### Changed

- Bluesky Jetstream is now a bounded in-memory discovery stream instead of a raw Turso event archive; only scanner-selected trend evidence is persisted.
- Jetstream durable cursor checkpoints are rate-limited to a five-second cadence while preserving a forced final checkpoint on reconnect/shutdown boundaries.
- Production durable scoring is bounded to publish capacity, so a Top 20 scan performs remote candidate work for at most 20 shortlisted clusters.
- Identical signal UPSERTs are true no-ops, and existing trend memberships refresh at most hourly instead of every scan.

### Fixed

- Added `idx_trend_signal_memberships_signal_id` so signal deletes and foreign-key cascades no longer scan memberships by the wrong key order.
- Credential-free public Jetstream resumes now abandon stale history after the freshness grace period even when the cursor continues advancing; authenticated archive replay remains allowed to catch up the full gap.
- Scanner persistence now happens after final Top 20 selection, eliminating durable writes for candidates that cannot be published.
- Added regression coverage proving raw Jetstream posts are not persisted and repeated unchanged evidence does not consume Turso writes.


## [0.6.3] - 2026-09-12

### Changed

- Production scanner durable scoring now evaluates the strongest two times the publish-capacity candidate shortlist (40 candidates for the Top 20) instead of paying remote Turso entity, history, baseline, propagation, snapshot, and membership costs for up to 100 clusters.
- The full discovered cluster universe is still pre-ranked before the durable-work shortlist, preserving ranking competition while keeping a production scan comfortably inside its cadence.

### Fixed

- Stalled Bluesky Jetstream subscriptions now use a per-subscription cancellable context, allowing the watchdog to interrupt a blocked Events iterator instead of relying on transport Close alone.
- Credential-free stale public cursor recovery can now actually exit the stalled iterator and attach at the current live tip while preserving parent collector shutdown semantics.
- Added regression coverage for the Top 20 durable-work budget and watchdog-driven subscription cancellation.


## [0.6.2] - 2026-09-12

### Added

- Source-health snapshots are now exposed by `/api/v1/health/streams`, including polling success, failure, cache, and contribution telemetry needed for production source audits.
- `TRENDINARY_SCAN_TIMEOUT` allows the production scanner budget to be tuned explicitly; the default is 90 seconds within the two-minute scan cadence.

### Changed

- Top 20 chart history reads and writes are batched into a small fixed number of database round trips instead of issuing per-trend and per-scan queries against remote libSQL.
- The Bluesky Jetstream idle watchdog now recycles a no-progress public resume after one minute while preserving authenticated archive-replay cursor semantics.

### Fixed

- Scanner runs no longer exhaust the shared deadline before chart finalization and fail with `persist chart: latest chart scan: context deadline exceeded` under production libSQL latency.
- Credential-free Jetstream recovery no longer reconnects indefinitely to the same stale durable cursor; when a public resume is stale and makes no progress, Trendinary deliberately attaches at the current live tip.
- Added regression coverage for repeated 20-entry chart history, stale public Jetstream resume recovery, and public source-health serialization.

## [0.6.1] - 2026-09-12

### Changed

- Core discovery now warms Google Trends US, WIRED Top Stories, Ars Technica All, ABC News Top Stories, and TechCrunch on the first scanner pass while preserving staggered polling for the wider source fleet.
- GitHub repository discovery now uses a higher minimum-star threshold, and GitHub-only multi-owner bursts no longer manufacture independent publisher breadth for Score v4 corroboration.

### Fixed

- Batched signal and trend-membership UPSERTs reduce remote libSQL round trips and prevent scanner and Jetstream persistence from exhausting the scanner runtime budget.
- Independent discovery sources are polled concurrently with bounded per-source timeouts while preserving deterministic aggregation.
- A stalled Bluesky Jetstream connection is recycled when cursor/event progress stops instead of remaining falsely connected indefinitely.
- Jetstream cursor advancement counts as healthy replay progress even when replayed event timestamps are older than the live freshness window.
- Chart persistence is now required for scan success so public trends cannot be published without durable `NEW`, `RE`, `▲`, or `▼` movement metadata.
- Added regression coverage for real-libSQL batched writes, concurrent discovery, cold-start source bootstrapping, GitHub-only provenance discounting, and Jetstream progress detection.


## [0.6.0] - 2026-09-12

### Added

- Persisted Top 20 chart snapshots with contiguous ranks, score, confidence tier, publisher count, platform count, signal count, and per-trend movement history (`NEW`, `RE`, `▲`, `▼`).
- Publisher/platform provenance on live chart entries so each ranked topic can show the independent evidence behind its position instead of only an aggregate score.
- Official Google Trends Trending Now RSS discovery with source-health telemetry, cadence control, attribution, and deduplication; the official Google Trends API integration remains dormant until API access is approved.
- Release-gate acceptance coverage that feeds enough synthetic independent stories to prove Trendinary can publish credible ranks #1 through #20 without filling the chart with obvious noise.

### Changed

- Trendinary Score advances to Score v4, including duplicate/clone discounting, social-noise filtering, stronger independent-source corroboration, and serialization of the new scoring model in chart output.
- The scanner now targets a genuine internet Top 20 while preserving quality gates, with chart history persisted by Goose migration `00003_top20_chart.sql`.
- Source contribution auditing now tracks the discovery path and publisher/platform breadth needed to explain how RSS/news, GDELT, Hacker News, Wikipedia, Bluesky, GitHub, and Google Trends contribute to real chart entries.

### Fixed

- Readiness/database test fixtures now apply every Goose migration instead of hard-coding migrations `00001` and `00002`, preventing tests from silently missing future schema changes.
- Top 20 regression coverage now verifies chart persistence, rank/movement calculations, publisher/platform counts, clone discounting, social-noise filtering, and Score v4 serialization under the same exact-head CI gate used for release.

## [0.5.7] - 2026-09-08

### Changed

- Database schema ownership moves completely out of Trendinary application startup: Atlas now owns `CREATE`, `ALTER`, and schema evolution from the declarative `schema/trendinary.sql` desired state.
- Production deployment now verifies the release, applies the Turso schema with Atlas, then deploys Fly.io and Cloudflare before running the production smoke gate; production schema/deploy workflows are serialized.
- `trendinary db reset` is now an explicit destructive drop-only command. It never recreates schema, and normal application startup performs only read-only schema verification and fails with `database schema is not migrated` until Atlas has prepared the database.
- GitHub production deployments now require `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` for the Atlas step while Fly retains its own copies as application runtime secrets.

### Fixed

- Removed application-startup migration/reset responsibilities that could race across rolling Fly Machines against the shared Turso database.
- Local libSQL, process-integration, and Docker/Playwright release gates now exercise the real lifecycle: Atlas prepares the schema, reset removes it, ordinary startup refuses the unmigrated database, Atlas reapplies it, and multiple non-migrating application processes start against the same prepared schema.

## [0.5.6] - 2026-09-08

### Fixed

- Turso startup no longer retains idle libSQL/Hrana streams created under the short-lived ping/migration context; the normal idle pool is enabled only after startup work completes.
- The one-time production reset drains all idle Turso connections before acquiring its pinned reset connection, preventing a closed startup stream from failing the first reset-ledger statement with `stream is closed: driver: bad connection`.
- Closed/bad libSQL connections during the reset are retried up to three times while preserving the existing atomic reset transaction, durable reset ID, and mandatory post-reset Store reopen.
- Added regression coverage for the exact closed-stream error observed in the Fly production crash loop.

## [0.5.5] - 2026-09-07

### Fixed

- Rolling Fly Machines now always close and reopen the Turso history Store after participating in a one-time reset, even when another Machine already applied the reset. This prevents a reset loser from continuing with a connection that migrated before the winner dropped the shared schema.
- Added regression coverage for both the reset-winning process and the reset-losing process to prove each startup reopens against the rebuilt schema before the application continues.

## [0.5.4] - 2026-09-07

### Changed

- Production performs one idempotent clean-slate Turso application-data reset under reset ID `2026-09-07-v0.5.4-clean-slate`, rebuilding Trendinary's schemas from current migrations because the service has no production users and pre-v0.5.3 detector state is not trustworthy.
- The reset is serialized with `BEGIN IMMEDIATE` on one pinned SQL connection so rolling Fly Machines cannot race a partial wipe/rebuild; the reset marker and table/view drops commit atomically.
- `store.NewMemory()` is now production-safe and starts with no trends. Prototype fixtures moved behind explicit `NewDemoMemory()` so an empty quality-gated scan can never fall back to fake trends such as `AT Protocol` or `That Blue Chair`.
- A quiet/empty leaderboard is now valid production output. Deployment smoke validates every published trend instead of requiring Trendinary to manufacture a minimum count.
- The header no longer routes `Search / Ask` to the prototype `AT Protocol` trend; it links back to the live leaderboard until a real search surface exists.

### Fixed

- Jetstream cursor restoration no longer manufactures a fresh `last_event_at` timestamp. Only a real observed event advances stream freshness, and production smoke requires a real Jetstream event less than ten minutes old.
- Added regression coverage for complete database wiping/rebuild, idempotent reset IDs, concurrent reset callers, production memory containing no prototype trends, explicit demo fixtures, and cursor-only versus real-event stream freshness.

## [0.5.3] - 2026-09-06

### Fixed

- Public trend admission now requires at least two non-context observations with either independent publisher corroboration or independent community actors; a single Wikipedia pageview, article, or repository can no longer become a public trend by itself.
- Wikipedia pageviews remain supporting attention evidence but cannot seed a candidate or dominate candidate ordering, and duplicate discovery of the same publisher through multiple channels no longer masquerades as independent corroboration.
- Detection Quality v3 now uses cohesive complete-link event clusters instead of transitive single-link components, preventing bridge headlines from collapsing unrelated events; generic sentence-leading pseudo-entities such as `List` no longer create false named-entity matches.
- Rolling historical evidence must agree with at least two-thirds of the current cluster before affecting public source breadth, explanation, or propagation.
- Production V2 stable-entity resolution now requires strong term agreement for wording changes, quarantines already-exploded entities from fuzzy matching, and bounds active terms and aliases instead of accumulating an ever-growing identity vocabulary.
- Public propagation is rendered from currently validated rolling evidence rather than the entity's unbounded historical propagation table, preventing old bad merges from leaking unrelated source networks into current trend detail.
- Production smoke now fails on an empty leaderboard, Wikipedia-only public trends, one-observation public trends, or exploded alias sets, in addition to the existing stream/scanner health checks.
- Regression coverage includes the observed `Andy Ruiz Jr.` Wikipedia singleton, `List of highest-grossing films` versus `List of Intel codenames`, transitive bridge clustering, same-publisher RSS/GDELT duplication, polluted legacy entities, and rolling-evidence bridge contamination.

## [0.5.2] - 2026-09-06

### Fixed

- Credential-free Jetstream recovery now handles durable cursors that have fallen below Bluesky's bounded live lookback floor. Trendinary records the gap as unreplayable without an archive key, drops the stale resume cursor, and attaches at the current public live tip instead of terminating the stream.
- Added regression coverage for the exact production `live cursor too old` failure and for the cursor-free live-tip subscription used by recovery.

## [0.5.1] - 2026-09-06

### Fixed

- Bluesky Jetstream archive replay now uses `TRENDINARY_JETSTREAM_API_KEY` when configured. When no archive key is present, Trendinary resumes the public live tail from its durable cursor instead of requesting authenticated archive replay and terminating with a 401.
- Production smoke now fails when enabled Jetstream is disconnected, scanner success is more than 15 minutes stale, or a scanner run has remained active for more than 3 minutes; startup gets a bounded retry window before the deployment is declared unhealthy.
- Jetstream regression coverage now verifies environment-key loading plus authenticated archive-replay and credential-free live-resume mode selection without logging bearer material.

## [0.5.0] - 2026-09-05

### Added

- Anonymous server-backed Radar Keys generated from 256 bits of entropy; only the SHA-256 radar identity is persisted server-side.
- Durable Turso-backed follows, preferences, baselines, alert history, and Web Push subscriptions that sync across devices without an account or email address.
- Exact-trend, recurring-topic, and recurring-entity follows, with automatic migration of v0.4 device-local trend follows after a server radar is created.
- Server-side one-minute radar evaluation for lifecycle, resurfacing, acceleration, and independent-source corroboration changes even when every browser is closed.
- Early, balanced, and quiet sensitivity presets plus independent alert-family controls.
- Standards-based AES128GCM Web Push with a durable VAPID P-256 key, service-worker notification delivery, and click-through to the matching trend.
- Cross-device Radar Key copy/import, explicit radar deletion, shared alert inbox, and immediate `Sync now` evaluation.
- Release-gate regression coverage for successful encrypted push payloads, concurrent evaluator deduplication, hundreds-of-radars evaluation, and complete durable radar deletion.

### Changed

- Following is now a server-backed abnormal-attention radar rather than a browser-local foreground monitor; browser polling only synchronizes state while the server owns alert evaluation.
- One browser push endpoint can belong to only one Radar Key at a time, so importing another radar rebinds the endpoint instead of allowing the old radar to continue pushing to that browser.
- The first observation for a followed trend remains baseline-only and never creates a synthetic alert; later material changes are fingerprinted so concurrent Fly Machines cannot duplicate notifications.

### Fixed

- A browser that still has a valid PushManager subscription after its old server radar was deleted now rebinds that subscription automatically when a replacement radar is created.
- Gone Web Push endpoints are disabled after HTTP 404/410 responses instead of being retried indefinitely.
- Radar deletion removes follows, baselines, alerts, and push subscriptions so the anonymous credential has a complete server-side lifecycle.

## [0.4.0] - 2026-09-04

### Added

- Device-local Following radar under `/following` with no account, email address, or server-side user profile required.
- Follow baselines captured at follow time so already-popular trends do not generate fake alerts.
- Edge-triggered alerts for lifecycle transitions, resurfacing, meaningful acceleration, and broader independent-source corroboration.
- Durable device-local alert inbox with unread state, mark-read and clear controls, plus an explicit quiet-state when nothing material changed.
- Optional browser notifications, shown only after explicit permission and while Trendinary is open in a hidden tab.
- Live trend browse-and-follow controls plus query-parameter bootstrap for deep-linking a trend into Following.

### Changed

- Following monitor checks once per minute while the app is open and refreshes immediately when a hidden tab becomes visible.
- Each monitor pass reuses one leaderboard request across all follows, falling back to trend-detail requests only for followed topics that have left the leaderboard.
- v0.4.0 deliberately keeps follows device-local; cross-device sync and true closed-browser Web Push remain separate future capabilities rather than implied behavior.

### Fixed

- Following now falls back to session-memory state when browser storage is unavailable or rejects writes, instead of reloading stale persisted state.
- Follow/Unfollow controls derive from reactive Solid state so their labels update immediately after user actions.

## [0.3.2] - 2026-09-03

### Added

- Per-source reliability telemetry for poll attempts, successes, signals produced, fetch latency, active failures, RSS request counts, and HTTP 304 conditional-cache reuse.
- Seven-day source contribution analytics split by original publisher and discovery channel, including distinct signals, trends touched, first-hit counts, solo-source trends, and average lead time to the first BREAKING snapshot.
- Private `GET /api/v1/admin/sources/analytics` source-intelligence endpoint and expanded `/admin/sources` fleet view for reliability and contribution analysis.
- Durable immutable `first_observed_at` membership timestamps alongside rolling last-observed timestamps so first-discovery and lead-time claims survive repeated evidence refreshes.
- Calibration v1 with deterministic replay threshold search from 0.30 through 0.70, explicit labeling progress, and reviewed recommendations rather than automatic production tuning.
- Calibration safety gates requiring at least 100 distinct human-labeled trends and a persisted replay corpus with at least 50 signals before a cluster-threshold recommendation is produced.

### Changed

- `/admin/quality` prioritizes currently unlabeled live trends so the human calibration corpus can be built efficiently without manufacturing synthetic judgments.
- Source operations now distinguish process-lifetime reliability from durable historical contribution; a healthy source is not automatically treated as a useful source and a high-volume source is not automatically treated as early.
- Calibration recommendations remain advisory. Applying a score or clustering threshold still requires an explicit reviewed code change.

### Fixed

- Source first-hit and lead-to-BREAKING analytics no longer misuse the rolling membership `observed_at` value, which intentionally refreshes while evidence remains active; they now use immutable first-observed membership time.
- Membership schema migration is additive and rolling-deploy safe for existing SQLite/Turso databases.
- Calibration UI safely renders an unavailable threshold while the corpus is still below the recommendation gate.

## [0.3.1] - 2026-09-03

### Added

- Curated public source registry with policy metadata and source-specific polling cadences for dozens of official RSS/API sources, including WIRED, Ars Technica, TechCrunch, ABC News, NIST, NASA/JPL, CISA, AWS, GitHub, Cloudflare, Mozilla, BleepingComputer, Krebs on Security, and developer ecosystems.
- Per-source scheduling with deterministic jitter, exponential backoff, cached last-good evidence, ETag/Last-Modified conditional RSS requests, and runtime source-health status.
- Unified private `/admin/sources` operations dashboard showing public trend sources alongside private editorial discovery sources, including cadence, health, errors, last success, next run, and cached signal counts.
- Rolling 24-hour persisted evidence reload for stable trends so source breadth, explanation evidence, and propagation can retain cross-scan corroboration.
- Durable discovery-channel provenance stored independently from publisher identity, allowing a WIRED article discovered through Hacker News, RSS, or GDELT to remain WIRED evidence while retaining how Trendinary found it.
- Regression coverage for v0.3.0 database upgrades, concurrent rolling-deploy schema migration, HN-to-publisher provenance, Wikipedia over-clustering, and differently-worded cross-publisher event merging.

### Changed

- Event clustering now uses explicit clustering text, headline/entity overlap, and publication proximity while excluding publisher domains and descriptive boilerplate from semantic similarity.
- Source breadth now counts rolling distinct publishers rather than only the current scanner pass; the configured source-universe value remains a corroboration saturation target rather than the number of feed adapters.
- Hacker News and other aggregators now separate discovery channel from original publisher where an outbound article URL exists.
- The scanner continues using cached last-good source evidence when a scheduled upstream refresh fails while still surfacing the upstream warning.
- Wikipedia pageview polling is reduced to a source-appropriate cadence instead of refetching the same daily ranking every global scan.

### Fixed

- Unrelated Wikipedia top pages can no longer collapse into a single high-attention trend because they share boilerplate pageview text.
- Differently-worded reports of the same event from independent publishers are substantially more likely to resolve to one stable trend.
- Public trend detail no longer reports source breadth solely from one in-memory scan when durable corroborating evidence already exists.
- Quality feedback latest-label selection is deterministic when timestamps tie.

## [0.3.0] - 2026-09-02

### Added

- Signal Quality v3 private `/admin/quality` calibration workspace with durable labels for real trends, noise, duplicates, early/late timing, bad clusters, and canonical-name mistakes.
- Exact persisted signal-to-stable-trend memberships so production human labels can replay the observations that actually formed each detected trend.
- Human-label quality reporting for Top-10 and Top-25 precision, precision proxy, false-positive rate, duplicate-cluster rate, early-hit rate, cluster health, naming health, source breadth, lifecycle timing, score separation, and a conservative publication-threshold recommendation.
- Deterministic production replay benchmark that feeds persisted labeled signal memberships back through the entity-aware clusterer and reports pairwise precision/recall.
- Dedicated PEEP score and `GET /api/v1/peep` endpoint for confidence-gated early-signal ranking by velocity, source breadth, community spread, and novelty.
- History-backed finite FOMO briefing through `GET /api/v1/fomo`, defaulting to the strongest seven trends from the last 24 hours.
- Propagation timing deltas that show elapsed time from the first observed source rather than only absolute timestamps.
- Production API smoke assertions for health, stream status, and trends, executed after deploy and from a standalone scheduled/manual workflow.
- Optional read-only `GET /api/v1/integrations/notes` publishing bridge protected by `TRENDINARY_INTEGRATION_TOKEN`, exposing only saved/consumed notes explicitly marked worth sharing.

### Changed

- Trendinary Score advances to model version `0.3`, reducing raw-attention weight and increasing velocity plus independent-source/community breadth so unexpected acceleration matters more than fame.
- `/peep` now consumes the live PEEP API instead of filtering the main leaderboard client-side.
- `/fomo` now consumes persisted trend history instead of static prototype data.
- Trend detail propagation presents source order with observed elapsed-time labels such as `origin`, `+14m`, and `+1h 12m`.
- `GET /api/v1/admin/notes?worth_sharing=true` now honors the requested worth-sharing filter instead of returning every saved/consumed item with a note.

## [0.2.5] - 2026-09-01

### Fixed

- Production Cloudflare deployment no longer creates a legacy Page Rule for `www.trendinary.com`; the apex `trendinary.com` Worker/DNS deployment now avoids the Page Rules permission entirely. A modern `www` redirect can be added separately.

## [0.2.4] - 2026-09-01

### Fixed

- Production SST deployment now prints full provider/deployment logs so Cloudflare failures expose their actionable root cause in GitHub Actions instead of only the generic unexpected-error message.

## [0.2.3] - 2026-09-01

### Fixed

- Production Cloudflare deployment now uses Cloudflare provider `6.15.0`, satisfying the minimum provider version required by SST 4.17.1.

## [0.2.2] - 2026-09-01

### Fixed

- Production Fly deploy now invokes `flyctl deploy`, matching the binary installed by the GitHub Actions Fly setup step.

## [0.2.1] - 2026-09-01

### Fixed

- Production deploy preflight now recognizes staged Fly secrets correctly before the first Machine exists.

## [0.2.0] - 2026-09-01

### Added

- Private authenticated editorial workspace under `/admin` with inbox, queue, notes, sources, trash, and newsletter issue assembly.
- TechURLs private discovery adapter with conservative URL normalization/deduplication, ingestion-run accounting, enrichment, deterministic editorial scoring, and optional raw HTML archival.
- RSS/Atom, Wikipedia pageview, GDELT DOC, and optional YouTube public discovery adapters.
- Optional NewsData discovery with durable database-backed daily quota pacing, pagination, replay persistence, and corroboration-only trend behavior.
- Detection Quality v2 entity-aware second-stage clustering, source/author flood controls, repeated-text suppression, stronger candidate gating, historical calibration, and replay evaluation.
- Turso/libSQL production persistence for trend history, stable identities, Jetstream cursors, quota state, and private editorial data while retaining local SQLite for development/tests.
- CI branch-flow guard enforcing normal `feature/* -> dev -> main` release direction.

### Changed

- Fly.io is now a stateless origin; the production database no longer depends on a Fly volume or `/data/trendinary.db`.
- Production startup fails closed unless `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` are configured.
- Production database recovery uses Turso point-in-time recovery/export tooling; the former Fly-volume-to-R2 SQLite backup workflow was removed. R2 remains the raw/provenance archive.
- Public discovery no longer depends on direct Reddit API access. Reddit-adjacent material may arrive indirectly through TechURLs/original publisher links; no Reddit HTML-scraping bypass is added.
- Default source-universe calibration expands for the broader v0.2 discovery set.

## [0.1.0] - 2026-09-01

### Added

- Go 1.27 API server under `/api/v1/*` with the Solid 2/Vite application embedded and served from `/`.
- Fly.io production origin plus SST-managed Cloudflare Worker, DNS, KV, and R2 infrastructure.
- SQLite history, versioned Trendinary Score, lifecycle detection, historical baselines, and momentum history.
- Hacker News and Bluesky/AT Protocol signal ingestion, including Jetstream v2 continuous discovery and durable cursor replay.
- Candidate gating, bounded high-volume stream processing, and source-independent normalized signals.
- GitHub emerging-repository discovery and OAuth-only Reddit discovery.
- Stable trend entities, aliases, canonical terms, and persistent public slugs.
- Selective Bluesky engagement hydration and cached DID/handle/profile resolution.
- Durable propagation/origin tracking across source networks.
- Evidence-backed Source Lens / Perspective Mix metadata that keeps political leaning separate from reliability and truth claims.
- Grounded deterministic WTF, LORE, evidence, and Ask Trendinary responses.
- Live trend intelligence UI with score history, evidence, propagation, source perspective, top voices, and Ask Trendinary.
- Stream/scanner observability through `/api/v1/health/streams`.
- CI verification for TypeScript, Go tests, Vite/server builds, module cleanliness, and the Fly-target Docker image.
