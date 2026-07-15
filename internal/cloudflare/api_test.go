package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestZoneByNameReturnsAccountID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "example.com" {
			t.Fatalf("name query = %q", r.URL.Query().Get("name"))
		}
		if r.URL.Query().Get("per_page") != "1" {
			t.Fatalf("per_page query = %q", r.URL.Query().Get("per_page"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{{
				"id":   "zone-123",
				"name": "example.com",
				"account": map[string]any{
					"id":   "account-123",
					"name": "Example Account",
				},
			}},
		})
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	got, err := client.ZoneByName(context.Background(), "example.com", "setup-token")
	if err != nil {
		t.Fatalf("ZoneByName() error = %v", err)
	}
	if got.Name != "example.com" || got.Account.ID != "account-123" {
		t.Fatalf("ZoneByName() = %#v", got)
	}
}

func TestFindZoneForHostnameTriesParentZones(t *testing.T) {
	var names []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		names = append(names, name)
		result := []map[string]any{}
		if name == "syscode.uk" {
			result = append(result, map[string]any{
				"id":   "zone-123",
				"name": "syscode.uk",
				"account": map[string]any{
					"id": "account-123",
				},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	got, err := client.FindZoneForHostname(context.Background(), "quickserve.syscode.uk", "setup-token")
	if err != nil {
		t.Fatalf("FindZoneForHostname() error = %v", err)
	}
	if got.Name != "syscode.uk" {
		t.Fatalf("zone name = %q", got.Name)
	}
	if strings.Join(names, ",") != "quickserve.syscode.uk,syscode.uk" {
		t.Fatalf("queried names = %#v", names)
	}
}

func TestTunnelsReturnsNamedTunnel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/account-123/tunnels" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "quickserve" {
			t.Fatalf("name query = %q", r.URL.Query().Get("name"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{{
				"id":     "tunnel-123",
				"name":   "quickserve",
				"status": "healthy",
			}},
		})
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL}
	got, err := client.Tunnels(context.Background(), "account-123", "quickserve", "setup-token")
	if err != nil {
		t.Fatalf("Tunnels() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "tunnel-123" || got[0].Name != "quickserve" {
		t.Fatalf("Tunnels() = %#v", got)
	}
}
