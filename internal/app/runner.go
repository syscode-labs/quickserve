package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/syscode-labs/quickserve/internal/netinfo"
	"github.com/syscode-labs/quickserve/internal/tunnel"
	"github.com/syscode-labs/quickserve/internal/ui"
	"github.com/syscode-labs/quickserve/internal/upnp"
)

type NetInfo interface {
	LANIPv4(context.Context) (string, error)
	PublicIPv4(context.Context) (string, error)
}

type UPnPMapper interface {
	Map(context.Context, upnp.Request) (*upnp.Mapping, error)
}

type TunnelStarter interface {
	Start(context.Context, string, tunnel.Options) (tunnel.Session, error)
}

type Runner struct {
	cfg      Config
	net      NetInfo
	mapper   UPnPMapper
	tunneler TunnelStarter
}

type Started struct {
	Ready chan struct{}
	Root  string
	Port  int
}

func NewRunner(cfg Config, ni NetInfo, mapper UPnPMapper) *Runner {
	return &Runner{cfg: cfg, net: ni, mapper: mapper}
}

func NewRunnerWithTunnel(cfg Config, ni NetInfo, mapper UPnPMapper, tunneler TunnelStarter) *Runner {
	return &Runner{cfg: cfg, net: ni, mapper: mapper, tunneler: tunneler}
}

func (r *Runner) Start(ctx context.Context, out io.Writer) (*Started, <-chan error) {
	started := &Started{Ready: make(chan struct{})}
	errc := make(chan error, 1)
	go func() {
		errc <- r.run(ctx, out, started)
	}()
	return started, errc
}

func (r *Runner) run(ctx context.Context, out io.Writer, started *Started) error {
	styles := ui.NewStyles(out)
	if err := r.cfg.Validate(); err != nil {
		return err
	}
	root, err := filepath.Abs(r.cfg.Dir)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", r.cfg.Port))
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	started.Root = root
	started.Port = port

	server := &http.Server{
		Handler:           http.FileServer(http.Dir(root)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	lan, _ := r.net.LANIPv4(ctx)
	public, publicErr := r.net.PublicIPv4(ctx)
	if publicErr != nil && !errors.Is(publicErr, netinfo.ErrNonGlobalAddress) {
		fmt.Fprintf(out, "%s\n", styles.LabelValue("Public address", fmt.Sprintf("unavailable (%v)", publicErr)))
	}

	var mapping *upnp.Mapping
	var tunnelSession tunnel.Session
	if r.cfg.UPnP {
		if r.mapper == nil {
			_ = listener.Close()
			return errors.New("UPnP requested but no mapper is configured")
		}
		localIP := net.ParseIP(lan)
		if localIP == nil {
			_ = listener.Close()
			return errors.New("UPnP requested but no LAN IPv4 address is available")
		}
		mapping, err = r.mapper.Map(ctx, upnp.Request{
			LocalIP:      localIP,
			LocalPort:    port,
			ExternalPort: r.cfg.UPnPPort,
			Lease:        r.cfg.UPnPLease,
			Description:  "quickserve",
		})
		if err != nil {
			_ = listener.Close()
			return fmt.Errorf("UPnP mapping failed: %w", err)
		}
		if netinfo.IsGlobalIPv4(mapping.ExternalIP) {
			public = mapping.ExternalIP
		}
		fmt.Fprintln(out, styles.Warning("WARNING: UPnP mapping enabled. Files are exposed publicly without TLS or authentication."))
		fmt.Fprintln(out, styles.Muted("         Double NAT, CGNAT, firewall policy, or ISP filtering can still block inbound access."))
	}
	if r.cfg.Tunnel == "cloudflare" {
		if r.tunneler == nil {
			_ = listener.Close()
			return errors.New("Cloudflare tunnel requested but no tunnel runner is configured")
		}
		tunnelSession, err = r.tunneler.Start(ctx, fmt.Sprintf("http://127.0.0.1:%d", port), tunnel.Options{
			Hostname: r.cfg.TunnelHostname,
			Name:     r.cfg.TunnelName,
			TokenEnv: r.cfg.TunnelTokenEnv,
		})
		if err != nil {
			_ = listener.Close()
			if mapping != nil {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = mapping.Cleanup(cleanupCtx)
				cleanupCancel()
			}
			return fmt.Errorf("Cloudflare tunnel failed: %w", err)
		}
		defer func() {
			if tunnelSession != nil {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = tunnelSession.Close(cleanupCtx)
				cleanupCancel()
			}
		}()
		fmt.Fprintln(out, styles.Warning("WARNING: Cloudflare Tunnel enabled. Files are exposed through a public HTTPS tunnel."))
		fmt.Fprintln(out, styles.Muted("         Anyone with the tunnel URL can reach this server unless Cloudflare Access is configured."))
	}

	fmt.Fprintln(out, styles.LabelValue("Serving", root))
	fmt.Fprintln(out, styles.URLValue("Local", fmt.Sprintf("http://localhost:%d/", port)))
	if lan != "" {
		fmt.Fprintln(out, styles.URLValue("LAN", fmt.Sprintf("http://%s:%d/", lan, port)))
	}
	if netinfo.IsGlobalIPv4(public) {
		publicPort := port
		if mapping != nil {
			publicPort = int(mapping.ExternalPort)
		}
		fmt.Fprintln(out, styles.URLValue("Public", fmt.Sprintf("http://%s:%d/", public, publicPort)))
	}
	if tunnelSession != nil {
		fmt.Fprintln(out, styles.URLValue("Tunnel", tunnelSession.URL()))
	}
	fmt.Fprintln(out, styles.Warning("WARNING: This HTTP server has no TLS or authentication. Serve only files you intend to share."))
	fmt.Fprintln(out, styles.Muted("         It binds to all interfaces intentionally for LAN/public serving."))

	serveErr := make(chan error, 1)
	close(started.Ready)
	go func() {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if mapping != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = mapping.Cleanup(cleanupCtx)
			cleanupCancel()
		}
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			return err
		}
		return <-serveErr
	case err := <-serveErr:
		return err
	}
}
