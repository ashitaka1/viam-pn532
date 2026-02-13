package pn532

import (
	"context"
	"testing"

	sensor "go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestValidateTransportValues(t *testing.T) {
	valid := []string{"", "auto", "uart", "i2c", "spi"}
	for _, transport := range valid {
		cfg := &Config{Transport: transport, DevicePath: "/dev/test"}
		_, _, err := cfg.Validate("test")
		if err != nil {
			t.Errorf("Validate(%q) returned error: %v", transport, err)
		}
	}

	invalid := []string{"usb", "bluetooth", "serial", "UART"}
	for _, transport := range invalid {
		cfg := &Config{Transport: transport, DevicePath: "/dev/test"}
		_, _, err := cfg.Validate("test")
		if err == nil {
			t.Errorf("Validate(%q) should have returned error", transport)
		}
	}
}

func TestValidateDevicePathRequirement(t *testing.T) {
	// Explicit transports require device_path
	for _, transport := range []string{"uart", "i2c", "spi"} {
		cfg := &Config{Transport: transport}
		_, _, err := cfg.Validate("test")
		if err == nil {
			t.Errorf("transport %q with no device_path should fail validation", transport)
		}

		cfg.DevicePath = "/dev/ttyAMA0"
		_, _, err = cfg.Validate("test")
		if err != nil {
			t.Errorf("transport %q with device_path should pass: %v", transport, err)
		}
	}

	// Auto and empty don't require device_path
	for _, transport := range []string{"", "auto"} {
		cfg := &Config{Transport: transport}
		_, _, err := cfg.Validate("test")
		if err != nil {
			t.Errorf("transport %q without device_path should pass: %v", transport, err)
		}
	}
}

func newTestSensor(t *testing.T, cfg *Config) *pn532Sensor {
	t.Helper()
	logger := logging.NewTestLogger(t)
	s, err := NewPn532(context.Background(), resource.Dependencies{}, sensor.Named("test"), cfg, logger)
	if err != nil {
		t.Fatalf("NewPn532 failed: %v", err)
	}
	return s.(*pn532Sensor)
}

func TestValidateDefaults(t *testing.T) {
	// Zero-value fields get populated with defaults
	s := newTestSensor(t, &Config{})
	if s.cfg.PollIntervalMs == 0 {
		t.Error("PollIntervalMs should be set to a default, got 0")
	}
	if s.cfg.CardRemovalTimeoutMs == 0 {
		t.Error("CardRemovalTimeoutMs should be set to a default, got 0")
	}
	if s.cfg.ConnectTimeoutSec == 0 {
		t.Error("ConnectTimeoutSec should be set to a default, got 0")
	}
	if s.cfg.ReadNDEF == nil || !*s.cfg.ReadNDEF {
		t.Error("ReadNDEF should default to true")
	}
	if s.cfg.Transport != "auto" {
		t.Errorf("Transport should default to \"auto\", got %q", s.cfg.Transport)
	}

	// Explicit values are preserved
	readNDEF := false
	s2 := newTestSensor(t, &Config{
		PollIntervalMs:       500,
		CardRemovalTimeoutMs: 1000,
		ConnectTimeoutSec:    30,
		ReadNDEF:             &readNDEF,
		Transport:            "uart",
		DevicePath:           "/dev/ttyAMA0",
	})
	if s2.cfg.PollIntervalMs != 500 {
		t.Errorf("PollIntervalMs should preserve 500, got %d", s2.cfg.PollIntervalMs)
	}
	if s2.cfg.CardRemovalTimeoutMs != 1000 {
		t.Errorf("CardRemovalTimeoutMs should preserve 1000, got %d", s2.cfg.CardRemovalTimeoutMs)
	}
	if s2.cfg.ConnectTimeoutSec != 30 {
		t.Errorf("ConnectTimeoutSec should preserve 30, got %d", s2.cfg.ConnectTimeoutSec)
	}
	if *s2.cfg.ReadNDEF != false {
		t.Error("ReadNDEF should preserve explicit false")
	}
}

func TestReadingsReturnsStatus(t *testing.T) {
	s := newTestSensor(t, &Config{})
	readings, err := s.Readings(context.Background(), nil)
	if err != nil {
		t.Fatalf("Readings returned error: %v", err)
	}

	expected := map[string]interface{}{
		"status":         "not_connected",
		"tag_present":    false,
		"device_healthy": false,
	}
	for key, want := range expected {
		got, ok := readings[key]
		if !ok {
			t.Errorf("missing key %q in readings", key)
			continue
		}
		if got != want {
			t.Errorf("readings[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestDoCommandUnknownAction(t *testing.T) {
	s := newTestSensor(t, &Config{})

	_, err := s.DoCommand(context.Background(), map[string]interface{}{"action": "bogus"})
	if err == nil {
		t.Fatal("DoCommand with unknown action should return error")
	}

	_, err = s.DoCommand(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("DoCommand with missing action should return error")
	}
}

func TestConstructorDoesNotMutateInput(t *testing.T) {
	cfg := &Config{Transport: "uart", DevicePath: "/dev/ttyAMA0"}
	newTestSensor(t, cfg)
	if cfg.PollIntervalMs != 0 {
		t.Error("constructor should not mutate the input config")
	}
}

func TestCloseIdempotent(t *testing.T) {
	s := newTestSensor(t, &Config{})

	for i := range 3 {
		err := s.Close(context.Background())
		if err != nil {
			t.Errorf("Close call %d returned error: %v", i+1, err)
		}
	}
}
