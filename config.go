package pn532

import (
	"fmt"
	"slices"
)

var validTransports = []string{"uart", "i2c", "spi"}

// Config holds the configuration for the PN532 sensor component.
type Config struct {
	Transport           string `json:"transport"`
	DevicePath          string `json:"device_path"`
	PollIntervalMs      int    `json:"poll_interval_ms,omitempty"`
	CardRemovalTimeoutMs int   `json:"card_removal_timeout_ms,omitempty"`
	ReadNDEF            *bool  `json:"read_ndef,omitempty"`
	Debug               bool   `json:"debug,omitempty"`
	ConnectTimeoutSec   int    `json:"connect_timeout_sec,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.Transport == "" {
		return nil, nil, fmt.Errorf("transport is required, must be one of %v", validTransports)
	}
	if !slices.Contains(validTransports, cfg.Transport) {
		return nil, nil, fmt.Errorf("invalid transport %q, must be one of %v", cfg.Transport, validTransports)
	}
	if cfg.DevicePath == "" {
		return nil, nil, fmt.Errorf("device_path is required when transport is %q", cfg.Transport)
	}

	return nil, nil, nil
}
