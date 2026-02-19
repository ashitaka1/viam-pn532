package pn532

import (
	"context"
	"testing"
	"time"
)

func TestAwaitScanReturnsCachedSnapshot(t *testing.T) {
	readNDEF := false
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
		ReadNDEF:   &readNDEF,
	})

	tag := setupNTAG215Mock(mock)

	// Deliver a detection in the background after a short delay.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = s.onCardDetected(context.Background(), tag)
	}()

	result, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action":     "await_scan",
		"timeout_ms": float64(2000),
	})
	if err != nil {
		t.Fatalf("await_scan returned error: %v", err)
	}

	if result["tag_present"] != true {
		t.Error("tag_present should be true in await_scan result")
	}
	if result["uid"] != tag.UID {
		t.Errorf("uid = %q, want %q", result["uid"], tag.UID)
	}
}

func TestAwaitScanSnapshotIsolatedFromRemoval(t *testing.T) {
	readNDEF := false
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
		ReadNDEF:   &readNDEF,
	})

	tag := setupNTAG215Mock(mock)
	// Detect then immediately remove — the snapshot should still carry tag data.
	_ = s.onCardDetected(context.Background(), tag)
	s.onCardRemoved()

	result, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action":     "await_scan",
		"timeout_ms": float64(1000),
	})
	if err != nil {
		t.Fatalf("await_scan returned error: %v", err)
	}

	if result["tag_present"] != true {
		t.Error("snapshot should show tag_present=true even after removal")
	}
	if result["uid"] != tag.UID {
		t.Errorf("snapshot uid = %q, want %q", result["uid"], tag.UID)
	}
}

func TestAwaitScanTimeoutMs(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})

	start := time.Now()
	_, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action":     "await_scan",
		"timeout_ms": float64(100),
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("await_scan should return error on timeout")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took %v, expected ~100ms", elapsed)
	}
}

func TestAwaitScanContextCancellation(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := s.DoCommand(ctx, map[string]interface{}{
		"action": "await_scan",
	})
	if err == nil {
		t.Fatal("await_scan should return error on context cancellation")
	}
}

func TestAwaitScanSensorClosed(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})

	go func() {
		time.Sleep(50 * time.Millisecond)
		s.cancelFunc()
	}()

	_, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action":     "await_scan",
		"timeout_ms": float64(5000),
	})
	if err == nil {
		t.Fatal("await_scan should return error when sensor closes")
	}
}

func TestOnCardDetectedNotifyReplaceStale(t *testing.T) {
	readNDEF := false
	s, mock := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
		ReadNDEF:   &readNDEF,
	})

	// First detection — not consumed.
	tagA := setupNTAG215Mock(mock)
	tagA.UID = "04aaaaaa"
	_ = s.onCardDetected(context.Background(), tagA)

	// Second detection — should replace stale entry.
	mock.Reset()
	tagB := setupNTAG215Mock(mock)
	tagB.UID = "04bbbbbb"
	_ = s.onCardDetected(context.Background(), tagB)

	result, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action":     "await_scan",
		"timeout_ms": float64(100),
	})
	if err != nil {
		t.Fatalf("await_scan returned error: %v", err)
	}
	if result["uid"] != "04bbbbbb" {
		t.Errorf("uid = %q, want 04bbbbbb (freshest detection)", result["uid"])
	}
}

func TestDiagnosticsDisconnected(t *testing.T) {
	s := newTestSensor(t, &Config{Transport: "i2c", DevicePath: "/dev/i2c-1"})
	// session is nil — no device connected.

	_, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action": "diagnostics",
	})
	if err == nil {
		t.Fatal("diagnostics should return error when session is nil")
	}
}

func TestDiagnosticsPartialResults(t *testing.T) {
	s, _ := newTestSensorWithDevice(t, &Config{
		Transport:  "i2c",
		DevicePath: "/dev/i2c-1",
	})

	// Start a real polling session so PauseAndRun exercises the actual
	// pause/ack/resume path. The mock has no queued responses for poll
	// or diagnostic commands, so the polling loop gets errors (handled
	// gracefully) and the diagnostic device calls populate error keys
	// in the result map.
	s.startSession(s.device)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	// Give the polling loop time to start and enter its poll cycle.
	time.Sleep(50 * time.Millisecond)

	result, err := s.DoCommand(context.Background(), map[string]interface{}{
		"action": "diagnostics",
	})
	if err != nil {
		t.Fatalf("diagnostics returned outer error: %v", err)
	}

	if result == nil {
		t.Fatal("diagnostics should return a result map")
	}
	// MockTransport won't have valid diagnostic responses queued, so
	// we expect error keys for the individual tests. The important thing
	// is that the outer call succeeds and returns a populated map.
	hasAnyKey := false
	for _, key := range []string{
		"comm_test_ok", "comm_test_error",
		"firmware_version", "firmware_error",
		"field_present", "general_status_error",
	} {
		if _, ok := result[key]; ok {
			hasAnyKey = true
			break
		}
	}
	if !hasAnyKey {
		t.Error("diagnostics result should contain at least one diagnostic key")
	}
}
