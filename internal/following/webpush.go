package following

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const vapidPrivateKeyKV = "webpush-vapid-p256-private-v1"
const vapidSubject = "mailto:alerts@trendinary.com"

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")
var errVAPIDKeyNotInitialized = errors.New("Web Push VAPID key is not initialized; provision it outside normal runtime startup")

type PushSender struct {
	store      *Store
	client     *http.Client
	privateKey []byte
	publicKey  string
}

type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

func blockedPushIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() || cgnatPrefix.Contains(addr)
}

func defaultPushHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Push endpoints are untrusted browser-provided URLs. Do not honor proxy
	// environment variables: every outbound connection must pass the resolver
	// and public-address check below, including redirected requests.
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 6 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid push address: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve push endpoint: %w", err)
		}
		if len(ips) == 0 {
			return nil, errors.New("push endpoint resolved to no addresses")
		}
		for _, ip := range ips {
			if blockedPushIP(ip) {
				return nil, errors.New("push endpoint resolved to a non-public address")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	return &http.Client{
		Transport: transport,
		Timeout:   12 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("too many Web Push redirects")
			}
			return nil
		},
	}
}

func NewPushSender(ctx context.Context, store *Store, client *http.Client) (*PushSender, error) {
	if store == nil {
		return nil, errors.New("following store is required")
	}
	privateKey, err := ensureVAPIDPrivateKey(ctx, store)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = defaultPushHTTPClient()
	}
	key, err := ecdsaPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	public := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	return &PushSender{
		store: store, client: client, privateKey: privateKey,
		publicKey: base64.RawURLEncoding.EncodeToString(public),
	}, nil
}

func (p *PushSender) PublicKey() string {
	if p == nil {
		return ""
	}
	return p.publicKey
}

func ensureVAPIDPrivateKey(ctx context.Context, store *Store) ([]byte, error) {
	if encoded, ok, err := store.KV(ctx, vapidPrivateKeyKV); err != nil {
		return nil, err
	} else if ok {
		privateKey, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(privateKey) != 32 {
			return nil, errors.New("stored VAPID private key is invalid")
		}
		return privateKey, nil
	}
	if store.externallyManagedRuntime() {
		return nil, errVAPIDKeyNotInitialized
	}

	generated, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	privateKey := generated.D.FillBytes(make([]byte, 32))
	if err := store.PutKVIfAbsent(ctx, vapidPrivateKeyKV, base64.RawURLEncoding.EncodeToString(privateKey)); err != nil {
		return nil, err
	}
	encoded, ok, err := store.KV(ctx, vapidPrivateKeyKV)
	if err != nil || !ok {
		return nil, fmt.Errorf("load persisted VAPID key: %w", err)
	}
	return base64.RawURLEncoding.DecodeString(encoded)
}

func ecdsaPrivateKey(raw []byte) (*ecdsa.PrivateKey, error) {
	if len(raw) != 32 {
		return nil, errors.New("P-256 private key must be 32 bytes")
	}
	curve := elliptic.P256()
	d := new(big.Int).SetBytes(raw)
	if d.Sign() <= 0 || d.Cmp(curve.Params().N) >= 0 {
		return nil, errors.New("invalid P-256 private scalar")
	}
	x, y := curve.ScalarBaseMult(raw)
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}, nil
}

func hkdfExtract(salt, input []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	_, _ = mac.Write(input)
	return mac.Sum(nil)
}

func hkdfExpand(prk, info []byte, length int) []byte {
	out := make([]byte, 0, length)
	previous := []byte{}
	for counter := byte(1); len(out) < length; counter++ {
		mac := hmac.New(sha256.New, prk)
		_, _ = mac.Write(previous)
		_, _ = mac.Write(info)
		_, _ = mac.Write([]byte{counter})
		previous = mac.Sum(nil)
		out = append(out, previous...)
	}
	return out[:length]
}

