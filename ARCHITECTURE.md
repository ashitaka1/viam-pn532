# Architecture

The module is one Viam sensor component, `avery:nfc:pn532`, around a go-pn532 `polling.Session`. The session owns the PN532 device and runs a background polling loop. Its callbacks write tag state into a cache; `Readings()` returns the cache, and `DoCommand` either waits on detection events or pauses the session to talk to the device.

## Codemap

| File | Contents |
|---|---|
| `cmd/module/main.go` | `ModularMain` entry point |
| `cmd/cli/main.go` | Local harness that constructs the sensor outside viam-server |
| `config.go` | `Config`, `Validate` |
| `sensor.go` | Registration, `pn532Sensor`, config defaults, `startSession`, polling callbacks (`onCardDetected`, `onCardRemoved`, `onDeviceDisconnected`) |
| `transport.go` | `transportFactory` (uart, i2c, spi), `connectDevice` with retries |
| `lifecycle.go` | `Close` |
| `polling.go` | `tagState`, `buildReadingsFromState` |
| `readings.go` | `Readings` |
| `docommand.go` | `DoCommand` dispatch, `await_scan`, `diagnostics` |

## Lifecycle

`pn532Sensor` embeds `resource.AlwaysRebuild`: a config change closes the component and constructs a new one. `NewPn532` applies defaults, connects with `connectDevice`, and starts the session goroutine.

`Close` cancels the session context, then closes the device before waiting on the goroutine. An I2C `InListPassiveTarget` can block for about 5 s and ignores context cancellation; closing the file descriptor makes that syscall return.

`connectDevice` retries ten times with exponential backoff across the connect timeout, because a PN532 still finishing a stale `InListPassiveTarget` NAKs its I2C address.

## Data flow

**Readings.** `onCardDetected` runs tagops (tag info, and NDEF if `read_ndef`) without holding `s.mu`, then writes `s.state` under the lock. `onCardRemoved` and `onDeviceDisconnected` clear it. `Readings` takes `RLock` and builds a map from `s.state`.

**await_scan.** `onCardDetected` drains and refills `scanNotify`, a one-slot channel of `tagState`, so a waiter receives the newest detection. `handleAwaitScan` receives from it, bounded by `timeout_ms`, the caller's context, or the sensor's context. Callers over gRPC pass `timeout_ms` and loop; in-process callers can wait on their own context.

**diagnostics.** `handleDiagnostics` calls `Session.PauseAndRun`, which suspends polling and hands the device to the closure for a communication test, general status, and firmware version.

## Invariants

- `Readings()` performs no device I/O.
- go-pn532's `Device` is not thread-safe. Device access happens only inside session callbacks or `PauseAndRun`.
- `s.state` is written only under `s.mu.Lock()`; callbacks check `s.closed` before writing.
- PN532 protocol handling lives in go-pn532.

## Readings schema

| Key | Present when |
|---|---|
| `status`, `device_healthy`, `tag_present` | always |
| `uid`, `tag_type`, `manufacturer`, `is_genuine`, `ntag_variant`, `mifare_variant`, `user_memory_bytes`, `ndef_text`, `ndef_record_count` | a tag is present and the device is healthy |

`await_scan` returns the same map.
