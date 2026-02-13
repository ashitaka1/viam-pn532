# Project Specification: viam-pn532

## Purpose

Viam module wrapping the go-pn532 NFC reader library as a sensor component, enabling PN532 NFC readers to be managed by viam-server on SBCs.

## User Profile

1. **Robotics/IoT developers** — using Viam to manage hardware on SBCs, needing NFC tag detection and NDEF read/write

## Goals

**Goal:** Expose PN532 NFC tag detection and NDEF read/write through Viam's sensor API
**Goal:** Support UART, I2C, SPI transports with auto-detection
**Goal:** Deploy reliably on Raspberry Pi and similar SBCs

**Non-Goal:** Reimplementing PN532 protocol logic (delegated entirely to go-pn532)
**Non-Goal:** Custom Viam API — use standard sensor API with DoCommand for extended operations

## Features

### Required
- Continuous tag polling via `Readings()` (cached, no hardware I/O per call)
- Reactive tag detection via `DoCommand` `await_scan` (blocks until tag presented)
- NDEF text/URI read on tag detection
- NDEF write via `DoCommand`
- Auto-detection and manual transport configuration
- Device health monitoring and disconnect detection
- Graceful Reconfigure and Close lifecycle

### Milestones

1. ⏳ Skeleton — config, registration, stub Readings/DoCommand/Close, verify loads in viam-server
2. ⏳ Device lifecycle — connectDevice, full Reconfigure/Close, verify hardware connection
3. ⏳ Polling + Readings — background session, tag state caching, full Readings output
4. ⏳ DoCommand — await_scan, write_text, write_ndef, read_ndef, diagnostics, get_firmware
5. ⏳ Cross-compile + deploy — arm64 build, RPi testing, all transports

### Nice-to-Have
- Viam data management integration (structured tag event logging)
- Multiple simultaneous readers (multiple component instances)

## Tech Stack

### Language(s)
- Go 1.24

### Frameworks/Libraries
- `github.com/ZaparooProject/go-pn532` — PN532 NFC reader library (all hardware interaction)
- `go.viam.com/rdk` — Viam Go SDK (sensor API, module framework)

### Platform/Deployment
- Cross-compiled for linux/arm64 (RPi) and linux/amd64
- Deployed via Viam module registry

### Infrastructure
- Hardware: NXP PN532 NFC reader boards connected to SBC via UART/I2C/SPI
- SBC permissions: user must be in `dialout` (UART), `i2c`, or `spidev` groups

## Technical Architecture

### Architecture Decision: Sensor API

Implements `rdk:component:sensor` with model triplet `viam:nfc:pn532`.

`Readings()` maps to tag detection state — returns cached tag info (UID, type, NDEF content) with no hardware I/O per call. `DoCommand` handles writes, diagnostics, and extended operations. Single component (not reader+writer split) because go-pn532's `polling.Session` already coordinates write pausing internally.

### Module Structure

```
viam-pn532/
  cmd/module/
    main.go              -- ModularMain entry point
  pn532sensor/
    config.go            -- Config struct + Validate
    sensor.go            -- Registration, struct, constructor
    lifecycle.go         -- Reconfigure (connect/teardown) + Close
    transport.go         -- Transport factory from config + blank imports for detectors
    polling.go           -- Background polling session, tag state caching
    readings.go          -- Readings() from cached state
    docommand.go         -- DoCommand dispatch (write_text, write_ndef, diagnostics)
  meta.json
  Makefile
  go.mod
```

### Configuration Schema

```json
{
  "transport": "uart",
  "device_path": "/dev/ttyAMA0",
  "poll_interval_ms": 250,
  "card_removal_timeout_ms": 600,
  "read_ndef": true,
  "debug": false,
  "connect_timeout_sec": 10
}
```

`transport: "auto"` runs the detection registry in parallel. When explicit, `device_path` is required. Typical RPi device paths: `/dev/ttyAMA0` or `/dev/serial0` (UART via GPIO 14/15), `/dev/i2c-1` (I2C via GPIO 2/3), `/dev/spidev0.0` (SPI via GPIO 7-11).

