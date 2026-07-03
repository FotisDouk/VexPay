# Security Policy

VexPay handles money. We take security seriously and appreciate responsible disclosure.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report privately via GitHub's ["Report a vulnerability"](../../security/advisories/new)
feature, or email the maintainers. Include:

- A description of the issue and its impact.
- Steps to reproduce (a proof of concept helps).
- Affected version / commit.

We aim to acknowledge reports within 72 hours and to keep you updated as we work on a fix.
Please give us a reasonable window to release a patch before any public disclosure.

## Security model

VexPay is **non-custodial**. The gateway never stores private keys and never takes custody of
funds — merchants provide watch-only material (an xpub, a receive address, or a view key) and
payments settle directly into the merchant's own wallet. The most valuable target, private
keys, simply is not present on the server.

Areas we care about most:

- Correctness of payment detection and confirmation (no false "paid").
- Webhook authenticity (HMAC signing + replay protection).
- API key and admin session handling (hashing, rotation).
- Input validation and address normalisation per chain.
- Supply-chain integrity of releases.

## Supported versions

During pre-1.0 development, only the latest release receives security fixes.
