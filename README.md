# viam-pn532

A [Viam](https://viam.com/) module that wraps the [go-pn532](https://github.com/ZaparooProject/go-pn532) NFC reader library as a `rdk:component:sensor`. Provides continuous NFC tag detection, NDEF reading, and device diagnostics for PN532 boards connected to Raspberry Pi and other SBCs.

## Features

- **Continuous tag polling** via `Readings()` — cached tag state with zero hardware I/O per call, compatible with Viam's data collection scheduler
- **Reactive tag detection** via `DoCommand` `await_scan` — blocks until a tag is presented
- **NDEF text/URI reading** — automatically reads NDEF content on tag detection
- **Device diagnostics** — firmware version, communication test, RF field detection
- **Transport support** — UART, I2C, SPI connections
- **Automatic reconnection** — exponential backoff retry on connection failure

## Requirements

- PN532 NFC reader board (connected via UART, I2C, or SPI)
- Raspberry Pi or similar SBC running viam-server (or macOS with USB-serial adapter)
- Appropriate device permissions:
  - UART: user in `dialout` group
  - I2C: user in `i2c` group
  - SPI: user in `spidev` group

## Configuration

Add the sensor component to your Viam machine config:

```json
{
  "name": "nfc-reader",
  "api": "rdk:component:sensor",
  "model": "viam:nfc:pn532",
  "attributes": {
    "transport": "uart",
    "device_path": "/dev/ttyAMA0"
  }
}
```

### Attributes

| Attribute | Type | Required | Default | Description |
|---|---|---|---|---|
| `transport` | string | Yes | — | Connection type: `"uart"`, `"i2c"`, or `"spi"` |
| `device_path` | string | Yes | — | Device file path |
| `poll_interval_ms` | int | No | 250 | How often to poll for tags (ms) |
| `card_removal_timeout_ms` | int | No | 600 | Time before a missing tag is considered removed (ms) |
| `read_ndef` | bool | No | true | Automatically read NDEF content on tag detection |
| `debug` | bool | No | false | Enable debug logging |
| `connect_timeout_sec` | int | No | 10 | Device connection timeout (seconds) |

### Common device paths

| Transport | Platform | Path |
|---|---|---|
| UART (GPIO 14/15) | Raspberry Pi | `/dev/ttyAMA0` or `/dev/serial0` |
| UART (USB-serial) | macOS | `/dev/tty.usbserial-*` |
| I2C (GPIO 2/3) | Raspberry Pi | `/dev/i2c-1` |
| SPI (GPIO 7-11) | Raspberry Pi | `/dev/spidev0.0` |

## API

### Readings

Returns the current tag detection state. This is a pure memory read — the background polling goroutine handles all hardware interaction.

**No tag present:**

```json
{
  "status": "connected",
  "device_healthy": true,
  "tag_present": false
}
```

**Tag detected:**

```json
{
  "status": "connected",
  "device_healthy": true,
  "tag_present": true,
  "uid": "04abcdef123456",
  "tag_type": "NTAG",
  "manufacturer": "NXP",
  "is_genuine": true,
  "ntag_variant": "NTAG215",
  "mifare_variant": "",
  "user_memory_bytes": 504,
  "ndef_text": "Hello, NFC!",
  "ndef_record_count": 1
}
```

**Device disconnected:**

```json
{
  "status": "connected",
  "device_healthy": false,
  "tag_present": false
}
```

### DoCommand

#### `await_scan`

Blocks until a new tag is detected or the timeout expires. Returns the same fields as Readings at the moment of detection.

```json
{
  "action": "await_scan",
  "timeout_ms": 5000
}
```

External callers (CLI, SDK over gRPC) should use `timeout_ms` to avoid gRPC deadline issues and retry in a loop. In-process callers can omit `timeout_ms` and rely on context cancellation.

#### `diagnostics`

Returns device health and firmware information. Briefly pauses polling to run diagnostic commands.

```json
{
  "action": "diagnostics"
}
```

Response:

```json
{
  "firmware_version": "PN532 v1.6",
  "comm_test_ok": true,
  "support_iso14443a": true,
  "support_iso14443b": true,
  "support_iso18092": true,
  "field_present": false,
  "last_error": 0,
  "targets": 0
}
```

## Building

```bash
make build           # Build for current platform
make build-arm64     # Cross-compile for linux/arm64 (Raspberry Pi)
make build-amd64     # Cross-compile for linux/amd64
make test            # Run tests with race detection
```

## Development

The module delegates all PN532 protocol handling to go-pn532. Key architectural decisions:

- **Single component model** — no reader/writer split; go-pn532's `polling.Session` coordinates access internally
- **Readings are zero-cost** — background goroutine writes to cached state under a mutex; `Readings()` only reads memory
- **Device is not thread-safe** — all hardware access flows through the polling session, which serializes operations

### Project structure

```
cmd/module/main.go   Entry point (ModularMain)
config.go            Config struct + validation
sensor.go            Registration, struct, callbacks
lifecycle.go         Reconfigure + Close
transport.go         Transport factory + retry logic
polling.go           Tag state caching
readings.go          Readings() implementation
docommand.go         DoCommand dispatch
```

## License

See [LICENSE](LICENSE) for details.
