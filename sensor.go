package pn532

import (
	"context"
	"sync"

	pn532 "github.com/ZaparooProject/go-pn532"
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
	device *pn532.Device

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

func applyConfigDefaults(conf *Config) *Config {
	cfg := *conf
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
	return &cfg
}

func NewPn532(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (sensor.Sensor, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	cfg := applyConfigDefaults(conf)

	device, err := connectDevice(ctx, cfg, logger)
	if err != nil {
		cancelFunc()
		return nil, err
	}

	s := &pn532Sensor{
		name:       name,
		logger:     logger,
		cfg:        cfg,
		device:     device,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return s, nil
}

func (s *pn532Sensor) Name() resource.Name {
	return s.name
}
