package pn532

import (
	"context"
)

func (s *pn532Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"status":         "not_connected",
		"tag_present":    false,
		"device_healthy": false,
	}, nil
}