### API Mapping

| go-pn532 Feature | Viam Surface | Details |
|---|---|---|
| `polling.Session` + callbacks | Background goroutine | Started on Reconfigure, stopped on Close |
| `DetectedTag` (UID, type) | `Readings()` | `uid`, `tag_type`, `manufacturer`, `is_genuine` |
| `tagops.ReadNDEF()` | `Readings()` | `ndef_text`, `ndef_record_count` (auto-read on detect) |
| `tagops.GetTagInfo()` | `Readings()` | `ntag_variant`, `mifare_variant`, `user_memory_bytes` |
| `polling.Session` + channel | `DoCommand` | `{"action": "await_scan"}` — blocks until tag detected or ctx cancelled. Optional `timeout_ms` for bounded wait. |
| `Session.WriteToNextTag()` | `DoCommand` | `{"action": "write_text", "text": "..."}` |
| `Tag.WriteNDEF()` | `DoCommand` | `{"action": "write_ndef", "records": [...]}` |
| Device health/firmware | `Readings()` + `DoCommand` | `device_healthy`, `firmware_version`, `{"action": "diagnostics"}` |
| Tag removal | `Readings()` | `tag_present: false` |
| Device disconnect | `Readings()` | `device_healthy: false` |

### Data Flow

**Readings** are a pure memory read. The polling goroutine writes to `cachedTagState` under a lock; `Readings()` reads the cache. No hardware I/O per call — critical for Viam's data collection scheduler.

**await_scan** blocks the caller until the polling goroutine detects a new tag. The `OnCardDetected` callback signals a channel; `await_scan` waits on that channel (with optional `timeout_ms` deadline, or the caller's context). In-process callers (e.g., another module using `FromDependencies`) can block indefinitely on their own context. External callers (CLI, SDK over network) should use `timeout_ms` to avoid gRPC timeouts and retry in a loop.

**Writes** go through `Session.WriteToNextTag` which pauses polling, waits for a tag, writes, and resumes. The caller blocks until complete or timeout.

**Reconfigure** tears down existing session/device, reconnects with new config, restarts polling. Atomic swap under mutex.

### Thread Safety

`pn532Sensor` uses `sync.RWMutex` (standard sync — separate module, not subject to go-pn532's `syncutil` convention). `Device` is not thread-safe but `polling.Session` owns all device access. Write operations coordinate through the session's `pauseWithAck`/`Resume` mechanism.

## Milestone Architecture Decisions

### Milestone 1: Sensor API Choice

**Approach:** Implement `rdk:component:sensor` rather than `rdk:component:generic` or a custom API.

**Key Decisions:**
- Sensor API chosen because `Readings()` provides a natural polling-compatible data path that integrates with Viam's data management and visualization tools
- `DoCommand` provides extensibility for write operations and diagnostics without a custom API
- Single component model — no reader/writer split

**Trade-offs considered:**
- Generic component: rejected because Readings provides structured data integration that generic lacks
- Custom API: rejected because it requires client-side SDK support and adds deployment complexity
- Multi-component (reader + writer): rejected because write coordination is already handled by go-pn532's Session internally

## Implementation Notes

- Blank imports of `detection/{uart,i2c,spi}` are required in `transport.go` to register platform detectors via `init()`
- go-pn532 `Device` is NOT thread-safe — all device access flows through `polling.Session` which serializes operations
- The `Session.WriteToNextTag` method handles the pause/write/resume dance — the module just needs to call it and block

## Development Process

**Testing approach:**
- Unit tests for config validation, readings construction, DoCommand dispatch
- Integration tests with go-pn532's `MockTransport` for lifecycle/polling without hardware
- Manual hardware testing on RPi with real PN532

**Deployment:**
- Binary deployed via Viam module registry
- `meta.json` declares the module manifest
- Cross-compile targets: `linux/arm64`, `linux/amd64`
