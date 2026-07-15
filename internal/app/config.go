package app

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	Dir                  string
	Port                 int
	UPnP                 bool
	UPnPPort             int
	UPnPLease            time.Duration
	Tunnel               string
	TunnelHostname       string
	TunnelName           string
	TunnelTokenEnv       string
	CloudflareHostname   string
	CloudflareTokenEnv   string
	CloudflareTunnelName string
	Version              bool
}

func (c Config) Validate() error {
	if c.Dir == "" {
		return errors.New("directory is required")
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("port %d is invalid", c.Port)
	}
	if c.UPnPPort < 0 || c.UPnPPort > 65535 {
		return fmt.Errorf("UPnP port %d is invalid", c.UPnPPort)
	}
	if c.UPnPLease < 0 {
		return errors.New("UPnP lease duration must not be negative")
	}
	if c.Tunnel != "" && c.Tunnel != "cloudflare" {
		return fmt.Errorf("tunnel provider %q is not supported", c.Tunnel)
	}
	if c.TunnelHostname != "" && c.Tunnel != "cloudflare" {
		return errors.New("tunnel hostname requires -tunnel cloudflare")
	}
	if c.TunnelHostname != "" && c.TunnelName == "" && c.TunnelTokenEnv == "" {
		return errors.New("tunnel hostname requires -tunnel-name or -tunnel-token-env")
	}
	if c.TunnelName != "" && c.Tunnel != "cloudflare" {
		return errors.New("tunnel name requires -tunnel cloudflare")
	}
	if c.TunnelTokenEnv != "" && c.Tunnel != "cloudflare" {
		return errors.New("tunnel token env requires -tunnel cloudflare")
	}
	if c.TunnelTokenEnv != "" && c.TunnelHostname == "" {
		return errors.New("tunnel token env requires -tunnel-hostname")
	}
	return nil
}
