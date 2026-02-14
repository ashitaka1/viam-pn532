# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Skeleton sensor module implementing `rdk:component:sensor` with model `viam:nfc:pn532`
- Config validation for transport selection (auto/uart/i2c/spi) and device paths
- Stub Readings returning connection status shape for integration testing
- Stub DoCommand with action parsing for write and diagnostic operations
- Device lifecycle: connect, close, and health reporting
- Idempotent Close lifecycle management
- go-pn532 NFC library integration

### Changed
- Switch go-pn532 dependency to fork (ashitaka1/go-pn532) with I2C bus fixes (7-bit address correction, status byte stripping)
- Cross-platform build support for linux/arm64, linux/amd64, and darwin/arm64
