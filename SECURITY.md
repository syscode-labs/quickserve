# Security Policy

## Reporting a Vulnerability

Please report vulnerabilities privately through GitHub Security Advisories for this repository:

https://github.com/syscode-labs/quickserve/security/advisories/new

Do not open a public issue for security-sensitive reports.

## Scope

`quickserve` is a read-only static file server. It has no TLS, no authentication, and no access control. When `-upnp` succeeds, files under the selected directory may be reachable from the public internet.

Only enable `-upnp` for files you intend to share publicly.
