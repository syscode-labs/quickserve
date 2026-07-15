package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigFileAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".quickserverc")
	err := os.WriteFile(path, []byte(`
# quickserve defaults
dir=public
port=9000
upnp=true
upnp-port=19000
upnp-lease=30m
tunnel=cloudflare
tunnel-hostname=share.syscode.uk
tunnel-token-env=CLOUDFLARE_TOKEN_SYSCODE
cloudflare-hostname=share.example.com
cloudflare-token-env=CLOUDFLARE_API_TOKEN
cloudflare-tunnel-name=quickserve
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfigFile(path, Config{Dir: ".", Port: 8000, UPnPLease: time.Hour})
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}
	if cfg.Dir != "public" || cfg.Port != 9000 || !cfg.UPnP || cfg.UPnPPort != 19000 {
		t.Fatalf("config not applied: %+v", cfg)
	}
	if cfg.UPnPLease != 30*time.Minute {
		t.Fatalf("UPnPLease = %v", cfg.UPnPLease)
	}
	if cfg.Tunnel != "cloudflare" || cfg.TunnelHostname != "share.syscode.uk" || cfg.TunnelTokenEnv != "CLOUDFLARE_TOKEN_SYSCODE" {
		t.Fatalf("tunnel config not applied: %+v", cfg)
	}
	if cfg.CloudflareHostname != "share.example.com" || cfg.CloudflareTokenEnv != "CLOUDFLARE_API_TOKEN" || cfg.CloudflareTunnelName != "quickserve" {
		t.Fatalf("cloudflare config not applied: %+v", cfg)
	}
}

func TestLoadConfigFileIgnoresMissingFile(t *testing.T) {
	cfg, err := LoadConfigFile(filepath.Join(t.TempDir(), ".quickserverc"), Config{Dir: ".", Port: 8000})
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}
	if cfg.Dir != "." || cfg.Port != 8000 {
		t.Fatalf("config changed for missing file: %+v", cfg)
	}
}

func TestLoadConfigFileRejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".quickserverc")
	if err := os.WriteFile(path, []byte("bogus=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfigFile(path, Config{})
	if err == nil {
		t.Fatal("LoadConfigFile() accepted unknown key")
	}
}
