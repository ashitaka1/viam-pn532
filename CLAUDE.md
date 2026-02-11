# CLAUDE.md

This file provides project-specific guidance to Claude Code (claude.ai/code) for this repository.

## Current Status

Project phase: Planning

Architecture designed, not yet implemented. See `project_spec.md` for the full plan.

## About This Project

Viam module wrapping the [go-pn532](https://github.com/ZaparooProject/go-pn532) NFC reader library as a `rdk:component:sensor`. Deploys to SBCs (Raspberry Pi) running viam-server.

See @project_spec.md for technical architecture, milestones, and implementation decisions.

## Project-Specific Conventions

### Testing

```bash
make test          # Run all tests with race detection
```

Run a single test:
```bash
go test -v -race -run TestName ./pn532sensor/...
```

### Build / Run

```bash
make build         # Build for current platform
make build-arm64   # Cross-compile for linux/arm64 (RPi)
make build-amd64   # Cross-compile for linux/amd64
```
