# Following and alerts

Trendinary v0.4.0 starts Following as a **device-local abnormal-activity radar**.

## Privacy model

The first version does not require an account, email address, or server-side user profile. Follows, baselines, and the alert inbox are stored in the browser under `trendinary.following.v1`.

This means follows do not sync across devices yet. That is deliberate: cross-device identity should be added only when it provides enough value to justify an account boundary.

## What creates an alert

Following compares the current trend against the last baseline observed by this browser. The initial baseline is captured when the user follows a trend, so already-existing popularity does not generate a fake notification.

An alert may be created when:

- **Lifecycle:** a trend advances into RISING, BREAKING, or PEAKING.
- **Resurfacing:** a trend enters RESURFACING after previously being in another lifecycle state.
- **Acceleration:** normalized velocity increases by at least 0.20 while reaching at least 0.55, or the Trendinary Score jumps by at least 15 points between observations.
- **Corroboration:** evidence expands by at least two independent publisher/source identities, or normalized source breadth rises by at least 0.25 while at least two identities are present.

Alerts are edge-triggered from baseline changes and also use fingerprints to suppress repeated notices for the same observed condition.

## Polling and browser notifications

While Trendinary is open, the Following monitor checks once per minute and immediately checks again when a hidden tab becomes visible. The user can also choose **Check now**.

A monitor pass fetches the live leaderboard once and reuses those results across every followed trend. It falls back to an individual trend-detail request only for a followed trend that is no longer present in the leaderboard, avoiding one network request per follow during normal operation.

The in-app alert inbox is the durable record for this version. If the user explicitly grants the browser Notification permission, Trendinary may show a browser notification when the page is open but hidden.

v0.4.0 does **not** claim closed-browser push delivery. Reliable closed-browser delivery requires a Web Push/service-worker subscription or another delivery channel and should be implemented as a separate capability rather than implied by foreground polling.

## Quiet is valid output

Following is not an engagement feed. If none of the material-change rules fire, the correct result is silence. The UI explicitly treats a quiet radar as healthy output.
