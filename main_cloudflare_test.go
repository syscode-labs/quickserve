package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCloudflareTokenPrintsConnectorToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`"connector-token"`))
	}))
	defer server.Close()
	oldBaseURL := cloudflareAPIBaseURL
	cloudflareAPIBaseURL = server.URL
	defer func() {
		cloudflareAPIBaseURL = oldBaseURL
	}()

	var out bytes.Buffer
	err := runCloudflareToken(context.Background(), []string{
		"-account-id", "account-123",
		"-tunnel-id", "tunnel-456",
		"-api-token-env", "CF_SETUP",
	}, &out, func(key string) string {
		if key == "CF_SETUP" {
			return "setup-token"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("runCloudflareToken() error = %v", err)
	}
	if out.String() != "connector-token\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCloudflareTokenRejectsMissingSetupTokenEnv(t *testing.T) {
	var out bytes.Buffer
	err := runCloudflareToken(context.Background(), []string{
		"-account-id", "account-123",
		"-tunnel-id", "tunnel-456",
		"-api-token-env", "CF_SETUP",
	}, &out, func(string) string { return "" })
	if err == nil {
		t.Fatal("runCloudflareToken() succeeded unexpectedly")
	}
	if !strings.Contains(err.Error(), "CF_SETUP is not set") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunCloudflareDiscoverPrintsAccountAndTunnelIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zones":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{{
					"id":   "zone-123",
					"name": "syscode.uk",
					"account": map[string]any{
						"id": "account-123",
					},
				}},
			})
		case "/accounts/account-123/tunnels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{{
					"id":     "tunnel-123",
					"name":   "quickserve",
					"status": "healthy",
				}},
			})
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	oldBaseURL := cloudflareAPIBaseURL
	cloudflareAPIBaseURL = server.URL
	defer func() {
		cloudflareAPIBaseURL = oldBaseURL
	}()

	var out bytes.Buffer
	err := runCloudflareDiscover(context.Background(), []string{
		"-hostname", "quickserve.syscode.uk",
		"-tunnel-name", "quickserve",
		"-api-token-env", "CF_SETUP",
	}, &out, func(key string) string {
		if key == "CF_SETUP" {
			return "setup-token"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("runCloudflareDiscover() error = %v", err)
	}
	want := "account-id=account-123\ntunnel-id=tunnel-123\ntunnel-name=quickserve\ntunnel-status=healthy\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}