func encryptWebPush(payload []byte, receiverPublic, authSecret []byte) ([]byte, error) {
	if len(authSecret) == 0 {
		return nil, errors.New("push auth secret is empty")
	}
	curve := ecdh.P256()
	receiver, err := curve.NewPublicKey(receiverPublic)
	if err != nil {
		return nil, fmt.Errorf("invalid push p256dh key: %w", err)
	}
	sender, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := sender.ECDH(receiver)
	if err != nil {
		return nil, err
	}
	senderPublic := sender.PublicKey().Bytes()

	keyInfo := make([]byte, 0, len("WebPush: info\x00")+len(receiverPublic)+len(senderPublic))
	keyInfo = append(keyInfo, []byte("WebPush: info\x00")...)
	keyInfo = append(keyInfo, receiverPublic...)
	keyInfo = append(keyInfo, senderPublic...)
	ikm := hkdfExpand(hkdfExtract(authSecret, shared), keyInfo, 32)

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	prk := hkdfExtract(salt, ikm)
	cek := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext := append(append([]byte{}, payload...), 0x02)
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	body := bytes.NewBuffer(make([]byte, 0, 16+4+1+len(senderPublic)+len(ciphertext)))
	body.Write(salt)
	_ = binary.Write(body, binary.BigEndian, uint32(4096))
	body.WriteByte(byte(len(senderPublic)))
	body.Write(senderPublic)
	body.Write(ciphertext)
	return body.Bytes(), nil
}

func vapidAuthorization(endpoint string, privateRaw []byte, publicEncoded string, now time.Time) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("invalid Web Push endpoint")
	}
	audience := parsed.Scheme + "://" + parsed.Host
	headerJSON, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	claimsJSON, _ := json.Marshal(map[string]any{
		"aud": audience,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": vapidSubject,
	})
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	claims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := header + "." + claims
	hash := sha256.Sum256([]byte(unsigned))
	privateKey, err := ecdsaPrivateKey(privateRaw)
	if err != nil {
		return "", err
	}
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return "", err
	}
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	jwt := unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	return "vapid t=" + jwt + ", k=" + publicEncoded, nil
}

func (p *PushSender) SendAlert(ctx context.Context, radarID string, alert Alert) (int, error) {
	if p == nil || p.store == nil {
		return 0, nil
	}
	subscriptions, err := p.store.PushSubscriptions(ctx, radarID)
	if err != nil {
		return 0, err
	}
	payload, _ := json.Marshal(pushPayload{
		Title: alert.Title,
		Body:  alert.Body,
		URL:   "/trend/" + url.PathEscape(alert.Slug),
		Tag:   alert.ID,
	})
	attempts := 0
	var firstErr error
	for _, subscription := range subscriptions {
		attempts++
		if err := p.send(ctx, subscription, payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
	}
	return attempts, firstErr
}

func (p *PushSender) send(ctx context.Context, subscription PushSubscription, payload []byte) error {
	receiverPublic, err := base64.RawURLEncoding.DecodeString(subscription.P256DH)
	if err != nil {
		p.store.PushFailed(ctx, subscription.ID, true)
		return errors.New("invalid subscription p256dh")
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(subscription.Auth)
	if err != nil {
		p.store.PushFailed(ctx, subscription.ID, true)
		return errors.New("invalid subscription auth")
	}
	body, err := encryptWebPush(payload, receiverPublic, authSecret)
	if err != nil {
		p.store.PushFailed(ctx, subscription.ID, true)
		return err
	}
	authorization, err := vapidAuthorization(subscription.Endpoint, p.privateKey, p.publicKey, time.Now().UTC())
	if err != nil {
		p.store.PushFailed(ctx, subscription.ID, true)
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "300")
	res, err := p.client.Do(req)
	if err != nil {
		p.store.PushFailed(ctx, subscription.ID, false)
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		p.store.PushSucceeded(ctx, subscription.ID)
		return nil
	}
	gone := res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone
	p.store.PushFailed(ctx, subscription.ID, gone)
	return fmt.Errorf("push endpoint returned %d %s", res.StatusCode, strings.TrimSpace(res.Status))
}
