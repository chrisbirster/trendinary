/// <reference path="./.sst/platform/config.d.ts" />

export default $config({
  app(input) {
    return {
      name: "trendinary",
      home: "cloudflare",
      removal: input?.stage === "production" ? "retain" : "remove",
      protect: input?.stage === "production",
      providers: {
        cloudflare: "6.15.0",
      },
    };
  },

  async run() {
    // KV is for durable edge metadata/configuration, not sub-minute trend data.
    const edgeMetadata = new sst.cloudflare.Kv("TrendinaryEdgeMetadata");

    // R2 is the long-lived raw/provenance archive. Turso owns relational data.
    const rawArchive = new sst.cloudflare.Bucket("TrendinaryRawArchive");

    // Cloudflare-native rate-limit bindings use Cloudflare's distributed rate
    // limiting infrastructure rather than isolate-local in-memory counters.
    // Keep namespaces stable once production traffic starts using them.
    const cheapReadRateLimit = new sst.cloudflare.RateLimit("TrendinaryCheapReadRateLimit", {
      namespaceId: 4701,
      limit: 600,
      period: "1 minute",
    });
    const expensiveRateLimit = new sst.cloudflare.RateLimit("TrendinaryExpensiveRateLimit", {
      namespaceId: 4702,
      limit: 90,
      period: "1 minute",
    });
    const mutationRateLimit = new sst.cloudflare.RateLimit("TrendinaryMutationRateLimit", {
      namespaceId: 4703,
      limit: 60,
      period: "1 minute",
    });
    const authRateLimit = new sst.cloudflare.RateLimit("TrendinaryAuthRateLimit", {
      namespaceId: 4704,
      limit: 20,
      period: "1 minute",
    });

    const edge = new sst.cloudflare.Worker("TrendinaryEdge", {
      handler: "infra/edge.ts",
      link: [
        edgeMetadata,
        rawArchive,
        cheapReadRateLimit,
        expensiveRateLimit,
        mutationRateLimit,
        authRateLimit,
      ],
      url: true,
      domain: {
        name: "trendinary.com",
        dns: sst.cloudflare.dns(),
      },
    });

    return {
      edge: edge.url,
      origin: "https://trendinary.fly.dev",
      edgeMetadata: edgeMetadata.namespaceId,
      rawArchive: rawArchive.name,
      rateLimits: {
        cheap: cheapReadRateLimit.namespaceId,
        expensive: expensiveRateLimit.namespaceId,
        mutation: mutationRateLimit.namespaceId,
        auth: authRateLimit.namespaceId,
      },
    };
  },
});
