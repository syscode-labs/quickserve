# quickserve

`quickserve` is a tiny Go CLI that serves a directory over HTTP and prints URLs for local and LAN access. It can also request an opt-in UPnP router mapping or start an outbound Cloudflare Tunnel when you explicitly ask for public sharing.

## Security Warning

`quickserve` has no authentication. Anyone who can reach the server can read files under the selected directory. When `-upnp` or `-tunnel cloudflare` succeeds, those files may be reachable from the public internet.

Only serve directories you intend to share.

## Install With Homebrew

```bash
brew install syscode-labs/public/quickserve
```

## Install From GitHub Releases

Download the archive for your platform from:

https://github.com/syscode-labs/quickserve/releases/latest

macOS Apple Silicon:

```bash
curl -LO https://github.com/syscode-labs/quickserve/releases/download/v0.1.3/quickserve_v0.1.3_darwin_arm64.tar.gz
tar -xzf quickserve_v0.1.3_darwin_arm64.tar.gz
install -m 0755 quickserve /opt/homebrew/bin/quickserve
```

macOS Intel:

```bash
curl -LO https://github.com/syscode-labs/quickserve/releases/download/v0.1.3/quickserve_v0.1.3_darwin_amd64.tar.gz
tar -xzf quickserve_v0.1.3_darwin_amd64.tar.gz
install -m 0755 quickserve /usr/local/bin/quickserve
```

Linux amd64:

```bash
curl -LO https://github.com/syscode-labs/quickserve/releases/download/v0.1.3/quickserve_v0.1.3_linux_amd64.tar.gz
tar -xzf quickserve_v0.1.3_linux_amd64.tar.gz
sudo install -m 0755 quickserve /usr/local/bin/quickserve
```

Linux arm64:

```bash
curl -LO https://github.com/syscode-labs/quickserve/releases/download/v0.1.3/quickserve_v0.1.3_linux_arm64.tar.gz
tar -xzf quickserve_v0.1.3_linux_arm64.tar.gz
sudo install -m 0755 quickserve /usr/local/bin/quickserve
```

Windows amd64:

Download `quickserve_v0.1.3_windows_amd64.zip`, extract `quickserve.exe`, and place it in a directory on your `PATH`.

## Install With Go

```bash
go install github.com/syscode-labs/quickserve@latest
```

## Basic Use

Serve the current directory on port `8000`:

```bash
quickserve
```

Serve another directory:

```bash
quickserve -dir ~/Downloads
```

Choose a port:

```bash
quickserve -port 9000
```

Let the OS choose a free port:

```bash
quickserve -port 0
```

## Config File

`quickserve` reads `.quickserverc` from the current directory when present. Command-line flags override config-file values. Use `-config path` to choose another file or `-config ""` to disable config loading.

Example:

```text
dir=~/Public
port=8000
cloudflare-hostname=share.example.com
cloudflare-token-env=CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
cloudflare-tunnel-name=quickserve
```

Supported keys match the long flag names without the leading dash: `dir`, `port`, `upnp`, `upnp-port`, `upnp-lease`, `cloudflare-hostname`, `cloudflare-token-env`, `cloudflare-tunnel-name`, `tunnel`, `tunnel-hostname`, `tunnel-name`, and `tunnel-token-env`.

Do not put Cloudflare token values in `.quickserverc` if the file may be committed. Store the token in your shell or secret manager and reference its environment variable with `cloudflare-token-env`.

## UPnP

UPnP is disabled by default. Enable it only when you want to ask your router to expose the server.

```bash
quickserve -dir ~/Public -upnp
```

Use a different external port:

```bash
quickserve -dir ~/Public -port 8000 -upnp -upnp-port 18080
```

Request a shorter temporary lease:

```bash
quickserve -upnp -upnp-lease 30m
```

Request a permanent mapping:

```bash
quickserve -upnp -upnp-lease 0
```

Temporary mappings are renewed while `quickserve` runs. On `Ctrl-C` or `SIGTERM`, it removes only the mapping created by that process.

## Cloudflare Tunnel

Cloudflare Tunnel works when direct inbound access is blocked by CGNAT. The simplest setup is an existing `cloudflared` service plus a Cloudflare API token that can update the tunnel and DNS route.

