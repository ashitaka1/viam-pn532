package pn532

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	pn532lib "github.com/ZaparooProject/go-pn532"
	sensor "go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
)

func TestValidateTransportValues(t *testing.T) {
	valid := []string{"uart", "i2c", "spi"}
	for _, transport := range valid {
		cfg := &Config{Transport: transport, DevicePath: "/dev/test"}
		_, _, err := cfg.Validate("test")
		if err != nil {
			t.Errorf("Validate(%q) returned error: %v", transport, err)
		}
	}

	invalid := []string{"", "auto", "usb", "bluetooth", "serial", "UART"}
	for _, transport := range invalid {
		cfg := &Config{Transport: transport, DevicePath: "/dev/test"}
		_, _, err := cfg.Validate("test")
		if err == nil {
			t.Errorf("Validate(%q) should have returned error", transport)
		}
	}
}

func TestValidateDevicePathRequirement(t *testing.T) {
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

	cfg := &Config{}
	_, _, err := cfg.Validate("test")
	if err == nil {
		t.Error("empty transport should fail validation")
	}
}

func newTestSensor(t *testing.T, cfg *Config) *pn532Sensor {
	t.Helper()
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	t.Cleanup(cancelFunc)

	resolved := applyConfigDefaults(cfg)

	return &pn532Sensor{
		name:       sensor.Named("test"),
		logger:     logging.NewTestLogger(t),
		cfg:        resolved,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
}

// newTestSensorWithDevice creates a sensor with a mock device and healthy state.
func newTestSensorWithDevice(t *testing.T, cfg *Config) (*pn532Sensor, *pn532lib.MockTransport) {
	t.Helper()
	mock := pn532lib.NewMockTransport()
	device, err := pn532lib.New(mock)
	if err != nil {
		t.Fatalf("failed to create mock device: %v", err)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	t.Cleanup(cancelFunc)

	resolved := applyConfigDefaults(cfg)

	s := &pn532Sensor{
		name:       sensor.Named("test"),
		logger:     logging.NewTestLogger(t),
		cfg:        resolved,
		device:     device,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		state:      tagState{deviceHealthy: true},
	}
	return s, mock
}

func TestValidateDefaults(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})
	if s.cfg.PollIntervalMs != defaultPollIntervalMs {
		t.Errorf("PollIntervalMs = %d, want %d", s.cfg.PollIntervalMs, defaultPollIntervalMs)
	}
	if s.cfg.CardRemovalTimeoutMs != defaultCardRemovalTimeoutMs {
		t.Errorf("CardRemovalTimeoutMs = %d, want %d", s.cfg.CardRemovalTimeoutMs, defaultCardRemovalTimeoutMs)
	}
	if s.cfg.ConnectTimeoutSec != defaultConnectTimeoutSec {
		t.Errorf("ConnectTimeoutSec = %d, want %d", s.cfg.ConnectTimeoutSec, defaultConnectTimeoutSec)
	}
	if s.cfg.ReadNDEF == nil || !*s.cfg.ReadNDEF {
		t.Error("ReadNDEF should default to true")
	}

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

func TestReadingsDeviceUnhealthy(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})
	s.state.deviceHealthy = false

	readings, err := s.Readings(context.Background(), nil)
	if err != nil {
		t.Fatalf("Readings returned error: %v", err)
	}

	expected := map[string]interface{}{
		"status":         "connected",
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

func TestReadingsConnected(t *testing.T) {
	s, _ := newTestSensorWithDevice(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})

	readings, err := s.Readings(context.Background(), nil)
	if err != nil {
		t.Fatalf("Readings returned error: %v", err)
	}

	expected := map[string]interface{}{
		"status":         "connected",
		"tag_present":    false,
		"device_healthy": true,
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
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})

	_, err := s.DoCommand(context.Background(), map[string]interface{}{"action": "bogus"})
	if err == nil {
		t.Fatal("DoCommand with unknown action should return error")
	}

	_, err = s.DoCommand(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("DoCommand with missing action should return error")
	}
}

func TestCloseIdempotent(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})

	for i := range 3 {
		err := s.Close(context.Background())
		if err != nil {
			t.Errorf("Close call %d returned error: %v", i+1, err)
		}
	}
}

// --- Milestone 3 tests ---

func TestBuildReadingsNoTag(t *testing.T) {
	state := &tagState{deviceHealthy: true}
	readings := buildReadingsFromState(state)

	if readings["status"] != "connected" {
		t.Errorf("status = %v, want connected", readings["status"])
	}
	if readings["device_healthy"] != true {
		t.Errorf("device_healthy = %v, want true", readings["device_healthy"])
	}
	if readings["tag_present"] != false {
		t.Errorf("tag_present = %v, want false", readings["tag_present"])
	}
	// No tag detail fields when tag_present is false
	if _, exists := readings["uid"]; exists {
		t.Error("uid should not be present when no tag")
	}
}

