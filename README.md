# VexPay

**Self-hosted, non-custodial crypto payments for companies — fast, secure, and easy.**

VexPay is a payment gateway you run on your own server. Point it at your own wallet, get an
API key, add a few lines of code, and start accepting cryptocurrency. Funds go **directly to
your wallet** — VexPay never holds your private keys or your money.

> Status: early development. The API and data model are still changing.

## Why VexPay

- **Non-custodial by design.** You supply an extended public key (xpub), address, or view key.
  VexPay only *watches* the blockchain and derives receive addresses. There are no hot wallets
  and no private keys on the server, so there is nothing there to steal.
- **Runs anywhere.** A single static binary and a one-command Docker image. SQLite by default,
  Postgres when you need it.
- **Any language, PHP first.** A clean REST API described by an OpenAPI spec, with a flagship
  PHP SDK and thin clients for other languages.
- **Built for real commerce.** Fiat-priced invoices with locked exchange rates, automatic
  on-chain confirmation, signed webhooks, one-click invoices, and QR codes.
- **Test without spending a cent.** First-class sandbox mode with a payment simulator.

## Quick start (development)

Requires Go 1.21+.

```sh
git clone https://github.com/vexarnetwork/vexpay
cd vexpay
go run ./cmd/vexpay
```

Then:

```sh
curl http://localhost:8080/healthz   # {"status":"ok",...}
curl http://localhost:8080/readyz    # {"status":"ready"}
```

## Configuration

All settings are read from `VEXPAY_`-prefixed environment variables.

| Variable                       | Default                 | Description                                   |
| ------------------------------ | ----------------------- | --------------------------------------------- |
| `VEXPAY_ENV`                   | `development`           | `development`, `sandbox`, or `production`.     |
| `VEXPAY_ADDR`                  | `:8080`                 | HTTP listen address.                          |
| `VEXPAY_PUBLIC_URL`            | `http://localhost:8080` | Externally reachable base URL.                |
| `VEXPAY_DATABASE_URL`          | `sqlite:vexpay.db`      | `sqlite:<path>` or `postgres://...`.          |
| `VEXPAY_INVOICE_EXPIRY`        | `15m`                   | Invoice / locked-rate validity window.        |
| `VEXPAY_REQUEST_TIMEOUT`       | `30s`                   | Per-request timeout.                          |
| `VEXPAY_ADMIN_SESSION_SECRET`  | —                       | Signs dashboard sessions (required in prod).  |

In `production`, `VEXPAY_PUBLIC_URL` must be HTTPS and `VEXPAY_ADMIN_SESSION_SECRET` is
required.

## Architecture

VexPay is a single Go binary. Coins are added through a small **chain-adapter** interface, so
supporting a new cryptocurrency means writing one adapter — not touching the core. Payment
monitoring uses public block explorers out of the box and can be pointed at your own node for
full trustlessness.

See [`docs/`](docs/) for the full documentation site (open `docs/index.html` in a browser — no
build step required).

## Documentation

The documentation is a self-contained static site under [`docs/`](docs/). Open it directly in
your browser, or serve the folder with any static file server.

## License

[AGPL-3.0](LICENSE). If you run a modified version as a network service, you must make your
modifications available under the same license.
