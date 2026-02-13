package pn532

import (
	"context"
	"sync"

	sensor "go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("viam", "nfc", "pn532")

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newPn532Sensor,
		},
	)
}

type pn532Sensor struct {
	resource.AlwaysRebuild

	mu     sync.RWMutex
	name   resource.Name
	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()
	closed     bool
}

func newPn532Sensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	return NewPn532(ctx, deps, rawConf.ResourceName(), conf, logger)
}

const (
	defaultPollIntervalMs      = 250
	defaultCardRemovalTimeoutMs = 600
	defaultConnectTimeoutSec   = 10
)

func NewPn532(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (sensor.Sensor, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	cfg := *conf
	if cfg.Transport == "" {
		cfg.Transport = "auto"
	}
	if cfg.PollIntervalMs <= 0 {
		cfg.PollIntervalMs = defaultPollIntervalMs
	}
	if cfg.CardRemovalTimeoutMs <= 0 {
		cfg.CardRemovalTimeoutMs = defaultCardRemovalTimeoutMs
	}
	if cfg.ConnectTimeoutSec <= 0 {
		cfg.ConnectTimeoutSec = defaultConnectTimeoutSec
	}
	if cfg.ReadNDEF == nil {
		readNDEF := true
		cfg.ReadNDEF = &readNDEF
	}

	s := &pn532Sensor{
		name:       name,
		logger:     logger,
		cfg:        &cfg,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return s, nil
}

func (s *pn532Sensor) Name() resource.Name {
	return s.name
}
