# Trendinary Score

Trendinary is not trying to rank the most famous things on the internet. It is trying to rank **unexpected attention**.

A topic that normally receives millions of mentions should not automatically beat a tiny project that suddenly crosses several communities at 20x its normal rate.

## Version 0.1

The public score is 0–100 and the methodology is versioned.

Current normalized inputs:

| Metric | Weight | What it asks |
| --- | ---: | --- |
| Velocity | 30% | How unusually fast is attention changing? |
| Attention | 20% | How much attention exists after baseline normalization? |
| Source breadth | 18% | How many independent source classes are carrying it? |
| Community breadth | 12% | Is it escaping its original community? |
| Novelty | 12% | How unusual is this subject/event relative to recent history? |
| Confidence | 8% | How confident are we that the observed movement is genuine signal? |

The weights sum to 100%.

Velocity intentionally outweighs raw attention. **Popularity is late.**

## Baseline normalization

The engine expects each metric as a 0–1 normalized value. The scoring function itself does not decide what "high velocity" means for every topic.

That belongs to the historical metrics pipeline. Examples:

- 5,000 mentions of Taylor Swift may be normal.
- 5,000 mentions of an unknown compiler project may be extraordinary.
- A subject appearing on one subreddit may be noise.
- The same subject independently accelerating on Hacker News, Bluesky, YouTube and news sites is much stronger evidence.

This is why persistent snapshots and baselines are required before the live score can replace the seeded prototype scores.

## Lifecycle

Trendinary converts movement into human-readable lifecycle states:

```text
EMERGING -> RISING -> BREAKING -> PEAKING -> COOLING
                  ^                         |
                  |------ RESURFACING <-----|
```

The current thresholds are deterministic and test-covered, but they are calibration defaults—not eternal product truth.

## Clustering

Raw source events are not trends.

Version 0.1 begins with a transparent lexical clusterer using meaningful-token overlap. It is deliberately understandable and replayable. The cluster API is designed so we can add:

- named-entity extraction
- aliases and canonical entities
- URL/story identity
- semantic embeddings
- source-specific relationship signals

without making downstream scoring code source-specific.

## What the score must never become

The Trendinary Score is **not**:

- a truth score
- a political score
- a reliability score
- a recommendation to agree with the topic
- an engagement-maximization score

It describes the shape of attention.

Source Lens political leaning and future reliability assessments remain separate metadata dimensions.

## Public transparency

`GET /api/v1/methodology/score` exposes the current score version, weights, range, principle and calibration warning.

When the score changes materially, bump the methodology version and keep historical snapshots associated with the version that produced them. This lets Trendinary explain why a past ranking looked the way it did instead of rewriting history every time the algorithm changes.
