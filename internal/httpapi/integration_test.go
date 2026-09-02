package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/editorial"
	"github.com/chrisbirster/trendinary/internal/history"
)

func TestAdminNotesFiltersWorthSharing(t *testing.T) {
	service := testEditorialService(t)
	seedEditorialNote(t, service, "approved", true)
	seedEditorialNote(t, service, "private", false)

	handler := NewAdmin(http.NotFoundHandler(), service, "admin-secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/notes?worth_sharing=true", nil)
	req.SetBasicAuth("admin", "admin-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data []editorial.ContentItem `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].ID != "approved" {
		t.Fatalf("unexpected filtered notes: %+v", envelope.Data)
	}
}

func TestIntegrationNotesRequireBearerAndOnlyExposeApprovedItems(t *testing.T) {
	service := testEditorialService(t)
	seedEditorialNote(t, service, "approved", true)
	seedEditorialNote(t, service, "private", false)

	handler := NewIntegration(http.NotFoundHandler(), service, "machine-secret")

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/integrations/notes", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/notes?worth_sharing=true", nil)
	req.Header.Set("Authorization", "Bearer machine-secret")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var items []publishingNote
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].ID != "approved" || items[0].Title != "approved title" || items[0].URL != "https://example.com/approved" || items[0].ContentType != "article" || items[0].Note != "approved note" {
		t.Fatalf("unexpected publishing item: %+v", items[0])
	}
}

func testEditorialService(t *testing.T) *editorial.Service {
	t.Helper()
	historical, err := history.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = historical.Close() })
	store, err := editorial.NewStore(historical.DB())
	if err != nil {
		t.Fatal(err)
	}
	service, err := editorial.NewService(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func seedEditorialNote(t *testing.T, service *editorial.Service, id string, worthSharing bool) {
	t.Helper()
	now := time.Now().UTC()
	_, _, err := service.Store().UpsertContent(context.Background(), editorial.ContentItem{
		ID:               id,
		CanonicalURL:     "https://example.com/" + id,
		OriginalURL:      "https://example.com/" + id,
		Title:            id + " title",
		Publisher:        "Example",
		PublisherDomain:  "example.com",
		ContentType:      editorial.ContentArticle,
		DiscoveredAt:     now,
		UpdatedAt:        now,
		EnrichmentStatus: editorial.EnrichmentPending,
	}, editorial.Discovery{
		DiscoverySourceID:  "techurls",
		ExternalSourceName: "Example",
		DiscoveredAt:       now,
		MetadataJSON:       "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SetState(context.Background(), id, editorial.StateSaved); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Store().PutNote(context.Background(), editorial.Note{
		ContentItemID: id,
		Text:          id + " note",
		WorthSharing:  &worthSharing,
	}); err != nil {
		t.Fatal(err)
	}
}
