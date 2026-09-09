import { Resource } from "sst/resource";
import {
  RATE_CLASS,
  applyCorsHeaders,
  applySecurityHeaders,
  classifyRequest,
  clientRateKey,
  isAllowedBrowserOrigin,
  normalizeForwardingHeaders,
  retryAfterSeconds,
} from "./edge-policy.mjs";

const ORIGIN = "https://trendinary.fly.dev";

function finalize(response: Response, request: Request): Response {
  const result = new Response(response.body, response);
  result.headers.set("x-trendinary-edge", "cloudflare");
  applySecurityHeaders(result.headers, request.url);
  applyCorsHeaders(result.headers, request.url, request.headers.get("origin"));
  return result;
}

function rateLimited(request: Request, rateClass: string): Response {
  const headers = new Headers({
    "cache-control": "no-store",
    "content-type": "application/json; charset=utf-8",
    "retry-after": String(retryAfterSeconds(rateClass)),
  });
  const response = new Response(
    JSON.stringify({
      error: "rate_limited",
      class: rateClass,
      retry_after: retryAfterSeconds(rateClass),
    }),
    { status: 429, headers },
  );
  return finalize(response, request);
}

async function enforceRateLimit(request: Request, rateClass: string | null): Promise<Response | null> {
  if (!rateClass) return null;
  const key = await clientRateKey(request, rateClass);

  let outcome: { success: boolean };
  switch (rateClass) {
    case RATE_CLASS.AUTH:
      outcome = await Resource.TrendinaryAuthRateLimit.limit({ key });
      break;
    case RATE_CLASS.MUTATION:
      outcome = await Resource.TrendinaryMutationRateLimit.limit({ key });
      break;
    case RATE_CLASS.EXPENSIVE:
      outcome = await Resource.TrendinaryExpensiveRateLimit.limit({ key });
      break;
    default:
      outcome = await Resource.TrendinaryCheapReadRateLimit.limit({ key });
      break;
  }

  return outcome.success ? null : rateLimited(request, rateClass);
}

export default {
  async fetch(request: Request): Promise<Response> {
    const incoming = new URL(request.url);
    const originHeader = request.headers.get("origin");

    if (request.method === "OPTIONS" && incoming.pathname.startsWith("/api/")) {
      if (!isAllowedBrowserOrigin(request.url, originHeader)) {
        return finalize(
          new Response(JSON.stringify({ error: "cross_origin_request_denied" }), {
            status: 403,
            headers: {
              "cache-control": "no-store",
              "content-type": "application/json; charset=utf-8",
            },
          }),
          request,
        );
      }
      return finalize(new Response(null, { status: 204 }), request);
    }

    const rateClass = classifyRequest(request.method, incoming.pathname);
    const limited = await enforceRateLimit(request, rateClass);
    if (limited) return limited;

    const origin = new URL(ORIGIN);
    origin.pathname = incoming.pathname;
    origin.search = incoming.search;

    const headers = new Headers(request.headers);
    normalizeForwardingHeaders(headers, request.url, request.headers.get("cf-connecting-ip"));

    const upstreamRequest = new Request(origin.toString(), {
      method: request.method,
      headers,
      body: request.method === "GET" || request.method === "HEAD" ? undefined : request.body,
      redirect: "manual",
    });

    const upstream = await fetch(upstreamRequest);
    const response = finalize(upstream, request);

    // Hashed Vite assets are immutable. API and HTML caching remain controlled
    // by the Go origin until the data pipeline has explicit freshness rules.
    if (incoming.pathname.startsWith("/assets/")) {
      response.headers.set("cache-control", "public, max-age=31536000, immutable");
    }

    return response;
  },
};
