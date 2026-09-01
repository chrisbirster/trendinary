package editorial

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const techURLsEndpoint = "https://techurls.com/"

var relativeAgePattern = regexp.MustCompile(`(?i)(^|\s)(\d+(?:\.\d+)?[mhdwy])($|\s)`)

type DiscoverySource interface {
	Name() string
	Fetch(context.Context) (FetchResult, error)
}

type TechURLs struct {
	client     *http.Client
	url        string
	archiveDir string
	now        func() time.Time
}

func NewTechURLs(client *http.Client) *TechURLs {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &TechURLs{client: client, url: techURLsEndpoint, now: func() time.Time { return time.Now().UTC() }}
}

func NewTechURLsWithEndpoint(client *http.Client, endpoint string) *TechURLs {
	source := NewTechURLs(client)
	if strings.TrimSpace(endpoint) != "" { source.url = endpoint }
	return source
}

func (t *TechURLs) SetArchiveDir(dir string) { t.archiveDir = strings.TrimSpace(dir) }
func (t *TechURLs) Name() string { return "techurls" }

func (t *TechURLs) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil { return FetchResult{}, err }
	req.Header.Set("User-Agent", "Trendinary/0.2 (+https://trendinary.com; editorial-discovery)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := t.client.Do(req)
	if err != nil { return FetchResult{}, err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return FetchResult{}, fmt.Errorf("techurls returned HTTP %d", resp.StatusCode) }
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil { return FetchResult{}, err }
	if t.archiveDir != "" {
		if err := t.archive(raw); err != nil {
			// Archival must never make discovery unavailable. Surface it in metrics.
			result, parseErr := ParseTechURLs(bytes.NewReader(raw))
			if parseErr != nil { return FetchResult{}, parseErr }
			result.Errors = append(result.Errors, "raw archive: "+err.Error())
			return result, nil
		}
	}
	return ParseTechURLs(bytes.NewReader(raw))
}

func (t *TechURLs) archive(raw []byte) error {
	folder := filepath.Join(t.archiveDir, "techurls", t.now().UTC().Format("2006/01/02"))
	if err := os.MkdirAll(folder, 0o750); err != nil { return err }
	path := filepath.Join(folder, t.now().UTC().Format("150405.000000000")+".html")
	return os.WriteFile(path, raw, 0o640)
}

func ParseTechURLs(reader io.Reader) (FetchResult, error) {
	doc, err := html.Parse(reader)
	if err != nil { return FetchResult{}, fmt.Errorf("parse techurls html: %w", err) }
	result := FetchResult{}
	sections := map[string]struct{}{}
	currentSection := ""
	seen := map[string]struct{}{}

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && isSectionNode(node) {
			if label := sectionLabel(node); label != "" { currentSection = label; sections[label] = struct{}{} }
		}
		if node.Type == html.ElementNode && node.Data == "a" {
			href := attr(node, "href")
			title := cleanText(textContent(node))
			age := ageBefore(node)
			if title != "" && href != "" && age != "" {
				absolute, ok := outboundURL(href)
				if !ok {
					result.Malformed++
				} else if _, duplicate := seen[absolute]; !duplicate {
					seen[absolute] = struct{}{}
					publisher := currentSection
					if publisher == "" { publisher = PublisherDomain(absolute) }
					result.Items = append(result.Items, DiscoveredItem{Title:title,URL:absolute,ExternalSourceName:publisher,SourceAgeText:age,ContentType:DetectContentType(absolute)})
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling { walk(child) }
	}
	walk(doc)
	result.SectionsSeen = len(sections)
	result.ItemsSeen = len(result.Items) + result.Malformed
	return result, nil
}

func isSectionNode(node *html.Node) bool {
	if node.Type != html.ElementNode { return false }
	if node.Data == "h1" || node.Data == "h2" || node.Data == "h3" || node.Data == "h4" { return true }
	class := strings.ToLower(attr(node,"class"))
	return strings.Contains(class,"site-name") || strings.Contains(class,"site-title") || strings.Contains(class,"source-name") || strings.Contains(class,"source-title")
}

func sectionLabel(node *html.Node) string {
	text := cleanText(textContent(node))
	if text == "" || len(text) > 100 { return "" }
	lower := strings.ToLower(text)
	for _, noise := range []string{"search headlines","preferences","rearrange","subscribe","visible sites","hidden sites"} {
		if strings.Contains(lower, noise) { return "" }
	}
	for _, suffix := range []string{" latest day week month"," latest•day•week•month"," latest · day · week · month"} {
		if strings.HasSuffix(strings.ToLower(text), suffix) { text = strings.TrimSpace(text[:len(text)-len(suffix)]) }
	}
	return text
}

func ageBefore(anchor *html.Node) string {
	for current, depth := anchor, 0; current != nil && depth < 4; current, depth = current.Parent, depth+1 {
		for prev, scanned := current.PrevSibling, 0; prev != nil && scanned < 5; prev, scanned = prev.PrevSibling, scanned+1 {
			if match := relativeAgePattern.FindStringSubmatch(cleanText(textContent(prev))); len(match) > 2 { return strings.ToLower(match[2]) }
		}
	}
	for current, depth := anchor.Parent, 0; current != nil && depth < 3; current, depth = current.Parent, depth+1 {
		if match := relativeAgePattern.FindStringSubmatch(cleanText(textContent(current))); len(match) > 2 { return strings.ToLower(match[2]) }
	}
	return ""
}

func outboundURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw)); if err != nil { return "", false }
	if !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") { return "", false }
	host := strings.ToLower(parsed.Hostname())
	if host == "techurls.com" || strings.HasSuffix(host,".techurls.com") || host == "browserling.com" || strings.HasSuffix(host,".browserling.com") { return "", false }
	return parsed.String(), true
}

func attr(node *html.Node, key string) string {
	for _, value := range node.Attr { if strings.EqualFold(value.Key,key) { return value.Val } }
	return ""
}

func textContent(node *html.Node) string {
	if node == nil { return "" }
	if node.Type == html.TextNode { return node.Data }
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling { builder.WriteString(" "); builder.WriteString(textContent(child)) }
	return builder.String()
}

func cleanText(value string) string { return strings.Join(strings.Fields(strings.TrimSpace(value))," ") }
