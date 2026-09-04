package following

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBlockedPushIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"100.64.0.1",
		"::1",
		"fc00::1",
		"fe80::1",
	}
	for _, value := range blocked {
		if !blockedPushIP(net.ParseIP(value)) {
			t.Fatalf("%s should be blocked for Web Push", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if blockedPushIP(net.ParseIP(value)) {
			t.Fatalf("%s should be allowed as a public Web Push destination", value)
		}
	}
}

func TestDefaultPushClientRejectsLoopbackBeforeDial(t *testing.T) {
	client := defaultPushHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://127.0.0.1/push", strings.NewReader("payload"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req)
	if err == nil {
		t.Fatal("default Web Push client allowed loopback destination")
	}
	if !strings.Contains(err.Error(), "non-public address") {
		t.Fatalf("loopback rejection = %q", err)
	}
}
