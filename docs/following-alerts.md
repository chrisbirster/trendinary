# Following and alerts

Trendinary v0.5.0 turns Following into a **server-backed abnormal-activity radar** that keeps evaluating when every browser is closed.

## Privacy and cross-device identity

Following still does not require an account, email address, or server-side user profile. The identity boundary is an anonymous **Radar Key** generated from 32 random bytes.

- The browser receives and stores the Radar Key.
- The server derives a SHA-256 radar ID from the decoded key and stores only that one-way identity.
- The original Radar Key cannot be recovered from the Turso database.
- Copying/importing the same Radar Key on another device gives that device access to the same follows, preferences, and alert inbox.
- Anyone who obtains the Radar Key can control that radar, so it must be treated like a password.
- `DELETE /api/v1/following/radar` deletes the radar's follows, baselines, alerts, and push subscriptions.

A fresh v0.5 browser automatically migrates any v0.4 `trendinary.following.v1` exact-trend follows into the new server radar. The old local state is retained until migration succeeds so a temporary outage cannot destroy the user's only copy.

## Durable state

Turso/libSQL stores:

- radar preferences,
- exact trend follows,
- recurring topic and entity follows,
- per-follow/per-trend baselines,
- deduplicated alert history,
- Web Push subscriptions,
- the server's VAPID private key.

The VAPID private key is generated once inside Trendinary and persisted in Turso. It is not a new Fly secret and is never exposed by the public API. Only the corresponding public key is returned to browsers that subscribe to Web Push.

## Follow kinds

### Trend

An exact trend follow tracks one stable trend slug/identity. Its first matched observation establishes the baseline.

### Entity

An entity follow matches current or future stable trends by trend name and aliases. This is intended for named things such as `OpenAI`, `Artemis`, or `Audacity`.

### Topic

A topic follow is broader. It matches trend category, name, reason, and aliases. This lets a subject such as `cybersecurity` remain followed even after today's individual trend cluster cools.

## What creates an alert

The first matched observation never generates an alert. It establishes the baseline. Later observations are compared against that durable baseline.

Alert families are independently configurable:

- **Lifecycle:** a trend advances into RISING, BREAKING, or PEAKING.
- **Resurfacing:** a cooled subject returns as RESURFACING.
- **Acceleration:** normalized velocity or Trendinary Score jumps meaningfully.
- **Corroboration:** evidence expands across additional independent publisher identities/source breadth.

The three sensitivity presets change the material-change thresholds:

| Preset | Velocity floor | Velocity delta | Score delta | Publisher delta | Breadth delta |
| --- | ---: | ---: | ---: | ---: | ---: |
| Early | 0.45 | 0.12 | 10 | 1 | 0.15 |
| Balanced | 0.55 | 0.20 | 15 | 2 | 0.25 |
| Quiet | 0.70 | 0.25 | 20 | 3 | 0.35 |

Alerts are persisted with deterministic fingerprints. A unique insert is the concurrency boundary: if multiple Fly Machines evaluate the same radar, only the process that successfully inserts a new alert may attempt push delivery.

## Evaluation cadence

Production evaluates every server-backed radar once per minute against the current published trend snapshot. Browser polling is no longer the alert engine; it only synchronizes the shared inbox and controls.

`POST /api/v1/following/check` performs an immediate evaluation for one authenticated radar. It is useful for the UI's **Sync now** action without scanning every user's radar.

## Web Push

If the user explicitly grants notification permission, Trendinary registers `/sw.js` and creates a standards-based Web Push subscription.

The server implements:

- P-256 ECDH,
- the Web Push authentication-secret HKDF step,
- `aes128gcm` content encryption,
- VAPID ES256 authorization,
- short push TTL,
- automatic disabling of push endpoints that return HTTP 404 or 410.

One browser push endpoint belongs to one Radar Key at a time. Importing a different Radar Key on a device rebinds the existing browser subscription so the old radar cannot continue pushing to that device.

The service worker displays the notification and opens/focuses the matching Trendinary trend when clicked.

## HTTP API

Creating a radar is the only unauthenticated radar write:

- `POST /api/v1/following/radar`

All radar-private requests use:

```text
Authorization: Bearer <Radar Key>
```

Private endpoints use `Cache-Control: no-store`.

The v0.5 API includes:

- `GET /api/v1/following/state`
- `POST /api/v1/following/check`
- `DELETE /api/v1/following/radar`
- `POST /api/v1/following/follows`
- `DELETE /api/v1/following/follows/{id}`
- `PATCH /api/v1/following/preferences`
- `POST /api/v1/following/alerts/read`
- `DELETE /api/v1/following/alerts`
- `GET /api/v1/following/push/public-key`
- `PUT /api/v1/following/push/subscription`
- `DELETE /api/v1/following/push/subscription`

## Quiet is valid output

Following is not an engagement feed. If none of the material-change rules fire, the correct result is silence. Server-backed operation does not change that product rule.
