package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chrisbirster/trendinary/internal/editorial"
)

type adminServer struct {
	service  *editorial.Service
	password string
	mux      *http.ServeMux
}

// NewAdmin layers the private editorial API and /admin guard in front of the
// public Trendinary handler. Personal editorial data never enters public APIs.
func NewAdmin(next http.Handler, service *editorial.Service, password string) http.Handler {
	if next == nil { next = http.NotFoundHandler() }
	server:=&adminServer{service:service,password:password,mux:http.NewServeMux()}
	server.routes()
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if r.URL.Path=="/admin"||strings.HasPrefix(r.URL.Path,"/admin/")||strings.HasPrefix(r.URL.Path,"/api/v1/admin/") {
			if !server.authorized(w,r){return}
			if strings.HasPrefix(r.URL.Path,"/api/v1/admin/"){server.mux.ServeHTTP(w,r);return}
		}
		next.ServeHTTP(w,r)
	})
}

func (s *adminServer) routes(){
	s.mux.HandleFunc("GET /api/v1/admin/inbox",s.list(StateQuery{State:editorial.StateInbox}))
	s.mux.HandleFunc("GET /api/v1/admin/queue",s.list(StateQuery{State:editorial.StateQueued}))
	s.mux.HandleFunc("GET /api/v1/admin/notes",s.notes)
	s.mux.HandleFunc("GET /api/v1/admin/trash",s.list(StateQuery{State:editorial.StateRejected}))
	s.mux.HandleFunc("GET /api/v1/admin/content/{id}",s.content)
	s.mux.HandleFunc("POST /api/v1/admin/content/{id}/open",s.openContent)
	s.mux.HandleFunc("POST /api/v1/admin/content/{id}/enrich",s.enrichContent)
	s.mux.HandleFunc("PATCH /api/v1/admin/content/{id}/state",s.state)
	s.mux.HandleFunc("PUT /api/v1/admin/content/{id}/note",s.note)
	s.mux.HandleFunc("GET /api/v1/admin/sources",s.sources)
	s.mux.HandleFunc("PATCH /api/v1/admin/sources/{source}",s.sourceState)
	s.mux.HandleFunc("POST /api/v1/admin/sources/{source}/ingest",s.ingest)
	s.mux.HandleFunc("GET /api/v1/admin/sources/{source}/runs",s.runs)
	s.mux.HandleFunc("GET /api/v1/admin/issues",s.issues)
	s.mux.HandleFunc("GET /api/v1/admin/issues/suggestions",s.issueSuggestions)
	s.mux.HandleFunc("POST /api/v1/admin/issues",s.createIssue)
	s.mux.HandleFunc("GET /api/v1/admin/issues/{id}",s.issue)
	s.mux.HandleFunc("PUT /api/v1/admin/issues/{id}",s.updateIssue)
	s.mux.HandleFunc("POST /api/v1/admin/issues/{id}/items",s.addIssueItem)
	s.mux.HandleFunc("DELETE /api/v1/admin/issues/{id}/items/{itemId}",s.removeIssueItem)
}

func (s *adminServer) authorized(w http.ResponseWriter,r *http.Request)bool{
	if s.service==nil {writeJSON(w,http.StatusServiceUnavailable,map[string]any{"error":"editorial admin is not configured"});return false}
	if s.password=="" {writeJSON(w,http.StatusServiceUnavailable,map[string]any{"error":"TRENDINARY_ADMIN_PASSWORD is required to enable /admin"});return false}
	user,password,ok:=r.BasicAuth()
	validUser:=subtle.ConstantTimeCompare([]byte(user),[]byte("admin"))==1
	validPassword:=subtle.ConstantTimeCompare([]byte(password),[]byte(s.password))==1
	if !ok||!validUser||!validPassword {w.Header().Set("WWW-Authenticate",`Basic realm="Trendinary Admin", charset="UTF-8"`);writeJSON(w,http.StatusUnauthorized,map[string]any{"error":"admin authentication required"});return false}
	return true
}

type StateQuery struct{State editorial.State}

