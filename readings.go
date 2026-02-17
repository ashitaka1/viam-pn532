package pn532

import (
	"context"
)

func (s *pn532Sensor) Readings(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return buildReadingsFromState(&s.state), nil
}
