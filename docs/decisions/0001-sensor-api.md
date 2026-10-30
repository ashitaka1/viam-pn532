# 1. Implement the sensor API as a single component

## Status

Accepted

## Context

The module needs a Viam API for continuous tag detection, NDEF reads, writes, and diagnostics. The options:

- `rdk:component:sensor`. `Readings()` feeds Viam data management and visualization; `DoCommand` carries everything else.
- `rdk:component:generic`. Only `DoCommand`, with no structured data path.
- A custom API. Needs client SDK support and complicates deployment.

Separately, reading and writing could be split into two components. go-pn532's `polling.Session` already pauses polling for writes, so one component can own both.

## Decision

Implement `rdk:component:sensor` as one component, model `avery:nfc:pn532`. `Readings()` returns cached tag state; writes, diagnostics, and blocking scans go through `DoCommand`.

## Consequences

Tag data flows into Viam data capture with no client code. Operations beyond reading use `DoCommand`'s untyped map, so callers rely on documented action names and fields.