```bash
quickserve -dir ~/Public \
  -cloudflare-hostname quickserve.example.com \
  -cloudflare-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

This command does the Cloudflare setup and serves files in one process:

- Adds or updates the tunnel ingress rule for `quickserve.example.com -> http://localhost:8000`.
- Creates or updates a proxied CNAME for `quickserve.example.com -> <tunnel-id>.cfargotunnel.com`.
- Starts the local HTTP server and keeps running until `Ctrl-C`.

Use `-cloudflare-tunnel-name` if the tunnel is not named `quickserve`.

If you only want to update Cloudflare and exit, use the setup-only subcommand:

```bash
quickserve cloudflare route \
  -hostname quickserve.example.com \
  -tunnel-name quickserve \
  -service http://localhost:8000 \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

Temporary Quick Tunnel mode is still available when you do not need a stable hostname. It requires the `cloudflared` command on your `PATH`.

```bash
quickserve -dir ~/Public -tunnel cloudflare
```

This starts a temporary Quick Tunnel and prints an HTTPS `trycloudflare.com` URL. The tunnel is closed when `quickserve` exits.

Advanced token commands are available if you need to inspect IDs or fetch a connector token manually:

```bash
quickserve cloudflare discover \
  -hostname quickserve.example.com \
  -tunnel-name quickserve \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP

quickserve cloudflare token \
  -account-id '<account-id>' \
  -tunnel-id '<tunnel-id>' \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

## Flags

```text
-dir string
      directory to serve (default ".")
-config string
      config file path; empty disables config loading (default ".quickserverc")
-port int
      local TCP port; use 0 to select an available port (default 8000)
-upnp
      request a public TCP port mapping using UPnP IGD
-upnp-port int
      external UPnP port; 0 uses the selected local port
-upnp-lease duration
      UPnP lease duration; 0 requests a permanent mapping (default 1h0m0s)
-tunnel string
      outbound tunnel provider; supported: cloudflare
-tunnel-hostname string
      Cloudflare hostname to route to this tunnel
-tunnel-name string
      Cloudflare tunnel name for custom hostname mode
-tunnel-token-env string
      environment variable containing a Cloudflare tunnel token
-cloudflare-hostname string
      configure this Cloudflare hostname and serve through an existing cloudflared service
-cloudflare-token-env string
      environment variable containing the Cloudflare API token for -cloudflare-hostname
-cloudflare-tunnel-name string
      Cloudflare tunnel name for -cloudflare-hostname
-version
      print version information and exit
```

## Example Output

```text
Serving: /Users/giovanni/Downloads
Local:   http://localhost:8000/
LAN:     http://192.168.1.42:8000/
Public:  http://203.0.113.10:8000/
Tunnel:  https://example.trycloudflare.com
WARNING: This HTTP server has no TLS or authentication. Serve only files you intend to share.
         It binds to all interfaces intentionally for LAN/public serving.
```

## Network Notes

`quickserve` binds to all interfaces intentionally so other devices on the LAN can connect. macOS may ask whether to allow incoming network connections; allow them if LAN access is required.

Public access may still fail when UPnP succeeds. Common causes are double NAT, carrier-grade NAT, firewall policy, ISP filtering, or a router that accepts a mapping but does not route inbound traffic correctly.

Use `-cloudflare-hostname` when CGNAT blocks inbound connections and you want a stable hostname through an existing `cloudflared` service. The tunnel makes an outbound connection to Cloudflare, so it does not need port forwarding or UPnP.

The server uses Go's standard `http.FileServer`. A selected root must be a valid directory. Directory listings use the standard Go behavior. Symlinks inside the served root follow normal filesystem behavior, so do not serve a directory containing symlinks to files you do not intend to share.

## Verify Checksums

Download `checksums.txt` and your archive, then run:

```bash
shasum -a 256 -c checksums.txt --ignore-missing
```

On Windows, use:

```powershell
Get-FileHash .\quickserve_v0.1.3_windows_amd64.zip -Algorithm SHA256
```

Compare the hash to `checksums.txt`.

## Verify Artifact Attestation

Install the GitHub CLI, then run:

```bash
gh attestation verify --owner syscod3 quickserve_v0.1.3_darwin_arm64.tar.gz
```

## Build From Source

```bash
git clone https://github.com/syscode-labs/quickserve.git
cd quickserve
go build ./...
go build -o quickserve .
```

## Supported Platforms

Release binaries are published for:

- macOS arm64
- macOS amd64
- Linux arm64
- Linux amd64
- Windows amd64

## License

`quickserve` is released under CC0-1.0.

It depends on `github.com/huin/goupnp` for UPnP IGD discovery and SOAP calls.