func (s *adminServer) list(base StateQuery) http.HandlerFunc{return func(w http.ResponseWriter,r *http.Request){
	contentType:=editorial.ContentType(strings.TrimSpace(r.URL.Query().Get("type")))
	serendipity:=r.URL.Query().Get("serendipity")=="1"||r.URL.Query().Get("serendipity")=="true"
	sortBy:=r.URL.Query().Get("sort")
	var since time.Time
	switch r.URL.Query().Get("period"){case "today":since=time.Now().UTC().Add(-24*time.Hour);case "week":since=time.Now().UTC().Add(-7*24*time.Hour)}
	values,err:=s.service.Store().ListContent(r.Context(),base.State,contentType,serendipity,sortBy,since,200)
	if err!=nil{writeJSON(w,http.StatusInternalServerError,map[string]any{"error":"editorial content unavailable"});return}
	w.Header().Set("Cache-Control","no-store");writeJSON(w,http.StatusOK,map[string]any{"data":values})
}}

func(s *adminServer) notes(w http.ResponseWriter,r *http.Request){values,err:=s.service.Store().ListContent(r.Context(),"","",false,"newest",time.Time{},500);if err!=nil{writeJSON(w,500,map[string]any{"error":"notes unavailable"});return};out:=make([]editorial.ContentItem,0);for _,item:=range values{if item.Note!=nil&&(item.State==editorial.StateConsumed||item.State==editorial.StateSaved){out=append(out,item)}};writeJSON(w,200,map[string]any{"data":out})}
func(s *adminServer) content(w http.ResponseWriter,r *http.Request){value,ok,err:=s.service.Store().Content(r.Context(),r.PathValue("id"));if err!=nil{writeJSON(w,500,map[string]any{"error":"content unavailable"});return};if !ok{writeJSON(w,404,map[string]any{"error":"content not found"});return};writeJSON(w,200,map[string]any{"data":value})}
func(s *adminServer) openContent(w http.ResponseWriter,r *http.Request){if err:=s.service.Store().MarkOpened(r.Context(),r.PathValue("id"));err!=nil{writeJSON(w,500,map[string]any{"error":"could not mark opened"});return};s.content(w,r)}
func(s *adminServer) enrichContent(w http.ResponseWriter,r *http.Request){value,err:=s.service.Enrich(r.Context(),r.PathValue("id"));if err!=nil{writeJSON(w,502,map[string]any{"error":err.Error()});return};writeJSON(w,200,map[string]any{"data":value})}
func(s *adminServer) state(w http.ResponseWriter,r *http.Request){var body struct{State editorial.State `json:"state"`};if json.NewDecoder(r.Body).Decode(&body)!=nil{writeJSON(w,400,map[string]any{"error":"invalid JSON"});return};if err:=s.service.Store().SetState(r.Context(),r.PathValue("id"),body.State);err!=nil{writeJSON(w,400,map[string]any{"error":err.Error()});return};s.content(w,r)}
func(s *adminServer) note(w http.ResponseWriter,r *http.Request){var body struct{Note string `json:"note"`;WorthSharing *bool `json:"worth_sharing"`};if json.NewDecoder(r.Body).Decode(&body)!=nil{writeJSON(w,400,map[string]any{"error":"invalid JSON"});return};value,err:=s.service.Store().PutNote(r.Context(),editorial.Note{ContentItemID:r.PathValue("id"),Text:body.Note,WorthSharing:body.WorthSharing});if err!=nil{writeJSON(w,500,map[string]any{"error":"note unavailable"});return};writeJSON(w,200,map[string]any{"data":value})}
func(s *adminServer) sources(w http.ResponseWriter,r *http.Request){values,err:=s.service.Store().Sources(r.Context());if err!=nil{writeJSON(w,500,map[string]any{"error":"sources unavailable"});return};writeJSON(w,200,map[string]any{"data":values})}
func(s *adminServer) sourceState(w http.ResponseWriter,r *http.Request){var body struct{Enabled bool `json:"enabled"`};if json.NewDecoder(r.Body).Decode(&body)!=nil{writeJSON(w,400,map[string]any{"error":"invalid JSON"});return};if err:=s.service.Store().SetSourceEnabled(r.Context(),r.PathValue("source"),body.Enabled);err!=nil{writeJSON(w,500,map[string]any{"error":"source update failed"});return};s.sources(w,r)}
func(s *adminServer) ingest(w http.ResponseWriter,r *http.Request){run,err:=s.service.Ingest(r.Context(),r.PathValue("source"));if err!=nil{writeJSON(w,502,map[string]any{"error":err.Error(),"data":run});return};writeJSON(w,200,map[string]any{"data":run})}
func(s *adminServer) runs(w http.ResponseWriter,r *http.Request){values,err:=s.service.Store().Runs(r.Context(),r.PathValue("source"),20);if err!=nil{writeJSON(w,500,map[string]any{"error":"ingestion runs unavailable"});return};writeJSON(w,200,map[string]any{"data":values})}
func(s *adminServer) issues(w http.ResponseWriter,r *http.Request){values,err:=s.service.Store().Issues(r.Context());if err!=nil{writeJSON(w,500,map[string]any{"error":"issues unavailable"});return};writeJSON(w,200,map[string]any{"data":values})}
func(s *adminServer) issue(w http.ResponseWriter,r *http.Request){value,ok,err:=s.service.Store().Issue(r.Context(),r.PathValue("id"));if err!=nil{writeJSON(w,500,map[string]any{"error":"issue unavailable"});return};if !ok{writeJSON(w,404,map[string]any{"error":"issue not found"});return};writeJSON(w,200,map[string]any{"data":value})}
func(s *adminServer) createIssue(w http.ResponseWriter,r *http.Request){var body editorial.NewsletterIssue;if json.NewDecoder(r.Body).Decode(&body)!=nil{writeJSON(w,400,map[string]any{"error":"invalid JSON"});return};value,err:=s.service.Store().PutIssue(r.Context(),body);if err!=nil{writeJSON(w,400,map[string]any{"error":err.Error()});return};writeJSON(w,201,map[string]any{"data":value})}
func(s *adminServer) updateIssue(w http.ResponseWriter,r *http.Request){var body editorial.NewsletterIssue;if json.NewDecoder(r.Body).Decode(&body)!=nil{writeJSON(w,400,map[string]any{"error":"invalid JSON"});return};body.ID=r.PathValue("id");if existing,ok,_:=s.service.Store().Issue(r.Context(),body.ID);ok{body.CreatedAt=existing.CreatedAt};value,err:=s.service.Store().PutIssue(r.Context(),body);if err!=nil{writeJSON(w,400,map[string]any{"error":err.Error()});return};writeJSON(w,200,map[string]any{"data":value})}
func(s *adminServer) addIssueItem(w http.ResponseWriter,r *http.Request){var body editorial.NewsletterIssueItem;if json.NewDecoder(r.Body).Decode(&body)!=nil{writeJSON(w,400,map[string]any{"error":"invalid JSON"});return};body.IssueID=r.PathValue("id");value,err:=s.service.Store().AddIssueItem(r.Context(),body);if err!=nil{writeJSON(w,400,map[string]any{"error":err.Error()});return};writeJSON(w,201,map[string]any{"data":value})}
func(s *adminServer) removeIssueItem(w http.ResponseWriter,r *http.Request){if err:=s.service.Store().RemoveIssueItem(r.Context(),r.PathValue("id"),r.PathValue("itemId"));err!=nil{writeJSON(w,500,map[string]any{"error":"remove failed"});return};w.WriteHeader(http.StatusNoContent)}
func(s *adminServer) issueSuggestions(w http.ResponseWriter,r *http.Request){values,err:=s.service.Store().ListContent(r.Context(),"","",false,"score",time.Time{},500);if err!=nil{writeJSON(w,500,map[string]any{"error":"suggestions unavailable"});return};out:=make([]editorial.ContentItem,0);for _,item:=range values{if (item.State==editorial.StateSaved||item.State==editorial.StateConsumed)&&item.Note!=nil{out=append(out,item);if len(out)>=20{break}}};writeJSON(w,200,map[string]any{"data":out,"meta":map[string]any{"rule":"Only saved/consumed items with your notes are suggested; no opinions are fabricated."}})}

var _ = errors.Is
