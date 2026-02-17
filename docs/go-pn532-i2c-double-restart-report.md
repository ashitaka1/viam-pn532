# Problem Report: I2C Double-Transaction Pattern in go-pn532

**Library:** [ZaparooProject/go-pn532](https://github.com/ZaparooProject/go-pn532)
**File:** `transport/i2c/i2c.go`
**Severity:** Low (functional correctness unaffected; protocol compliance and bus efficiency issue)
**Reporter:** ashitaka1 (viam-pn532 module, testing on Raspberry Pi 5 with PN532 via I2C)
**Date:** 2026-02-17

## Summary

Every I2C read path in the transport issues **two separate I2C transactions** (each with its own START/STOP) where the PN532 datasheet specifies a **single transaction**. The PN532 tolerates this, but it doubles the bus traffic per read, deviates from the documented protocol, and creates a window between transactions where another I2C master or bus event could interfere.

## PN532 Datasheet Reference (Section 6.2.4)

The PN532 I2C read protocol is defined as a single transaction:

```
START → [address + READ] → read status byte → if 0x01 (ready), continue reading frame bytes → STOP
```

The status byte (`0x01` = ready, `0x00` = not ready) and the frame data are part of **one contiguous read**. The host issues a single START, reads the status byte, and if the device is ready, keeps clocking out the frame data — all in the same transaction, terminated by a single STOP.

## Current Implementation

The I2C transport splits this into two (sometimes three) independent transactions per read operation. Each `dev.Tx()` call maps to a separate `ioctl(fd, I2C_RDWR, ...)` syscall via periph.io, producing a full START...STOP pair on the bus.

### `waitAck` — two transactions per ACK read

```go
// transport/i2c/i2c.go — waitAck()

// Transaction 1: read 1 byte to check readiness
func (t *Transport) checkReady() error {
    ready := frame.GetSmallBuffer(1)
    err := t.dev.Tx(nil, ready)           // START → [addr+R] → 1 byte → STOP
    if ready[0] == pn532Ready { return nil }
    // ...retry with backoff
}

// Transaction 2: read 7 bytes (1 status + 6 ACK), strip status byte
func (t *Transport) readI2C(buf []byte) error {
    tmpBuf := frame.GetSmallBuffer(1 + len(buf))
    err := t.dev.Tx(nil, tmpBuf)          // START → [addr+R] → 7 bytes → STOP
    if tmpBuf[0] != pn532Ready { return err }
    copy(buf, tmpBuf[1:])
    return nil
}
```

**What happens on the wire:**

```
Transaction 1 (checkReady):
  S → [0x24+R] → [status=0x01] → P

Transaction 2 (readI2C for 6-byte ACK):
  S → [0x24+R] → [status=0x01] → [0x00 0x00 0xFF 0x00 0xFF 0x00] → P
```

The status byte is read **twice** — once in `checkReady()` and again as the first byte of `readI2C()`. The PN532 prepends a fresh status byte to each new transaction, so this works, but it's redundant.

### `receiveFrameAttempt` — two or three transactions per frame read

```go
// Transaction 1: checkReady() — 1-byte read (same as above)
// Transaction 2: readI2C(headerBuf) — 33-byte read (1 status + 32 header)
// Transaction 3 (large frames only): readI2C(remainingBuf) — remaining bytes
```

For a typical response frame, this produces 2 START/STOP pairs. For frames larger than 32 bytes, it produces 3.

## What the Datasheet Intends

A single read transaction that combines the ready-check and data read:

```
S → [0x24+R] → [status byte] → [frame bytes...] → P
```

If the status byte is `0x00` (not ready), the host issues a STOP, waits, and retries the entire single-transaction read. The host never reads *just* the status byte in one transaction and then comes back for the data in another.

### Proposed single-transaction read

```go
func (t *Transport) readWithStatus(buf []byte) (ready bool, err error) {
    // Read status + data in one transaction
    tmpBuf := frame.GetSmallBuffer(1 + len(buf))
    defer frame.PutBuffer(tmpBuf)

    if err := t.dev.Tx(nil, tmpBuf); err != nil {
        return false, fmt.Errorf("I2C read failed: %w", err)
    }

    if tmpBuf[0] != pn532Ready {
        return false, nil  // not ready — caller retries the whole read
    }

    copy(buf, tmpBuf[1:])
    return true, nil
}
```

With this, `waitAck` becomes a single transaction per attempt:

```go
func (t *Transport) waitAck(ctx context.Context) error {
    ackBuf := frame.GetSmallBuffer(6)
    defer frame.PutBuffer(ackBuf)

    deadline := time.Now().Add(t.timeout)
    for time.Now().Before(deadline) {
        ready, err := t.readWithStatus(ackBuf)
        if err != nil {
            return err
        }
        if ready && bytes.Equal(ackBuf, ackFrame) {
            return nil
        }
        sleepCtx(ctx, time.Millisecond)
    }
    return pn532.NewNoACKError("waitAck", t.busName)
}
```

And `readFrameData` similarly folds the ready-check into the header read.

## Impact

### Why it works today

The PN532 generates a fresh status byte for every I2C read transaction, regardless of whether the previous transaction consumed any frame data. So reading the status in one transaction and the (status + frame) in a second transaction is tolerated — the second transaction's status byte just confirms readiness again.

### Why it should be fixed

1. **Protocol compliance.** The datasheet specifies one transaction. Other PN532 firmware revisions or clones may not tolerate the split (e.g., a clone could advance its output buffer pointer on every START, causing the second read to miss the first byte of data).

2. **Bus efficiency.** Each extra START/STOP pair costs ~20 µs of bus time (START condition + address byte + ACK/NAK). At 400 kHz with continuous polling, this adds up.

3. **Multi-master safety.** The gap between the two STOPs is a window where another master could arbitrate and win the bus. On a shared I2C bus this could cause the PN532 to reset its output state between the ready-check and the data read.

4. **Large frame correctness risk.** The `readFrameData` function splits large frames across *three* transactions (ready-check, 32-byte header, remainder). If the PN532 ever resets its output pointer between transactions, the third read would get stale or repeated data. A single-transaction read for the full frame (once the length is known from the header) eliminates this risk.

## Reproducing

The issue is structural and always present when using the I2C transport. It can be observed with a logic analyzer on the SDA/SCL lines — every ACK read and every frame read shows two START conditions instead of one.

Test environment used:
- Raspberry Pi 5, Raspberry Pi OS (bookworm)
- PN532 board connected via I2C on `/dev/i2c-1` (GPIO 2/3)
- periph.io/x/conn/v3 v3.7.2, periph.io/x/host/v3 v3.8.5
- go-pn532 at commit `0f449be` (after address and status-byte fixes)

## Related Fixes Already Applied

This report builds on two earlier I2C fixes (both already merged to the ashitaka1 fork):

1. **7-bit address correction** (commit `5e6612d`): Changed `pn532WriteAddr = 0x48` → `pn532Addr = 0x24`. Without this fix, the kernel puts `0x90/0x91` on the wire and nothing responds.

2. **Status byte stripping** (commit `3e5a761`): Added `readI2C()` to strip the hardware-prepended status byte from every read. Without this, ACK comparison fails and frame data is shifted by one byte.

The double-transaction pattern was visible during the status-byte fix work. The `readI2C()` helper was designed as a minimal targeted fix to unblock I2C functionality. Consolidating `checkReady()` into the read itself is the natural next step.

## Suggested Fix

Remove `checkReady()` from the ACK and frame read paths. Replace `readI2C()` with `readWithStatus()` that returns `(ready bool, err error)`, and have callers retry the entire read on `ready=false`. This reduces every read path to a single I2C transaction.

The mock (`MockI2CBus`, `JitteryMockI2CBus`) already prepends the status byte to every read, so the test infrastructure supports this change with no modification.
