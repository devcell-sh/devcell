# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.9.x   | Yes       |
| 0.8.x   | Security fixes only |
| < 0.8   | No        |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Instead, use GitHub's private [Security Advisory](https://github.com/devcell-sh/devcell/security/advisories/new) feature to report directly.

Include in your report:

1. Description of the vulnerability
2. Steps to reproduce or proof of concept
3. Affected versions
4. Potential impact (especially any container escape or host exposure scenarios)

## Response Timeline

| Step | Target |
|------|--------|
| Acknowledge receipt | 48 hours |
| Initial triage and severity assessment | 7 days |
| Fix development and private disclosure | 30 days |
| Public disclosure (release + advisory) | 90 days |

## Scope

DevCell's security boundary is the container (or VM) wall. Vulnerabilities that allow any of the following are treated as critical:

- Container escape or host filesystem access outside the mounted project directory
- Credential leakage from the host into the cell
- Bypass of `--solitary` egress lockdown
- Exposure of 1Password secrets beyond the RAM-only tmpfs

## Recognition

We credit researchers who responsibly report vulnerabilities in the release notes, with your permission.
