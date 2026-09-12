import { expect, test } from "@playwright/test";

const fixtureTrend = {
  slug: "atlas-browser-agent",
  aliases: ["Atlas browser agent"],
  rank: 1,
  name: "Atlas browser agent",
  category: "INTERNET",
  score: 71,
  change: "+32%",
  status: "RISING",
  reason: "Two independent publishers are reporting the same launch.",
  started: "12M AGO",
  vibe: "ACCELERATING",
  quality: {
    attention: 0.7,
    velocity: 0.8,
    source_breadth: 0.6,
    community_breadth: 0.4,
    novelty: 0.8,
    confidence: 0.9,
    peep_score: 0.8,
  },
  sources: [
    { name: "First News", domain: "first.example", url: "https://first.example/atlas" },
    { name: "Second News", domain: "second.example", url: "https://second.example/atlas" },
  ],
  timeline: [
    { time: "10:00", label: "FIRST SIGNAL", text: "First News published." },
    { time: "10:05", label: "CORROBORATED", text: "Second News independently published." },
  ],
};

test("production image serves a healthy empty application without demo trends", async ({ page, request }) => {
  const health = await request.get("/api/v1/healthz");
  expect(health.ok()).toBeTruthy();

  const trends = await request.get("/api/v1/trends");
  expect(trends.ok()).toBeTruthy();
  const payload = await trends.json();
  expect(payload.data).toEqual([]);

  await page.goto("/");
  await expect(page.getByRole("heading", { name: /Know what's happening/i })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Trendinary Top 20" })).toBeVisible();
  await expect(page.getByRole("link", { name: /AT Protocol/i })).toHaveCount(0);
  await expect(page.getByRole("link", { name: /That Blue Chair/i })).toHaveCount(0);
});

test("SPA deep links are served by the production Go binary", async ({ page }) => {
  const response = await page.goto("/following");
  expect(response?.ok()).toBeTruthy();
  await expect(page.getByRole("heading", { name: /Tell me when something changes/i })).toBeVisible();
});

test("trend detail renders coherent independent sources", async ({ page }) => {
  await page.route("**/api/v1/trends/atlas-browser-agent/history**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: [] }),
    });
  });
  await page.route("**/api/v1/trends/atlas-browser-agent", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: fixtureTrend }),
    });
  });

  await page.goto("/trend/atlas-browser-agent");
  await expect(page.getByRole("heading", { name: "Atlas browser agent" })).toBeVisible();
  await expect(page.getByRole("link", { name: "First News" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Second News" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Wikipedia" })).toHaveCount(0);
});
