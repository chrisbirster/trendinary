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

type ActorProfile struct {
	DID         string `json:"did,omitempty"`
	Handle      string `json:"handle,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

type Engagement struct {
	Score   int `json:"score,omitempty"`
	Replies int `json:"replies,omitempty"`
	Likes   int `json:"likes,omitempty"`
	Reposts int `json:"reposts,omitempty"`
	Quotes  int `json:"quotes,omitempty"`
}

// Signal is Trendinary's source-independent observation shape. Source is the
// original publisher/community being observed, while DiscoveryChannel records
// how Trendinary found it (for example rss, gdelt, hacker-news, or bluesky).
// ClusterText is an internal semantic hint: adapters may keep descriptive
// metadata in Text without allowing boilerplate to influence event clustering.
type Signal struct {
	ID               string        `json:"id"`
	Source           Source        `json:"source"`
	DiscoveryChannel string        `json:"discovery_channel,omitempty"`
	Title            string        `json:"title,omitempty"`
	Text             string        `json:"text,omitempty"`
	ClusterText      string        `json:"-"`
	URL              string        `json:"url,omitempty"`
	Author           string        `json:"author,omitempty"`
	AuthorID         string        `json:"author_id,omitempty"`
	Actor            *ActorProfile `json:"actor,omitempty"`
	PublishedAt      string        `json:"published_at,omitempty"`
	Engagement       Engagement    `json:"engagement,omitempty"`
}

type TimelineEvent struct {
	Time  string `json:"time"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type PropagationHop struct {
	Source       Source `json:"source"`
	FirstSeen    string `json:"first_seen"`
	LastSeen     string `json:"last_seen"`
	SignalCount  int    `json:"signal_count"`
	Engagement   int    `json:"engagement"`
	DelayMinutes int    `json:"delay_minutes,omitempty"`
	DelayLabel   string `json:"delay_label,omitempty"`
}

type PerspectiveMix struct {
	RatedSources int    `json:"rated_sources"`
	TotalSources int    `json:"total_sources"`
	Left         int    `json:"left"`
	LeanLeft     int    `json:"lean_left"`
	Center       int    `json:"center"`
	LeanRight    int    `json:"lean_right"`
	Right        int    `json:"right"`
	Mixed        int    `json:"mixed"`
	Unrated      int    `json:"unrated"`
	Note         string `json:"note"`
}

type Evidence struct {
	Source      Source `json:"source"`
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	Author      string `json:"author,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type Explanation struct {
	Summary      string     `json:"summary"`
	WhatChanged string     `json:"what_changed"`
	Lore         string     `json:"lore"`
	Confidence   string     `json:"confidence"`
	Mode         string     `json:"mode"`
	Evidence     []Evidence `json:"evidence"`
}

// TrendQuality exposes the normalized inputs that make an early signal useful.
// PEEPScore intentionally emphasizes slope and spread rather than raw fame.
// PublisherBreadth and PlatformBreadth are the v4 names; SourceBreadth and
// CommunityBreadth remain for API compatibility with pre-v4 clients.
type TrendQuality struct {
	Attention        float64  `json:"attention"`
	Velocity         float64  `json:"velocity"`
	SourceBreadth    float64  `json:"source_breadth"`
	CommunityBreadth float64  `json:"community_breadth"`
	PublisherBreadth float64  `json:"publisher_breadth"`
	PlatformBreadth  float64  `json:"platform_breadth"`
	Novelty          float64  `json:"novelty"`
	Confidence       float64  `json:"confidence"`
	PeepScore        int      `json:"peep_score"`
	WhyWatching      []string `json:"why_watching,omitempty"`
}

// TrendProvenance separates who published an observation from where Trendinary
// found it. Ten section feeds from the same publisher therefore count as one
// publisher, while GitHub/Bluesky accounts can be independent publishers on a
// shared platform.
type TrendProvenance struct {
	PublisherCount    int      `json:"publisher_count"`
	PlatformCount     int      `json:"platform_count"`
	SignalCount       int      `json:"signal_count"`
	Publishers        []string `json:"publishers,omitempty"`
	Platforms         []string `json:"platforms,omitempty"`
	DiscoveryChannels []string `json:"discovery_channels,omitempty"`
}

// ChartStats gives the Top 20 continuity semantics users expect from a chart.
// Status is NEW for a first appearance, RE for a re-entry, otherwise empty.
type ChartStats struct {
	Movement         string `json:"movement"`
	Status           string `json:"status,omitempty"`
	PreviousRank     int    `json:"previous_rank,omitempty"`
	PeakRank         int    `json:"peak_rank,omitempty"`
	TotalScans       int    `json:"total_scans,omitempty"`
	ConsecutiveScans int    `json:"consecutive_scans,omitempty"`
	NumberOneScans   int    `json:"number_one_scans,omitempty"`
}

type Trend struct {
	ID             string       `json:"id,omitempty"`
	Slug           string       `json:"slug"`
	Aliases        []string     `json:"aliases,omitempty"`
	Rank           int          `json:"rank"`
	Name           string       `json:"name"`
	Category       string       `json:"category"`
	Score          int          `json:"score"`
	Change         string       `json:"change"`
	Status         string       `json:"status"`
	ConfidenceTier string       `json:"confidence_tier"`
	Reason         string       `json:"reason"`
	Started        string       `json:"started"`
	Vibe           string       `json:"vibe"`
	Quality        TrendQuality `json:"quality"`
	Provenance     TrendProvenance `json:"provenance"`
	Chart          ChartStats      `json:"chart"`

	Sources     []Source         `json:"sources"`
	Timeline    []TimelineEvent  `json:"timeline,omitempty"`
	TopVoices   []ActorProfile   `json:"top_voices,omitempty"`
	Propagation []PropagationHop `json:"propagation,omitempty"`
	Perspective PerspectiveMix   `json:"perspective"`
	Explanation *Explanation     `json:"explanation,omitempty"`
	Lore        string           `json:"lore,omitempty"`
	Why         string           `json:"why,omitempty"`
}
