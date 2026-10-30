# Contributing

## Setup

Go 1.24.

```bash
make setup     # go mod tidy
```

The module depends on `github.com/ashitaka1/go-pn532`, a fork of go-pn532 carrying I2C fixes. Update it with `go get github.com/ashitaka1/go-pn532@<version>` and `go mod tidy`.

## Build

```bash
make build                                              # current platform → bin/pn532
VIAM_BUILD_OS=linux VIAM_BUILD_ARCH=arm64 make build    # Raspberry Pi
VIAM_BUILD_OS=linux VIAM_BUILD_ARCH=amd64 make build
make module.tar.gz                                      # stripped binary + meta.json
```

## Test

```bash
make test                               # go test -v -race ./...
go test -v -race -run TestName ./...
```

Unit tests cover config validation, readings construction, and DoCommand dispatch. Hardware behavior is tested by hand on a Raspberry Pi with a PN532 board.

## Release

Push a tag (`v0.2.2`, or `v0.2.2-rc1`). `.github/workflows/deploy.yml` builds `linux/amd64`, `linux/arm64`, and `darwin/arm64` with `viamrobotics/build-action` and uploads to the Viam registry as `avery:nfc-pn532`, using the repo secrets `viam_key_id` and `viam_key_value`.
