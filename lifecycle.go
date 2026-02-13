package pn532

import (
	"context"
)

func (s *pn532Sensor) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	s.cancelFunc()
	return nil
}
