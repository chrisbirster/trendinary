package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type Source struct{client *http.Client;limit int;now func()time.Time}
func New(client *http.Client,limit int)*Source{if client==nil{client=&http.Client{Timeout:10*time.Second}};if limit<=0{limit=40};return &Source{client:client,limit:limit,now:func()time.Time{return time.Now().UTC()}}}
func(s *Source)Name()string{return "wikipedia"}
func(s *Source)Discover(ctx context.Context)([]model.Signal,error){day:=s.now().UTC().Add(-24*time.Hour);endpoint:=fmt.Sprintf("https://wikimedia.org/api/rest_v1/metrics/pageviews/top/en.wikipedia/all-access/%04d/%02d/%02d",day.Year(),day.Month(),day.Day());req,err:=http.NewRequestWithContext(ctx,http.MethodGet,endpoint,nil);if err!=nil{return nil,err};req.Header.Set("User-Agent","Trendinary/0.2 (+https://trendinary.com; public-discovery)");resp,err:=s.client.Do(req);if err!=nil{return nil,err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("wikipedia pageviews returned HTTP %d",resp.StatusCode)};var payload struct{Items []struct{Articles []struct{Article string `json:"article"`;Views int `json:"views"`;Rank int `json:"rank"`} `json:"articles"`} `json:"items"`};if err:=json.NewDecoder(resp.Body).Decode(&payload);err!=nil{return nil,err};if len(payload.Items)==0{return nil,nil};out:=[]model.Signal{};for _,item:=range payload.Items[0].Articles{if len(out)>=s.limit{break};title:=strings.ReplaceAll(item.Article,"_"," ");if skip(title){continue};link:="https://en.wikipedia.org/wiki/"+url.PathEscape(item.Article);out=append(out,model.Signal{ID:"wikipedia:"+item.Article,Source:model.Source{Name:"Wikipedia",Domain:"wikipedia.org",URL:link},Title:title,Text:fmt.Sprintf("Top English Wikipedia page by pageviews (rank %d).",item.Rank),URL:link,PublishedAt:day.Format(time.RFC3339),Engagement:model.Engagement{Score:item.Views}})};return out,nil}
func skip(title string)bool{lower:=strings.ToLower(title);return lower=="main page"||strings.HasPrefix(lower,"special:")||strings.HasPrefix(lower,"wikipedia:")||strings.HasPrefix(lower,"portal:")||strings.Contains(lower,"404.php")}
