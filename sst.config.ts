/// <reference path="./.sst/platform/config.d.ts" />

export default $config({
  app(input) {
    return {
      name: "trendinary",
      home: "cloudflare",
      removal: input?.stage === "production" ? "retain" : "remove",
      protect: input?.stage === "production",
      providers: {
        cloudflare: "5.37.1",
      },
    };
  },

  async run() {
    const edge = new sst.cloudflare.Worker("TrendinaryEdge", {
      handler: "infra/edge.ts",
      url: true,
      domain: {
        name: "trendinary.com",
        redirects: ["www.trendinary.com"],
      },
    });

    return {
      edge: edge.url,
      origin: "https://trendinary.fly.dev",
    };
  },
});
