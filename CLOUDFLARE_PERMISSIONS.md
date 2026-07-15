# Cloudflare Permissions for quickserve

`quickserve` uses two different Cloudflare credentials for two different jobs.

## 1. Setup API token

This token is for setup automation only. It is the control-plane credential that can create or update Cloudflare resources.

Use it to configure the Cloudflare route and serve in one command:

```bash
quickserve \
  -cloudflare-hostname quickserve.example.com \
  -cloudflare-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

That command:

- Finds the DNS zone from the hostname.
- Finds the Cloudflare Tunnel named `quickserve`, unless `-cloudflare-tunnel-name` is supplied.
- Adds or updates the tunnel ingress rule for the hostname.
- Ensures the tunnel config has a `http_status:404` fallback rule.
- Creates or updates a proxied CNAME pointing the hostname at `<tunnel-id>.cfargotunnel.com`.
- Starts quickserve on the configured local port and keeps running until `Ctrl-C`.

If you only want to inspect the account and tunnel IDs, use:

```bash
quickserve cloudflare discover \
  -hostname quickserve.example.com \
  -tunnel-name quickserve \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

That prints:

```text
account-id=<account-id>
tunnel-id=<tunnel-id>
tunnel-name=quickserve
tunnel-status=healthy
```

If you only want to update the Cloudflare route and exit without serving, use:

```bash
quickserve cloudflare route \
  -hostname quickserve.example.com \
  -tunnel-name quickserve \
  -service http://localhost:8000 \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

That command:

- Finds the DNS zone from the hostname, unless `-zone` is supplied.
- Finds the named tunnel with `-tunnel-name`.
- Adds or updates the tunnel ingress rule for the hostname.
- Ensures the tunnel config has a `http_status:404` fallback rule.
- Creates or updates a proxied CNAME pointing the hostname at `<tunnel-id>.cfargotunnel.com`.

`quickserve cloudflare route` is setup-only. It exits after updating Cloudflare. The normal command is `quickserve -cloudflare-hostname ...`, which sets up the route and serves in one process.

To fetch a runtime connector token manually, use:

```bash
quickserve cloudflare token \
  -account-id '<account-id>' \
  -tunnel-id '<tunnel-id>' \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

Recommended local env var name:

```text
CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

Required scope, least-privilege version:

- Account scope: the Cloudflare account that owns the Zero Trust/Tunnel configuration.
- Zone scope: only the DNS zone that contains the hostname you want to publish, for example `example.com`.
- Account permission: Cloudflare Tunnel edit, or equivalent Zero Trust tunnel edit permission.
- Zone permission: DNS edit for that zone.
- Zone permission: Zone read for that zone, if setup needs to discover the zone ID from the hostname.

What it can do:

- Create or reuse a Cloudflare Tunnel.
- Configure the public hostname, for example `quickserve.syscode.uk`.
- Create or update the DNS route/CNAME needed for that hostname.
- Fetch or create a connector token for the tunnel.

Why it is sensitive:

- It can change Cloudflare configuration.
- If scoped too broadly, it could alter other tunnels or DNS records.
- Do not use it as the day-to-day runtime token.

Storage guidance:

```fish
set -Ux CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP '<api-token>'
```

Do not put this value in `.quickserverc`.

## 2. Runtime connector token

This token is for day-to-day `quickserve` runs. It is consumed by `cloudflared`, not by quickserve's Cloudflare setup API code.

Recommended local env var name:

```text
CLOUDFLARE_TOKEN_QUICKSERVE
```

`.quickserverc` should reference the env var name, not the secret value:

```text
tunnel=cloudflare
tunnel-hostname=quickserve.example.com
tunnel-token-env=CLOUDFLARE_TOKEN_QUICKSERVE
```

What it can do:

- Let `cloudflared` connect as a connector for one existing Cloudflare Tunnel.
- Carry traffic from Cloudflare to the local quickserve origin.

What it should not be able to do:

- Create tunnels.
- Change DNS records.
- Change public hostnames.
- Reconfigure Cloudflare Access policies.

Why this is safer for runtime:

- It is scoped to an existing tunnel.
- It is useful even if the machine is behind CGNAT.
- If leaked, rotate the connector token in Cloudflare without replacing the setup API token.

Storage guidance:

```fish
set -Ux CLOUDFLARE_TOKEN_QUICKSERVE '<connector-token>'
```

### How to get the runtime connector token

The easiest path is the Cloudflare One dashboard:

1. Open Cloudflare One.
2. Go to Networks > Connectors > Cloudflare Tunnels.
3. Create a tunnel, or open an existing remotely-managed tunnel.
4. Choose `cloudflared` as the connector.
5. Copy the install/run command Cloudflare shows for your operating system.
6. Extract the long token value from that command.
7. Store it locally:

```fish
set -Ux CLOUDFLARE_TOKEN_QUICKSERVE '<connector-token>'
```

The copied command usually runs `cloudflared tunnel run` with a token. That token is the value quickserve needs in `CLOUDFLARE_TOKEN_QUICKSERVE`.

For automation, use the Cloudflare Tunnel API after the tunnel exists:

```text
GET /accounts/{account_id}/cfd_tunnel/{tunnel_id}/token
```

That API call requires the setup API token described above. `quickserve cloudflare token` wraps that API call and prints the returned string. Store it as `CLOUDFLARE_TOKEN_QUICKSERVE`; do not put it directly in `.quickserverc`.

Fish example:

```fish
set -Ux CLOUDFLARE_TOKEN_QUICKSERVE (quickserve cloudflare token \
  -account-id '<account-id>' \
  -tunnel-id '<tunnel-id>' \
  -api-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP)
```

## Existing `cloudflared` system service

If `cloudflared` is already installed as a system service for the tunnel, quickserve does not need the runtime connector token during normal serving. The simplest command is:

```bash
quickserve \
  -cloudflare-hostname quickserve.example.com \
  -cloudflare-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

This configures the public hostname to point at the local quickserve origin, for example:

```text
http://localhost:8000
```

In this mode, the system service owns the Cloudflare connection. quickserve only runs the local HTTP server.

Use `-tunnel cloudflare -tunnel-token-env ...` only when you want quickserve to start and stop its own `cloudflared` process for the duration of the quickserve run.

## Setup Pattern

1. Store the setup API token in your shell:

```fish
set -Ux CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP '<api-token>'
```

2. Run:

```bash
quickserve \
  -cloudflare-hostname quickserve.example.com \
  -cloudflare-token-env CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
```

3. Optionally save those defaults in `.quickserverc`:

```text
dir=.
port=8000
cloudflare-hostname=quickserve.example.com
cloudflare-token-env=CLOUDFLARE_API_TOKEN_QUICKSERVE_SETUP
cloudflare-tunnel-name=quickserve
```

## Notes

- `cloudflared` reads the connector token through `TUNNEL_TOKEN`; quickserve maps `tunnel-token-env` to that environment variable without putting the token in process arguments.
- If `cloudflared` says `Provided Tunnel token is not valid`, the runtime connector token is wrong, expired, revoked, or not a connector token.
- A Cloudflare API token and a Cloudflare Tunnel connector token are not interchangeable.
- For a real deployment, replace `quickserve.example.com` with your own hostname and scope the setup API token to that hostname's DNS zone.

References:

- Cloudflare Tunnel dashboard setup: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/create-remote-tunnel/
- Cloudflare Tunnel API: https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/
- Cloudflare locally-managed tunnel docs: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/create-local-tunnel/
