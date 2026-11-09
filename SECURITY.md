# Security Policy

## Supported Versions

| Version        | Supported  |
| -------------- | ---------- |
| Latest release | ✅ Current |

Only the latest release receives security fixes. Please update to the latest
version before reporting issues found in older releases.

## Reporting a Vulnerability

Please do **not** open public issues for security problems. Report privately
instead:

- **GitHub Security Advisory**: https://github.com/LarsArtmann/google-domain-detector/security/advisories/new
- **Email**: security@artmann.tech

### What to include

- Vulnerability type and severity
- Steps to reproduce
- Potential impact
- Proof of concept (if available)

### Response commitments

| Phase            | Timeline        |
| ---------------- | --------------- |
| Initial response | Within 48 hours |
| Triage           | Within 72 hours |
| Disclosure       | Coordinated, within 90 days |

## Security Practices

- This tool reads only public DNS records and one public Google feed — it
  accepts no secrets, credentials, or tokens, and writes nothing except its
  own range-feed cache.
- All changes are code-reviewed before merge.
- Automated security scanning runs in CI; Dependabot keeps dependencies
  current (`.github/dependabot.yml`).
- Hermetic `nix build` hashes the vendor set (`vendorHash` in `flake.nix`).
