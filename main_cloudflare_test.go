package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/syscode-labs/quickserve/internal/app"
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

func TestRunCloudflareRouteConfiguresIngressAndDNS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/zones":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{{
					"id":   "zone-123",
					"name": "example.com",
					"account": map[string]any{
						"id": "account-123",
					},
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/accounts/account-123/tunnels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{{
					"id":     "tunnel-123",
					"name":   "quickserve",
					"status": "healthy",
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/accounts/account-123/cfd_tunnel/tunnel-123/configurations":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"config": map[string]any{"ingress": []map[string]any{{"service": "http_status:404"}}},
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/accounts/account-123/cfd_tunnel/tunnel-123/configurations":
			var req struct {
				Config struct {
					Ingress []map[string]string `json:"ingress"`
				} `json:"config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if len(req.Config.Ingress) != 2 || req.Config.Ingress[0]["hostname"] != "quickserve.example.com" || req.Config.Ingress[0]["service"] != "http://localhost:8000" {
				t.Fatalf("ingress request = %#v", req.Config.Ingress)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"config": map[string]any{"ingress": req.Config.Ingress}}})
		case r.Method == http.MethodGet && r.URL.Path == "/zones/zone-123/dns_records":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/zones/zone-123/dns_records":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{
				"id":      "record-123",
				"type":    "CNAME",
				"name":    "quickserve.example.com",
				"content": "tunnel-123.cfargotunnel.com",
				"proxied": true,
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	oldBaseURL := cloudflareAPIBaseURL
	cloudflareAPIBaseURL = server.URL
	defer func() {
		cloudflareAPIBaseURL = oldBaseURL
	}()

	var out bytes.Buffer
	err := runCloudflareRoute(context.Background(), []string{
		"-hostname", "quickserve.example.com",
		"-tunnel-name", "quickserve",
		"-service", "http://localhost:8000",
		"-api-token-env", "CF_SETUP",
	}, &out, func(key string) string {
		if key == "CF_SETUP" {
			return "setup-token"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("runCloudflareRoute() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Cloudflare route configured.",
		"This setup command exits after updating Cloudflare.",
		"Hostname: https://quickserve.example.com/",
		"Origin service: http://localhost:8000",
		"Next: run quickserve -port 8000 and keep it running.",
		"dns-record-id: record-123",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestPrepareCloudflareServeModeDisablesManagedTunnel(t *testing.T) {
	cfg := app.Config{
		Port:                 8000,
		Tunnel:               "cloudflare",
		TunnelHostname:       "quickserve.syscode.uk",
		TunnelTokenEnv:       "CLOUDFLARE_TOKEN_QUICKSERVE",
		CloudflareHostname:   "quickserve.syscode.uk",
		CloudflareTokenEnv:   "CLOUDFLARE_TOKEN_SYSCODE",
		CloudflareTunnelName: "quickserve",
	}
	got, route, enabled, err := prepareCloudflareServeMode(cfg)
	if err != nil {
		t.Fatalf("prepareCloudflareServeMode() error = %v", err)
	}
	if !enabled {
		t.Fatal("cloudflare serve mode not enabled")
	}
	if got.Tunnel != "" || got.TunnelHostname != "" || got.TunnelTokenEnv != "" {
		t.Fatalf("managed tunnel settings were not cleared: %+v", got)
	}
	if route.Hostname != "quickserve.syscode.uk" || route.TunnelName != "quickserve" || route.Service != "http://localhost:8000" || route.APITokenEnv != "CLOUDFLARE_TOKEN_SYSCODE" {
		t.Fatalf("route = %+v", route)
	}
}

func TestPrepareCloudflareServeModeRejectsPortZero(t *testing.T) {
	_, _, _, err := prepareCloudflareServeMode(app.Config{
		Port:               0,
		CloudflareHostname: "quickserve.syscode.uk",
		CloudflareTokenEnv: "CLOUDFLARE_TOKEN_SYSCODE",
	})
	if err == nil {
		t.Fatal("prepareCloudflareServeMode() accepted port 0")
	}
}
