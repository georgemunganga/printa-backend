package comms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type pushTestTransport func(*http.Request) (*http.Response, error)

func (f pushTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUnconfiguredPushNeverClaimsDelivery(t *testing.T) {
	id, err := (&PushAdapter{}).Send(context.Background(), Message{Recipient: "test-token"})
	if err == nil || id != "" {
		t.Fatalf("unconfigured push reported success: %q", id)
	}
}

func TestPushRequiresProviderAcceptance(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		accepted bool
	}{
		{"unauthorized", 401, `{}`, false},
		{"invalid json", 200, `invalid`, false},
		{"missing receipt", 200, `{}`, false},
		{"rejected token", 200, `{"success":0,"results":[{"error":"NotRegistered"}]}`, false},
		{"accepted", 200, `{"success":1,"results":[{"message_id":"provider-id"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			http.DefaultClient = &http.Client{Transport: pushTestTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			id, err := (&PushAdapter{FCMServerKey: "test-only"}).Send(context.Background(), Message{Recipient: "test-token"})
			if tc.accepted && (err != nil || id != "provider-id") {
				t.Fatalf("accepted response: id=%q err=%v", id, err)
			}
			if !tc.accepted && (err == nil || id != "") {
				t.Fatalf("rejected response reported success: %q", id)
			}
		})
	}
}