func TestBuildReadingsTagPresent(t *testing.T) {
	state := &tagState{
		deviceHealthy:   true,
		tagPresent:      true,
		uid:             "04abcdef123456",
		tagType:         "NTAG",
		manufacturer:    "NXP",
		isGenuine:       true,
		ntagVariant:     "NTAG215",
		mifareVariant:   "",
		userMemoryBytes: 504,
		ndefText:        "hello",
		ndefRecordCount: 1,
	}
	readings := buildReadingsFromState(state)

	checks := map[string]interface{}{
		"status":           "connected",
		"device_healthy":   true,
		"tag_present":      true,
		"uid":              "04abcdef123456",
		"tag_type":         "NTAG",
		"manufacturer":     "NXP",
		"is_genuine":       true,
		"ntag_variant":     "NTAG215",
		"mifare_variant":   "",
		"user_memory_bytes": 504,
		"ndef_text":        "hello",
		"ndef_record_count": 1,
	}
	for key, want := range checks {
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

func TestBuildReadingsDeviceUnhealthyState(t *testing.T) {
	state := &tagState{deviceHealthy: false}
	readings := buildReadingsFromState(state)

	if readings["device_healthy"] != false {
		t.Errorf("device_healthy = %v, want false", readings["device_healthy"])
	}
	if readings["tag_present"] != false {
		t.Errorf("tag_present = %v, want false", readings["tag_present"])
	}
}

// setupNTAG215Mock configures mock responses for an NTAG215 tag detection sequence.
// Returns a DetectedTag matching the configured UID.
func setupNTAG215Mock(mock *pn532lib.MockTransport) *pn532lib.DetectedTag {
	uid := []byte{0x04, 0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}

	// CC read (page 3) and page 4 read both use InDataExchange (0x40).
	// Response: [0x41=response code, 0x00=success, 16 bytes data]
	ccResp := make([]byte, 18) // 2 header + 16 data
	ccResp[0] = 0x41
	ccResp[1] = 0x00
	ccResp[2] = 0xE1 // NDEF magic
	ccResp[3] = 0x10 // Version
	ccResp[4] = 0x3E // Size field for NTAG215
	ccResp[5] = 0x00 // Access

	page4Resp := make([]byte, 18)
	page4Resp[0] = 0x41
	page4Resp[1] = 0x00

	mock.QueueResponse(0x40, ccResp)
	mock.QueueResponse(0x40, page4Resp)

	return &pn532lib.DetectedTag{
		DetectedAt: time.Now(),
		UID:        "0412345678",
		Type:       pn532lib.TagTypeNTAG,
		UIDBytes:   uid,
		SAK:        0x00,
	}
}

func TestOnCardDetectedPopulatesState(t *testing.T) {
	readNDEF := false // skip NDEF to simplify mock setup
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
		ReadNDEF:   &readNDEF,
	})

	tag := setupNTAG215Mock(mock)

	err := s.onCardDetected(context.Background(), tag)
	if err != nil {
		t.Fatalf("onCardDetected returned error: %v", err)
	}

	if !s.state.tagPresent {
		t.Error("tagPresent should be true after detection")
	}
	if s.state.uid != tag.UID {
		t.Errorf("uid = %q, want %q", s.state.uid, tag.UID)
	}
	if s.state.tagType != string(pn532lib.TagTypeNTAG) {
		t.Errorf("tagType = %q, want %q", s.state.tagType, string(pn532lib.TagTypeNTAG))
	}
	if s.state.manufacturer != string(pn532lib.ManufacturerNXP) {
		t.Errorf("manufacturer = %q, want %q", s.state.manufacturer, string(pn532lib.ManufacturerNXP))
	}
	if !s.state.isGenuine {
		t.Error("isGenuine should be true for NXP UID")
	}
	if s.state.ntagVariant != "NTAG215" {
		t.Errorf("ntagVariant = %q, want NTAG215", s.state.ntagVariant)
	}
	if s.state.userMemoryBytes != 504 {
		t.Errorf("userMemoryBytes = %d, want 504", s.state.userMemoryBytes)
	}
}

