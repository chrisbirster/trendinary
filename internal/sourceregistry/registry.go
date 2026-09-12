package sourceregistry

import "time"

type Policy string

const (
	PolicyOfficialRSS     Policy = "official-rss"
	PolicyOfficialAPI     Policy = "official-api"
	PolicyLicenseRequired Policy = "license-required"
	PolicyTermsReview     Policy = "terms-review"
)

// Entry describes a public discovery endpoint and the cadence Trendinary will
// respect. Enabled entries are metadata-only RSS/API discovery sources: titles,
// links, timestamps, authors and feed-provided summaries are used as signals;
// Trendinary does not mirror publisher article bodies.
type Entry struct {
	ID       string
	Name     string
	Kind     string
	URL      string
	TermsURL string
	Policy   Policy
	Cadence  time.Duration
	Enabled  bool
}

// Default is deliberately code-reviewed rather than remotely editable. New
// publishers should land with an explicit policy/cadence so increasing coverage
// cannot silently turn into an aggressive crawler.
func Default() []Entry {
	return []Entry{
		// Google Trends exposes an official Trending Now RSS export. It is used as
		// an attention/corroboration signal; the private alpha API can replace or
		// enrich this feed when Trendinary receives approved access.
		{ID: "google-trends-us", Name: "Google Trends · Trending Now (US)", Kind: "rss", URL: "https://trends.google.com/trending/rss?geo=US", TermsURL: "https://trends.google.com/trending", Policy: PolicyOfficialRSS, Cadence: 15 * time.Minute, Enabled: true},

		// WIRED publishes these feeds on its official RSS directory.
		{ID: "wired-top", Name: "WIRED · Top Stories", Kind: "rss", URL: "https://www.wired.com/feed/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "wired-business", Name: "WIRED · Business", Kind: "rss", URL: "https://www.wired.com/feed/category/business/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "wired-ai", Name: "WIRED · AI", Kind: "rss", URL: "https://www.wired.com/feed/tag/ai/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "wired-culture", Name: "WIRED · Culture", Kind: "rss", URL: "https://www.wired.com/feed/category/culture/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "wired-gear", Name: "WIRED · Gear", Kind: "rss", URL: "https://www.wired.com/feed/category/gear/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 45 * time.Minute, Enabled: true},
		{ID: "wired-ideas", Name: "WIRED · Ideas", Kind: "rss", URL: "https://www.wired.com/feed/category/ideas/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 45 * time.Minute, Enabled: true},
		{ID: "wired-science", Name: "WIRED · Science", Kind: "rss", URL: "https://www.wired.com/feed/category/science/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "wired-security", Name: "WIRED · Security", Kind: "rss", URL: "https://www.wired.com/feed/category/security/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "wired-backchannel", Name: "WIRED · Backchannel", Kind: "rss", URL: "https://www.wired.com/feed/category/backchannel/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "wired-guides", Name: "WIRED · Guides", Kind: "rss", URL: "https://www.wired.com/feed/tag/wired-guide/latest/rss", TermsURL: "https://www.wired.com/about/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},

		// Ars Technica's official RSS directory exposes main and section feeds.
		{ID: "ars-all", Name: "Ars Technica · All", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/index", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "ars-features", Name: "Ars Technica · Features", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/features", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "ars-tech", Name: "Ars Technica · Technology Lab", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/technology-lab", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "ars-gadgets", Name: "Ars Technica · Gear & Gadgets", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/gadgets", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 45 * time.Minute, Enabled: true},
		{ID: "ars-policy", Name: "Ars Technica · Tech Policy", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/tech-policy", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "ars-apple", Name: "Ars Technica · Apple", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/apple", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 45 * time.Minute, Enabled: true},
		{ID: "ars-gaming", Name: "Ars Technica · Gaming", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/gaming", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 45 * time.Minute, Enabled: true},
		{ID: "ars-science", Name: "Ars Technica · Science", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/science", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "ars-cars", Name: "Ars Technica · Cars", Kind: "rss", URL: "https://feeds.arstechnica.com/arstechnica/cars", TermsURL: "https://arstechnica.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},

		// ABC News explicitly publishes headline feeds for news aggregators.
		{ID: "abc-top", Name: "ABC News · Top Stories", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/topstories", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 15 * time.Minute, Enabled: true},
		{ID: "abc-us", Name: "ABC News · U.S.", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/usheadlines", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "abc-international", Name: "ABC News · International", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/internationalheadlines", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "abc-politics", Name: "ABC News · Politics", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/politicsheadlines", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "abc-business", Name: "ABC News · Business", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/moneyheadlines", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "abc-technology", Name: "ABC News · Technology", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/technologyheadlines", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "abc-health", Name: "ABC News · Health", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/healthheadlines", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "abc-most-read", Name: "ABC News · Most Read", Kind: "rss", URL: "https://feeds.abcnews.com/abcnews/mostreadstories", TermsURL: "https://abcnews.go.com/Site/page/rss-feeds-3520115", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},

		// Publisher and public-sector feeds with explicit syndication endpoints.
		{ID: "techcrunch", Name: "TechCrunch", Kind: "rss", URL: "https://techcrunch.com/feed/", TermsURL: "https://techcrunch.com/rss-terms-of-use/", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "nist-news", Name: "NIST · News", Kind: "rss", URL: "https://www.nist.gov/news-events/news/rss.xml", TermsURL: "https://www.nist.gov/coo/nist-rss-feeds", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "nist-cyber", Name: "NIST · Cybersecurity", Kind: "rss", URL: "https://www.nist.gov/news-events/cybersecurity/rss.xml", TermsURL: "https://www.nist.gov/coo/nist-rss-feeds", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "nist-it", Name: "NIST · Information Technology", Kind: "rss", URL: "https://www.nist.gov/news-events/information%20technology/rss.xml", TermsURL: "https://www.nist.gov/coo/nist-rss-feeds", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "jpl-news", Name: "NASA JPL · News", Kind: "rss", URL: "https://www.jpl.nasa.gov/feeds/news/", TermsURL: "https://www.jpl.nasa.gov/rss/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "jpl-cneos", Name: "NASA JPL · CNEOS", Kind: "rss", URL: "https://cneos.jpl.nasa.gov/feed/news.xml", TermsURL: "https://cneos.jpl.nasa.gov/feed/", Policy: PolicyOfficialRSS, Cadence: 2 * time.Hour, Enabled: true},
		{ID: "cisa-news", Name: "CISA · News", Kind: "rss", URL: "https://www.cisa.gov/news.xml", TermsURL: "https://www.cisa.gov/news-events", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "cisa-advisories", Name: "CISA · Cybersecurity Advisories", Kind: "rss", URL: "https://www.cisa.gov/cybersecurity-advisories/all.xml", TermsURL: "https://www.cisa.gov/news-events/cybersecurity-advisories", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "cisa-blog", Name: "CISA · Blog", Kind: "rss", URL: "https://www.cisa.gov/blog.xml", TermsURL: "https://www.cisa.gov/news-events", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},

		// First-party engineering/security publications are useful early signals.
		{ID: "aws-news", Name: "AWS News Blog", Kind: "rss", URL: "https://aws.amazon.com/blogs/aws/feed/", TermsURL: "https://aws.amazon.com/blogs/aws/heads-up-aws-news-blog-rss-feed-change/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "aws-security", Name: "AWS Security Blog", Kind: "rss", URL: "https://aws.amazon.com/blogs/security/feed/", TermsURL: "https://aws.amazon.com/blogs/security/", Policy: PolicyOfficialRSS, Cadence: 45 * time.Minute, Enabled: true},
		{ID: "aws-architecture", Name: "AWS Architecture Blog", Kind: "rss", URL: "https://aws.amazon.com/blogs/architecture/feed/", TermsURL: "https://aws.amazon.com/blogs/architecture/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "github-blog", Name: "GitHub Blog", Kind: "rss", URL: "https://github.blog/feed/", TermsURL: "https://github.blog/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "cloudflare-blog", Name: "Cloudflare Blog", Kind: "rss", URL: "https://blog.cloudflare.com/rss/", TermsURL: "https://blog.cloudflare.com/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},
		{ID: "mozilla-blog", Name: "Mozilla Blog", Kind: "rss", URL: "https://blog.mozilla.org/feed/", TermsURL: "https://blog.mozilla.org/en/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "mozilla-hacks", Name: "Mozilla Hacks", Kind: "rss", URL: "https://hacks.mozilla.org/feed/", TermsURL: "https://hacks.mozilla.org/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "bleepingcomputer", Name: "BleepingComputer", Kind: "rss", URL: "https://www.bleepingcomputer.com/feed/", TermsURL: "https://www.bleepingcomputer.com/rss-feeds/", Policy: PolicyOfficialRSS, Cadence: 20 * time.Minute, Enabled: true},
		{ID: "krebs", Name: "Krebs on Security", Kind: "rss", URL: "https://krebsonsecurity.com/feed/", TermsURL: "https://krebsonsecurity.com/", Policy: PolicyOfficialRSS, Cadence: 30 * time.Minute, Enabled: true},

		// Developer ecosystems often surface launches before mainstream coverage.
		{ID: "go-blog", Name: "Go Blog", Kind: "rss", URL: "https://go.dev/blog/feed.atom", TermsURL: "https://go.dev/blog/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "node-blog", Name: "Node.js Blog", Kind: "rss", URL: "https://nodejs.org/en/feed/blog.xml", TermsURL: "https://nodejs.org/en/blog", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "deno-blog", Name: "Deno Blog", Kind: "rss", URL: "https://deno.com/blog/feed.xml", TermsURL: "https://deno.com/blog", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "typescript-blog", Name: "TypeScript Blog", Kind: "rss", URL: "https://devblogs.microsoft.com/typescript/feed/", TermsURL: "https://devblogs.microsoft.com/typescript/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "swift-blog", Name: "Swift Blog", Kind: "rss", URL: "https://www.swift.org/atom.xml", TermsURL: "https://www.swift.org/blog/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "vercel-blog", Name: "Vercel Blog", Kind: "rss", URL: "https://vercel.com/atom", TermsURL: "https://vercel.com/blog", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "supabase-blog", Name: "Supabase Blog", Kind: "rss", URL: "https://supabase.com/blog/rss.xml", TermsURL: "https://supabase.com/blog", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "stripe-blog", Name: "Stripe Blog", Kind: "rss", URL: "https://stripe.com/blog/feed.rss", TermsURL: "https://stripe.com/blog", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},
		{ID: "google-developers", Name: "Google Developers Blog", Kind: "rss", URL: "https://developers.googleblog.com/feeds/posts/default/", TermsURL: "https://developers.googleblog.com/", Policy: PolicyOfficialRSS, Cadence: time.Hour, Enabled: true},

		// Visible but never called without explicit contractual/configuration work.
		{ID: "ap-media-api", Name: "Associated Press Media API", Kind: "api", URL: "https://api.ap.org/media/v/", TermsURL: "https://api.ap.org/media/v/docs/Getting_Started_API.htm", Policy: PolicyLicenseRequired, Cadence: time.Hour, Enabled: false},
		{ID: "youtube-api", Name: "YouTube Data API", Kind: "api", URL: "https://www.googleapis.com/youtube/v3/", TermsURL: "https://developers.google.com/youtube/v3/", Policy: PolicyOfficialAPI, Cadence: time.Hour, Enabled: false},
		{ID: "newsdata-api", Name: "NewsData", Kind: "api", URL: "https://newsdata.io/api/1/latest", TermsURL: "https://newsdata.io/documentation", Policy: PolicyOfficialAPI, Cadence: 2 * time.Hour, Enabled: false},
		{ID: "google-trends-api", Name: "Google Trends API (alpha)", Kind: "api", URL: "https://developers.google.com/search/apis/trends", TermsURL: "https://developers.google.com/search/apis/trends", Policy: PolicyOfficialAPI, Cadence: time.Hour, Enabled: false},
	}
}

func EnabledRSS() []Entry {
	all := Default()
	out := make([]Entry, 0, len(all))
	for _, entry := range all {
		if entry.Enabled && entry.Kind == "rss" {
			out = append(out, entry)
		}
	}
	return out
}
