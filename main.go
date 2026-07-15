package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/syscod3/quickserve/internal/app"
	"github.com/syscod3/quickserve/internal/cloudflare"
	"github.com/syscod3/quickserve/internal/netinfo"
	"github.com/syscod3/quickserve/internal/tunnel"
	"github.com/syscod3/quickserve/internal/upnp"
)

var cloudflareAPIBaseURL string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 && os.Args[1] == "cloudflare" {
		if err := runCloudflare(ctx, os.Args[2:], os.Stdout, os.Getenv); err != nil {
			fmt.Fprintf(os.Stderr, "quickserve: %v\n", err)
			os.Exit(1)
		}
		return
	}

	configPath := configPathFromArgs(os.Args[1:])
	cfg := app.Config{Dir: ".", Port: 8000, UPnPLease: time.Hour}
	var err error
	cfg, err = app.LoadConfigFile(configPath, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "quickserve: config: %v\n", err)
		os.Exit(1)
	}

	flag.StringVar(&configPath, "config", configPath, "config file path; empty disables config loading")
	flag.StringVar(&cfg.Dir, "dir", cfg.Dir, "directory to serve")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "local TCP port; use 0 to select an available port")
	flag.BoolVar(&cfg.UPnP, "upnp", cfg.UPnP, "request a public TCP port mapping using UPnP IGD")
	flag.IntVar(&cfg.UPnPPort, "upnp-port", cfg.UPnPPort, "external UPnP port; 0 uses the selected local port")
	flag.DurationVar(&cfg.UPnPLease, "upnp-lease", cfg.UPnPLease, "UPnP lease duration; 0 requests a permanent mapping")
	flag.StringVar(&cfg.Tunnel, "tunnel", cfg.Tunnel, "outbound tunnel provider; supported: cloudflare")
	flag.StringVar(&cfg.TunnelHostname, "tunnel-hostname", cfg.TunnelHostname, "Cloudflare hostname to route to this tunnel")
	flag.StringVar(&cfg.TunnelName, "tunnel-name", cfg.TunnelName, "Cloudflare tunnel name for custom hostname mode")
	flag.StringVar(&cfg.TunnelTokenEnv, "tunnel-token-env", cfg.TunnelTokenEnv, "environment variable containing a Cloudflare tunnel token")
	flag.BoolVar(&cfg.Version, "version", false, "print version information and exit")
	flag.Parse()

	if cfg.Version {
		app.PrintVersion(os.Stdout, app.CurrentBuildInfo())
		return
	}

	runner := app.NewRunnerWithTunnel(cfg, netinfo.DefaultProvider(), upnp.NewDefaultManager(), tunnel.CloudflareQuick{})
	started, errc := runner.Start(ctx, os.Stdout)
	select {
	case err := <-errc:
		if err != nil {
			fmt.Fprintf(os.Stderr, "quickserve: %v\n", err)
			os.Exit(1)
		}
	case <-started.Ready:
	}

	err = <-errc
	if err != nil {
		fmt.Fprintf(os.Stderr, "quickserve: %v\n", err)
		os.Exit(1)
	}
}

func runCloudflare(ctx context.Context, args []string, out io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return fmt.Errorf("cloudflare command is required; supported: token")
	}
	switch args[0] {
	case "discover":
		return runCloudflareDiscover(ctx, args[1:], out, getenv)
	case "token":
		return runCloudflareToken(ctx, args[1:], out, getenv)
	default:
		return fmt.Errorf("unsupported cloudflare command %q; supported: discover, token", args[0])
	}
}

func runCloudflareDiscover(ctx context.Context, args []string, out io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("quickserve cloudflare discover", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var hostname string
	var zoneName string
	var tunnelName string
	var apiTokenEnv string
	fs.StringVar(&hostname, "hostname", "", "Cloudflare hostname used to infer the DNS zone")
	fs.StringVar(&zoneName, "zone", "", "Cloudflare DNS zone name")
	fs.StringVar(&tunnelName, "tunnel-name", "", "Cloudflare tunnel name filter")
	fs.StringVar(&apiTokenEnv, "api-token-env", "CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP", "environment variable containing the Cloudflare setup API token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if zoneName == "" && hostname == "" {
		return fmt.Errorf("-hostname or -zone is required")
	}
	apiToken := getenv(apiTokenEnv)
	if apiToken == "" {
		return fmt.Errorf("%s is not set", apiTokenEnv)
	}
	client := cloudflare.Client{BaseURL: cloudflareAPIBaseURL}
	var zone cloudflare.Zone
	var err error
	if zoneName != "" {
		zone, err = client.ZoneByName(ctx, zoneName, apiToken)
	} else {
		zone, err = client.FindZoneForHostname(ctx, hostname, apiToken)
	}
	if err != nil {
		return err
	}
	tunnels, err := client.Tunnels(ctx, zone.Account.ID, tunnelName, apiToken)
	if err != nil {
		return err
	}
	if len(tunnels) == 0 {
		return fmt.Errorf("no tunnels found for account %s", zone.Account.ID)
	}
	if len(tunnels) > 1 && tunnelName != "" {
		return fmt.Errorf("multiple tunnels found for name %q", tunnelName)
	}
	if len(tunnels) > 1 {
		fmt.Fprintf(out, "account-id=%s\n", zone.Account.ID)
		for _, tunnel := range tunnels {
			fmt.Fprintf(out, "tunnel-id=%s tunnel-name=%s tunnel-status=%s\n", tunnel.ID, tunnel.Name, tunnel.Status)
		}
		return nil
	}
	tunnel := tunnels[0]
	fmt.Fprintf(out, "account-id=%s\n", zone.Account.ID)
	fmt.Fprintf(out, "tunnel-id=%s\n", tunnel.ID)
	fmt.Fprintf(out, "tunnel-name=%s\n", tunnel.Name)
	fmt.Fprintf(out, "tunnel-status=%s\n", tunnel.Status)
	return nil
}

func runCloudflareToken(ctx context.Context, args []string, out io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("quickserve cloudflare token", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var accountID string
	var tunnelID string
	var apiTokenEnv string
	fs.StringVar(&accountID, "account-id", "", "Cloudflare account ID")
	fs.StringVar(&tunnelID, "tunnel-id", "", "Cloudflare tunnel ID")
	fs.StringVar(&apiTokenEnv, "api-token-env", "CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP", "environment variable containing the Cloudflare setup API token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	apiToken := getenv(apiTokenEnv)
	if apiToken == "" {
		return fmt.Errorf("%s is not set", apiTokenEnv)
	}
	client := cloudflare.Client{BaseURL: cloudflareAPIBaseURL}
	token, err := client.TunnelToken(ctx, accountID, tunnelID, apiToken)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, token)
	return err
}

func configPathFromArgs(args []string) string {
	path := app.DefaultConfigPath
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-config" || arg == "--config" {
			if i+1 < len(args) {
				path = args[i+1]
			}
			continue
		}
		for _, prefix := range []string{"-config=", "--config="} {
			if len(arg) >= len(prefix) && arg[:len(prefix)] == prefix {
				path = arg[len(prefix):]
			}
		}
	}
	return path
}
