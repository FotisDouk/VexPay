# Contributing to VexPay

Thanks for your interest in improving VexPay. This project aims to be a payment gateway people
can actually trust with real money, so correctness, security, and clear documentation matter as
much as features.

## Getting started

```sh
go build ./...
go test ./...
go vet ./...
go run ./cmd/vexpay
```

Requires Go 1.21+.

## Ground rules

- **Keep the core non-custodial.** Nothing should ever require the server to hold a private key
  or take custody of funds.
- **Money is integers.** Represent amounts in the smallest unit (satoshi, wei, etc.). Never use
  floating point for balances or comparisons.
- **Fail closed.** When a rate, confirmation, or explorer response is uncertain, do not mark an
  invoice paid.
- **Small, focused commits** with clear messages. We loosely follow Conventional Commits
  (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`).

## Adding a new cryptocurrency

Adding a coin means implementing the chain-adapter interface in `internal/chain` and nothing
else in the core:

1. Create `internal/chain/<coin>/` implementing the `Adapter` interface.
2. Provide a `Backend` that talks to a public explorer/RPC, and allow a self-hosted node via
   config.
3. Pass the adapter conformance tests.
4. Add a documentation page under `docs/` describing wallet setup and the receive strategy.

## Pull requests

- Include tests for new behaviour.
- Run `go test ./...` and `go vet ./...` before pushing.
- Update relevant docs in `docs/`.
- Describe the security implications of your change if any.

## License

By contributing, you agree that your contributions are licensed under the project's
[AGPL-3.0](LICENSE) license.
