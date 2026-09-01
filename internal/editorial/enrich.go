package editorial

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type PageMetadata struct {
	Title        string
	CanonicalURL string
	Description  string
	ImageURL     string
	Author       string
	PublishedAt  time.Time
}

func FetchMetadata(ctx context.Context, client *http.Client, rawURL string) (PageMetadata,error) {
	if client==nil { client=&http.Client{Timeout:12*time.Second} }
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,rawURL,nil); if err!=nil{return PageMetadata{},err}
	req.Header.Set("User-Agent","Trendinary/0.2 (+https://trendinary.com; metadata-enrichment)")
	req.Header.Set("Accept","text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	resp,err:=client.Do(req); if err!=nil{return PageMetadata{},err}; defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return PageMetadata{},fmt.Errorf("destination returned HTTP %d",resp.StatusCode)}
	contentType:=strings.ToLower(resp.Header.Get("Content-Type")); if contentType!=""&&!strings.Contains(contentType,"html"){return PageMetadata{},fmt.Errorf("destination is not HTML")}
	doc,err:=html.Parse(io.LimitReader(resp.Body,2<<20)); if err!=nil{return PageMetadata{},err}
	metadata:=PageMetadata{}
	var walk func(*html.Node)
	walk=func(n *html.Node){
		if n.Type==html.ElementNode {
			switch n.Data {
			case "title": if metadata.Title==""{metadata.Title=cleanText(textContent(n))}
			case "link": if strings.EqualFold(attr(n,"rel"),"canonical")&&metadata.CanonicalURL==""{metadata.CanonicalURL=resolveMetadataURL(resp.Request.URL,attr(n,"href"))}
			case "meta":
				name:=strings.ToLower(attr(n,"name")); property:=strings.ToLower(attr(n,"property")); content:=strings.TrimSpace(attr(n,"content"))
				switch {
				case property=="og:title"&&metadata.Title=="": metadata.Title=content
				case (property=="og:description"||name=="description")&&metadata.Description=="": metadata.Description=content
				case property=="og:image"&&metadata.ImageURL=="": metadata.ImageURL=resolveMetadataURL(resp.Request.URL,content)
				case (name=="author"||property=="article:author")&&metadata.Author=="": metadata.Author=content
				case property=="article:published_time"||name=="date"||name=="datepublished": if metadata.PublishedAt.IsZero(){metadata.PublishedAt=parsePublished(content)}
				}
			}
		}
		for child:=n.FirstChild;child!=nil;child=child.NextSibling{walk(child)}
	}
	walk(doc)
	return metadata,nil
}

func resolveMetadataURL(base *url.URL, raw string) string {
	if strings.TrimSpace(raw)==""{return ""}; parsed,err:=url.Parse(strings.TrimSpace(raw));if err!=nil{return ""};return base.ResolveReference(parsed).String()
}

func parsePublished(value string) time.Time {
	for _,layout:=range []string{time.RFC3339,time.RFC3339Nano,"2006-01-02T15:04:05Z07:00","2006-01-02"}{if parsed,err:=time.Parse(layout,value);err==nil{return parsed}}
	return time.Time{}
}
