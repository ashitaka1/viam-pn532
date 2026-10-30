# CLAUDE.md

Viam module wrapping the [go-pn532](https://github.com/ZaparooProject/go-pn532) NFC reader library (through the `ashitaka1/go-pn532` fork) as a `rdk:component:sensor`, model `avery:nfc:pn532`.

Tracking: GitHub issues (ashitaka1/viam-pn532)

- `README.md` — configuration and API
- `CONTRIBUTING.md` — build, test, release
- `ARCHITECTURE.md` — codemap, invariants, data flow
- `docs/decisions/` — decision records

## Commands

```bash
make test                          # all tests, with race detection
go test -v -race -run TestName ./...
make build                         # current platform
```

## Invariants

- `Readings()` reads cached state only; it never touches the device.
- go-pn532's `Device` is not thread-safe. All device access goes through `polling.Session`: inside its callbacks, or through `PauseAndRun`.
- PN532 protocol logic stays in go-pn532. A protocol fix goes to the fork, not this module.
