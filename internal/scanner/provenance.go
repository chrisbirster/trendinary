package scanner

import (
	"sort"
	"strings"
	"unicode"

	"github.com/chrisbirster/trendinary/internal/model"
)

var publisherDisplayNames = map[string]string{
	"wired.com":             "WIRED",
	"abcnews.go.com":        "ABC News",
	"arstechnica.com":       "Ars Technica",
	"techcrunch.com":        "TechCrunch",
	"bleepingcomputer.com":  "BleepingComputer",
	"krebsonsecurity.com":   "Krebs on Security",
	"github.blog":           "GitHub Blog",
	"blog.cloudflare.com":   "Cloudflare Blog",
	"blog.mozilla.org":      "Mozilla Blog",
	"hacks.mozilla.org":     "Mozilla Hacks",
	"aws.amazon.com":        "AWS",
	"nist.gov":              "NIST",
	"jpl.nasa.gov":          "NASA JPL",
	"cneos.jpl.nasa.gov":    "NASA JPL CNEOS",
	"cisa.gov":              "CISA",
	"go.dev":                "Go Blog",
	"nodejs.org":            "Node.js",
	"deno.com":              "Deno",
	"devblogs.microsoft.com": "Microsoft DevBlogs",
	"swift.org":             "Swift",
	"vercel.com":            "Vercel",
	"supabase.com":          "Supabase",
	"stripe.com":            "Stripe",
	"developers.googleblog.com": "Google Developers",
}

func provenanceSummary(values []model.Signal) model.TrendProvenance {
	publishers := map[string]string{}
	platforms := map[string]string{}
	channels := map[string]string{}
	for _, signal := range values {
		if key, label := signalPublisher(signal); key != "" {
			publishers[key] = label
		}
		if key, label := signalPlatform(signal); key != "" {
			platforms[key] = label
		}
		if channel := strings.ToLower(strings.TrimSpace(signal.DiscoveryChannel)); channel != "" {
			channels[channel] = channel
		}
	}
	return model.TrendProvenance{
		PublisherCount:    len(publishers),
		PlatformCount:     len(platforms),
		SignalCount:       len(values),
		Publishers:        sortedMapValues(publishers),
		Platforms:         sortedMapValues(platforms),
		DiscoveryChannels: sortedMapValues(channels),
	}
}

func signalPublisher(signal model.Signal) (string, string) {
	domain := strings.ToLower(strings.TrimSpace(signal.Source.Domain))
	channel := strings.ToLower(strings.TrimSpace(signal.DiscoveryChannel))
	identity := strings.TrimSpace(signal.AuthorID)
	if identity == "" {
		identity = strings.TrimSpace(signal.Author)
	}

	if channel == "github" || domain == "github.com" {
		if identity != "" {
			return "github:" + strings.ToLower(identity), identity
		}
	}
	if channel == "bluesky" || domain == "bsky.app" {
		if identity != "" {
			return "bluesky:" + strings.ToLower(identity), identity
		}
	}
	if channel == "youtube" || domain == "youtube.com" || domain == "youtu.be" {
		if identity != "" {
			return "youtube:" + strings.ToLower(identity), identity
		}
	}
	if channel == "hacker-news" && (domain == "" || domain == "news.ycombinator.com") {
		if identity != "" {
			return "hacker-news:" + strings.ToLower(identity), identity
		}
		return "news.ycombinator.com", "Hacker News"
	}
	if channel == "google-trends" || domain == "trends.google.com" {
		return "trends.google.com", "Google Trends"
	}

	if domain != "" {
		label := publisherDisplayNames[domain]
		if label == "" {
			label = strings.TrimSpace(signal.Source.Name)
		}
		if label == "" || strings.Contains(label, " · ") {
			label = domain
		}
		return domain, label
	}
	name := strings.TrimSpace(signal.Source.Name)
	if name != "" {
		return strings.ToLower(name), name
	}
	return "", ""
}

func signalPlatform(signal model.Signal) (string, string) {
	channel := strings.ToLower(strings.TrimSpace(signal.DiscoveryChannel))
	domain := strings.ToLower(strings.TrimSpace(signal.Source.Domain))
	switch {
	case channel == "bluesky" || domain == "bsky.app":
		return "bluesky", "Bluesky"
	case channel == "github" || domain == "github.com":
		return "github", "GitHub"
	case channel == "hacker-news" || domain == "news.ycombinator.com":
		return "hacker-news", "Hacker News"
	case channel == "youtube" || domain == "youtube.com" || domain == "youtu.be":
		return "youtube", "YouTube"
	case channel == "wikipedia" || domain == "wikipedia.org":
		return "wikipedia", "Wikipedia"
	case channel == "google-trends" || domain == "trends.google.com":
		return "google-trends", "Google Trends"
	default:
		// RSS, GDELT, NewsData and direct article URLs are discovery methods for
		// the same open-web publishing platform, not separate publishers.
		return "web", "Web"
	}
}

func confidenceTier(provenance model.TrendProvenance, confidence float64) string {
	switch {
	case provenance.PublisherCount >= 4 && provenance.PlatformCount >= 2 && confidence >= .65:
		return "CONFIRMED"
	case provenance.PublisherCount >= 2 || provenance.PlatformCount >= 2:
		return "CORROBORATED"
	default:
		return "EMERGING"
	}
}

func sortedMapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

// independentEvidenceSignals keeps every signal available for provenance while
// returning a conservative subset for scoring. Near-identical long bodies from
// mirrors/clones do not count as independent corroboration.
func independentEvidenceSignals(values []model.Signal) []model.Signal {
	seen := map[string]struct{}{}
	out := make([]model.Signal, 0, len(values))
	for _, signal := range values {
		fingerprint := evidenceFingerprint(signal)
		if fingerprint != "" {
			if _, exists := seen[fingerprint]; exists {
				continue
			}
			seen[fingerprint] = struct{}{}
		}
		out = append(out, signal)
	}
	return out
}

func evidenceFingerprint(signal model.Signal) string {
	text := strings.TrimSpace(signal.Text)
	if len([]rune(text)) < 80 {
		return ""
	}
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	value := strings.Join(strings.Fields(b.String()), " ")
	if len(value) > 320 {
		value = value[:320]
	}
	return value
}

func isLowInformationSocial(signal model.Signal) bool {
	channel := strings.ToLower(strings.TrimSpace(signal.DiscoveryChannel))
	domain := strings.ToLower(strings.TrimSpace(signal.Source.Domain))
	if channel != "bluesky" && domain != "bsky.app" {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(signal.Text))
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	if len(words) < 4 {
		return true
	}
	if len(words) <= 7 {
		generic := map[string]struct{}{
			"more": {}, "less": {}, "yes": {}, "no": {}, "yeah": {}, "yep": {}, "nope": {},
			"lol": {}, "lmao": {}, "okay": {}, "ok": {}, "same": {}, "true": {}, "this": {},
			"that": {}, "really": {}, "maybe": {}, "sure": {}, "thanks": {}, "thank": {}, "you": {},
		}
		meaningful := 0
		for _, word := range words {
			if _, blocked := generic[word]; !blocked && len(word) > 2 {
				meaningful++
			}
		}
		return meaningful < 2
	}
	return false
}
