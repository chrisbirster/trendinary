package model

// BiasLabel is a coarse political-lean classification supplied by an external
// rating methodology. Trendinary does not infer a source-level label merely
// from the source name or from a single article.
type BiasLabel string

const (
	BiasNotRated  BiasLabel = "not-rated"
	BiasLeft      BiasLabel = "left"
	BiasLeanLeft  BiasLabel = "lean-left"
	BiasCenter    BiasLabel = "center"
	BiasLeanRight BiasLabel = "lean-right"
	BiasRight     BiasLabel = "right"
	BiasMixed     BiasLabel = "mixed"
)

type BiasAssessment struct {
	Label          BiasLabel `json:"label"`
	Provider       string    `json:"provider"`
	Score          *float64  `json:"score,omitempty"`
	Confidence     string    `json:"confidence,omitempty"`
	Scope          string    `json:"scope,omitempty"`
	MethodologyURL string    `json:"methodology_url,omitempty"`
	RatingURL      string    `json:"rating_url,omitempty"`
	AsOf           string    `json:"as_of,omitempty"`
}

type Source struct {
	Name   string          `json:"name"`
	Domain string          `json:"domain,omitempty"`
	URL    string          `json:"url,omitempty"`
	Bias   *BiasAssessment `json:"bias,omitempty"`
}

type Engagement struct {
	Score    int `json:"score,omitempty"`
	Replies  int `json:"replies,omitempty"`
	Likes    int `json:"likes,omitempty"`
	Reposts  int `json:"reposts,omitempty"`
	Quotes   int `json:"quotes,omitempty"`
}

// Signal is Trendinary's source-independent observation shape. Adapters may
// retain richer source-specific fields internally, but clustering and scoring
// consume this common representation.
type Signal struct {
	ID          string     `json:"id"`
	Source      Source     `json:"source"`
	Title       string     `json:"title,omitempty"`
	Text        string     `json:"text,omitempty"`
	URL         string     `json:"url,omitempty"`
	Author      string     `json:"author,omitempty"`
	PublishedAt string     `json:"published_at,omitempty"`
	Engagement  Engagement `json:"engagement,omitempty"`
}

type TimelineEvent struct {
	Time  string `json:"time"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type Trend struct {
	Slug     string `json:"slug"`
	Rank     int    `json:"rank"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Score    int    `json:"score"`
	Change   string `json:"change"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Started  string `json:"started"`
	Vibe     string `json:"vibe"`

	Sources  []Source        `json:"sources"`
	Timeline []TimelineEvent `json:"timeline,omitempty"`
	Lore     string          `json:"lore,omitempty"`
	Why      string          `json:"why,omitempty"`
}
