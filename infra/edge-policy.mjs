export const RATE_CLASS = Object.freeze({
  CHEAP: "cheap",
  EXPENSIVE: "expensive",
  MUTATION: "mutation",
  AUTH: "auth",
});

const API_PREFIX = "/api/v1/";
const PRIVATE_PREFIXES = ["/api/v1/admin/", "/api/v1/integrations/", "/admin/", "/admin"];
const EXPENSIVE_PREFIXES = [
  "/api/v1/signals/",
  "/api/v1/sources/",
];

export function classifyRequest(method, pathname) {
  const upperMethod = method.toUpperCase();

  if (PRIVATE_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(prefix))) {
    return RATE_CLASS.AUTH;
  }

  if (!pathname.startsWith(API_PREFIX) && pathname !== "/api/v1") {
    return null;
  }

  if (upperMethod !== "GET" && upperMethod !== "HEAD" && upperMethod !== "OPTIONS") {
    if (pathname.startsWith("/api/v1/following/")) return RATE_CLASS.MUTATION;
    if (/^\/api\/v1\/trends\/[^/]+\/ask$/.test(pathname)) return RATE_CLASS.EXPENSIVE;
    return RATE_CLASS.MUTATION;
  }

  if (
    EXPENSIVE_PREFIXES.some((prefix) => pathname.startsWith(prefix)) ||
    pathname === "/api/v1/peep" ||
    pathname === "/api/v1/fomo" ||
    /^\/api\/v1\/trends\/[^/]+\/(history|propagation|explanation)$/.test(pathname) ||
    pathname === "/api/v1/following/quality" ||
    pathname === "/api/v1/following/briefing"
  ) {
    return RATE_CLASS.EXPENSIVE;
  }

  return RATE_CLASS.CHEAP;
}

export function retryAfterSeconds(rateClass) {
  return rateClass ? 60 : 0;
}

export function isAllowedBrowserOrigin(requestUrl, origin) {
  if (!origin) return false;
  const url = new URL(requestUrl);
  return origin === `${url.protocol}//${url.host}`;
}

export function applyCorsHeaders(headers, requestUrl, origin) {
  headers.append("Vary", "Origin");
  if (!isAllowedBrowserOrigin(requestUrl, origin)) return;
  headers.set("Access-Control-Allow-Origin", origin);
  headers.set("Access-Control-Allow-Credentials", "true");
  headers.set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS");
  headers.set("Access-Control-Allow-Headers", "Authorization, Content-Type");
  headers.set("Access-Control-Max-Age", "600");
}

export function applySecurityHeaders(headers, requestUrl) {
  const url = new URL(requestUrl);
  headers.set("X-Content-Type-Options", "nosniff");
  headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  headers.set("Permissions-Policy", "camera=(), microphone=(), geolocation=()");
  headers.set("X-Frame-Options", "DENY");
  headers.set("Cross-Origin-Opener-Policy", "same-origin");
  headers.set("Cross-Origin-Resource-Policy", "same-site");
  headers.set(
    "Content-Security-Policy",
    "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' https: data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; upgrade-insecure-requests",
  );
  if (url.protocol === "https:" && url.hostname === "trendinary.com") {
    headers.set("Strict-Transport-Security", "max-age=31536000; includeSubDomains");
  }
}

export function normalizeForwardingHeaders(headers, requestUrl, clientIp) {
  const url = new URL(requestUrl);
  headers.delete("forwarded");
  headers.delete("x-forwarded-for");
  headers.delete("x-forwarded-host");
  headers.delete("x-forwarded-proto");
  headers.delete("x-real-ip");
  headers.delete("x-trendinary-edge");

  if (clientIp) headers.set("x-forwarded-for", clientIp);
  headers.set("x-forwarded-host", url.host);
  headers.set("x-forwarded-proto", url.protocol.replace(":", ""));
  headers.set("x-trendinary-edge", "cloudflare");
}

export async function clientRateKey(request, rateClass) {
  const authorization = request.headers.get("authorization");
  const clientIp = request.headers.get("cf-connecting-ip") || "unknown";
  const source = authorization ? `authorization:${authorization}` : `ip:${clientIp}`;
  const bytes = new TextEncoder().encode(`${rateClass}:${source}`);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  const hash = Array.from(new Uint8Array(digest), (value) => value.toString(16).padStart(2, "0")).join("");
  return `${rateClass}:${hash}`;
}
