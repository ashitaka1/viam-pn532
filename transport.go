package pn532

import (
	"context"
	"fmt"
	"time"

	pn532 "github.com/ZaparooProject/go-pn532"
	transportI2C "github.com/ZaparooProject/go-pn532/transport/i2c"
	transportSPI "github.com/ZaparooProject/go-pn532/transport/spi"
	transportUART "github.com/ZaparooProject/go-pn532/transport/uart"
	"go.viam.com/rdk/logging"
)

func transportFactory(transportType string) pn532.TransportFactory {
	return func(path string) (pn532.Transport, error) {
		switch transportType {
		case "uart":
			return transportUART.New(path)
		case "i2c":
			return transportI2C.New(path)
		case "spi":
			return transportSPI.New(path)
		default:
			return nil, fmt.Errorf("unsupported transport type %q", transportType)
		}
	}
}

func connectDevice(ctx context.Context, cfg *Config, logger logging.Logger) (*pn532.Device, error) {
	timeout := time.Duration(cfg.ConnectTimeoutSec) * time.Second

	logger.Infof("Connecting to PN532 via %s at %s (timeout %s)", cfg.Transport, cfg.DevicePath, timeout)
	return pn532.ConnectDevice(ctx, cfg.DevicePath,
		pn532.WithConnectTimeout(timeout),
		pn532.WithTransportFactory(transportFactory(cfg.Transport)),
	)
}
