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

	if s.device != nil {
		if err := s.device.Close(); err != nil {
			s.logger.Errorw("error closing PN532 device", "error", err)
		}
	}

	s.cancelFunc()
	return nil
}
