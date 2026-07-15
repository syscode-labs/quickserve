package app

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultConfigPath = ".quickserverc"

func LoadConfigFile(path string, cfg Config) (Config, error) {
	if path == "" {
		return cfg, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := stripComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return cfg, fmt.Errorf("%s:%d: expected key=value", path, lineNo)
		}
		var err error
		cfg, err = applyConfigValue(cfg, strings.TrimSpace(key), strings.TrimSpace(value))
		if err != nil {
			return cfg, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func stripComment(line string) string {
	if before, _, ok := strings.Cut(line, "#"); ok {
		return before
	}
	return line
}

func applyConfigValue(cfg Config, key, value string) (Config, error) {
	switch key {
	case "dir":
		cfg.Dir = value
	case "port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return cfg, fmt.Errorf("invalid port %q", value)
		}
		cfg.Port = port
	case "upnp":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("invalid upnp %q", value)
		}
		cfg.UPnP = enabled
	case "upnp-port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return cfg, fmt.Errorf("invalid upnp-port %q", value)
		}
		cfg.UPnPPort = port
	case "upnp-lease":
		lease, err := time.ParseDuration(value)
		if err != nil {
			return cfg, fmt.Errorf("invalid upnp-lease %q", value)
		}
		cfg.UPnPLease = lease
	case "tunnel":
		cfg.Tunnel = value
	case "tunnel-hostname":
		cfg.TunnelHostname = value
	case "tunnel-name":
		cfg.TunnelName = value
	case "tunnel-token-env":
		cfg.TunnelTokenEnv = value
	case "cloudflare-hostname":
		cfg.CloudflareHostname = value
	case "cloudflare-token-env":
		cfg.CloudflareTokenEnv = value
	case "cloudflare-tunnel-name":
		cfg.CloudflareTunnelName = value
	default:
		return cfg, fmt.Errorf("unknown key %q", key)
	}
	return cfg, nil
}
