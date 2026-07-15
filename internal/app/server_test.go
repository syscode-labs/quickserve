package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/syscode-labs/quickserve/internal/netinfo"
	"github.com/syscode-labs/quickserve/internal/tunnel"
)

func TestServerServesSelectedRootAndReportsPortZero(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/known.txt", []byte("known content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(Config{Dir: root, Port: 0, UPnPLease: time.Hour}, StaticNetInfo{
		LAN: "192.0.2.10",
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, errc := runner.Start(ctx, io.Discard)
	select {
	case err := <-errc:
		t.Fatalf("Start() failed: %v", err)
	case <-started.Ready:
	}
	defer func() {
		cancel()
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("shutdown error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("server did not shut down")
		}
	}()

	if started.Port == 0 {
		t.Fatal("selected port was not reported")
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", started.Port)
	resp, err := http.Get(base + "/known.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "known content\n" {
		t.Fatalf("body = %q", body)
	}

	resp, err = http.Get(base + "/missing.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d", resp.StatusCode)
	}

	expectedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(started.Root, expectedRoot) {
		t.Fatalf("started root = %q, want under %q", started.Root, expectedRoot)
	}
}

func TestServerStartsCloudflareTunnelAndReportsURL(t *testing.T) {
	root := t.TempDir()
	tunneler := &fakeTunnelStarter{session: &fakeTunnelSession{url: "https://example.trycloudflare.com"}}
	runner := NewRunnerWithTunnel(Config{Dir: root, Port: 0, UPnPLease: time.Hour, Tunnel: "cloudflare"}, StaticNetInfo{
		LAN: "192.0.2.10",
	}, nil, tunneler)

	ctx, cancel := context.WithCancel(context.Background())
	started, errc := runner.Start(ctx, io.Discard)
	select {
	case err := <-errc:
		t.Fatalf("Start() failed: %v", err)
	case <-started.Ready:
	}

	if tunneler.localURL == "" {
		t.Fatal("tunnel was not started")
	}
	wantLocal := fmt.Sprintf("http://127.0.0.1:%d", started.Port)
	if tunneler.localURL != wantLocal {
		t.Fatalf("tunnel local URL = %q, want %q", tunneler.localURL, wantLocal)
	}
	if tunneler.opts.Hostname != "" {
		t.Fatalf("tunnel hostname = %q, want empty", tunneler.opts.Hostname)
	}

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
	if !tunneler.session.closed {
		t.Fatal("tunnel was not closed")
	}
}

func TestServerPassesCloudflareTunnelHostname(t *testing.T) {
	root := t.TempDir()
	tunneler := &fakeTunnelStarter{session: &fakeTunnelSession{url: "https://share.example.com"}}
	runner := NewRunnerWithTunnel(Config{
		Dir:            root,
		Port:           0,
		UPnPLease:      time.Hour,
		Tunnel:         "cloudflare",
		TunnelHostname: "share.example.com",
		TunnelName:     "quickserve-test",
		TunnelTokenEnv: "CLOUDFLARE_TOKEN_SYSCODE",
	}, StaticNetInfo{
		LAN: "192.0.2.10",
	}, nil, tunneler)

	ctx, cancel := context.WithCancel(context.Background())
	started, errc := runner.Start(ctx, io.Discard)
	select {
	case err := <-errc:
		t.Fatalf("Start() failed: %v", err)
	case <-started.Ready:
	}
	cancel()
	<-errc

	if tunneler.opts.Hostname != "share.example.com" {
		t.Fatalf("tunnel hostname = %q, want share.example.com", tunneler.opts.Hostname)
	}
	if tunneler.opts.Name != "quickserve-test" {
		t.Fatalf("tunnel name = %q, want quickserve-test", tunneler.opts.Name)
	}
	if tunneler.opts.TokenEnv != "CLOUDFLARE_TOKEN_SYSCODE" {
		t.Fatalf("tunnel token env = %q, want CLOUDFLARE_TOKEN_SYSCODE", tunneler.opts.TokenEnv)
	}
}

type StaticNetInfo struct {
	LAN    string
	Public string
	Err    error
}

func (s StaticNetInfo) LANIPv4(context.Context) (string, error) { return s.LAN, nil }
func (s StaticNetInfo) PublicIPv4(context.Context) (string, error) {
	if s.Err != nil {
		return "", s.Err
	}
	if s.Public != "" && !netinfo.IsGlobalIPv4(s.Public) {
		return "", netinfo.ErrNonGlobalAddress
	}
	return s.Public, nil
}

type fakeTunnelStarter struct {
	localURL string
	opts     tunnel.Options
	session  *fakeTunnelSession
}

func (f *fakeTunnelStarter) Start(_ context.Context, localURL string, opts tunnel.Options) (tunnel.Session, error) {
	f.localURL = localURL
	f.opts = opts
	return f.session, nil
}

type fakeTunnelSession struct {
	url    string
	closed bool
}

func (f *fakeTunnelSession) URL() string { return f.url }

func (f *fakeTunnelSession) Close(context.Context) error {
	f.closed = true
	return nil
}
