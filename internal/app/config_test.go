package app

import (
	"testing"
	"time"
)

func TestValidateConfigRejectsInvalidPorts(t *testing.T) {
	cases := []Config{
		{Dir: ".", Port: -1, UPnPLease: time.Hour},
		{Dir: ".", Port: 65536, UPnPLease: time.Hour},
		{Dir: ".", Port: 8000, UPnPPort: -1, UPnPLease: time.Hour},
		{Dir: ".", Port: 8000, UPnPPort: 65536, UPnPLease: time.Hour},
	}

	for _, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate() accepted invalid config: %+v", cfg)
		}
	}
}

func TestValidateConfigAcceptsPortZeroAndPermanentLease(t *testing.T) {
	cfg := Config{Dir: ".", Port: 0, UPnPPort: 0, UPnPLease: 0}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected valid config: %v", err)
	}
}

func TestValidateConfigRejectsNegativeLease(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: -time.Second}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted negative UPnP lease")
	}
}

func TestValidateConfigAcceptsCloudflareTunnel(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, Tunnel: "cloudflare"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected Cloudflare tunnel: %v", err)
	}
}

func TestValidateConfigRejectsUnsupportedTunnel(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, Tunnel: "bogus"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted unsupported tunnel")
	}
}

func TestValidateConfigRejectsTunnelHostnameWithoutCloudflareTunnel(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, TunnelHostname: "share.example.com", TunnelName: "quickserve"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted tunnel hostname without Cloudflare tunnel")
	}
}

func TestValidateConfigRejectsTunnelHostnameWithoutTunnelName(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, Tunnel: "cloudflare", TunnelHostname: "share.example.com"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted tunnel hostname without tunnel name or token env")
	}
}

func TestValidateConfigRejectsTunnelNameWithoutCloudflareTunnel(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, TunnelName: "quickserve"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted tunnel name without Cloudflare tunnel")
	}
}

func TestValidateConfigAcceptsTunnelHostnameWithTokenEnv(t *testing.T) {
	cfg := Config{
		Dir:            ".",
		Port:           8000,
		UPnPLease:      time.Hour,
		Tunnel:         "cloudflare",
		TunnelHostname: "share.example.com",
		TunnelTokenEnv: "CLOUDFLARE_TOKEN_SYSCODE",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected tunnel hostname with token env: %v", err)
	}
}

func TestValidateConfigRejectsTunnelTokenEnvWithoutCloudflareTunnel(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, TunnelTokenEnv: "CLOUDFLARE_TOKEN_SYSCODE"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted tunnel token env without Cloudflare tunnel")
	}
}

func TestValidateConfigRejectsTunnelTokenEnvWithoutHostname(t *testing.T) {
	cfg := Config{Dir: ".", Port: 8000, UPnPLease: time.Hour, Tunnel: "cloudflare", TunnelTokenEnv: "CLOUDFLARE_TOKEN_SYSCODE"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted tunnel token env without hostname")
	}
}
