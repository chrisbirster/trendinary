# Source Lens: political leaning and reliability

Trendinary should help people understand **who is telling the story**, but it must not turn a complex media ecosystem into unsupported labels.

## Product rule

A source-level political leaning label is metadata from a named methodology, not a fact Trendinary invents from reputation.

When Trendinary displays a leaning, the UI and API should preserve:

- rating provider
- categorical label
- numeric score when the provider supplies one
- confidence
- scope (web, TV, opinion, specific author, etc.)
- rating date
- methodology link
- source-specific rating link

If that evidence is unavailable, the source is `not-rated`.

## Initial vocabulary

Trendinary normalizes provider classifications into:

- Left
- Lean Left
- Center
- Lean Right
- Right
- Mixed
- Not Rated

The original provider data must remain attached so normalization never hides nuance.

## Do not conflate these concepts

### Source political leaning
A historical assessment of patterns in a publisher's coverage.

### Article stance
The position or framing expressed by one specific article.

### Reliability
How consistently a source or article supports factual claims. This is a different dimension from political leaning.

### Popularity
How much attention the source receives. Popularity says nothing about leaning or reliability.

### Truth of a claim
A claim needs evidence. A source being Left, Center, or Right does not make an individual claim true or false.

## Providers

The initial implementation supports provider-attributed assessments and seeds one AllSides example to prove the data contract. AllSides uses multiple review methods and publishes confidence levels. Ad Fontes is a strong candidate for a second provider because it separately scores political bias and reliability and uses politically balanced analyst panels.

Provider data must only be integrated in ways permitted by its license/API terms. Trendinary should not scrape or republish proprietary rating datasets without permission.

## Example

For Fox News Digital, the seed data records an AllSides `Right` assessment, its numeric rating and confidence, while preserving the important scope limitation that the assessment refers to online news coverage and not automatically to Fox cable TV, radio, or separately rated opinion content.

This is the behavior we want everywhere: **specific, attributed, scoped, and revisable**.

## Future Source Lens UI

A source card should eventually look roughly like:

```text
FOX NEWS DIGITAL
Lean: RIGHT
Provider: AllSides
Confidence: Medium
Scope: foxnews.com news coverage

Reliability: [separate provider/metric]

View methodology  View rating
```

For a trend cluster, Trendinary can then show perspective breadth without pretending that a three-column Left/Center/Right split captures all viewpoints:

```text
SOURCE MIX
Left / Lean Left      3
Center                5
Right / Lean Right    4
Not rated             7
```

The goal is not to tell users what to believe. The goal is to make the information environment visible.
