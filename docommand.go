package pn532

import (
	"context"
	"fmt"
	"time"

	pn532lib "github.com/ZaparooProject/go-pn532"
)

func (s *pn532Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	action, ok := cmd["action"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid \"action\" field in command")
	}

	switch action {
	case "await_scan":
		return s.handleAwaitScan(ctx, cmd)
	case "diagnostics":
		return s.handleDiagnostics(ctx)
	default:
		return nil, fmt.Errorf("action %q is not implemented", action)
	}
}

func (s *pn532Sensor) handleAwaitScan(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	waitCtx := ctx
	if timeoutMs, ok := cmd["timeout_ms"].(float64); ok && timeoutMs > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}

	// Priority-check: deliver a buffered result even if the deadline is
	// also ready, since Go's select chooses uniformly at random.
	select {
	case snap := <-s.scanNotify:
		return buildReadingsFromState(&snap), nil
	default:
	}

	select {
	case snap := <-s.scanNotify:
		return buildReadingsFromState(&snap), nil
	case <-waitCtx.Done():
		return nil, fmt.Errorf("await_scan: %w", waitCtx.Err())
	case <-s.cancelCtx.Done():
		return nil, fmt.Errorf("await_scan: sensor closed")
	}
}

func (s *pn532Sensor) handleDiagnostics(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	sess := s.session
	s.mu.RUnlock()

	if sess == nil {
		return nil, fmt.Errorf("diagnostics: device not connected")
	}

	result := map[string]interface{}{}

	err := sess.PauseAndRun(ctx, func(dev *pn532lib.Device) error {
		if dev == nil {
			return fmt.Errorf("device not available")
		}

		commResult, commErr := dev.Diagnose(ctx, pn532lib.DiagnoseCommunicationTest, []byte{0xAB})
		if commErr != nil {
			result["comm_test_ok"] = false
			result["comm_test_error"] = commErr.Error()
		} else {
			result["comm_test_ok"] = commResult.Success
		}

		statusResult, statusErr := dev.GetGeneralStatus(ctx)
		if statusErr != nil {
			result["general_status_error"] = statusErr.Error()
		} else {
			result["field_present"] = statusResult.FieldPresent
			result["last_error"] = int(statusResult.LastError)
			result["targets"] = int(statusResult.Targets)
		}

		fwResult, fwErr := dev.GetFirmwareVersion(ctx)
		if fwErr != nil {
			result["firmware_error"] = fwErr.Error()
		} else {
			result["firmware_version"] = fwResult.Version
			result["support_iso14443a"] = fwResult.SupportIso14443a
			result["support_iso14443b"] = fwResult.SupportIso14443b
			result["support_iso18092"] = fwResult.SupportIso18092
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("diagnostics: %w", err)
	}

	return result, nil
}
