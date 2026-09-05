package following

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
)

func decryptPushBody(t *testing.T, body, receiverPublic []byte, receiver *ecdh.PrivateKey, auth []byte) []byte {
	t.Helper()
	if len(body) < 22 {
		t.Fatalf("encrypted body too short: %d", len(body))
	}
	salt := body[:16]
	if size := binary.BigEndian.Uint32(body[16:20]); size != 4096 {
		t.Fatalf("record size=%d", size)
	}
	keyLength := int(body[20])
	if keyLength <= 0 || 21+keyLength >= len(body) {
		t.Fatalf("sender key length=%d", keyLength)
	}
	senderPublicBytes := body[21 : 21+keyLength]
	senderPublic, err := ecdh.P256().NewPublicKey(senderPublicBytes)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := receiver.ECDH(senderPublic)
	if err != nil {
		t.Fatal(err)
	}
	info := append([]byte("WebPush: info\x00"), receiverPublic...)
	info = append(info, senderPublicBytes...)
	ikm := hkdfExpand(hkdfExtract(auth, shared), info, 32)
	prk := hkdfExtract(salt, ikm)
	cek := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)
	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := gcm.Open(nil, nonce, body[21+keyLength:], nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) == 0 || plaintext[len(plaintext)-1] != 0x02 {
		t.Fatalf("invalid aes128gcm record delimiter: %x", plaintext)
	}
	return plaintext[:len(plaintext)-1]
}

func TestSuccessfulPushCarriesEncryptedTrendPayload(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	key, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	radarID, _ := RadarID(key)

	receiver, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}

	var requestBody []byte
	var requestHeaders http.Header
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders = r.Header.Clone()
		requestBody, _ = io.ReadAll(io.LimitReader(r.Body, 64<<10))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	_, err = store.RebindPushSubscription(ctx, radarID, PushSubscription{
		Endpoint: server.URL,
		P256DH:   base64.RawURLEncoding.EncodeToString(receiver.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
	})
	if err != nil {
		t.Fatal(err)
	}
	sender, err := NewPushSender(ctx, store, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := sender.SendAlert(ctx, radarID, Alert{ID: "alert-1", Slug: "audacity-4", Title: "Audacity accelerated", Body: "Independent coverage expanded."})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("push attempts=%d, want 1", attempts)
	}
	if requestHeaders.Get("Content-Encoding") != "aes128gcm" || requestHeaders.Get("TTL") != "300" {
		t.Fatalf("unexpected push headers: encoding=%q ttl=%q", requestHeaders.Get("Content-Encoding"), requestHeaders.Get("TTL"))
	}
	if requestHeaders.Get("Authorization") == "" {
		t.Fatal("missing VAPID authorization")
	}
	plaintext := decryptPushBody(t, requestBody, receiver.PublicKey().Bytes(), receiver, auth)
	var payload pushPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.URL != "/trend/audacity-4" || payload.Tag != "alert-1" || payload.Title != "Audacity accelerated" {
		t.Fatalf("push payload=%+v", payload)
	}
}

func TestConcurrentEvaluatorsDeduplicateAlerts(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	key, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	radarID, _ := RadarID(key)
	if _, err := store.AddFollow(ctx, radarID, "trend", "audacity-4", "Audacity 4.0"); err != nil {
		t.Fatal(err)
	}
	provider := &mutableTrends{values: []model.Trend{trendFixture("EMERGING", 30, .20, .10, 1)}}
	service := NewService(store, provider, nil)
	if _, err := service.EvaluateRadar(ctx, radarID); err != nil {
		t.Fatal(err)
	}
	provider.values = []model.Trend{trendFixture("BREAKING", 70, .80, .70, 4)}

	const workers = 8
	results := make(chan Evaluation, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.EvaluateRadar(ctx, radarID)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	inserted := 0
	for result := range results {
		inserted += result.Alerts
	}
	alerts, err := store.Alerts(ctx, radarID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts) != 3 || inserted != 3 {
		t.Fatalf("concurrent evaluation inserted=%d persisted=%d, want exactly 3 unique alerts", inserted, len(alerts))
	}
}

func TestEvaluateHundredsOfRadarsInOnePass(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	const radars = 200
	for index := 0; index < radars; index++ {
		key, _, err := store.CreateRadar(ctx)
		if err != nil {
			t.Fatal(err)
		}
		radarID, _ := RadarID(key)
		if _, err := store.AddFollow(ctx, radarID, "topic", "tech", "Tech"); err != nil {
			t.Fatal(err)
		}
	}
	provider := &mutableTrends{values: []model.Trend{trendFixture("EMERGING", 30, .20, .10, 1)}}
	service := NewService(store, provider, nil)
	result, err := service.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Radars != radars || result.Follows != radars || result.Matches != radars || result.Alerts != 0 {
		t.Fatalf("initial fleet evaluation=%+v", result)
	}
	provider.values = []model.Trend{trendFixture("BREAKING", 70, .80, .70, 4)}
	result, err = service.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Alerts != radars*3 {
		t.Fatalf("fleet alerts=%d, want %d", result.Alerts, radars*3)
	}
}

func TestDeleteRadarErasesDurableState(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	key, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	radarID, _ := RadarID(key)
	follow, err := store.AddFollow(ctx, radarID, "trend", "audacity-4", "Audacity")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutBaseline(ctx, Baseline{FollowID: follow.ID, TrendKey: "trend-audacity", Slug: "audacity-4", Lifecycle: "RISING", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAlert(ctx, radarID, "fingerprint", Alert{ID: "alert", FollowID: follow.ID, TrendKey: "trend-audacity", Slug: "audacity-4", Name: "Audacity", Kind: "lifecycle", Title: "Changed", Body: "Changed", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RebindPushSubscription(ctx, radarID, PushSubscription{Endpoint: "https://push.example.test/sub", P256DH: "key", Auth: "auth"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRadar(ctx, radarID); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name  string
		query string
		args  []any
	}{
		{"radar", `SELECT COUNT(*) FROM following_radars WHERE radar_id=?`, []any{radarID}},
		{"follow", `SELECT COUNT(*) FROM following_follows WHERE radar_id=?`, []any{radarID}},
		{"baseline", `SELECT COUNT(*) FROM following_baselines WHERE follow_id=?`, []any{follow.ID}},
		{"alert", `SELECT COUNT(*) FROM following_alerts WHERE radar_id=?`, []any{radarID}},
		{"push", `SELECT COUNT(*) FROM following_push_subscriptions WHERE radar_id=?`, []any{radarID}},
	}
	for _, check := range checks {
		var count int
		if err := store.db.QueryRowContext(ctx, check.query, check.args...).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows remaining=%d", check.name, count)
		}
	}
}
