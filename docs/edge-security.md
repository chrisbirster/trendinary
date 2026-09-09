# Edge security policy

`https://trendinary.com` is fronted by the Cloudflare Worker in `infra/edge.ts`. The edge is responsible for request classification, abuse throttling, forwarding-header normalization, browser CORS policy, and production browser security headers before proxying to the Fly origin.

## Rate-limit classes

Trendinary uses Cloudflare native Rate Limit bindings. Counters are Cloudflare-managed; they are not in-memory JavaScript counters inside a Worker isolate.

| Class | Limit | Examples |
| --- | ---: | --- |
| cheap public reads | 600/min/client | health, readiness, trend list/detail, methodology |
| expensive/live reads | 90/min/client | live HN/Bluesky signals, source lookup, history/propagation/explanation, peep/fomo |
| public state changes | 60/min/client | Following/radar/follow/push/feedback mutations |
| auth-sensitive | 20/min/client | `/admin/*`, `/api/v1/admin/*`, `/api/v1/integrations/*` |

Anonymous traffic is keyed by a SHA-256 digest of the Cloudflare-provided connecting IP plus rate class. Requests carrying `Authorization` are keyed by a SHA-256 digest of that credential plus class so the raw credential is never used as a counter key. IP-based anonymous throttling is necessarily imperfect for shared/mobile NATs, so limits are intentionally generous enough for normal interactive use and production smoke.

Cloudflare's binding is designed for abuse limiting rather than exact accounting: counters are permissive/eventually consistent and scoped to a Cloudflare location. Do not use these counters for billing, quotas that require exact global accounting, or security decisions that require a strict single global counter.

A rejected API request returns:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 60
Cache-Control: no-store
```

with a stable `rate_limited` error payload.

## Route classification

`infra/edge-policy.mjs` owns classification. Authentication-sensitive paths take precedence over method-based mutation classification. Non-API assets are not rate-limited by these API namespaces.

`scripts/test-edge-policy.mjs` is the regression contract for route classes, CORS, security headers, and forwarded-header normalization.

## Browser security headers

The edge sets:

- `Strict-Transport-Security: max-age=31536000; includeSubDomains` on `https://trendinary.com`;
- CSP with `frame-ancestors 'none'`, `object-src 'none'`, same-origin script/style/connect defaults, and HTTPS upgrade;
- `X-Frame-Options: DENY` as a legacy framing fallback;
- `X-Content-Type-Options: nosniff`;
- `Referrer-Policy: strict-origin-when-cross-origin`;
- restrictive camera/microphone/geolocation `Permissions-Policy`;
- `Cross-Origin-Opener-Policy: same-origin`;
- `Cross-Origin-Resource-Policy: same-site`.

The CSP is intentionally compatible with the current Vite/Solid build, which serves scripts/styles as same-origin compiled assets. Any future third-party script, font, WebSocket, or API dependency must update and review CSP explicitly.

## CORS

The browser API policy is same-origin only. An `Origin` is accepted only when it exactly matches the origin of the incoming Trendinary URL. Allowed preflight methods are GET, HEAD, POST, PUT, PATCH, DELETE and OPTIONS; allowed request headers are `Authorization` and `Content-Type`. Cross-origin preflights are rejected.

Server-to-server clients are not CORS clients and continue to authenticate normally.

## Forwarding headers

Before proxying to Fly, the edge removes client-provided `Forwarded`, `X-Forwarded-*`, `X-Real-IP`, and `X-Trendinary-Edge` values. It then reconstructs:

- `X-Forwarded-For` from Cloudflare's `CF-Connecting-IP`;
- `X-Forwarded-Host` from the actual incoming URL;
- `X-Forwarded-Proto` from the actual incoming scheme;
- `X-Trendinary-Edge: cloudflare` as an observability marker.

The marker is not an authentication secret and must never be used by the origin as proof that a request is trusted.

## Production regression smoke

The production smoke checks the edge marker, HSTS, CSP framing/object restrictions, `nosniff`, and framing fallback before checking API health. This means a Cloudflare deployment that accidentally drops the browser-security boundary cannot pass the normal production deployment gate.

## Origin note

The Fly hostname remains a separately reachable origin unless infrastructure policy changes it. Authentication remains enforced by the Go origin, not by trusting edge headers. Cloudflare rate limiting protects the canonical public hostname; do not treat it as a replacement for application authorization or exact global quota enforcement.
