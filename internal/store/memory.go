package store

import (
	"strings"
	"sync"

	"github.com/chrisbirster/trendinary/internal/model"
)

type Memory struct {
	mu      sync.RWMutex
	trends  []model.Trend
	sources map[string]model.Source
}

func NewMemory() *Memory {
	foxScore := 3.85
	fox := model.Source{
		Name:   "Fox News Digital",
		Domain: "foxnews.com",
		URL:    "https://www.foxnews.com/",
		Bias: &model.BiasAssessment{
			Label:          model.BiasRight,
			Provider:       "AllSides",
			Score:          &foxScore,
			Confidence:     "medium",
			Scope:          "Online news coverage only; does not represent Fox cable TV or radio content.",
			MethodologyURL: "https://www.allsides.com/about/media-bias-rating-methods",
			RatingURL:      "https://www.allsides.com/news-source/fox-news-media-bias",
			AsOf:           "2026-08",
		},
	}

	sources := map[string]model.Source{
		"news.ycombinator.com": {Name: "Hacker News", Domain: "news.ycombinator.com", URL: "https://news.ycombinator.com/"},
		"bsky.app":            {Name: "Bluesky", Domain: "bsky.app", URL: "https://bsky.app/"},
		"reddit.com":          {Name: "Reddit", Domain: "reddit.com", URL: "https://www.reddit.com/"},
		"github.com":          {Name: "GitHub", Domain: "github.com", URL: "https://github.com/"},
		"youtube.com":         {Name: "YouTube", Domain: "youtube.com", URL: "https://www.youtube.com/"},
		"foxnews.com":         fox,
	}

	trends := []model.Trend{
		{
			Slug: "at-protocol", Rank: 1, Name: "AT Protocol", Category: "TECH", Score: 96,
			Change: "+842%", Status: "BREAKING", Started: "38m ago", Vibe: "Curious, bullish, chaotic",
			Reason: "A wave of new social apps is pushing the open protocol back into the center of the decentralized-web conversation.",
			Why: "A burst of app launches and developer discussion pushed AT Protocol back into the spotlight. The notable signal is breadth: the same topic accelerated across developer communities, social feeds, and technology coverage at roughly the same time.",
			Lore: "AT Protocol is an open protocol for decentralized social applications. Trendinary keeps this section as the durable context that survives after today's attention spike cools.",
			Sources: []model.Source{sources["bsky.app"], sources["news.ycombinator.com"], sources["reddit.com"], sources["github.com"]},
			Timeline: []model.TimelineEvent{
				{Time: "08:42", Label: "First signal", Text: "A developer thread begins climbing Hacker News."},
				{Time: "09:13", Label: "Acceleration", Text: "Reddit discussion triples in velocity over 20 minutes."},
				{Time: "10:04", Label: "Cross-platform breakout", Text: "Bluesky posts begin linking the same story cluster."},
				{Time: "11:31", Label: "Mainstream pickup", Text: "Technology publications start covering the broader shift."},
			},
		},
		{Slug: "midnight-sun", Rank: 2, Name: "Midnight Sun", Category: "CULTURE", Score: 91, Change: "+1,204%", Status: "RISING", Started: "1h ago", Vibe: "Confused, obsessed", Reason: "A cryptic trailer and one celebrity repost turned a niche project into a cross-platform mystery.", Sources: []model.Source{sources["youtube.com"], sources["reddit.com"]}},
		{Slug: "aster-1", Rank: 3, Name: "Aster-1", Category: "AI", Score: 87, Change: "+611%", Status: "RISING", Started: "2h ago", Vibe: "Impressed, skeptical", Reason: "Developers are sharing surprising benchmark results from a small open model released this morning.", Sources: []model.Source{sources["news.ycombinator.com"], sources["github.com"], sources["bsky.app"]}},
		{Slug: "that-blue-chair", Rank: 4, Name: "That Blue Chair", Category: "MEME", Score: 83, Change: "+2,970%", Status: "EMERGING", Started: "22m ago", Vibe: "Unhinged", Reason: "A background prop from a livestream has somehow become the internet's newest reaction image.", Sources: []model.Source{sources["reddit.com"], sources["bsky.app"]}},
		{Slug: "orbit-cup", Rank: 5, Name: "Orbit Cup", Category: "SPORTS", Score: 79, Change: "+189%", Status: "PEAKING", Started: "3h ago", Vibe: "Electric, argumentative", Reason: "A last-second finish is generating clips, arguments, and instant remixes across sports feeds.", Sources: []model.Source{sources["youtube.com"], sources["reddit.com"]}},
		{Slug: "quiet-quitting-2", Rank: 6, Name: "Quiet Quitting 2.0", Category: "WORK", Score: 73, Change: "+344%", Status: "RESURFACING", Started: "5h ago", Vibe: "Tired, cynical", Reason: "An old workplace phrase is returning with a new meaning after a viral CEO memo.", Sources: []model.Source{sources["reddit.com"]}},
	}

	return &Memory{trends: trends, sources: sources}
}

// NewProductionMemory keeps the production leaderboard empty until the scanner
// publishes real, quality-gated observations. Demo fixtures remain available
// through NewMemory for focused tests, but they must never leak into production.
func NewProductionMemory() *Memory {
	memory := NewMemory()
	memory.trends = nil
	return memory
}

func (m *Memory) Trends() []model.Trend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Trend, len(m.trends))
	copy(out, m.trends)
	return out
}

func (m *Memory) Trend(slug string) (model.Trend, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, trend := range m.trends {
		if trend.Slug == slug {
			return trend, true
		}
	}
	return model.Trend{}, false
}

// ReplaceTrends publishes a complete scanner snapshot atomically. An empty
// result is ignored so a temporary upstream outage cannot blank a previously
// valid live leaderboard. A fresh production process starts empty instead of
// falling back to prototype data.
func (m *Memory) ReplaceTrends(trends []model.Trend) {
	if len(trends) == 0 {
		return
	}
	copyOfTrends := make([]model.Trend, len(trends))
	copy(copyOfTrends, trends)
	m.mu.Lock()
	m.trends = copyOfTrends
	m.mu.Unlock()
}

func (m *Memory) Source(domain string) (model.Source, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	m.mu.RLock()
	defer m.mu.RUnlock()
	source, ok := m.sources[domain]
	return source, ok
}
