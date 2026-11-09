# Security Policy

## Supported Versions

Only the latest `master` is supported.

## Reporting a Vulnerability

This tool reads public DNS records and one public Google feed — it accepts
no secrets, credentials, or tokens, and it writes nothing except its own
range-feed cache. The realistic attack surface is the standard Go supply
chain (dependency vulnerabilities).

To report a vulnerability in this repository, open a private security
advisory on GitHub (`Security` → `Report a vulnerability`) or contact
Lars Artmann directly. Please include reproduction steps and the affected
commit. You will hear back within a few days; fixes ship as patch releases.

## Supply chain

- Dependencies: Go module proxy with checked-in `go.sum`; Dependabot keeps
  them current (`.github/dependabot.yml`); CI builds and tests every push.
- Hermetic builds: `nix build` hashes the vendor set (`vendorHash`).
- Code scanning: BuildFlow runs golangci-lint, govulncheck-class audits, and
  gitleaks on demand (`buildflow -s gitleaks`).
