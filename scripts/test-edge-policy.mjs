import assert from "node:assert/strict";
import {
  RATE_CLASS,
  applyCorsHeaders,
  applySecurityHeaders,
  classifyRequest,
  isAllowedBrowserOrigin,
  normalizeForwardingHeaders,
  retryAfterSeconds,
} from "../infra/edge-policy.mjs";

const cases = [
  ["GET", "/api/v1/healthz", RATE_CLASS.CHEAP],
  ["GET", "/api/v1/trends", RATE_CLASS.CHEAP],
  ["GET", "/api/v1/signals/bluesky", RATE_CLASS.EXPENSIVE],
  ["GET", "/api/v1/trends/example/history", RATE_CLASS.EXPENSIVE],
  ["POST", "/api/v1/trends/example/ask", RATE_CLASS.EXPENSIVE],
  ["POST", "/api/v1/following/follows", RATE_CLASS.MUTATION],
  ["DELETE", "/api/v1/following/alerts", RATE_CLASS.MUTATION],
  ["GET", "/api/v1/admin/queue", RATE_CLASS.AUTH],
  ["GET", "/api/v1/integrations/notes", RATE_CLASS.AUTH],
  ["GET", "/admin", RATE_CLASS.AUTH],
  ["GET", "/assets/index-hash.js", null],
];

for (const [method, path, expected] of cases) {
  assert.equal(classifyRequest(method, path), expected, `${method} ${path}`);
}
assert.equal(retryAfterSeconds(RATE_CLASS.CHEAP), 60);

const security = new Headers();
applySecurityHeaders(security, "https://trendinary.com/");
assert.equal(security.get("strict-transport-security"), "max-age=31536000; includeSubDomains");
assert.match(security.get("content-security-policy") || "", /frame-ancestors 'none'/);
assert.match(security.get("content-security-policy") || "", /object-src 'none'/);
assert.equal(security.get("x-content-type-options"), "nosniff");
assert.equal(security.get("x-frame-options"), "DENY");

const previewSecurity = new Headers();
applySecurityHeaders(previewSecurity, "https://preview.example.workers.dev/");
assert.equal(previewSecurity.has("strict-transport-security"), false);

assert.equal(isAllowedBrowserOrigin("https://trendinary.com/api/v1/trends", "https://trendinary.com"), true);
assert.equal(isAllowedBrowserOrigin("https://trendinary.com/api/v1/trends", "https://evil.example"), false);

const cors = new Headers();
applyCorsHeaders(cors, "https://trendinary.com/api/v1/trends", "https://trendinary.com");
assert.equal(cors.get("access-control-allow-origin"), "https://trendinary.com");
assert.equal(cors.get("access-control-allow-credentials"), "true");

const deniedCors = new Headers();
applyCorsHeaders(deniedCors, "https://trendinary.com/api/v1/trends", "https://evil.example");
assert.equal(deniedCors.has("access-control-allow-origin"), false);

const forwarded = new Headers({
  forwarded: "for=attacker",
  "x-forwarded-for": "203.0.113.200",
  "x-forwarded-host": "attacker.example",
  "x-forwarded-proto": "http",
  "x-real-ip": "203.0.113.201",
  "x-trendinary-edge": "spoofed",
});
normalizeForwardingHeaders(forwarded, "https://trendinary.com/api/v1/trends", "198.51.100.8");
assert.equal(forwarded.has("forwarded"), false);
assert.equal(forwarded.has("x-real-ip"), false);
assert.equal(forwarded.get("x-forwarded-for"), "198.51.100.8");
assert.equal(forwarded.get("x-forwarded-host"), "trendinary.com");
assert.equal(forwarded.get("x-forwarded-proto"), "https");
assert.equal(forwarded.get("x-trendinary-edge"), "cloudflare");

console.log("edge policy: route classes, CORS, forwarding normalization and browser security headers verified");
