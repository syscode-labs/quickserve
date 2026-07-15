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

	"github.com/syscode-labs/quickserve/internal/app"
	"github.com/syscode-labs/quickserve/internal/cloudflare"
	"github.com/syscode-labs/quickserve/internal/netinfo"
	"github.com/syscode-labs/quickserve/internal/tunnel"
	"github.com/syscode-labs/quickserve/internal/upnp"
)

var cloudflareAPIBaseURL string

type cloudflareRouteOptions struct {
	Hostname    string
	Zone        string
	TunnelName  string
	Service     string
	APITokenEnv string
}

type cloudflareRouteResult struct {
	AccountID   string
	TunnelID    string
	TunnelName  string
	Hostname    string
	Service     string
	DNSRecordID string
	DNSContent  string
}

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
	flag.StringVar(&cfg.CloudflareHostname, "cloudflare-hostname", cfg.CloudflareHostname, "configure this Cloudflare hostname and serve through an existing cloudflared service")
	flag.StringVar(&cfg.CloudflareTokenEnv, "cloudflare-token-env", cfg.CloudflareTokenEnv, "environment variable containing the Cloudflare API token for -cloudflare-hostname")
	flag.StringVar(&cfg.CloudflareTunnelName, "cloudflare-tunnel-name", cfg.CloudflareTunnelName, "Cloudflare tunnel name for -cloudflare-hostname")
	flag.BoolVar(&cfg.Version, "version", false, "print version information and exit")
	flag.Parse()

	if cfg.Version {
		app.PrintVersion(os.Stdout, app.CurrentBuildInfo())
		return
	}

	var route cloudflareRouteOptions
	var routeEnabled bool
	cfg, route, routeEnabled, err = prepareCloudflareServeMode(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "quickserve: %v\n", err)
		os.Exit(1)
	}
	if routeEnabled {
		result, err := configureCloudflareRoute(ctx, route, os.Getenv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "quickserve: Cloudflare route setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "Cloudflare: https://%s/ -> %s via tunnel %s\n", result.Hostname, result.Service, result.TunnelName)
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

func prepareCloudflareServeMode(cfg app.Config) (app.Config, cloudflareRouteOptions, bool, error) {
	if cfg.CloudflareHostname == "" {
		return cfg, cloudflareRouteOptions{}, false, nil
	}
	if cfg.Port == 0 {
		return cfg, cloudflareRouteOptions{}, false, fmt.Errorf("-cloudflare-hostname requires a fixed -port")
	}
	if cfg.CloudflareTokenEnv == "" {
		cfg.CloudflareTokenEnv = "CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP"
	}
	if cfg.CloudflareTunnelName == "" {
		cfg.CloudflareTunnelName = "quickserve"
	}
	route := cloudflareRouteOptions{
		Hostname:    cfg.CloudflareHostname,
		TunnelName:  cfg.CloudflareTunnelName,
		Service:     fmt.Sprintf("http://localhost:%d", cfg.Port),
		APITokenEnv: cfg.CloudflareTokenEnv,
	}
	cfg.Tunnel = ""
	cfg.TunnelHostname = ""
	cfg.TunnelName = ""
	cfg.TunnelTokenEnv = ""
	return cfg, route, true, nil
}

func runCloudflare(ctx context.Context, args []string, out io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return fmt.Errorf("cloudflare command is required; supported: token")
	}
	switch args[0] {
	case "discover":
		return runCloudflareDiscover(ctx, args[1:], out, getenv)
	case "route":
		return runCloudflareRoute(ctx, args[1:], out, getenv)
	case "token":
		return runCloudflareToken(ctx, args[1:], out, getenv)
	default:
		return fmt.Errorf("unsupported cloudflare command %q; supported: discover, route, token", args[0])
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

func runCloudflareRoute(ctx context.Context, args []string, out io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("quickserve cloudflare route", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var hostname string
	var zoneName string
	var tunnelName string
	var service string
	var apiTokenEnv string
	fs.StringVar(&hostname, "hostname", "", "Cloudflare hostname to route")
	fs.StringVar(&zoneName, "zone", "", "Cloudflare DNS zone name")
	fs.StringVar(&tunnelName, "tunnel-name", "quickserve", "Cloudflare tunnel name")
	fs.StringVar(&service, "service", "http://localhost:8000", "origin service URL for the tunnel hostname")
	fs.StringVar(&apiTokenEnv, "api-token-env", "CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP", "environment variable containing the Cloudflare setup API token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if hostname == "" {
		return fmt.Errorf("-hostname is required")
	}
	result, err := configureCloudflareRoute(ctx, cloudflareRouteOptions{
		Hostname:    hostname,
		Zone:        zoneName,
		TunnelName:  tunnelName,
		Service:     service,
		APITokenEnv: apiTokenEnv,
	}, getenv)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Cloudflare route configured.")
	fmt.Fprintln(out, "This setup command exits after updating Cloudflare.")
	fmt.Fprintf(out, "Hostname: https://%s/\n", result.Hostname)
	fmt.Fprintf(out, "Origin service: %s\n", result.Service)
	fmt.Fprintf(out, "Tunnel: %s (%s)\n", result.TunnelName, result.TunnelID)
	fmt.Fprintf(out, "DNS record: %s -> %s\n", result.Hostname, result.DNSContent)
	fmt.Fprintf(out, "account-id: %s\n", result.AccountID)
	fmt.Fprintf(out, "dns-record-id: %s\n", result.DNSRecordID)
	fmt.Fprintln(out, "Next: run quickserve -port 8000 and keep it running.")
	return nil
}

func configureCloudflareRoute(ctx context.Context, opts cloudflareRouteOptions, getenv func(string) string) (cloudflareRouteResult, error) {
	apiToken := getenv(opts.APITokenEnv)
	if apiToken == "" {
		return cloudflareRouteResult{}, fmt.Errorf("%s is not set", opts.APITokenEnv)
	}
	client := cloudflare.Client{BaseURL: cloudflareAPIBaseURL}
	var zone cloudflare.Zone
	var err error
	if opts.Zone != "" {
		zone, err = client.ZoneByName(ctx, opts.Zone, apiToken)
	} else {
		zone, err = client.FindZoneForHostname(ctx, opts.Hostname, apiToken)
	}
	if err != nil {
		return cloudflareRouteResult{}, err
	}
	tunnels, err := client.Tunnels(ctx, zone.Account.ID, opts.TunnelName, apiToken)
	if err != nil {
		return cloudflareRouteResult{}, err
	}
	if len(tunnels) == 0 {
		return cloudflareRouteResult{}, fmt.Errorf("no tunnel found for name %q", opts.TunnelName)
	}
	if len(tunnels) > 1 {
		return cloudflareRouteResult{}, fmt.Errorf("multiple tunnels found for name %q", opts.TunnelName)
	}
	tunnel := tunnels[0]
	ingress, err := client.TunnelIngress(ctx, zone.Account.ID, tunnel.ID, apiToken)
	if err != nil {
		return cloudflareRouteResult{}, err
	}
	ingress = cloudflare.UpsertTunnelIngress(ingress, opts.Hostname, opts.Service)
	if err := client.PutTunnelIngress(ctx, zone.Account.ID, tunnel.ID, ingress, apiToken); err != nil {
		return cloudflareRouteResult{}, err
	}
	record, err := client.UpsertDNSCNAME(ctx, zone.ID, opts.Hostname, tunnel.ID+".cfargotunnel.com", apiToken)
	if err != nil {
		return cloudflareRouteResult{}, err
	}
	return cloudflareRouteResult{
		AccountID:   zone.Account.ID,
		TunnelID:    tunnel.ID,
		TunnelName:  tunnel.Name,
		Hostname:    opts.Hostname,
		Service:     opts.Service,
		DNSRecordID: record.ID,
		DNSContent:  record.Content,
	}, nil
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
