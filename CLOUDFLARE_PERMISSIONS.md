# Cloudflare Permissions for quickserve

`quickserve` uses two different Cloudflare credentials for two different jobs.

## 1. Setup API token

This token is for setup automation only. It is the control-plane credential that can create or update Cloudflare resources.

Use it for commands such as a future:

```bash
quickserve tunnel setup cloudflare \
  -hostname quickserve.syscode.uk \
  -token-env CLOUDFLARE_TOKEN_QUICKSERVE \
  -api-token-env CLOUDFLARE_API_TOKEN_SYSCODE
```

Recommended local env var name:

```text
CLOUDFLARE_API_TOKEN_SYSCODE
```

Required scope, least-privilege version:

- Account scope: the Cloudflare account that owns the Zero Trust/Tunnel configuration.
- Zone scope: only `syscode.uk`.
- Account permission: Cloudflare Tunnel edit, or equivalent Zero Trust tunnel edit permission.
- Zone permission: DNS edit for `syscode.uk`.
- Zone permission: Zone read for `syscode.uk`, if setup needs to discover the zone ID from the hostname.

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
set -Ux CLOUDFLARE_API_TOKEN_SYSCODE '<api-token>'
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
tunnel-hostname=quickserve.syscode.uk
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

## Setup Pattern

1. Use the setup API token once to create/update:
   - Tunnel name: `quickserve`
   - Public hostname: `quickserve.syscode.uk`
   - Origin service: `http://localhost:8000`
2. Save the generated connector token as `CLOUDFLARE_TOKEN_QUICKSERVE`.
3. Keep `.quickserverc` in the project directory:

```text
dir=.
port=8000
tunnel=cloudflare
tunnel-hostname=quickserve.syscode.uk
tunnel-token-env=CLOUDFLARE_TOKEN_QUICKSERVE
```

4. Run:

```bash
quickserve
```

## Notes

- `cloudflared` reads the connector token through `TUNNEL_TOKEN`; quickserve maps `tunnel-token-env` to that environment variable without putting the token in process arguments.
- If `cloudflared` says `Provided Tunnel token is not valid`, the runtime connector token is wrong, expired, revoked, or not a connector token.
- A Cloudflare API token and a Cloudflare Tunnel connector token are not interchangeable.

References:

- Cloudflare Tunnel dashboard setup: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/create-remote-tunnel/
- Cloudflare Tunnel API: https://developers.cloudflare.com/api/resources/zero_trust/subresources/tunnels/
- Cloudflare locally-managed tunnel docs: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/create-local-tunnel/
