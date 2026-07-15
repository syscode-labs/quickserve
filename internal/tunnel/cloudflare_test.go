package tunnel

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScanForURLFindsTryCloudflareURL(t *testing.T) {
	input := strings.NewReader("info\nYour quick Tunnel has been created! Visit it at https://blue-river-123.trycloudflare.com\n")
	got, err := scanForURL(input, "")
	if err != nil {
		t.Fatalf("scanForURL() error = %v", err)
	}
	if got != "https://blue-river-123.trycloudflare.com" {
		t.Fatalf("scanForURL() = %q", got)
	}
}

func TestScanForURLReportsMissingURLWithOutput(t *testing.T) {
	_, err := scanForURL(strings.NewReader("failed to connect\n"), "")
	if err == nil {
		t.Fatal("scanForURL() succeeded unexpectedly")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Fatalf("error does not include recent output: %v", err)
	}
}

func TestScanForURLReturnsCustomHostnameAfterReadyLine(t *testing.T) {
	input := strings.NewReader("INF Registered tunnel connection connIndex=0\n")
	got, err := scanForURL(input, "share.example.com")
	if err != nil {
		t.Fatalf("scanForURL() error = %v", err)
	}
	if got != "https://share.example.com" {
		t.Fatalf("scanForURL() = %q", got)
	}
}

func TestCloudflaredArgsUseQuickTunnelByDefault(t *testing.T) {
	got, env, err := cloudflaredArgs("http://127.0.0.1:8000", Options{})
	if err != nil {
		t.Fatalf("cloudflaredArgs() error = %v", err)
	}
	want := []string{"tunnel", "--no-autoupdate", "--url", "http://127.0.0.1:8000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if len(env) != 0 {
		t.Fatalf("env = %#v, want empty", env)
	}
}

func TestCloudflaredArgsUseRunTokenWithoutExposingTokenInArgs(t *testing.T) {
	t.Setenv("CF_TEST_TOKEN", "secret-token")
	got, env, err := cloudflaredArgs("http://127.0.0.1:8000", Options{TokenEnv: "CF_TEST_TOKEN"})
	if err != nil {
		t.Fatalf("cloudflaredArgs() error = %v", err)
	}
	want := []string{"tunnel", "--no-autoupdate", "run", "--url", "http://127.0.0.1:8000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if len(env) != 1 || env[0] != "TUNNEL_TOKEN=secret-token" {
		t.Fatalf("env = %#v, want token env", env)
	}
	for _, arg := range got {
		if strings.Contains(arg, "secret-token") {
			t.Fatalf("token leaked into args: %#v", got)
		}
	}
}

func TestCloudflaredArgsRejectMissingTokenEnv(t *testing.T) {
	_, _, err := cloudflaredArgs("http://127.0.0.1:8000", Options{TokenEnv: "CF_MISSING_TOKEN"})
	if err == nil {
		t.Fatal("cloudflaredArgs() succeeded with missing token env")
	}
}

func TestWaitForTokenTunnelReturnsWhenProcessStaysUp(t *testing.T) {
	done := make(chan error)
	if err := waitForTokenTunnel(done, time.Millisecond, nil); err != nil {
		t.Fatalf("waitForTokenTunnel() error = %v", err)
	}
}

func TestWaitForTokenTunnelReportsFastExitWithOutput(t *testing.T) {
	done := make(chan error, 1)
	done <- errors.New("bad token")
	output := newTailBuffer(1024)
	_, _ = output.Write([]byte("Provided Tunnel token is not valid.\n"))
	err := waitForTokenTunnel(done, time.Second, output)
	if err == nil {
		t.Fatal("waitForTokenTunnel() succeeded unexpectedly")
	}
	if !strings.Contains(err.Error(), "Provided Tunnel token is not valid") {
		t.Fatalf("error = %v, want cloudflared output", err)
	}
}
