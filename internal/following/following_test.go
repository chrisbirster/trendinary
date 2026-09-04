package following

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chrisbirster/trendinary/internal/model"
	_ "modernc.org/sqlite"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestRadarKeyIsStoredOnlyAsHash(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	key, state, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Follows) != 0 || len(state.Alerts) != 0 {
		t.Fatalf("new radar state = %+v", state)
	}
	id, err := RadarID(key)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := store.db.QueryRowContext(ctx, `SELECT radar_id FROM following_radars`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != id {
		t.Fatalf("stored radar id = %q, want hash %q", stored, id)
	}
	if stored == key {
		t.Fatal("raw Radar Key was persisted")
	}
}

func TestPushEndpointRebindMovesBrowserToNewRadar(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	firstKey, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstID, _ := RadarID(firstKey)
	secondID, _ := RadarID(secondKey)
	sub := PushSubscription{Endpoint: "https://push.example.test/sub", P256DH: "key", Auth: "auth"}
	if _, err := store.RebindPushSubscription(ctx, firstID, sub); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RebindPushSubscription(ctx, secondID, sub); err != nil {
		t.Fatal(err)
	}
	first, err := store.PushSubscriptions(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.PushSubscriptions(ctx, secondID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 0 || len(second) != 1 {
		t.Fatalf("push ownership after rebind: first=%d second=%d", len(first), len(second))
	}
}

type mutableTrends struct{ values []model.Trend }

func (m *mutableTrends) Trends() []model.Trend { return append([]model.Trend(nil), m.values...) }

func trendFixture(status string, score int, velocity, breadth float64, sources int) model.Trend {
	out := model.Trend{
		ID: "trend-audacity", Slug: "audacity-4", Name: "Audacity 4.0", Category: "TECH",
		Status: status, Score: score, Reason: "Audacity ships a major audio editor update",
		Quality: model.TrendQuality{Velocity: velocity, SourceBreadth: breadth},
	}
	for index := 0; index < sources; index++ {
		out.Sources = append(out.Sources, model.Source{Name: "Publisher", Domain: "publisher" + string(rune('a'+index)) + ".test"})
	}
	return out
}

func TestEvaluateEstablishesBaselineThenAlertsOnce(t *testing.T) {
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

	first, err := service.EvaluateRadar(ctx, radarID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Alerts != 0 {
		t.Fatalf("initial observation emitted %d alerts", first.Alerts)
	}

	provider.values = []model.Trend{trendFixture("BREAKING", 55, .70, .55, 3)}
	second, err := service.EvaluateRadar(ctx, radarID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Alerts < 2 {
		t.Fatalf("material change emitted only %d alerts", second.Alerts)
	}
	persisted, err := store.Alerts(ctx, radarID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != second.Alerts {
		t.Fatalf("persisted alerts=%d evaluation=%d", len(persisted), second.Alerts)
	}

	third, err := service.EvaluateRadar(ctx, radarID)
	if err != nil {
		t.Fatal(err)
	}
	if third.Alerts != 0 {
		t.Fatalf("unchanged observation emitted %d duplicate alerts", third.Alerts)
	}
}

func TestTopicAndEntityFollowsMatchFutureClusters(t *testing.T) {
	trend := model.Trend{Slug: "new-artemis-mission", ID: "trend-artemis", Name: "Artemis III", Category: "SPACE", Reason: "NASA updates the Moon mission schedule", Aliases: []string{"Artemis program"}}
	cases := []Follow{
		{Kind: "topic", Value: "space"},
		{Kind: "topic", Value: "moon mission"},
		{Kind: "entity", Value: "artemis"},
	}
	for _, follow := range cases {
		if !matchesFollow(follow, trend) {
			t.Fatalf("%s follow %q did not match future cluster", follow.Kind, follow.Value)
		}
	}
	if matchesFollow(Follow{Kind: "entity", Value: "openai"}, trend) {
		t.Fatal("unrelated entity matched trend")
	}
}

func TestSensitivityChangesAccelerationThreshold(t *testing.T) {
	follow := Follow{ID: "follow", Kind: "trend", Value: "audacity-4", DisplayName: "Audacity"}
	previous := Baseline{FollowID: follow.ID, TrendKey: "trend-audacity", Lifecycle: "RISING", Score: 40, Velocity: .30, SourceBreadth: .20, SourceCount: 1, ObservedAt: time.Now().Add(-time.Minute).Format(time.RFC3339Nano)}
	trend := trendFixture("RISING", 52, .40, .20, 1)
	now := time.Now()
	early := detectAlerts(follow, trend, previous, Preferences{Sensitivity: "early", Velocity: true}, now)
	balanced := detectAlerts(follow, trend, previous, Preferences{Sensitivity: "balanced", Velocity: true}, now)
	if len(early) == 0 {
		t.Fatal("early sensitivity missed a 12-point score jump")
	}
	if len(balanced) != 0 {
		t.Fatalf("balanced sensitivity emitted %d alerts for a 12-point score jump", len(balanced))
	}
}

func TestVAPIDKeyPersistsAcrossSenderRestarts(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	first, err := NewPushSender(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPushSender(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.PublicKey() == "" || first.PublicKey() != second.PublicKey() {
		t.Fatalf("VAPID public keys differ: %q vs %q", first.PublicKey(), second.PublicKey())
	}
}

func TestAES128GCMWebPushRoundTrip(t *testing.T) {
	curve := ecdh.P256()
	receiver, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"title":"test","body":"hello"}`)
	body, err := encryptWebPush(payload, receiver.PublicKey().Bytes(), auth)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) < 22 {
		t.Fatalf("encrypted body too short: %d", len(body))
	}
	salt := body[:16]
	recordSize := binary.BigEndian.Uint32(body[16:20])
	if recordSize != 4096 {
		t.Fatalf("record size=%d", recordSize)
	}
	keyLength := int(body[20])
	if keyLength <= 0 || 21+keyLength >= len(body) {
		t.Fatalf("sender key length=%d", keyLength)
	}
	senderPublicBytes := body[21 : 21+keyLength]
	senderPublic, err := curve.NewPublicKey(senderPublicBytes)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := receiver.ECDH(senderPublic)
	if err != nil {
		t.Fatal(err)
	}
	info := append([]byte("WebPush: info\x00"), receiver.PublicKey().Bytes()...)
	info = append(info, senderPublicBytes...)
	ikm := hkdfExpand(hkdfExtract(auth, shared), info, 32)
	prk := hkdfExtract(salt, ikm)
	cek := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)
	block, err := aesNewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := openGCM(block, nonce, body[21+keyLength:])
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) != len(payload)+1 || plaintext[len(plaintext)-1] != 0x02 || string(plaintext[:len(payload)]) != string(payload) {
		t.Fatalf("decrypted payload=%q", plaintext)
	}
}

// Wrappers keep the round-trip test focused without duplicating production key
// derivation. They are tiny enough to make failures point at encryption rather
// than HTTP delivery.
func aesNewCipher(key []byte) (cipherBlock, error) { return newAESBlock(key) }

type cipherBlock interface {
	BlockSize() int
	Encrypt(dst, src []byte)
	Decrypt(dst, src []byte)
}

func openGCM(block cipherBlock, nonce, ciphertext []byte) ([]byte, error) {
	return openAESGCM(block, nonce, ciphertext)
}

func TestGonePushEndpointIsDisabled(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	key, _, err := store.CreateRadar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	radarID, _ := RadarID(key)
	curve := ecdh.P256()
	receiver, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	_, err = store.RebindPushSubscription(ctx, radarID, PushSubscription{
		Endpoint: server.URL,
		P256DH: base64.RawURLEncoding.EncodeToString(receiver.PublicKey().Bytes()),
		Auth: base64.RawURLEncoding.EncodeToString(auth),
	})
	if err != nil {
		t.Fatal(err)
	}
	sender, err := NewPushSender(ctx, store, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = sender.SendAlert(ctx, radarID, Alert{ID: "a", Slug: "audacity-4", Title: "Changed", Body: "Body"})
	active, err := store.PushSubscriptions(ctx, radarID)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("gone push endpoint remained active: %d", len(active))
	}
}
