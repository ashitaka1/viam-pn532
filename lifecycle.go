package pn532

import (
	"context"
)

func (s *pn532Sensor) Close(_ context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Cancel context first — signals the polling loop in Start() to exit.
	s.cancelFunc()

	// Close the device (I2C transport) before waiting for the goroutine.
	// Session.Start() may be blocked on a ~5s InListPassiveTarget I2C
	// operation that context cancellation cannot interrupt. Closing the
	// transport's file descriptor causes the in-flight I2C syscall to
	// return immediately with an error, unblocking the polling loop.
	if s.device != nil {
		if err := s.device.Close(); err != nil {
			s.logger.Errorw("error closing PN532 device", "error", err)
		}
	}

	// Now the goroutine can exit promptly.
	s.sessionWg.Wait()

	if s.session != nil {
		if err := s.session.Close(); err != nil {
			s.logger.Errorw("error closing polling session", "error", err)
		}
	}

	return nil
}
