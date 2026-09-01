package editorial

import (
	"fmt"
	"hash/fnv"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

func NewID(prefix string) string {
	now := time.Now().UTC().UnixNano()
	return fmt.Sprintf("%s_%x", prefix, uint64(now))
}

func NormalizeURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil { return "", err }
	if parsed.Scheme != "http" && parsed.Scheme != "https" { return "", fmt.Errorf("unsupported URL scheme") }
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" || lower == "mc_cid" || lower == "mc_eid" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	if parsed.Path == "" { parsed.Path = "/" }
	return parsed.String(), nil
}

func DetectContentType(raw string) ContentType {
	parsed, err := url.Parse(raw)
	if err != nil { return ContentOther }
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	switch {
	case host == "youtube.com" || host == "youtu.be" || host == "vimeo.com":
		return ContentVideo
	case host == "github.com" || host == "gitlab.com" || host == "codeberg.org":
		return ContentRepository
	case host == "news.ycombinator.com" && strings.HasPrefix(parsed.Path, "/item"):
		return ContentDiscussion
	case strings.Contains(host, "podcast") || strings.HasSuffix(strings.ToLower(path.Ext(parsed.Path)), ".mp3"):
		return ContentPodcast
	default:
		return ContentArticle
	}
}

func PublisherDomain(raw string) string {
	parsed, err := url.Parse(raw); if err != nil { return "" }
	return strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
}

func ScoreEditorial(item DiscoveredItem, canonical string) (int, ScoreComponents, bool, string) {
	freshness := freshnessFromAge(item.SourceAgeText)
	serendipity := deterministicSerendipity(canonical)
	components := ScoreComponents{
		Freshness: freshness,
		PersonalRelevance: 0.50,
		Novelty: 0.50,
		SourceQuality: 0.50,
		TrendSignal: 0.65,
		Serendipity: 0,
	}
	if serendipity { components.Serendipity = 1 }
	score := components.PersonalRelevance*0.30 + components.Novelty*0.20 + components.SourceQuality*0.15 + components.TrendSignal*0.15 + components.Freshness*0.10 + components.Serendipity*0.10
	why := "Discovered through TechURLs; personalization and source quality remain neutral until Trendinary has stronger preference/evidence data."
	if freshness >= .8 { why = "Fresh TechURLs discovery; neutral personalization until preference history grows." }
	if serendipity { why += " Serendipity slot: deliberately outside the relevance-only ranking." }
	return int(score*100 + .5), components, serendipity, why
}

func freshnessFromAge(age string) float64 {
	value := strings.TrimSpace(strings.ToLower(age))
	if value == "" { return .5 }
	unit := value[len(value)-1:]
	number, err := strconv.ParseFloat(strings.TrimSpace(value[:len(value)-1]),64)
	if err != nil { return .5 }
	hours := number
	switch unit { case "m": hours=number/60; case "h": hours=number; case "d": hours=number*24; case "w": hours=number*24*7; case "y": hours=number*24*365; default: return .5 }
	switch { case hours <= 3: return 1; case hours <= 12: return .9; case hours <= 24: return .8; case hours <= 72: return .65; case hours <= 168: return .45; case hours <= 720: return .2; default: return .05 }
}

func deterministicSerendipity(key string) bool {
	h := fnv.New32a(); _, _ = h.Write([]byte(strings.ToLower(key)))
	return h.Sum32()%5 == 0
}
