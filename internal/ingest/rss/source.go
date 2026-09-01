package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type Source struct{client *http.Client; feeds []string; limit int}
func New(client *http.Client,feeds []string,limit int)*Source{if client==nil{client=&http.Client{Timeout:10*time.Second}};if limit<=0{limit=40};return &Source{client:client,feeds:feeds,limit:limit}}
func(s *Source)Name()string{return "rss/news"}
func(s *Source)Discover(ctx context.Context)([]model.Signal,error){out:=[]model.Signal{};errs:=[]string{};for _,feed:=range s.feeds{feed=strings.TrimSpace(feed);if feed==""{continue};items,err:=s.fetch(ctx,feed);if err!=nil{errs=append(errs,err.Error());continue};out=append(out,items...);if len(out)>=s.limit{out=out[:s.limit];break}};if len(out)==0&&len(errs)>0{return nil,fmt.Errorf("rss discovery: %s",strings.Join(errs,"; "))};return out,nil}

type rssDoc struct{Channel struct{Title string `xml:"title"`;Link string `xml:"link"`;Items []struct{Title string `xml:"title"`;Link string `xml:"link"`;Description string `xml:"description"`;PubDate string `xml:"pubDate"`;Author string `xml:"author"`} `xml:"item"`} `xml:"channel"`}
type atomDoc struct{Title string `xml:"title"`;Entries []struct{Title string `xml:"title"`;Summary string `xml:"summary"`;Updated string `xml:"updated"`;Published string `xml:"published"`;Author struct{Name string `xml:"name"`} `xml:"author"`;Links []struct{Href string `xml:"href"`;Rel string `xml:"rel"`} `xml:"link"`} `xml:"entry"`}
func(s *Source)fetch(ctx context.Context,feed string)([]model.Signal,error){req,err:=http.NewRequestWithContext(ctx,http.MethodGet,feed,nil);if err!=nil{return nil,err};req.Header.Set("User-Agent","Trendinary/0.2 (+https://trendinary.com; public-discovery)");resp,err:=s.client.Do(req);if err!=nil{return nil,err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("%s returned HTTP %d",feed,resp.StatusCode)};data,err:=io.ReadAll(io.LimitReader(resp.Body,4<<20));if err!=nil{return nil,err};var rss rssDoc;if xml.Unmarshal(data,&rss)==nil&&len(rss.Channel.Items)>0{sourceName:=strings.TrimSpace(rss.Channel.Title);if sourceName==""{sourceName=host(feed)};out:=make([]model.Signal,0,len(rss.Channel.Items));for _,item:=range rss.Channel.Items{link:=strings.TrimSpace(item.Link);if link==""{continue};out=append(out,signal(sourceName,link,item.Title,item.Description,item.Author,parseTime(item.PubDate)))};return out,nil};var atom atomDoc;if err:=xml.Unmarshal(data,&atom);err!=nil{return nil,err};sourceName:=strings.TrimSpace(atom.Title);if sourceName==""{sourceName=host(feed)};out:=make([]model.Signal,0,len(atom.Entries));for _,entry:=range atom.Entries{link:="";for _,candidate:=range entry.Links{if candidate.Rel==""||candidate.Rel=="alternate"{link=candidate.Href;break}};if link==""{continue};published:=entry.Published;if published==""{published=entry.Updated};out=append(out,signal(sourceName,link,entry.Title,entry.Summary,entry.Author.Name,parseTime(published)))};return out,nil}
func signal(sourceName,link,title,text,author,published string)model.Signal{sum:=sha256.Sum256([]byte(link));return model.Signal{ID:"rss:"+hex.EncodeToString(sum[:8]),Source:model.Source{Name:sourceName,Domain:host(link),URL:link},Title:strings.TrimSpace(title),Text:strip(text),URL:link,Author:strings.TrimSpace(author),PublishedAt:published}}
func host(raw string)string{u,_:=url.Parse(raw);return strings.ToLower(strings.TrimPrefix(u.Hostname(),"www."))}
func parseTime(value string)string{value=strings.TrimSpace(value);if value==""{return ""};for _,layout:=range []string{time.RFC1123Z,time.RFC1123,time.RFC822Z,time.RFC822,time.RFC3339,time.RFC3339Nano}{if t,err:=time.Parse(layout,value);err==nil{return t.UTC().Format(time.RFC3339)}};return ""}
func strip(value string)string{value=strings.ReplaceAll(value,"<![CDATA[","");value=strings.ReplaceAll(value,"]]>","");if len(value)>500{value=value[:500]};return strings.TrimSpace(value)}
