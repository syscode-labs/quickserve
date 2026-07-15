package main

import (
	"bytes"
	"context"
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
