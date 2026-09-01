package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

type Source struct{client *http.Client;apiKey string;region string;limit int}
func New(client *http.Client,apiKey,region string,limit int)*Source{if client==nil{client=&http.Client{Timeout:10*time.Second}};if region==""{region="US"};if limit<=0||limit>50{limit=25};return &Source{client:client,apiKey:strings.TrimSpace(apiKey),region:region,limit:limit}}
func(s *Source)Name()string{return "youtube"}
func(s *Source)Discover(ctx context.Context)([]model.Signal,error){if s.apiKey==""{return nil,fmt.Errorf("YouTube API key is not configured")};query:=url.Values{};query.Set("part","snippet,statistics");query.Set("chart","mostPopular");query.Set("regionCode",s.region);query.Set("videoCategoryId","28");query.Set("maxResults",strconv.Itoa(s.limit));query.Set("key",s.apiKey);endpoint:="https://www.googleapis.com/youtube/v3/videos?"+query.Encode();req,err:=http.NewRequestWithContext(ctx,http.MethodGet,endpoint,nil);if err!=nil{return nil,err};req.Header.Set("User-Agent","Trendinary/0.2 (+https://trendinary.com; public-discovery)");resp,err:=s.client.Do(req);if err!=nil{return nil,err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return nil,fmt.Errorf("youtube returned HTTP %d",resp.StatusCode)};var payload struct{Items []struct{ID string `json:"id"`;Snippet struct{PublishedAt string `json:"publishedAt"`;ChannelTitle string `json:"channelTitle"`;Title string `json:"title"`;Description string `json:"description"`} `json:"snippet"`;Statistics struct{ViewCount string `json:"viewCount"`;LikeCount string `json:"likeCount"`;CommentCount string `json:"commentCount"`} `json:"statistics"`} `json:"items"`};if err:=json.NewDecoder(resp.Body).Decode(&payload);err!=nil{return nil,err};out:=make([]model.Signal,0,len(payload.Items));for _,item:=range payload.Items{views:=atoi(item.Statistics.ViewCount);likes:=atoi(item.Statistics.LikeCount);comments:=atoi(item.Statistics.CommentCount);link:="https://www.youtube.com/watch?v="+url.QueryEscape(item.ID);text:=strings.TrimSpace(item.Snippet.Description);if len(text)>500{text=text[:500]};out=append(out,model.Signal{ID:"youtube:"+item.ID,Source:model.Source{Name:"YouTube",Domain:"youtube.com",URL:link},Title:item.Snippet.Title,Text:text,URL:link,Author:item.Snippet.ChannelTitle,PublishedAt:item.Snippet.PublishedAt,Engagement:model.Engagement{Score:views,Likes:likes,Replies:comments}})};return out,nil}
func atoi(value string)int{n,err:=strconv.ParseInt(value,10,64);if err!=nil||n<0{return 0};max:=int64(^uint(0)>>1);if n>max{return int(max)};return int(n)}