func TestOnCardDetectedNDEFError(t *testing.T) {
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
	})

	tag := setupNTAG215Mock(mock)
	// NDEF read will use InDataExchange (0x40) — set error for subsequent calls
	mock.SetError(0x40, errors.New("NDEF read failure"))

	err := s.onCardDetected(context.Background(), tag)
	if err != nil {
		t.Fatalf("onCardDetected returned error: %v", err)
	}

	// Basic tag fields should still be populated from DetectedTag
	if !s.state.tagPresent {
		t.Error("tagPresent should be true even when NDEF fails")
	}
	if s.state.uid != tag.UID {
		t.Errorf("uid = %q, want %q", s.state.uid, tag.UID)
	}
	// NDEF fields should be zero
	if s.state.ndefText != "" {
		t.Errorf("ndefText should be empty, got %q", s.state.ndefText)
	}
	if s.state.ndefRecordCount != 0 {
		t.Errorf("ndefRecordCount should be 0, got %d", s.state.ndefRecordCount)
	}
}

func TestOnCardRemovedClearsState(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})
	s.state = tagState{
		deviceHealthy:   true,
		tagPresent:      true,
		uid:             "04abcdef",
		tagType:         "NTAG",
		manufacturer:    "NXP",
		isGenuine:       true,
		ntagVariant:     "NTAG215",
		userMemoryBytes: 504,
		ndefText:        "hello",
		ndefRecordCount: 1,
	}

	s.onCardRemoved()

	if s.state.tagPresent {
		t.Error("tagPresent should be false after removal")
	}
	if s.state.uid != "" {
		t.Errorf("uid should be empty, got %q", s.state.uid)
	}
	if s.state.ndefText != "" {
		t.Errorf("ndefText should be empty, got %q", s.state.ndefText)
	}
	// deviceHealthy should remain true
	if !s.state.deviceHealthy {
		t.Error("deviceHealthy should remain true after card removal")
	}
}

func TestOnDeviceDisconnectedSetsUnhealthy(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})
	s.state = tagState{
		deviceHealthy: true,
		tagPresent:    true,
		uid:           "04abcdef",
	}

	s.onDeviceDisconnected(errors.New("transport gone"))

	if s.state.deviceHealthy {
		t.Error("deviceHealthy should be false after disconnect")
	}
	if s.state.tagPresent {
		t.Error("tagPresent should be false after disconnect")
	}
	if s.state.uid != "" {
		t.Errorf("uid should be empty after disconnect, got %q", s.state.uid)
	}
}

func TestTagReplacement(t *testing.T) {
	readNDEF := false
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
		ReadNDEF:   &readNDEF,
	})

	// Detect tag A
	tagA := setupNTAG215Mock(mock)
	tagA.UID = "04aaaaaa"
	if err := s.onCardDetected(context.Background(), tagA); err != nil {
		t.Fatalf("detect tag A: %v", err)
	}
	if s.state.uid != "04aaaaaa" {
		t.Fatalf("uid should be tag A, got %q", s.state.uid)
	}

	// Remove tag A
	s.onCardRemoved()
	if s.state.tagPresent {
		t.Fatal("tagPresent should be false after removal")
	}

	// Detect tag B — set up fresh mock responses
	mock.Reset()
	tagB := setupNTAG215Mock(mock)
	tagB.UID = "04bbbbbb"
	if err := s.onCardDetected(context.Background(), tagB); err != nil {
		t.Fatalf("detect tag B: %v", err)
	}

	if s.state.uid != "04bbbbbb" {
		t.Errorf("uid should be tag B, got %q", s.state.uid)
	}
	if !s.state.tagPresent {
		t.Error("tagPresent should be true for tag B")
	}
}

func TestReadingsConcurrentWithCallbacks(t *testing.T) {
	readNDEF := false
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
		ReadNDEF:   &readNDEF,
	})

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Goroutine: continuous Readings
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			readings, err := s.Readings(ctx, nil)
			if err != nil {
				t.Errorf("Readings error: %v", err)
				return
			}
			// Verify consistency: if tag_present is true, uid must be non-empty
			if tagPresent, ok := readings["tag_present"].(bool); ok && tagPresent {
				if uid, ok := readings["uid"].(string); !ok || uid == "" {
					t.Error("tag_present=true but uid is empty — inconsistent state")
					return
				}
			}
		}
	}()

	// Goroutine: rapid detect/remove cycles
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			mock.Reset()
			tag := setupNTAG215Mock(mock)
			_ = s.onCardDetected(ctx, tag)
			s.onCardRemoved()
		}
	}()

	wg.Wait()
}

func TestCloseStopsSession(t *testing.T) {
	s, _ := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
	})

	// Simulate an active session goroutine
	s.sessionWg.Add(1)
	go func() {
		defer s.sessionWg.Done()
		<-s.cancelCtx.Done()
	}()

	// Close should complete without deadlock within a reasonable time
	done := make(chan struct{})
	go func() {
		err := s.Close(context.Background())
		if err != nil {
			t.Errorf("Close returned error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Close deadlocked — did not complete within 2 seconds")
	}
}
