# Changelog

## v0.1.10

- Move the canonical repository to `syscode-labs/quickserve`.
- Update Go module path and release documentation for the new organization.

## v0.1.9

- Add `quickserve cloudflare route` to configure a Cloudflare Tunnel public hostname and proxied DNS record from the setup API token.

## v0.1.8

- Add `quickserve cloudflare discover` to print the Cloudflare account and tunnel IDs needed by the token command.

## v0.1.7

- Add `quickserve cloudflare token` to fetch a Cloudflare Tunnel connector token with a setup API token.
- Document when to use quickserve-managed Cloudflare tunnels versus an existing `cloudflared` system service.

## v0.1.6

- Add `-tunnel-token-env` for running existing Cloudflare tunnels from a token without exposing the token in process arguments.
- Add `.quickserverc` support for per-directory defaults.

## v0.1.5

- Add `-tunnel-name` and `-tunnel-hostname` for routing Cloudflare Tunnel traffic through a custom Cloudflare-managed hostname.

## v0.1.4

- Add opt-in Cloudflare Quick Tunnel support with `-tunnel cloudflare` for CGNAT networks.
- Keep `cloudflared` optional; it is only required when Cloudflare tunnel mode is used.

## v0.1.3

- Improve the error message when UPnP exists on a LAN but no NAT router exposes an IGD WAN service.

## v0.1.2

- Add root-device fallback discovery for routers that do not answer exact IGD service SSDP searches.
- Keep non-IGD UPnP device parse errors out of the final UPnP error.

## v0.1.1

- Fix UPnP discovery so routers exposing only WANIPConnection v1 or WANPPPConnection v1 can still be found after a WANIPConnection v2 search times out.

## v0.1.0

- Serve a selected directory over HTTP.
- Print localhost and LAN URLs, and a public address when one can be safely identified.
- Add opt-in UPnP IGD TCP port mapping with lease renewal and cleanup.
- Publish cross-platform release archives with checksums.
