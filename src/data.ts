export type Trend = {
  slug: string;
  rank: number;
  name: string;
  category: string;
  score: number;
  change: string;
  status: "BREAKING" | "RISING" | "EMERGING" | "PEAKING" | "COOLING" | "RESURFACING";
  reason: string;
  started: string;
  sources: string[];
  vibe: string;
};

export const trends: Trend[] = [
  {
    slug: "at-protocol",
    rank: 1,
    name: "AT Protocol",
    category: "TECH",
    score: 96,
    change: "+842%",
    status: "BREAKING",
    reason: "A wave of new social apps is pushing the open protocol back into the center of the decentralized-web conversation.",
    started: "38m ago",
    sources: ["Bluesky", "Hacker News", "Reddit", "News"],
    vibe: "Curious, bullish, chaotic",
  },
  {
    slug: "midnight-sun",
    rank: 2,
    name: "Midnight Sun",
    category: "CULTURE",
    score: 91,
    change: "+1,204%",
    status: "RISING",
    reason: "A cryptic trailer and one celebrity repost turned a niche project into a cross-platform mystery.",
    started: "1h ago",
    sources: ["TikTok", "YouTube", "Reddit"],
    vibe: "Confused, obsessed",
  },
  {
    slug: "aster-1",
    rank: 3,
    name: "Aster-1",
    category: "AI",
    score: 87,
    change: "+611%",
    status: "RISING",
    reason: "Developers are sharing surprising benchmark results from a small open model released this morning.",
    started: "2h ago",
    sources: ["Hacker News", "GitHub", "Bluesky"],
    vibe: "Impressed, skeptical",
  },
  {
    slug: "that-blue-chair",
    rank: 4,
    name: "That Blue Chair",
    category: "MEME",
    score: 83,
    change: "+2,970%",
    status: "EMERGING",
    reason: "A background prop from a livestream has somehow become the internet's newest reaction image.",
    started: "22m ago",
    sources: ["TikTok", "Reddit", "Bluesky"],
    vibe: "Unhinged",
  },
  {
    slug: "orbit-cup",
    rank: 5,
    name: "Orbit Cup",
    category: "SPORTS",
    score: 79,
    change: "+189%",
    status: "PEAKING",
    reason: "A last-second finish is generating clips, arguments, and instant remixes across sports feeds.",
    started: "3h ago",
    sources: ["YouTube", "News", "Reddit"],
    vibe: "Electric, argumentative",
  },
  {
    slug: "quiet-quitting-2",
    rank: 6,
    name: "Quiet Quitting 2.0",
    category: "WORK",
    score: 73,
    change: "+344%",
    status: "RESURFACING",
    reason: "An old workplace phrase is returning with a new meaning after a viral CEO memo.",
    started: "5h ago",
    sources: ["LinkedIn", "News", "Reddit"],
    vibe: "Tired, cynical",
  },
];

export const emerging = [trends[3], trends[2], trends[1]];

export const missed = [
  { name: "Aster-1", peak: "6h ago", summary: "A tiny AI model started outperforming expectations and developers noticed before major outlets did." },
  { name: "Orbit Cup finish", peak: "9h ago", summary: "One absurd final play produced the most-shared sports clip of the morning." },
  { name: "Browser agents", peak: "Yesterday", summary: "A cluster of launches turned browser-using AI agents into the day's biggest developer conversation." },
];

export const timeline = [
  ["08:42", "First signal", "A developer thread begins climbing Hacker News."],
  ["09:13", "Acceleration", "Reddit discussion triples in velocity over 20 minutes."],
  ["10:04", "Cross-platform breakout", "Bluesky posts begin linking the same story cluster."],
  ["11:31", "Mainstream pickup", "Technology publications start covering the broader shift."],
];
