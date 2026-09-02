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
    // Hot trend responses should use the Worker Cache API so freshness stays
    // measured in seconds rather than KV's eventually-consistent semantics.
    const edgeMetadata = new sst.cloudflare.Kv("TrendinaryEdgeMetadata");

    // R2 is the long-lived raw/provenance archive: fetched source payloads,
    // replay fixtures, and generated exports. Turso owns relational database
    // durability and recovery independently of the Fly origin.
    const rawArchive = new sst.cloudflare.Bucket("TrendinaryRawArchive");

    const edge = new sst.cloudflare.Worker("TrendinaryEdge", {
      handler: "infra/edge.ts",
      link: [edgeMetadata, rawArchive],
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
    };
  },
});
