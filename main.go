package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/syscod3/quickserve/internal/app"
	"github.com/syscod3/quickserve/internal/netinfo"
	"github.com/syscod3/quickserve/internal/tunnel"
	"github.com/syscod3/quickserve/internal/upnp"
)

func main() {
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
