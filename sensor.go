package pn532

import (
	"context"
	"sync"
	"time"

	pn532 "github.com/ZaparooProject/go-pn532"
	"github.com/ZaparooProject/go-pn532/polling"
	"github.com/ZaparooProject/go-pn532/tagops"
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

	mu      sync.RWMutex
	name    resource.Name
	logger  logging.Logger
	cfg     *Config
	device  *pn532.Device
	session *polling.Session
	state   tagState

	cancelCtx  context.Context
	cancelFunc func()
	sessionWg  sync.WaitGroup
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
	defaultPollIntervalMs       = 250
	defaultCardRemovalTimeoutMs = 600
	defaultConnectTimeoutSec    = 10
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

func NewPn532(ctx context.Context, _ resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (sensor.Sensor, error) {
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
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	s.startSession(device)
	return s, nil
}

// startSession wires up the polling session and goroutine for a connected device.
func (s *pn532Sensor) startSession(device *pn532.Device) {
	s.mu.Lock()
	s.device = device
	s.state.deviceHealthy = true
	s.mu.Unlock()

	sess := polling.NewSession(device, &polling.Config{
		PollInterval:       time.Duration(s.cfg.PollIntervalMs) * time.Millisecond,
		CardRemovalTimeout: time.Duration(s.cfg.CardRemovalTimeoutMs) * time.Millisecond,
	})
	sess.SetOnCardDetected(s.onCardDetected)
	sess.SetOnCardRemoved(s.onCardRemoved)
	sess.SetOnDeviceDisconnected(s.onDeviceDisconnected)

	s.mu.Lock()
	s.session = sess
	s.mu.Unlock()

	s.sessionWg.Add(1)
	go func() {
		defer s.sessionWg.Done()
		if err := sess.Start(s.cancelCtx); err != nil && s.cancelCtx.Err() == nil {
			s.logger.Errorw("polling session exited with error", "error", err)
		}
	}()
}

func (s *pn532Sensor) onCardDetected(ctx context.Context, detectedTag *pn532.DetectedTag) error {
	// I/O phase — no lock held. tagops calls go through the Device which
	// the polling session has exclusive access to during this callback.
	var ndefText string
	var ndefRecordCount int
	var ntagVariant, mifareVariant string
	var userMemoryBytes int

	ops := tagops.New(s.device)
	if err := ops.InitFromDetectedTag(ctx, detectedTag); err != nil {
		s.logger.Warnw("failed to initialize tag operations", "uid", detectedTag.UID, "error", err)
	} else {
		if info, err := ops.GetTagInfo(); err == nil {
			ntagVariant = info.NTAGType
			mifareVariant = info.MIFAREType
			userMemoryBytes = info.UserMemory
		}

		if s.cfg.ReadNDEF != nil && *s.cfg.ReadNDEF {
			if ndefMsg, err := ops.ReadNDEF(ctx); err != nil {
				s.logger.Warnw("failed to read NDEF", "uid", detectedTag.UID, "error", err)
			} else if ndefMsg != nil {
				ndefRecordCount = len(ndefMsg.Records)
				for _, r := range ndefMsg.Records {
					if r.Type == pn532.NDEFTypeText && r.Text != "" {
						ndefText = r.Text
						break
					}
				}
			}
		}
	}

	// Cache phase — write results under lock.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}

	s.state.tagPresent = true
	s.state.uid = detectedTag.UID
	s.state.tagType = string(detectedTag.Type)
	s.state.manufacturer = string(detectedTag.Manufacturer())
	s.state.isGenuine = detectedTag.IsGenuine()
	s.state.ndefText = ndefText
	s.state.ndefRecordCount = ndefRecordCount
	s.state.ntagVariant = ntagVariant
	s.state.mifareVariant = mifareVariant
	s.state.userMemoryBytes = userMemoryBytes

	return nil
}

func (s *pn532Sensor) onCardRemoved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}

	s.state.tagPresent = false
	s.state.uid = ""
	s.state.tagType = ""
	s.state.manufacturer = ""
	s.state.isGenuine = false
	s.state.ndefText = ""
	s.state.ndefRecordCount = 0
	s.state.ntagVariant = ""
	s.state.mifareVariant = ""
	s.state.userMemoryBytes = 0
}

func (s *pn532Sensor) onDeviceDisconnected(err error) {
	s.logger.Errorw("PN532 device disconnected", "error", err)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.deviceHealthy = false
	s.state.tagPresent = false
	s.state.uid = ""
	s.state.tagType = ""
	s.state.manufacturer = ""
	s.state.isGenuine = false
	s.state.ndefText = ""
	s.state.ndefRecordCount = 0
	s.state.ntagVariant = ""
	s.state.mifareVariant = ""
	s.state.userMemoryBytes = 0
}

func (s *pn532Sensor) Name() resource.Name {
	return s.name
}
