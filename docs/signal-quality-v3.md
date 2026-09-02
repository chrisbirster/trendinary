# Signal Quality v3

Trendinary v0.3.0 changes the product question from **“can we collect and rank signals?”** to **“can we measure whether the detector is actually good?”**

The core invariant remains:

> AI can explain a trend, but it does not create the trend. Detection is grounded in observed public signals, durable history, and explicit scoring rules.

## 1. Trendinary Score v3

The public 0–100 score rewards unexpected movement rather than fame.

| Input | Weight |
| --- | ---: |
| Attention | 14% |
| Velocity | 34% |
| Source breadth | 19% |
| Community breadth | 13% |
| Novelty | 12% |
| Confidence | 8% |

All inputs are normalized to 0–1 before scoring. Historical baselines and calibration remain outside the weighted function so the formula itself stays deterministic and versioned.

## 2. PEEP score

PEEP is deliberately separate from the main score. The main score answers “how important is this trend right now?” PEEP answers “how strangely early is this moving?”

The v3 PEEP score is:

```text
(
  52% velocity
  + 20% source breadth
  + 13% community breadth
  + 15% novelty
)
× confidence gate
```

The confidence gate ranges from 0.70 to 1.00. This prevents a tiny, poorly-supported spike from receiving a perfect PEEP score just because its estimated velocity is high.

`GET /api/v1/peep` only includes `EMERGING` and `RISING` trends and sorts them by PEEP score rather than their main leaderboard rank.

Public PEEP cards expose the normalized reasons for watching the signal, such as:

```text
velocity 88% · source breadth 43% · novelty 79%
```

Quiet is valid output. The endpoint is allowed to return an empty list.

## 3. Durable human labels

The private `/admin/quality` workspace records one of seven labels against a stable trend identity:

- `real-trend`
- `noise`
- `duplicate`
- `interesting-too-early`
- `detected-too-late`
- `bad-cluster`
- `wrong-canonical-name`

Labels include the trend ID/slug/name, the score and lifecycle at review time, an optional note, and an immutable timestamp. Historical labels are retained; quality summaries use the latest label per stable trend.

Human labels are private editorial/calibration data and never enter public APIs directly.

## 4. Exact signal membership

Each scanner pass persists the source-independent signals that resolved into each stable trend entity:

```text
trend_signal_memberships
  trend_key
  signal_id
  observed_at
```

This is important because evaluation must replay what the detector actually saw. A human-written summary generated later is not an acceptable substitute for the original evidence set.

## 5. Quality report

`GET /api/v1/admin/quality/report` derives the following from latest human labels:

- **Top-10 precision** — precision among the ten highest-scoring precision-eligible labeled candidates
- **Top-25 precision** — the same measurement over the top twenty-five
- **precision proxy** — usable trends divided by usable trends plus noise/duplicate/bad-cluster labels
- **false-positive rate** — the complement of the precision proxy over the same eligible labels
- **duplicate-cluster rate** — duplicate outcomes divided by all latest stable-trend labels
- **early-hit rate** — good early catches compared with “detected too late” outcomes
- **cluster health** — penalizes duplicate and bad-cluster labels
- **naming health** — penalizes wrong canonical-name labels
- positive mean score
- noise mean score
- a conservative recommended minimum public score

When persisted trend snapshots exist for positively labeled trends, the report also measures:

- average latest normalized source breadth
- average latest distinct source count
- average lead time from Trendinary's first observation to the first `BREAKING` snapshot
- average `EMERGING → RISING` elapsed time
- average `RISING → BREAKING` elapsed time

Timing measurements are based on observed lifecycle snapshots. They describe Trendinary's detection progression; they do not claim causal propagation between sources.

The threshold recommendation is diagnostic. It is not automatically applied to production scoring; changing a published detection gate remains a versioned code decision.

## 6. Deterministic replay

`GET /api/v1/admin/quality/replay?threshold=0.56` joins:

```text
latest human label
      ↓
stable trend ID
      ↓
trend_signal_memberships
      ↓
original normalized signals
      ↓
ClusterSignalsV2
```

Positive timing/quality labels share their stable trend group. Noise and structural-error labels receive unique negative groups. The existing deterministic evaluator reports pairwise cluster precision and recall.

This lets clustering changes be compared against a growing real production corpus without requiring a model call or online service.

## 7. Propagation timing

Durable propagation already records first/last source observation, signal count, and engagement. V3 annotates the ordered source path with elapsed time from the first source:

```text
Bluesky       origin
Hacker News   +14m
Tech press    +52m
Wikipedia     +2h 08m
```

These are observed timing deltas. Trendinary does not claim causal influence merely from ordering.

## 8. FOMO

`GET /api/v1/fomo` creates a finite catch-up briefing from persisted trend snapshots.

Defaults:

- previous 24 hours
- maximum 7 items
- rank by historical peak score, then velocity

The response includes peak score, max velocity, source breadth, novelty, first/last observed timestamps, and a deterministic explanation for why the trend made the briefing.

FOMO is intentionally not an infinite feed.

## 9. What v3 does not do

Signal Quality v3 does not:

- automatically train an opaque ML model from human labels
- auto-change score weights or publication thresholds
- infer truth from source popularity
- treat political leaning as reliability
- fabricate propagation causality
- pad PEEP or FOMO with low-quality content when the detector is quiet

The goal is a transparent feedback loop: observe → detect → label → replay → measure → intentionally change the next version.
