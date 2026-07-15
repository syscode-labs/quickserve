package cloudflare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTunnelTokenFetchesConnectorToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/accounts/account-123/cfd_tunnel/tunnel-456/token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`"connector-token"`))
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	got, err := client.TunnelToken(context.Background(), "account-123", "tunnel-456", "setup-token")
	if err != nil {
		t.Fatalf("TunnelToken() error = %v", err)
	}
	if got != "connector-token" {
		t.Fatalf("TunnelToken() = %q", got)
	}
	if gotAuth != "Bearer setup-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestTunnelTokenReportsCloudflareError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"errors":[{"message":"not allowed"}]}`, http.StatusForbidden)
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	_, err := client.TunnelToken(context.Background(), "account-123", "tunnel-456", "setup-token")
	if err == nil {
		t.Fatal("TunnelToken() succeeded unexpectedly")
	}
}
