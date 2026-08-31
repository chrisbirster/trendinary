const ORIGIN = "https://trendinary.fly.dev";

export default {
  async fetch(request: Request): Promise<Response> {
    const incoming = new URL(request.url);
    const origin = new URL(ORIGIN);
    origin.pathname = incoming.pathname;
    origin.search = incoming.search;

    const headers = new Headers(request.headers);
    headers.set("x-forwarded-host", incoming.host);
    headers.set("x-trendinary-edge", "cloudflare");

    const upstreamRequest = new Request(origin.toString(), {
      method: request.method,
      headers,
      body: request.method === "GET" || request.method === "HEAD" ? undefined : request.body,
      redirect: "manual",
    });

    const upstream = await fetch(upstreamRequest);
    const response = new Response(upstream.body, upstream);
    response.headers.set("x-trendinary-edge", "cloudflare");

    // Hashed Vite assets are immutable. API and HTML caching remain controlled
    // by the Go origin until the data pipeline has explicit freshness rules.
    if (incoming.pathname.startsWith("/assets/")) {
      response.headers.set("cache-control", "public, max-age=31536000, immutable");
    }

    return response;
  },
};
