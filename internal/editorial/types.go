package editorial

import "time"

type ContentType string

const (
	ContentArticle    ContentType = "article"
	ContentVideo      ContentType = "video"
	ContentPodcast    ContentType = "podcast"
	ContentRepository ContentType = "repository"
	ContentDiscussion ContentType = "discussion"
	ContentOther      ContentType = "other"
)

type State string

const (
	StateInbox    State = "inbox"
	StateQueued   State = "queued"
	StateConsumed State = "consumed"
	StateSaved    State = "saved"
	StateRejected State = "rejected"
	StateArchived State = "archived"
)

type EnrichmentStatus string

const (
	EnrichmentPending  EnrichmentStatus = "pending"
	EnrichmentComplete EnrichmentStatus = "complete"
	EnrichmentPartial  EnrichmentStatus = "partial"
	EnrichmentFailed   EnrichmentStatus = "failed"
)

type Source struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Kind               string     `json:"kind"`
	URL                string     `json:"url"`
	Enabled            bool       `json:"enabled"`
	LastAttemptedAt    *time.Time `json:"last_attempted_at,omitempty"`
	LastSuccessfulAt   *time.Time `json:"last_successful_at,omitempty"`
	LatestItemCount    int        `json:"latest_item_count"`
	LastError          string     `json:"last_error,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ScoreComponents struct {
	Freshness         float64 `json:"freshness"`
	PersonalRelevance float64 `json:"personal_relevance"`
	Novelty           float64 `json:"novelty"`
	SourceQuality     float64 `json:"source_quality"`
	TrendSignal       float64 `json:"trend_signal"`
	Serendipity       float64 `json:"serendipity"`
}

type ContentItem struct {
	ID                string           `json:"id"`
	CanonicalURL      string           `json:"canonical_url"`
	OriginalURL       string           `json:"original_url"`
	Title             string           `json:"title"`
	Publisher         string           `json:"publisher"`
	PublisherDomain   string           `json:"publisher_domain"`
	ContentType       ContentType      `json:"content_type"`
	Description       string           `json:"description,omitempty"`
	ImageURL          string           `json:"image_url,omitempty"`
	Author            string           `json:"author,omitempty"`
	PublishedAt       *time.Time       `json:"published_at,omitempty"`
	DiscoveredAt      time.Time        `json:"discovered_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
	State             State            `json:"state"`
	OpenedAt          *time.Time       `json:"opened_at,omitempty"`
	QueuedAt          *time.Time       `json:"queued_at,omitempty"`
	ConsumedAt        *time.Time       `json:"consumed_at,omitempty"`
	SavedAt           *time.Time       `json:"saved_at,omitempty"`
	RejectedAt        *time.Time       `json:"rejected_at,omitempty"`
	EditorialScore    int              `json:"editorial_score"`
	ScoreComponents   ScoreComponents  `json:"score_components"`
	Serendipity       bool             `json:"serendipity"`
	WhyInteresting    string           `json:"why_interesting"`
	EnrichmentStatus  EnrichmentStatus `json:"enrichment_status"`
	EnrichmentError   string           `json:"enrichment_error,omitempty"`
	DiscoverySource   string           `json:"discovery_source,omitempty"`
	ExternalSource    string           `json:"external_source_name,omitempty"`
	SourceAgeText     string           `json:"source_age_text,omitempty"`
	EstimatedMinutes  int              `json:"estimated_minutes,omitempty"`
	Note              *Note            `json:"note,omitempty"`
}

type Note struct {
	ContentItemID string    `json:"content_item_id"`
	Text          string    `json:"note"`
	WorthSharing  *bool     `json:"worth_sharing,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Discovery struct {
	ContentItemID      string    `json:"content_item_id"`
	DiscoverySourceID  string    `json:"discovery_source_id"`
	ExternalSourceName string    `json:"external_source_name"`
	SourceAgeText      string    `json:"source_age_text,omitempty"`
	DiscoveredAt       time.Time `json:"discovered_at"`
	MetadataJSON       string    `json:"metadata_json,omitempty"`
}

type IngestionMetrics struct {
	SectionsSeen  int `json:"sections_seen"`
	ItemsSeen     int `json:"items_seen"`
	ItemsInserted int `json:"items_inserted"`
	ItemsUpdated  int `json:"items_updated"`
	Duplicates    int `json:"duplicates"`
	Malformed     int `json:"malformed"`
	Errors        int `json:"errors"`
}

type IngestionRun struct {
	ID          string           `json:"id"`
	SourceID    string           `json:"source_id"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	Status      string           `json:"status"`
	Metrics     IngestionMetrics `json:"metrics"`
	Error       string           `json:"error,omitempty"`
}

type DiscoveredItem struct {
	Title              string
	URL                string
	ExternalSourceName string
	SourceAgeText      string
	ContentType        ContentType
	MetadataJSON       string
}

type FetchResult struct {
	Items        []DiscoveredItem
	SectionsSeen int
	ItemsSeen    int
	Malformed    int
	Errors       []string
}

type NewsletterIssue struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Status    string                `json:"status"`
	IssueDate string                `json:"issue_date,omitempty"`
	Intro     string                `json:"intro,omitempty"`
	Question  string                `json:"question,omitempty"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	Items     []NewsletterIssueItem `json:"items,omitempty"`
}

type NewsletterIssueItem struct {
	ID            string       `json:"id"`
	IssueID       string       `json:"issue_id"`
	ContentItemID string       `json:"content_item_id"`
	Section       string       `json:"section"`
	Position      int          `json:"position"`
	EditorNote    string       `json:"editor_note,omitempty"`
	Content       *ContentItem `json:"content,omitempty"`
}
